package cli

import (
	"flag"
	"fmt"
	"time"
)

// Options contain the command line options passed to the program.
type Options struct {
	Timer         string
	TimerDuration time.Duration
	Saver         string
	Help          bool
}

// ParseOptions parses the command line options.
func ParseOptions() (*Options, error) {
	var opt Options

	flag.StringVar(&opt.Timer, "timer", "15m", "Timer duration. Screensaver will show after timer runs out.")
	flag.StringVar(&opt.Saver, "saver", "", "The screensaver to show when the timer runs out.")
	flag.Parse()

	err := opt.valid()

	return &opt, err
}

func (opt *Options) valid() error {
	if opt.Saver == "" {
		return fmt.Errorf("no screensaver specified")
	}

	duration, err := time.ParseDuration(opt.Timer)
	if err != nil {
		return fmt.Errorf("cannot parse timer: %w", err)
	}

	opt.TimerDuration = duration

	return nil
}
