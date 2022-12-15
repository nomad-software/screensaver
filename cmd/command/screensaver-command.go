package main

import (
	"github.com/nomad-software/screensaver/cmd/command/cli"
	"github.com/nomad-software/screensaver/cmd/command/client"
	"github.com/nomad-software/screensaver/output"
)

func main() {
	opt := cli.ParseOptions()

	client, err := client.New(opt.Port)
	output.OnError(err, "failed to create client")

	switch true {
	case opt.Activate:
		client.Send("activate")
	case opt.Reset:
		client.Send("reset")
	default:
		output.Error("no command specified")
	}
}
