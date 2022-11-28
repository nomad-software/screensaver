package main

import (
	"flag"
	"time"

	"github.com/nomad-software/screensaver/cli"
	"github.com/nomad-software/screensaver/input"
	"github.com/nomad-software/screensaver/output"
	"github.com/nomad-software/screensaver/timer"
)

func main() {
	opt, err := cli.ParseOptions()

	if err != nil {
		flag.PrintDefaults()
		output.OnError(err, "options are not valid")
	}

	timer.Init(opt.TimerDuration)

	tick := time.Tick(time.Second)
	xinput := make(chan input.Signal)

	go input.GetXInput(xinput)

	for {
		select {

		case <-xinput:
			timer.Reset()

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
