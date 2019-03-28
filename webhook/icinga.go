package webhook

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"git.vshn.net/appuio/signalilo/config"
	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/bketelsen/logr"
	"github.com/prometheus/alertmanager/template"
)

// check that computed service name matches constraints given by icinga
func validateServiceName(serviceName string) bool {
	re := regexp.MustCompile(`^[-+_.:,a-zA-Z0-9]{1,128}$`)
	return re.MatchString(serviceName)
}

func MapToStableString(data map[string]string) string {
	var keys []string
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%v:%v ", k, data[k]))
	}
	return sb.String()
}

// compute the internal service name for icinga2
func computeServiceName(
	data template.Data,
	alert template.Alert,
	l logr.Logger) (string, error) {

	hash := sha256.New()
	hash.Write([]byte(MapToStableString(alert.Labels)))
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

func computeDisplayName(data template.Data, alert template.Alert) (string, error) {
	return alert.Labels["alertname"], nil
}

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

func updateOrCreateService(icinga icinga2.Client,
	hostname string,
	serviceName string,
	displayName string,
	alert template.Alert,
	c config.Configuration) (icinga2.Service, error) {

	l := c.GetLogger()
	config := c.GetConfig()

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
	serviceVars["dummy_text"] = "No passive check result received"
	serviceVars["dummy_state"] = 3
	// Create service attrs object
	serviceData := icinga2.Service{
		Name:         serviceName,
		DisplayName:  displayName,
		HostName:     hostname,
		CheckCommand: "dummy",
		Vars:         serviceVars,
		Notes:        alert.Annotations["description"],
		ActionURL:    alert.GeneratorURL,
		NotesURL:     alert.Annotations["runbook_url"],
	}

	icingaSvc, err := icinga.GetService(serviceData.FullName())
	// update or create service, depending on whether object exists
	if err == nil {
		l.Infof("updating service: %+v\n", icingaSvc.Name)
		err := icinga.UpdateService(serviceData)
		if err != nil {
			return serviceData, err
		}
	} else {
		l.Infof("creating service: %+v\n", serviceName)
		err := icinga.CreateService(serviceData)
		if err != nil {
			return serviceData, err
		}
	}
	return serviceData, nil
}
