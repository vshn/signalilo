package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"git.vshn.net/appuio/signalilo/config"
	"git.vshn.net/appuio/signalilo/webhook"
	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/bketelsen/logr"
	"gopkg.in/alecthomas/kingpin.v2"
)

// ServeCommand holds all the configuration and objects necessary to serve the
// Signalilo webhook
type ServeCommand struct {
	configFile      string
	port            int
	logLevel        int
	config          *config.SignaliloConfig
	logger          logr.Logger
	icingaClient    icinga2.Client
	heartbeatTicker *time.Ticker
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

func (s *ServeCommand) heartbeat(ts time.Time) error {
	icinga := s.GetIcingaClient()
	config := s.GetConfig()
	l := s.GetLogger()
	_, err := icinga.GetHost(config.HostName)
	if err != nil {
		l.Errorf("heartbeat: unable to get servicehost: %v", err)
		return err
	}
	svc, err := icinga.GetService(fmt.Sprintf("%v!heartbeat", config.HostName))
	if err != nil {
		l.Errorf("heartbeat: unable to get heartbeat service: %v", err)
		return err
	}
	msg := fmt.Sprintf("OK: %v", ts.Format(time.RFC3339))
	l.Infof("Sending heartbeat: '%v'", msg)
	err = icinga.ProcessCheckResult(svc, icinga2.Action{
		ExitStatus:   0,
		PluginOutput: msg,
	})
	if err != nil {
		l.Errorf("heartbeat: process_check_result: %v", err)
	}
	return nil
}

func (s *ServeCommand) startHeartbeat() error {
	hbInterval := s.GetConfig().HeartbeatInterval
	s.heartbeatTicker = time.NewTicker(hbInterval)
	s.logger.Infof("Starting heartbeat: interval %v", hbInterval)
	err := s.heartbeat(time.Now())
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to send initial heartbeat: %v", err))
	}
	go func() {
		for ts := range s.heartbeatTicker.C {
			s.heartbeat(ts)
		}
	}()
	return nil
}

func (s *ServeCommand) run(ctx *kingpin.ParseContext) error {
	http.HandleFunc("/healthz",
		func(w http.ResponseWriter, r *http.Request) { healthz(w, r, s) })
	http.HandleFunc("/webhook",
		func(w http.ResponseWriter, r *http.Request) { webhook.Webhook(w, r, s) })

	s.logger.Infof("Signalilo UUID: %v", s.GetConfig().UUID)
	s.logger.Infof("Keep for: %v", s.GetConfig().KeepFor)

	if err := s.startHeartbeat(); err != nil {
		return err
	}

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
