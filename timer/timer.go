package timer

import (
	"time"

	"github.com/nomad-software/screensaver/output"
)

var (
	ttl      = 15 * time.Minute
	deadline = time.Now().Add(ttl)
)

// Init initalises the timer.
func Init(d time.Duration) {
	output.LaunchInfo("initialising timer")
	ttl = d
	deadline = time.Now().Add(ttl)
}

// Reset resets the timer.
func Reset() {
	output.LaunchInfo("resetting timer")
	deadline = time.Now().Add(ttl)
}

// Expired returns true if the timer has expired, false if not.
func Expired() bool {
	if time.Now().After(deadline) {
		output.LaunchInfo("timer expired")
		return true
	}
	return false
}
