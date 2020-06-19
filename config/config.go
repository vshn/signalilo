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
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/bketelsen/logr"
	"github.com/corvus-ch/logr/buffered"
	log "github.com/corvus-ch/logr/logrus"
	"github.com/sirupsen/logrus"
	"github.com/vshn/go-icinga2-client/icinga2"
)

type icingaConfig struct {
	URL               string
	User              string
	Password          string
	InsecureTLS       bool
	DisableKeepAlives bool
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
	BearerToken string
	TLSCertPath string
	TLSKeyPath  string
	UseTLS      bool
}

type SignaliloConfig struct {
	UUID               string
	HostName           string
	IcingaConfig       icingaConfig
	GcInterval         time.Duration
	AlertManagerConfig alertManagerConfig
	HeartbeatInterval  time.Duration
	LogLevel           int
	KeepFor            time.Duration
	CAData             string
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
	if c.CAData != "" {
		rootCAs, err = makeCertPool(c, l)
		if err != nil {
			return nil, err
		}
	}

	client, err := icinga2.New(icinga2.WebClient{
		URL:               c.IcingaConfig.URL,
		Username:          c.IcingaConfig.User,
		Password:          c.IcingaConfig.Password,
		Debug:             c.IcingaConfig.Debug,
		InsecureTLS:       c.IcingaConfig.InsecureTLS,
		DisableKeepAlives: c.IcingaConfig.DisableKeepAlives,
		RootCAs:           rootCAs})
	if err != nil {
		return nil, err
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

func (c MockConfiguration) GetConfig() *SignaliloConfig {
	return &c.config
}
func (c MockConfiguration) GetLogger() logr.Logger {
	return c.logger
}
func (c MockConfiguration) GetIcingaClient() icinga2.Client {
	return c.icingaClient
}
func (c MockConfiguration) SetConfig(config SignaliloConfig) {
	c.config = config
}
func (c MockConfiguration) SetLogger(logger logr.Logger) {
	c.logger = logger
}
func (c MockConfiguration) SetIcingaClient(icinga icinga2.Client) {
	c.icingaClient = icinga
}

func NewMockConfiguration(verbosity int) Configuration {
	// TODO: fill out defaults for MockConfiguration, maybe move default
	// from serve.go to here
	signaliloCfg := SignaliloConfig{
		UUID:     "",
		HostName: "signalilo_appuio_lab",
		IcingaConfig: icingaConfig{
			URL:               "localhost:5665",
			User:              "sepp",
			Password:          "sepp1",
			InsecureTLS:       true,
                        DisableKeepAlives: false,
			Debug:             false,
		},
		GcInterval: 1 * time.Minute,
		AlertManagerConfig: alertManagerConfig{
			BearerToken: "aaaaaa",
		},
		HeartbeatInterval: 1 * time.Minute,
		LogLevel:          2,
		KeepFor:           5 * time.Minute,
		CAData:            "",
	}
	mockCfg := MockConfiguration{
		config: signaliloCfg,
	}
	mockCfg.logger = MockLogger(mockCfg.config.LogLevel)
	ConfigInitialize(mockCfg)
	// TODO: set mockCfg.icingaClient as mocked IcingaClient
	return mockCfg
}
