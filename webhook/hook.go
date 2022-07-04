/*
 * Authors:
 * Simon Gerber <simon.gerber@vshn.ch>
 *
 * License:
 * Copyright (c) 2019, VSHN AG, <info@vshn.ch>
 * Licensed under "BSD 3-Clause". See LICENSE file.
 */

package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/prometheus/alertmanager/template"
	"github.com/vshn/go-icinga2-client/icinga2"
	"github.com/vshn/signalilo/config"
)

// responseJSON is used to marshal responses to incoming webhook requests to
// JSON
type responseJSON struct {
	Status  int
	Message string
}

// asJSON formats a response to a webhook request using type responseJSON
func asJSON(w http.ResponseWriter, status int, message string) {
	data := responseJSON{
		Status:  status,
		Message: message,
	}
	bytes, _ := json.Marshal(data)
	json := string(bytes[:])

	w.WriteHeader(status)
	fmt.Fprint(w, json)
}

func checkBearerToken(r *http.Request, c config.Configuration) error {
	tokenHeader := r.Header.Get("Authorization")
	tokenQuery := r.URL.Query().Get("token")
	var token string
	if tokenHeader != "" {
		headerElems := strings.Split(tokenHeader, " ")
		if len(headerElems) != 2 || (len(headerElems) > 0 && headerElems[0] != "Bearer") {
			return fmt.Errorf("Malformed authorization header")
		}
		token = headerElems[1]
	} else if tokenQuery != "" {
		token = tokenQuery
	} else {
		return fmt.Errorf("Request dos not contain an authorization token")
	}
	if token != c.GetConfig().AlertManagerConfig.BearerToken {
		return fmt.Errorf("Invalid bearer token")
	}
	return nil
}

// Webhook handles incoming webhook HTTP requests
func Webhook(w http.ResponseWriter, r *http.Request, c config.Configuration) {
	defer r.Body.Close()

	l := c.GetLogger()
	if l == nil {
		panic("logger is nil")
	}

	if err := checkBearerToken(r, c); err != nil {
		l.Errorf("Checking webhook authentication: %v", err)
		asJSON(w, http.StatusUnauthorized, err.Error())
		return
	}

	icinga := c.GetIcingaClient()
	if icinga == nil {
		panic("icinga client is nil")
	}

	// Godoc: https://godoc.org/github.com/prometheus/alertmanager/template#Data
	data := template.Data{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		l.Errorf("Unable to decode request")
		asJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	l.Infof("Alerts: GroupLabels=%v, CommonLabels=%v", data.GroupLabels, data.CommonLabels)

	serviceHost := c.GetConfig().HostName
	l.V(2).Infof("Check service host: %v", serviceHost)
	host, err := icinga.GetHost(serviceHost)
	if err != nil {
		l.Errorf("Did not find service host %v: %v\n", host, err)
		asJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	sameAlertName := false
	groupedAlertName, sameAlertName := data.GroupLabels["alertname"]
	if sameAlertName {
		l.V(2).Infof("Grouped alerts with matching alertname: %v", groupedAlertName)
	} else if len(data.Alerts) > 1 {
		l.V(2).Infof("Grouped alerts without matching alertname: %d alerts", len(data.Alerts))
	}

	for _, alert := range data.Alerts {
		l.V(2).Infof("Processing %v alert: alertname=%v, severity=%v, message=%v",
			alert.Status,
			alert.Labels["alertname"],
			alert.Labels["severity"],
			alert.Annotations["message"])

		// Compute service and display name for alert
		serviceName, err := computeServiceName(data, alert, c)
		if err != nil {
			l.Errorf("Unable to compute internal service name: %v", err)
		}
		displayName, err := computeDisplayName(data, alert)
		if err != nil {
			l.Errorf("Unable to compute service display name: %v", err)
		}

		// Update or create service in icinga
		svc, err := updateOrCreateService(icinga, serviceHost, serviceName, displayName, alert, c)
		if err != nil {
			l.Errorf("Error in checkOrCreateService for %v: %v", serviceName, err)
		}
		// If we got an emtpy service object, the service was not
		// created, don't try to call process-check-result
		if svc.Name == "" {
			continue
		}

		exitStatus := severityToExitStatus(alert.Status, alert.Labels["severity"], c.GetConfig().MergedSeverityLevels)
		if _, ok := alert.Labels["heartbeat"]; ok {
			// override exitStatus for sending heartbeat
			exitStatus = 0

		}
		l.V(2).Infof("Executing ProcessCheckResult on icinga2 for %v: exit status %v",
			serviceName, exitStatus)

		// Get the Plugin Output from the first Annotation we find that has some data
		pluginOutput := ""
		for _, v := range c.GetConfig().AlertManagerConfig.PluginOutputAnnotations {

			// If the PluginOutputByStates option is enabled then first look for an annotation with the state suffix
			// otherwise fall back to just using the PluginOutputAnnotations value as is
			if c.GetConfig().AlertManagerConfig.PluginOutputByStates {
				pluginOutput = alert.Annotations[fmt.Sprintf("%s_%s", v, c.GetConfig().AlertManagerConfig.PluginOutputStateSuffixes[exitStatus])]
				if pluginOutput != "" {
					break
				}
			}

			pluginOutput = alert.Annotations[v]
			if pluginOutput != "" {
				break
			}
		}

		err = icinga.ProcessCheckResult(svc, icinga2.Action{
			ExitStatus:   exitStatus,
			PluginOutput: pluginOutput,
		})
		if err != nil {
			l.Errorf("Error in ProcessCheckResult for %v: %v", serviceName, err)
		}
	}

	asJSON(w, http.StatusOK, "success")
}
