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
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/template"
	"github.com/vshn/go-icinga2-client/icinga2"
	"github.com/vshn/signalilo/config"
)

// validateServiceName checks that computed service name matches constraints
// given by the Icinga configuration
func validateServiceName(serviceName string) bool {
	re := regexp.MustCompile(`^[-+_.:,a-zA-Z0-9]{1,128}$`)
	return re.MatchString(serviceName)
}

// mapToStableString converts a map of alert labels to a string
// representation which is stable if the same map of alert labels is provided
// to subsequent calls of mapToStableString.
func mapToStableString(data map[string]string) string {
	var keys []string
	for k := range data {
		if k != "severity" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%v:%v ", k, data[k]))
	}
	return sb.String()
}

// computeServiceName computes the internal service name used for Icinga2
func computeServiceName(
	data template.Data,
	alert template.Alert,
	c config.Configuration) (string, error) {

	l := c.GetLogger()

	hash := sha256.New()
	// use bridge uuid to ensure we can't accidentally touch another
	// instance's services
	hash.Write([]byte(c.GetConfig().UUID))
	hash.Write([]byte(mapToStableString(alert.Labels)))
	// 8 bytes gives us 16 characters
	labelhash := fmt.Sprintf("%x", hash.Sum(nil)[:8])

	serviceName := alert.Labels["alertname"]
	if serviceName == "" {
		l.V(2).Infof("alert doesn't have label 'alertname', just using %v as service name", labelhash)
	}
	serviceName = fmt.Sprintf("%v_%v", serviceName, labelhash)

	if validateServiceName(serviceName) {
		return serviceName, nil
	}

	return "", fmt.Errorf("Service name '%v' doesn't match icinga2 constraints", serviceName)
}

// computeDisplayName computes a "human-readable" display name for Icinga2
func computeDisplayName(data template.Data, alert template.Alert) (string, error) {
	return alert.Labels["alertname"], nil
}

// severityToExitStatus computes an exitstatus which Icinga2 understands from
// an alert's status and severity label
func severityToExitStatus(status string, severity string) int {
	// default to "UNKNOWN"
	exitstatus := 3
	if status == "firing" {
		switch severity {
		case "normal":
			exitstatus = 0
		case "warning":
			exitstatus = 1
		case "critical":
			exitstatus = 2
		default:
			exitstatus = 3
		}
	} else if status == "resolved" {
		// mark exit status as NORMAL when alert state is "resolved"
		exitstatus = 0
	}
	return exitstatus
}

func createServiceData(hostname string,
	serviceName string,
	displayName string,
	alert template.Alert,
	status int,
	heartbeatInterval time.Duration,
	c config.Configuration) icinga2.Service {
	l := c.GetLogger()
	config := c.GetConfig()

	// build Vars map
	serviceVars := make(icinga2.Vars)
	// Set defaults
	serviceVars["bridge_uuid"] = config.UUID
	serviceVars["keep_for"] = config.KeepFor
	serviceVars = mapIcingaVariables(serviceVars, alert.Labels, "label_", c.GetLogger())
	serviceVars = mapIcingaVariables(serviceVars, alert.Annotations, "annotation_", c.GetLogger())

	// Create service attrs object
	serviceData := icinga2.Service{
		Name:               serviceName,
		DisplayName:        displayName,
		HostName:           hostname,
		CheckCommand:       "dummy",
		EnableActiveChecks: false,
		Vars:               serviceVars,
		Notes:              alert.Annotations["description"],
		ActionURL:          alert.GeneratorURL,
		NotesURL:           alert.Annotations["runbook_url"],
		CheckInterval:      43200,
		RetryInterval:      43200,
		// We don't need soft states in Icinga, since the grace
		// periods are already managed by Prometheus/Alertmanager
		MaxCheckAttempts: 1,
	}

	// Check if this is a heartbeat service. Adjust serviceData
	// accordingly
	if heartbeatInterval.Seconds() > 0.0 {
		l.Infof("Creating alert as heartbeat with check interval %v", heartbeatInterval)
		// Set dummy text to message annotation on alert
		serviceData.Vars["dummy_text"] = alert.Annotations["message"]
		// Set exitStatus for missed heartbeat to Alert's severity
		serviceData.Vars["dummy_state"] = status
		// add 10% onto requested check interval to allow some network
		// latency for the check results
		serviceData.CheckInterval = heartbeatInterval.Seconds() * 1.1
		serviceData.RetryInterval = heartbeatInterval.Seconds() * 1.1
		// Enable active checks for heartbeat check
		serviceData.EnableActiveChecks = true
	}

	return serviceData
}

// updateOrCreateService updates or creates an Icinga2 service object from the
// alert passed to the method
func updateOrCreateService(icinga icinga2.Client,
	hostname string,
	serviceName string,
	displayName string,
	alert template.Alert,
	c config.Configuration) (icinga2.Service, error) {

	l := c.GetLogger()

	// Check if this alert is a heartbeat alert and extract interval if so
	heartbeatInterval := time.Duration(0)
	if val, ok := alert.Labels["heartbeat"]; ok {
		if alert.Status == "resolved" {
			l.Infof("Not processing resolved heartbeat for %v", serviceName)
			return icinga2.Service{}, nil
		}
		interval, err := time.ParseDuration(val)
		if err != nil {
			return icinga2.Service{}, fmt.Errorf("Unable to parse heartbeat interval: %v", err)
		}
		heartbeatInterval = interval
	}

	status := severityToExitStatus(alert.Status, alert.Labels["severity"])

	serviceData := createServiceData(hostname, serviceName, displayName, alert, status, heartbeatInterval, c)

	icingaSvc, err := icinga.GetService(serviceData.FullName())
	// update or create service, depending on whether object exists
	if err == nil {
		l.Infof("updating service: %+v\n", icingaSvc.Name)
		err := icinga.UpdateService(serviceData)
		if err != nil {
			return serviceData, err
		}
	} else if status > 0 {
		l.Infof("creating service: %+v\n", serviceName)
		err := icinga.CreateService(serviceData)
		if err != nil {
			return serviceData, err
		}
	} else {
		l.Infof("Not creating service %v; status = %v", serviceName, status)
		return icinga2.Service{}, nil
	}
	return serviceData, nil
}
