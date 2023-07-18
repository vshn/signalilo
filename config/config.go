/*
 * Authors:
 * Simon Gerber <simon.gerber@vshn.ch>
 *
 * License:
 * Copyright (c) 2019, VSHN AG, <info@vshn.ch>
 * Licensed under "BSD 3-Clause". See LICENSE file.
 */

package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bketelsen/logr"
	"github.com/corvus-ch/logr/buffered"
	log "github.com/corvus-ch/logr/logrus"
	"github.com/sirupsen/logrus"
	"github.com/vshn/go-icinga2-client/icinga2"
)

type icingaConfig struct {
	URL               []string
	User              string
	Password          string
	InsecureTLS       bool
	X509VerifyCN      bool
	DisableKeepAlives bool
	Templates         []string
	Debug             bool
}

type Configuration interface {
	GetConfig() *SignaliloConfig

	GetLogger() logr.Logger
	SetLogger(logger logr.Logger)

	GetIcingaClient() icinga2.Client
	SetIcingaClient(icinga icinga2.Client)
}

type alertManagerConfig struct {
	BearerToken               string
	TLSCertPath               string
	TLSKeyPath                string
	UseTLS                    bool
	PluginOutputAnnotations   []string
	PluginOutputByStates      bool
	PluginOutputStateSuffixes []string
}

type SignaliloConfig struct {
	UUID                     string
	HostName                 string
	IcingaConfig             icingaConfig
	GcInterval               time.Duration
	AlertManagerConfig       alertManagerConfig
	HeartbeatInterval        time.Duration
	LogLevel                 int
	DisplayNameAsServiceName bool
	KeepFor                  time.Duration
	CAData                   string
	StaticServiceVars        map[string]string
	CustomSeverityLevels     map[string]string
	MergedSeverityLevels     map[string]int
	ActiveChecks             bool
	ChecksInterval           time.Duration
	CheckCommand             string
	MaxCheckAttempts         int
	Reconnect                time.Duration
	CreateHosts              bool
	CreateHostsLabel         string
}

func ConfigInitialize(configuration Configuration) {
	l := configuration.GetLogger()
	config := configuration.GetConfig()

	// do first init of Logger and IcingaClient
	l.Infof("Configuring logger with LogLevel=%v", config.LogLevel)
	configuration.SetLogger(NewLogger(config.LogLevel))
	// Refresh local reference to logger after setup
	l = configuration.GetLogger()

	icinga, err := newIcingaClient(config, l)
	if err != nil {
		l.Errorf("Unable to create new icinga client: %s", err)
	} else {
		configuration.SetIcingaClient(icinga)
	}
	// finalize TLS config
	if config.AlertManagerConfig.TLSCertPath != "" && config.AlertManagerConfig.TLSKeyPath != "" {
		config.AlertManagerConfig.UseTLS = true
	}

	// Create the default severity levels and then merge any custom ones into it.
	// This keeps the defaults for backwards compatibility and allows both additions and overrides.
	allLevels := map[string]int{
		"normal":   0,
		"warning":  1,
		"critical": 2,
	}
	for k, v := range config.CustomSeverityLevels {
		// Ensure the user set configuration values are valid otherwise default to UNKNOWN
		l, err := strconv.ParseInt(v, 10, 32)
		if err != nil || l < 0 || l > 3 {
			l = 3
		}
		allLevels[strings.ToLower(k)] = int(l)
	}
	config.MergedSeverityLevels = allLevels

	// Set the suffixes used for the PluginOutputByStates
	config.AlertManagerConfig.PluginOutputStateSuffixes = []string{"ok", "warning", "critical", "unknown"}

}

func makeCertPool(c *SignaliloConfig, l logr.Logger) (*x509.CertPool, error) {
	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM([]byte(c.CAData)); !ok {
		return nil, fmt.Errorf("No certs appended")
	}
	return rootCAs, nil
}

func newIcingaClient(c *SignaliloConfig, l logr.Logger) (icinga2.Client, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil && c.CAData == "" {
		return nil, fmt.Errorf("could not load system rootCA and no CA provided: %w", err)
	}
	if c.CAData != "" {
		rootCAs, err = makeCertPool(c, l)
		if err != nil {
			return nil, err
		}
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.IcingaConfig.InsecureTLS,
		RootCAs:            rootCAs,
	}

	if c.IcingaConfig.X509VerifyCN {
		// Set InsecureSkipVerify to skip default verification. This
		// does not disable VerifyConnection
		tlsConfig.InsecureSkipVerify = true
		// Set custom VerifyConnection function which verifies the
		// server's name against the certificate's CN instead of the
		// certificate's SAN. The custom function still respects the
		// IcingaConfig.InsecureTLS setting.
		tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
			if c.IcingaConfig.InsecureTLS {
				// Don't verify anything if user requested insecure
				// TLS operation
				return nil
			}
			commonName := cs.PeerCertificates[0].Subject.CommonName
			if commonName != cs.ServerName {
				return fmt.Errorf("invalid certificate name %q, expected %q", commonName, cs.ServerName)
			}
			opts := x509.VerifyOptions{
				Roots:         rootCAs,
				Intermediates: x509.NewCertPool(),
			}
			for _, cert := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(cert)
			}
			_, err := cs.PeerCertificates[0].Verify(opts)
			return err
		}
	}

	var client *icinga2.WebClient

	for _, url := range c.IcingaConfig.URL {
		client, err = icinga2.New(icinga2.WebClient{
			URL:               url,
			Username:          c.IcingaConfig.User,
			Password:          c.IcingaConfig.Password,
			Debug:             c.IcingaConfig.Debug,
			DisableKeepAlives: c.IcingaConfig.DisableKeepAlives,
			TLSConfig:         tlsConfig})
		if err != nil {
			return nil, err
		}
		if err = client.TestIcingaApi(); err != nil {
			// clear client if the API url wasn't reachable
			client = nil
			continue
		} else {
			break
		}
	}
	if client == nil {
		return nil, fmt.Errorf("no valid Icinga API URL found")
	}
	return client, nil
}

func NewLogger(verbosity int) logr.Logger {
	jf := new(logrus.JSONFormatter)
	ll := &logrus.Logger{
		Out:       os.Stdout,
		Formatter: jf,
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}
	return log.New(verbosity, ll)
}

func MockLogger(verbosity int) logr.Logger {
	return buffered.New(verbosity)
}

type MockConfiguration struct {
	config       SignaliloConfig
	logger       logr.Logger
	icingaClient icinga2.Client
}

func (c *MockConfiguration) GetConfig() *SignaliloConfig {
	return &c.config
}
func (c *MockConfiguration) GetLogger() logr.Logger {
	return c.logger
}
func (c *MockConfiguration) GetIcingaClient() icinga2.Client {
	return c.icingaClient
}
func (c *MockConfiguration) SetConfig(config SignaliloConfig) {
	c.config = config
}
func (c *MockConfiguration) SetLogger(logger logr.Logger) {
	c.logger = logger
}
func (c *MockConfiguration) SetIcingaClient(icinga icinga2.Client) {
	c.icingaClient = icinga
}

func NewMockConfiguration(verbosity int) Configuration {
	// TODO: fill out defaults for MockConfiguration, maybe move default
	// from serve.go to here
	signaliloCfg := SignaliloConfig{
		UUID:     "",
		HostName: "signalilo_appuio_lab",
		IcingaConfig: icingaConfig{
			URL:               []string{"localhost:5665", "anotherhost:5665"},
			User:              "sepp",
			Password:          "sepp1",
			InsecureTLS:       true,
			DisableKeepAlives: false,
			Debug:             false,
			Templates:         []string{"generic-service", "example-template"},
		},
		GcInterval: 1 * time.Minute,
		AlertManagerConfig: alertManagerConfig{
			BearerToken: "aaaaaa",
		},
		HeartbeatInterval:        1 * time.Minute,
		LogLevel:                 2,
		DisplayNameAsServiceName: false,
		KeepFor:                  5 * time.Minute,
		CAData:                   "",
		ActiveChecks:             false,
		ChecksInterval:           12 * time.Hour,
		CheckCommand:             "dummy",
		MaxCheckAttempts:         1,
	}
	mockCfg := &MockConfiguration{
		config: signaliloCfg,
	}
	log := MockLogger(mockCfg.config.LogLevel)
	mockCfg.logger = log
	ConfigInitialize(mockCfg)
	// reset logger to the MockLogger, since ConfigInitialize overwrites
	// the logger.
	mockCfg.logger = log
	// TODO: set mockCfg.icingaClient as mocked IcingaClient
	return mockCfg
}
