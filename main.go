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
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	Version   = "undefined"
	BuildDate = "Now"
)

func main() {
	app := kingpin.New("signalilo", "Signalilo takes in Alertmanager alerts through a webhook, translates them into Icinga2 services and posts them to Icinga using the Icinga API").Version(Version)
	configureServeCommand(app)

	fmt.Printf("Signalilo %v\n", Version)
	fmt.Printf("Build time: %v\n\n", BuildDate)

	kingpin.MustParse(app.Parse(os.Args[1:]))
}
