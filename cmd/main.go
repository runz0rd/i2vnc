package main

import (
	"flag"
	"time"

	"github.com/runz0rd/i2vnc"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		scrollSpeed = flag.Int("n", 1, "n int")
		server      = flag.String("s", "localhost", "s string")
		port        = flag.String("p", "5900", "p string")
		pw          = flag.String("pw", "", "pw string")
		settle      = flag.Int("stl", 1, "stl int in ms")
		//todo debug flag
		//todo multivalue flag for remapping keys
	)
	flag.Parse()
	log := logrus.New()
	settleMS := time.Duration(*settle) * time.Millisecond
	remote, err := i2vnc.NewVncRemote(log, *server, *port, *pw, settleMS)
	if err != nil {
		log.WithError(err).Fatalf("Failed connecting to remote.")
	}
	input, err := i2vnc.NewInput(i2vnc.X11, log, uint8(*scrollSpeed), remote)
	if err != nil {
		log.WithError(err).Fatalf("Failed initializing input.")
	}
	if err := input.Grab(); err != nil {
		log.WithError(err).Fatalf("Failed grabbing input.")
	}
	//todo support for remapping keys
}
