package main

import (
	"flag"

	"github.com/runz0rd/i2vnc"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		debug   = flag.Bool("d", false, "debug mode")
		cfile   = flag.String("cfile", "~/.config/i2vnc.yaml", "path to the config file")
		forever = flag.Bool("forever", false, "run forever")
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

	input, err := i2vnc.NewX11Input(logger, remote, config, *forever)
	if err != nil {
		logger.WithError(err).Fatalf("failed initializing input")
	}
	if err := input.Grab(); err != nil {
		logger.WithError(err).Fatalf("failed grabbing input")
	}
}
