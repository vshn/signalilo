package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/bketelsen/logr"
	"github.com/prometheus/alertmanager/template"
)

type HostStruct struct {
	Name  string    `json:"name"`
	Type  string    `json:"type"`
	Attrs HostAttrs `json:"attrs"`
	Meta  struct{}  `json:"meta"`
	Joins struct{}  `json:"stuct"`
}

type HostAttrs struct {
	ActionURL    string      `json:"action_url"`
	Address      string      `json:"address"`
	Address6     string      `json:"address6"`
	CheckCommand string      `json:"check_command"`
	DisplayName  string      `json:"display_name"`
	Groups       []string    `json:"groups"`
	Notes        string      `json:"notes"`
	NotesURL     string      `json:"notes_url"`
	Templates    []string    `json:"templates"`
	Vars         interface{} `json:"vars"`
}
type responseJSON struct {
	Status  int
	Message string
}

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

func webhook(w http.ResponseWriter, r *http.Request, c *SignaliloConfig) {
	defer r.Body.Close()

	l := c.Logger
	if l == nil {
		panic("logger is nil")
	}
	icinga := c.IcingaClient
	if icinga == nil {
		panic("icinga client is nil")
	}

	// Godoc: https://godoc.org/github.com/prometheus/alertmanager/template#Data
	data := template.Data{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		asJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	l.Infof("Alerts: GroupLabels=%v, CommonLabels=%v", data.GroupLabels, data.CommonLabels)

	hostname := data.CommonLabels["customer"] + ".local"
	l.V(2).Infof("Checking/creating host for %v\n", hostname)
	host, err := checkOrCreateHost(icinga, hostname, l)
	if err != nil {
		l.Infof("Error in checkOrCreateHost for %v: %v\n", host, err)
	}

	for _, alert := range data.Alerts {
		//l.Infof("Alert: status=%s,Labels=%v,Annotations=%v", alert.Status, alert.Labels, alert.Annotations)

		apihost := HostStruct{}
		apihost.Name = alert.Labels["customer"] + ".local"
		apihost.Attrs = HostAttrs{}
		apihost.Attrs.CheckCommand = "hostalive"

		//l.V(2).Infof("Alert: %+v\n", alert)
		l.V(2).Infof("Alert: severity=%v", alert.Labels["severity"])
		l.V(2).Infof("Alert: message=%v", alert.Annotations["message"])
		service := alert.Annotations["summary"]
		if strings.HasPrefix(data.GroupLabels["alertname"], "service") {
			service = "Service " + alert.Labels["instance"]
		}
		if service == "" {
			service = data.GroupLabels["alertname"] + ":" + alert.Labels["instance"]
			service = strings.Replace(service, "/", "_", -1)
		}
		svc, _ := updateOrCreateService(icinga, hostname, service, alert, l)
		icinga.ProcessCheckResult(svc, icinga2.Action{
			ExitStatus:   severityToExitStatus(alert.Labels["severity"]),
			PluginOutput: alert.Annotations["value"],
		})

		// severity := alert.Labels["severity"]
		// switch strings.ToUpper(severity) {
		// case "CRITICAL":
		// 	l.Infof("severity: %s", severity)
		// case "WARNING":
		// 	l.Infof("severity: %s", severity)
		// default:
		// 	l.Infof("no action on severity: %s", severity)
		// }
	}

	asJSON(w, http.StatusOK, "success")
}

func checkOrCreateHost(icinga icinga2.Client, hostname string, l logr.Logger) (icinga2.Host, error) {
	host, err := icinga.GetHost(hostname)
	if err == nil {
		l.Infof("found host: %+v", hostname)
		return host, nil
	}
	l.Infof("creating host: %+v\n", hostname)
	host = icinga2.Host{
		Name:         hostname,
		Address:      "10.144.1.226",
		CheckCommand: "hostalive",
		Vars:         icinga2.Vars{"os": "Linux"},
	}
	err = icinga.CreateHost(host)
	if err != nil {
		l.Infof("Error creating host %v: %v\n", hostname, err)
	}
	return host, err
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
			l.Errorf("Error updating service %v: %v\n", service, err)
			return serviceData, err
		}
	} else {
		l.Infof("creating service: %+v\n", service)
		err := icinga.CreateService(serviceData)
		if err != nil {
			l.Errorf("Error creating service %v: %v\n", service, err)
			return serviceData, err
		}
	}
	return serviceData, nil
}
