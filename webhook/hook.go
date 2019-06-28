package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"git.vshn.net/appuio/signalilo/config"
	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/prometheus/alertmanager/template"
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
	if tokenHeader == "" {
		return fmt.Errorf("Request does not have Authorization header")
	}
	headerElems := strings.Split(tokenHeader, " ")
	if len(headerElems) != 2 || (len(headerElems) > 0 && headerElems[0] != "Bearer") {
		return fmt.Errorf("Malformed Authorization header")
	}
	token := headerElems[1]
	if token != c.GetConfig().AlertManagerConfig.BearerToken {
		return fmt.Errorf("Invalid Bearer token")
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
		os.Exit(1)
	}

	sameAlertName := false
	groupedAlertName, sameAlertName := data.GroupLabels["alertname"]
	if sameAlertName {
		l.V(2).Infof("Grouped alerts with matching alertname: %v", groupedAlertName)
	} else if len(data.Alerts) > 1 {
		l.V(2).Infof("Grouped alerts without matching alertname: %d alerts", len(data.Alerts))
	}

	for _, alert := range data.Alerts {
		l.V(2).Infof("Alert: alertname=%v", alert.Labels["alertname"])

		l.V(2).Infof("Alert: severity=%v", alert.Labels["severity"])
		l.V(2).Infof("Alert: message=%v", alert.Annotations["message"])

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
		err = icinga.ProcessCheckResult(svc, icinga2.Action{
			ExitStatus:   severityToExitStatus(alert.Status, alert.Labels["severity"]),
			PluginOutput: alert.Annotations["message"],
		})
		if err != nil {
			l.Errorf("Error in ProcessCheckResult for %v: %v", serviceName, err)
		}
	}

	asJSON(w, http.StatusOK, "success")
}
