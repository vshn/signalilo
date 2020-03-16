module git.vshn.net/appuio/signalilo

require (
	github.com/Nexinto/go-icinga2-client v0.0.0-20180829072643-d4f6001a2110
	github.com/bketelsen/logr v0.0.0-20170116012416-f3d070bdd1c5
	github.com/corvus-ch/logr v0.0.0-20180917163152-45217966b77e
	github.com/prometheus/alertmanager v0.20.0
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.5.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace github.com/Nexinto/go-icinga2-client => github.com/vshn/go-icinga2-client v0.0.5

go 1.13
