package main

import (
	"flag"
	"time"

	"github.com/nomad-software/screensaver/cmd/launcher/cli"
	"github.com/nomad-software/screensaver/cmd/launcher/input"
	"github.com/nomad-software/screensaver/cmd/launcher/server"
	"github.com/nomad-software/screensaver/cmd/launcher/timer"
	"github.com/nomad-software/screensaver/output"
)

func main() {
	opt, err := cli.ParseOptions()
	if err != nil {
		flag.PrintDefaults()
		output.OnError(err, "options are not valid")
	}

	reset := make(chan input.Signal)
	activate := make(chan input.Signal)

	server, err := server.New(opt.Port)
	output.OnError(err, "server failed to run")
	server.RegisterCommandSignal("activate", activate)
	server.RegisterCommandSignal("reset", reset)
	go server.Listen()

	go input.GetXInput(reset)

	timer := timer.New(opt.TimerDuration)
	tick := time.Tick(time.Second)

	for {
		select {

		case <-reset:
			timer.Reset()

		case <-activate:
			timer.Expire()

		case <-tick:
			if timer.Expired() {
				err := cli.Launch(opt.Saver)
				if err != nil {
					output.LaunchErr("screensaver error: %s", err)
				}
				timer.Reset()
			}
		}
	}
}
