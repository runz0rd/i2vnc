package main

import (
	"flag"

	"github.com/runz0rd/i2vnc"
	"github.com/sirupsen/logrus"
)

func main() {
	//todo tests!
	var (
		pw    = flag.String("p", "", "vnc server password string")
		debug = flag.Bool("d", false, "debug mode bool")
		cfile = flag.String("cfile", "~/.config/i2vnc.yaml", "path to the config file string")
		cname = flag.String("cname", "default", "name of the config to use string")
	)
	flag.Parse()
	logger := logrus.New()
	if *debug {
		logger.SetLevel(logrus.DebugLevel)
	}

	config, err := i2vnc.NewConfig(*cfile, *cname)
	if err != nil {
		logger.WithField(logrus.FieldKeyFile, *cfile).WithError(err).Fatalf("Failed loading configuration")
	}
	remote, err := i2vnc.NewVncRemote(logger, config, *pw)
	if err != nil {
		logger.WithError(err).Fatalf("Failed connecting to remote.")
	}
	input, err := i2vnc.NewX11Input(logger, remote, config)
	if err != nil {
		logger.WithError(err).Fatalf("Failed initializing input.")
	}
	if err := input.Grab(); err != nil {
		logger.WithError(err).Fatalf("Failed grabbing input.")
	}
}
