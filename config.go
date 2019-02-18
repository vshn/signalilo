package main

import (
	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/bketelsen/logr"
	log "github.com/corvus-ch/logr/logrus"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
)

type icingaConfig struct {
	URL         string `mapstructure:"url"`
	User        string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	InsecureTLS bool   `mapstructure:"insecure_tls"`
}

type alertManagerConfig struct {
	BearerToken string `mapstructure:"bearer_token"`
}

type SignaliloConfig struct {
	Customer           string             `mapstructure:"customer"`
	ServiceHost        string             `mapstructure:"servicehost"`
	IcingaConfig       icingaConfig       `mapstructure:"icinga_api"`
	GcInterval         int                `mapstructure:"gc_interval"`
	AlertManagerConfig alertManagerConfig `mapstructure:"alertmanager"`
	HeartbeatInterval  int                `mapstructure:"heartbeat_interval"`
	LogLevel           int                `mapstructure:"log_level"`
	Logger             logr.Logger
	IcingaClient       *icinga2.WebClient
}

func LoadConfig(l logr.Logger) *SignaliloConfig {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/signalilo")
	err := viper.ReadInConfig()
	if err != nil {
		l.Errorf("Error reading config file: %v", err)
	}
	config := new(SignaliloConfig)
	viper.Unmarshal(config)
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		l.Infof("Config file change: %v", e.Name)
		viper.Unmarshal(config)
		// Reinitialize logger, so we pick up changes to "log_level"
		config.InitLogger()
		// Reinitialize icinga client, so we pick up changes to icinga
		// config
		config.InitIcingaClient()
	})
	// do first init of Logger and IcingaClient
	config.InitLogger()
	config.InitIcingaClient()
	return config
}

func (c *SignaliloConfig) InitIcingaClient() error {
	l := c.Logger
	client, err := icinga2.New(icinga2.WebClient{
		URL:         c.IcingaConfig.URL,
		Username:    c.IcingaConfig.User,
		Password:    c.IcingaConfig.Password,
		Debug:       false,
		InsecureTLS: c.IcingaConfig.InsecureTLS})
	if err != nil {
		l.Errorf("Error creating Icinga client: %v\n", err)
	} else {
		c.IcingaClient = client
	}
	return err
}

func newLogger(verbosity int) logr.Logger {
	jf := new(logrus.JSONFormatter)
	ll := &logrus.Logger{
		Out:       os.Stdout,
		Formatter: jf,
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}
	return log.New(verbosity, ll)
}

func (c *SignaliloConfig) InitLogger() {
	c.Logger = newLogger(c.LogLevel)
}
