package main

import (
	"fmt"
	"net/http"
	"os"
)

func healthz(w http.ResponseWriter, r *http.Request, c *SignaliloConfig) {
	fmt.Fprint(w, "ok")
	c.Logger.V(2).Infof("Config: %+v", c)
}

func main() {
	verbosity := 1
	log := newLogger(verbosity)
	log.Info("Starting signalilo alertmanager-icinga2 bridge")

	config := LoadConfig(log)
	log = config.Logger

	log.V(2).Infof("Config: %+v", config)

	http.HandleFunc("/healthz",
		func(w http.ResponseWriter, r *http.Request) { healthz(w, r, config) })
	http.HandleFunc("/webhook",
		func(w http.ResponseWriter, r *http.Request) { webhook(w, r, config) })

	listenAddress := ":8888"
	if os.Getenv("PORT") != "" {
		listenAddress = ":" + os.Getenv("PORT")
	}

	log.Infof("listening on: %v", listenAddress)
	log.Error(http.ListenAndServe(listenAddress, nil))
}
