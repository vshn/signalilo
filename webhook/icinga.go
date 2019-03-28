package webhook

import (
	"strings"

	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/bketelsen/logr"
	"github.com/prometheus/alertmanager/template"
)

func computeServiceName(data template.Data, alert template.Alert) string {
	service := alert.Annotations["summary"]
	if service == "" && strings.HasPrefix(data.GroupLabels["alertname"], "service") {
		service = "Service " + alert.Labels["instance"]
	} else if service == "" {
		service = data.GroupLabels["alertname"] + ":" + alert.Labels["instance"]
		service = strings.Replace(service, "/", "_", -1)
	}
	return service
}

func severityToExitStatus(severity string) int {
	var exitstatus int
	switch severity {
	case "normal":
		exitstatus = 0
	case "major":
		exitstatus = 1
	case "critical":
		exitstatus = 2
	default:
		exitstatus = 3
	}
	return exitstatus
}

func updateOrCreateService(icinga icinga2.Client,
	hostname string,
	service string,
	alert template.Alert,
	l logr.Logger) (icinga2.Service, error) {

	// build Vars map
	serviceVars := make(icinga2.Vars)
	for k, v := range alert.Labels {
		serviceVars["label_"+k] = v
	}
	for k, v := range alert.Annotations {
		serviceVars["Annotation_"+k] = v
	}
	serviceVars["dummy_text"] = "No passive check result received"
	serviceVars["dummy_state"] = 3
	// Create service attrs object
	serviceData := icinga2.Service{
		Name:         service,
		HostName:     hostname,
		CheckCommand: "dummy",
		Vars:         serviceVars,
		Notes:        alert.Annotations["description"],
		//ActionURL:    alert.Annotations["action_url"],
		ActionURL: alert.GeneratorURL,
	}

	icingaSvc, err := icinga.GetService(serviceData.FullName())
	// update or create service, depending on whether object exists
	if err == nil {
		l.Infof("found service: %+v\n", icingaSvc.Name)
		err := icinga.UpdateService(serviceData)
		if err != nil {
			return serviceData, err
		}
	} else {
		l.Infof("creating service: %+v\n", service)
		err := icinga.CreateService(serviceData)
		if err != nil {
			return serviceData, err
		}
	}
	return serviceData, nil
}
