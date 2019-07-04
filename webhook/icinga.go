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

	"git.vshn.net/appuio/signalilo/config"
	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/prometheus/alertmanager/template"
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

// updateOrCreateService updates or creates an Icinga2 service object from the
// alert passed to the method
func updateOrCreateService(icinga icinga2.Client,
	hostname string,
	serviceName string,
	displayName string,
	alert template.Alert,
	c config.Configuration) (icinga2.Service, error) {

	l := c.GetLogger()
	config := c.GetConfig()

	status := severityToExitStatus(alert.Status, alert.Labels["severity"])

	// build Vars map
	serviceVars := make(icinga2.Vars)
	// Set defaults
	serviceVars["bridge_uuid"] = config.UUID
	serviceVars["keep_for"] = config.KeepFor
	for k, v := range alert.Labels {
		serviceVars["label_"+k] = v
	}
	for k, v := range alert.Annotations {
		serviceVars["annotation_"+k] = v
	}
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
	}

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
