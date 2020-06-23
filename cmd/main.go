package main

import (
	"flag"

	"github.com/runz0rd/i2vnc"
	"github.com/sirupsen/logrus"
)

func main() {
	//todo tests!
	var (
		debug = flag.Bool("d", false, "debug mode bool")
		cfile = flag.String("cfile", "~/.config/i2vnc.yaml", "path to the config file string")
	)
	flag.Parse()
	logger := logrus.New()
	if *debug {
		logger.SetLevel(logrus.DebugLevel)
	}

	config, err := i2vnc.LoadConfig(*cfile)
	if err != nil {
		logger.WithField(logrus.FieldKeyFile, *cfile).WithError(err).Fatalf("failed loading configuration")
	}

	remote := i2vnc.NewVncRemote(logger, config)

	input, err := i2vnc.NewX11Input(logger, remote, config)
	if err != nil {
		logger.WithError(err).Fatalf("failed initializing input")
	}
	if err := input.Grab(); err != nil {
		logger.WithError(err).Fatalf("failed grabbing input")
	}
	//todo daemon for app?
	//todo configuration validation
	//todo makefile install and update to rpi instead
	//todo interactive config generate?
	//todo minimal ui?
}
