package main

import (
	"fmt"
	"net/http"

	"git.vshn.net/appuio/signalilo/config"
	"git.vshn.net/appuio/signalilo/webhook"
	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/bketelsen/logr"
	"gopkg.in/alecthomas/kingpin.v2"
)

// ServeCommand holds all the configuration and objects necessary to serve the
// Signalilo webhook
type ServeCommand struct {
	configFile   string
	port         int
	logLevel     int
	config       *config.SignaliloConfig
	logger       logr.Logger
	icingaClient icinga2.Client
}

// GetConfigFile implements config.Configuration
func (s *ServeCommand) GetConfigFile() string {
	return s.configFile
}

// GetConfig implements config.Configuration
func (s *ServeCommand) GetConfig() *config.SignaliloConfig {
	return s.config
}

// GetLogger implements config.Configuration
func (s *ServeCommand) GetLogger() logr.Logger {
	return s.logger
}

// GetIcingaClient implements config.Configuration
func (s *ServeCommand) GetIcingaClient() icinga2.Client {
	return s.icingaClient
}

// SetLogger implements config.Configuration
func (s *ServeCommand) SetLogger(logger logr.Logger) {
	s.logger = logger
}

// SetIcingaClient implements config.Configuration
func (s *ServeCommand) SetIcingaClient(client icinga2.Client) {
	s.icingaClient = client
}

func healthz(w http.ResponseWriter, r *http.Request, c config.Configuration) {
	fmt.Fprint(w, "ok")
	c.GetLogger().V(3).Infof("Config: %+v", c.GetConfig())
}

func (s *ServeCommand) run(ctx *kingpin.ParseContext) error {
	http.HandleFunc("/healthz",
		func(w http.ResponseWriter, r *http.Request) { healthz(w, r, s) })
	http.HandleFunc("/webhook",
		func(w http.ResponseWriter, r *http.Request) { webhook.Webhook(w, r, s) })

	listenAddress := fmt.Sprintf(":%d", s.port)

	s.logger.Infof("listening on: %v", listenAddress)
	return http.ListenAndServe(listenAddress, nil)
}

func (s *ServeCommand) initialize(ctx *kingpin.ParseContext) error {
	s.logger = config.NewLogger(s.logLevel)
	var err error
	if s.config, err = config.LoadConfig(s); err != nil {
		return err
	}
	return nil
}

func configureServeCommand(app *kingpin.Application) {
	s := &ServeCommand{logLevel: 1}
	serve := app.Command("serve", "Run the Signalilo service").Default().Action(s.run).PreAction(s.initialize)
	serve.Flag("config-file", "Configuration file").Short('c').StringVar(&s.configFile)
	serve.Flag("port", "Listening port for the Alertmanager webhook").Default("8888").Envar("PORT").Short('p').IntVar(&s.port)
}
