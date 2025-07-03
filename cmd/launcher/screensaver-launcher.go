package main

import (
	"flag"
	"time"

	"github.com/nomad-software/screensaver/cmd/launcher/cli"
	"github.com/nomad-software/screensaver/cmd/launcher/input/mutter"
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

	server, err := server.New(opt.Port)
	output.OnError(err, "server failed")

	activate := server.CreateSignal("activate")
	reset := server.CreateSignal("reset")

	server.Listen()

	input := mutter.GetInput()

	tick := time.Tick(time.Second)
	timer := timer.New(opt.TimerDuration)

	for {
		select {

		case <-input:
			timer.Reset()

		case <-reset:
			timer.Reset()

		case <-activate:
			cli.Launch(opt.Saver)
			timer.Reset()

		case <-tick:
			if timer.Expired() {
				cli.Launch(opt.Saver)
				timer.Reset()
			}
		}
	}
}
