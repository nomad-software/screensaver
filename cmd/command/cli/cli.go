package cli

import (
	"flag"
)

// Options contain the command line options passed to the program.
type Options struct {
	Port     string
	Activate bool
	Reset    bool
}

// ParseOptions parses the command line options.
func ParseOptions() *Options {
	var opt Options

	flag.StringVar(&opt.Port, "port", "1337", "The communication port that the launcher is listening on.")
	flag.BoolVar(&opt.Activate, "activate", false, "Send an activate command to the launcher.")
	flag.BoolVar(&opt.Reset, "reset", false, "Send a reset command to the launcher.")
	flag.Parse()

	return &opt
}
