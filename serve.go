/*
 * Authors:
 * Simon Gerber <simon.gerber@vshn.ch>
 *
 * License:
 * Copyright (c) 2019, VSHN AG, <info@vshn.ch>
 * Licensed under "BSD 3-Clause". See LICENSE file.
 */

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bketelsen/logr"
	"github.com/vshn/go-icinga2-client/icinga2"
	"github.com/vshn/signalilo/config"
	"github.com/vshn/signalilo/gc"
	"github.com/vshn/signalilo/webhook"
	"gopkg.in/alecthomas/kingpin.v2"
)

// ServeCommand holds all the configuration and objects necessary to serve the
// Signalilo webhook
type ServeCommand struct {
	port            int
	logLevel        int
	config          config.SignaliloConfig
	logger          logr.Logger
	icingaClient    icinga2.Client
	heartbeatTicker *time.Ticker
	gcTicker        *time.Ticker
}

// GetConfig implements config.Configuration
func (s *ServeCommand) GetConfig() *config.SignaliloConfig {
	return &s.config
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

	go func() {
		// Send initial heartbeat from goroutine to make server
		// startup quicker
		err := s.heartbeat(time.Now())
		if err != nil {
			s.logger.Errorf("Unable to send initial heartbeat: %v", err)
		}

		for ts := range s.heartbeatTicker.C {
			if err := s.heartbeat(ts); err != nil {
				s.logger.Errorf("sending heartbeat: %s", err)

				// In some rare cases there could be an Icinga2 reload for a config update and the API is not reachable.
				// So, the switch off of the Config-Master would take in place, but we don't want an unnecessary switch
				// to the secondary Icinga2 instance.
				s.logger.Infof("Waiting %s for reconnect", s.config.Reconnect)
				time.Sleep(s.config.Reconnect)
				erro := s.icingaClient.TestIcingaApi(s.icingaClient.GetClientConfig().URL)

				if erro != nil {
					// Tests all configured '--icinga_url' and sets the URL which doesn't respond with error
					for _, url := range s.config.IcingaConfig.URL {
						erri := s.icingaClient.TestIcingaApi(url)

						if erri == nil {
							s.logger.Infof("Switching to new Icinga-API-URL: %v", url)
							s.icingaClient.SetIcingaUrl(url)
							break
						}
					}
				} else {
					s.logger.Infof("Reconnect successful: %v", s.icingaClient.GetClientConfig().URL)
					continue
				}
			}

			erro := s.icingaClient.TestIcingaApi(s.config.IcingaConfig.URL[0])
			if (s.icingaClient.GetClientConfig().URL != s.config.IcingaConfig.URL[0]) && erro == nil {
				// If all URLs are accessible, switch to the 'first one', it's the Icinga-Config-Master per default
				s.logger.Infof("Connecting to Icinga-Config-Master: %v", s.config.IcingaConfig.URL[0])
				s.icingaClient.SetIcingaUrl(s.config.IcingaConfig.URL[0])
				continue
			}
		}
	}()
	return nil
}

func (s *ServeCommand) startServiceGC() error {
	gcInterval := s.GetConfig().GcInterval
	s.gcTicker = time.NewTicker(gcInterval)
	s.logger.Infof("Starting service garbage collector: interval %v", gcInterval)
	go func() {
		for ts := range s.gcTicker.C {
			if err := gc.Collect(ts, s); err != nil {
				s.logger.Error(err)
			}
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
	s.logger.Infof("Icinga API: %s", s.GetIcingaClient().GetClientConfig().URL)

	if err := s.startHeartbeat(); err != nil {
		return err
	}
	if err := s.startServiceGC(); err != nil {
		return err
	}

	listenAddress := fmt.Sprintf(":%d", s.port)
	s.logger.Infof("listening on: %v", listenAddress)
	alertManagerConfig := s.config.AlertManagerConfig
	if alertManagerConfig.UseTLS {
		s.logger.Infof("Using TLS: certificate=%v, key=%v", alertManagerConfig.TLSCertPath, alertManagerConfig.TLSKeyPath)
		return http.ListenAndServeTLS(listenAddress, alertManagerConfig.TLSCertPath, alertManagerConfig.TLSKeyPath, nil)
	}

	return http.ListenAndServe(listenAddress, nil)
}

func (s *ServeCommand) initialize(ctx *kingpin.ParseContext) error {
	s.logger = config.NewLogger(s.logLevel)
	config.ConfigInitialize(s)
	return nil
}

func configureServeCommand(app *kingpin.Application) {
	s := &ServeCommand{logLevel: 1,
		config: config.SignaliloConfig{
			StaticServiceVars:    map[string]string{},
			CustomSeverityLevels: map[string]string{},
		},
	}
	serve := app.Command("serve", "Run the Signalilo service").Default().Action(s.run).PreAction(s.initialize)

	// General configuration
	serve.Flag("uuid", "Instance UUID").Envar("SIGNALILO_UUID").Required().StringVar(&s.config.UUID)
	serve.Flag("loglevel", "Signalilo Loglevel").Envar("SIGNALILO_LOG_LEVEL").Default("2").IntVar(&s.config.LogLevel)

	// Icinga2 client configuration
	serve.Flag("icinga_hostname", "Icinga Servicehost Name").Envar("SIGNALILO_ICINGA_HOSTNAME").Required().StringVar(&s.config.HostName)
	serve.Flag("icinga_url", "Icinga API URL (can be repeated)").Envar("SIGNALILO_ICINGA_URL").Required().StringsVar(&s.config.IcingaConfig.URL)
	serve.Flag("icinga_username", "Icinga Username").Envar("SIGNALILO_ICINGA_USERNAME").Required().StringVar(&s.config.IcingaConfig.User)
	serve.Flag("icinga_password", "Icinga Password").Envar("SIGNALILO_ICINGA_PASSWORD").Required().StringVar(&s.config.IcingaConfig.Password)
	serve.Flag("icinga_insecure_tls", "Skip Icinga TLS verification").Envar("SIGNALILO_ICINGA_INSECURE_TLS").Default("false").BoolVar(&s.config.IcingaConfig.InsecureTLS)
	serve.Flag("icinga_x509_verify_cn", "Use CN when verifying certificates. Overrides the default go1.15 behavior of rejecting certificates without matching SAN.").Envar("SIGNALILO_ICINGA_X509_VERIFY_CN").Default("true").BoolVar(&s.config.IcingaConfig.X509VerifyCN)
	serve.Flag("icinga_disable_keepalives", "Disable HTTP keepalives").Envar("SIGNALILO_ICINGA_DISABLE_KEEPALIVES").Default("false").BoolVar(&s.config.IcingaConfig.DisableKeepAlives)
	serve.Flag("icinga_debug", "Enable debug-level logging for icinga2 client library").Envar("SIGNALILO_ICINGA_DEBUG").Default("false").BoolVar(&s.config.IcingaConfig.Debug)
	serve.Flag("icinga_heartbeat_interval", "Heartbeat interval to Icinga").Envar("SIGNALILO_ICINGA_HEARTBEAT_INTERVAL").Default("1m").DurationVar(&s.config.HeartbeatInterval)
	serve.Flag("icinga_gc_interval", "Garbage collection interval for old alerts").Envar("SIGNALILO_ICINGA_GC_INTERVAL").Default("15m").DurationVar(&s.config.GcInterval)
	serve.Flag("icinga_keep_for", "How long to keep old alerts around after they've been resolved").Envar("SIGNALILO_ICINGA_KEEP_FOR").Default("168h").DurationVar(&s.config.KeepFor)
	serve.Flag("icinga_ca", "A custom CA certificate to use when connecting to the Icinga API").Envar("SIGNALILO_ICINGA_CA").StringVar(&s.config.CAData)
	serve.Flag("icinga_service_checks_active", "Create icinga services as active checks").Envar("SIGNALILO_ICINGA_SERVICE_CHECKS_ACTIVE").Default("false").BoolVar(&s.config.ActiveChecks)
	serve.Flag("icinga_service_checks_command", "Specify icinga check command during service creation").Envar("SIGNALILO_ICINGA_SERVICE_CHECKS_COMMAND").Default("dummy").StringVar(&s.config.CheckCommand)
	serve.Flag("icinga_service_checks_interval", "Interval (in seconds) to be used for icinga check_interval and retry_interval").Envar("SIGNALILO_ICINGA_SERVICE_CHECKS_INTERVAL").Default("12h").DurationVar(&s.config.ChecksInterval)
	serve.Flag("icinga_service_max_check_attempts", "The maximum number of checks which are executed before changing to a hard state").Envar("SIGNALILO_ICINGA_SERVICE_MAX_CHECK_ATTEMPTS").Default("1").IntVar(&s.config.MaxCheckAttempts)
	serve.Flag("icinga_static_service_var", "A variable to be set on each Icinga service created by Signalilo. The expected format is variable=value. Can be repeated.").Envar("SIGNALILO_ICINGA_STATIC_SERVICE_VAR").StringMapVar(&s.config.StaticServiceVars)
	serve.Flag("icinga_reconnect", "Interval to be used for Signalilo to wait for a reconnect instead of switching immediately to another URL").Envar("SIGNALILO_ICINGA_RECONNECT").Default("10s").DurationVar(&s.config.Reconnect)

	// Alert manager configuration
	serve.Flag("alertmanager_port", "Listening port for the Alertmanager webhook").Default("8888").Envar("SIGNALILO_ALERTMANAGER_PORT").IntVar(&s.port)
	serve.Flag("alertmanager_bearer_token", "Bearer token for incoming requests").Envar("SIGNALILO_ALERTMANAGER_BEARER_TOKEN").Required().StringVar(&s.config.AlertManagerConfig.BearerToken)
	serve.Flag("alertmanager_tls_cert", "Path of certificate file for TLS-enabled webhook endpoint. Should contain the full chain").Envar("SIGNALILO_ALERTMANAGER_TLS_CERT").StringVar(&s.config.AlertManagerConfig.TLSCertPath)
	serve.Flag("alertmanager_tls_key", "Path of private key file for TLS-enabled webhook endpoint").Envar("SIGNALILO_ALERTMANAGER_TLS_KEY").StringVar(&s.config.AlertManagerConfig.TLSKeyPath)

	serve.Flag("alertmanager_pluginoutput_annotations", "List of Annotation names to be used to set the Plugin Output for the Icinga Service").Default("message").Envar("SIGNALILO_ALERTMANAGER_PLUGINOUTPUT_ANNOTATIONS").StringsVar(&s.config.AlertManagerConfig.PluginOutputAnnotations)
	serve.Flag("alertmanager_custom_severity_levels", "Add or override the default mapping of Severity Levels to Service States. The expected format is Severity_Level=Service_State where the Service_State is 0=OK, 1=Warning, 2=Critical, 3=Unknown. Can be repeated.").Envar("SIGNALILO_ALERTMANAGER_CUSTOM_SEVERITY_LEVELS").StringMapVar(&s.config.CustomSeverityLevels)
	serve.Flag("alertmanager_pluginoutput_by_states", "Enables support for dynamically selecting the Annotation name used for the Plugin Output based on the computed Service State.").Default("false").Envar("SIGNALILO_ALERTMANAGER_PLUGINOUTPUT_BY_STATES").BoolVar(&s.config.AlertManagerConfig.PluginOutputByStates)

}
