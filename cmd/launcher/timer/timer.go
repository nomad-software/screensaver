package timer

import (
	"time"

	"github.com/nomad-software/screensaver/output"
)

// Timer creates a timer to track when we launch the saver.
type Timer struct {
	ttl      time.Duration
	deadline time.Time
}

// New creates a new timer.
func New(d time.Duration) *Timer {
	output.LaunchInfo("initialising timer")
	return &Timer{
		ttl:      d,
		deadline: time.Now().Add(d),
	}
}

// Reset resets the timer.
func (t *Timer) Reset() {
	output.LaunchInfo("resetting timer")
	t.deadline = time.Now().Add(t.ttl)
}

// Expire makes the timer expire.
func (t *Timer) Expire() {
	output.LaunchInfo("expiring timer")
	t.deadline = time.Now()
}

// Expired returns true if the timer has expired.
func (t *Timer) Expired() bool {
	if time.Now().After(t.deadline) {
		output.LaunchInfo("timer expired")
		return true
	}
	return false
}
