package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"

	"git.vshn.net/appuio/signalilo/config"
	"github.com/Nexinto/go-icinga2-client/icinga2"
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

func Webhook(w http.ResponseWriter, r *http.Request, c config.Configuration) {
	defer r.Body.Close()

	l := c.GetLogger()
	if l == nil {
		panic("logger is nil")
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

	hostname := data.CommonLabels["customer"] + ".local"
	l.V(2).Infof("Checking/creating host for %v", hostname)
	host, err := checkOrCreateHost(icinga, hostname, l)
	if err != nil {
		l.Errorf("Error in checkOrCreateHost for %v: %v\n", host, err)
	}

	for _, alert := range data.Alerts {
		//l.Infof("Alert: status=%s,Labels=%v,Annotations=%v", alert.Status, alert.Labels, alert.Annotations)

		l.V(2).Infof("Alert: severity=%v", alert.Labels["severity"])
		l.V(2).Infof("Alert: message=%v", alert.Annotations["message"])
		// Create or update service for alert in icinga
		service := computeServiceName(data, alert)
		svc, err := updateOrCreateService(icinga, hostname, service, alert, l)
		if err != nil {
			l.Errorf("Error in checkOrCreateService for %v: %v", service, err)
		}
		err = icinga.ProcessCheckResult(svc, icinga2.Action{
			ExitStatus:   severityToExitStatus(alert.Labels["severity"]),
			PluginOutput: alert.Annotations["value"],
		})
		if err != nil {
			l.Errorf("Error in ProcessCheckResult for %v: %v", service, err)
		}
	}

	asJSON(w, http.StatusOK, "success")
}
