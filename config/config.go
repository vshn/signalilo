package config

import (
	"fmt"
	"os"
	"time"

	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/bketelsen/logr"
	"github.com/corvus-ch/logr/buffered"
	log "github.com/corvus-ch/logr/logrus"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type icingaConfig struct {
	URL         string `mapstructure:"url"`
	User        string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	InsecureTLS bool   `mapstructure:"insecure_tls"`
	Debug       bool   `mapstructure:"debug"`
}

type Configuration interface {
	GetConfigFile() string
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

func LoadConfig(configuration Configuration) (*SignaliloConfig, error) {
	l := configuration.GetLogger()
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/signalilo")
	viper.SetConfigFile(configuration.GetConfigFile())
	viper.SetDefault("HeartbeatInterval", 60*time.Second)
	viper.SetDefault("KeepFor", 7*24*time.Hour)
	viper.SetDefault("IcingaConfig.Debug", false)
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}
	config := new(SignaliloConfig)
	viper.Unmarshal(config)
	if err != nil {
		return nil, err
	}
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		l.Infof("Config file change: %v", e.Name)
		viper.Unmarshal(config)
		// Reinitialize logger, so we pick up changes to "log_level"
		configuration.SetLogger(NewLogger(config.LogLevel))
		// Reinitialize icinga client, so we pick up changes to icinga
		// config
		icinga, err := newIcingaClient(config)
		if err != nil {
			l.Errorf("Unable to create new icinga client: %s", err)
		} else {
			configuration.SetIcingaClient(icinga)
		}
	})
	// do first init of Logger and IcingaClient
	configuration.SetLogger(NewLogger(config.LogLevel))
	icinga, err := newIcingaClient(config)
	if err != nil {
		l.Errorf("Unable to create new icinga client: %s", err)
	} else {
		configuration.SetIcingaClient(icinga)
	}
	return config, nil
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
	configFile   string
	config       *SignaliloConfig
	logger       logr.Logger
	icingaClient icinga2.Client
}

func (c MockConfiguration) GetConfigFile() string {
	return c.configFile
}
func (c MockConfiguration) GetConfig() *SignaliloConfig {
	return c.config
}
func (c MockConfiguration) GetLogger() logr.Logger {
	return c.logger
}
func (c MockConfiguration) GetIcingaClient() icinga2.Client {
	return c.icingaClient
}
func (c MockConfiguration) SetConfig(config *SignaliloConfig) {
	c.config = config
}
func (c MockConfiguration) SetLogger(logger logr.Logger) {
	c.logger = logger
}
func (c MockConfiguration) SetIcingaClient(icinga icinga2.Client) {
	c.icingaClient = icinga
}

func NewMockConfiguration(configFile string, verbosity int) Configuration {
	mockCfg := MockConfiguration{}
	mockCfg.configFile = configFile
	signaliloCfg, err := LoadConfig(mockCfg)
	if err != nil {
		fmt.Printf("Error Loading mock config: %v", err)
	}
	mockCfg.config = signaliloCfg
	mockCfg.logger = MockLogger(mockCfg.config.LogLevel)
	// TODO: set mockCfg.icingaClient as mocked IcingaClient
	return mockCfg
}
