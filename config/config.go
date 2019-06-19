package config

import (
	"os"
	"time"

	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/bketelsen/logr"
	"github.com/corvus-ch/logr/buffered"
	log "github.com/corvus-ch/logr/logrus"
	"github.com/sirupsen/logrus"
)

type icingaConfig struct {
	URL         string `mapstructure:"url"`
	User        string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	InsecureTLS bool   `mapstructure:"insecure_tls"`
	Debug       bool   `mapstructure:"debug"`
}

type Configuration interface {
	GetConfig() *SignaliloConfig

	GetLogger() logr.Logger
	SetLogger(logger logr.Logger)

	GetIcingaClient() icinga2.Client
	SetIcingaClient(icinga icinga2.Client)
}

type alertManagerConfig struct {
	BearerToken string `mapstructure:"bearer_token"`
}

type SignaliloConfig struct {
	UUID               string             `mapstructure:"uuid"`
	HostName           string             `mapstructure:"host_name"`
	IcingaConfig       icingaConfig       `mapstructure:"icinga_api"`
	GcInterval         time.Duration      `mapstructure:"gc_interval"`
	AlertManagerConfig alertManagerConfig `mapstructure:"alertmanager"`
	HeartbeatInterval  time.Duration      `mapstructure:"heartbeat_interval"`
	LogLevel           int                `mapstructure:"log_level"`
	KeepFor            time.Duration      `mapstructure:"keep_for"`
}

func ConfigInitialize(configuration Configuration) {
	l := configuration.GetLogger()
	config := configuration.GetConfig()

	// do first init of Logger and IcingaClient
	configuration.SetLogger(NewLogger(config.LogLevel))
	icinga, err := newIcingaClient(config)
	if err != nil {
		l.Errorf("Unable to create new icinga client: %s", err)
	} else {
		configuration.SetIcingaClient(icinga)
	}
}

func newIcingaClient(c *SignaliloConfig) (icinga2.Client, error) {
	client, err := icinga2.New(icinga2.WebClient{
		URL:         c.IcingaConfig.URL,
		Username:    c.IcingaConfig.User,
		Password:    c.IcingaConfig.Password,
		Debug:       c.IcingaConfig.Debug,
		InsecureTLS: c.IcingaConfig.InsecureTLS})
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
			URL:         "localhost:5665",
			User:        "sepp",
			Password:    "sepp1",
			InsecureTLS: true,
			Debug:       false,
		},
		GcInterval: 1 * time.Minute,
		AlertManagerConfig: alertManagerConfig{
			BearerToken: "aaaaaa",
		},
		HeartbeatInterval: 1 * time.Minute,
		LogLevel:          2,
		KeepFor:           5 * time.Minute,
	}
	mockCfg := MockConfiguration{
		config: signaliloCfg,
	}
	ConfigInitialize(mockCfg)
	mockCfg.logger = MockLogger(mockCfg.config.LogLevel)
	// TODO: set mockCfg.icingaClient as mocked IcingaClient
	return mockCfg
}
