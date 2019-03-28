package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"git.vshn.net/appuio/signalilo/config"
	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/prometheus/alertmanager/template"
)

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

	serviceHost := c.GetConfig().HostName
	l.V(2).Infof("Check service host: %v", serviceHost)
	host, err := icinga.GetHost(serviceHost)
	if err != nil {
		l.Errorf("Did not find service host %v: %v\n", host, err)
		os.Exit(1)
	}

	for _, alert := range data.Alerts {
		//l.Infof("Alert: status=%s,Labels=%v,Annotations=%v", alert.Status, alert.Labels, alert.Annotations)

		l.V(2).Infof("Alert: severity=%v", alert.Labels["severity"])
		l.V(2).Infof("Alert: message=%v", alert.Annotations["message"])
		// Create or update service for alert in icinga
		service := computeServiceName(data, alert)
		svc, err := updateOrCreateService(icinga, serviceHost, service, alert, c)
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
