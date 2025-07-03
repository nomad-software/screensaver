package mutter

import (
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/nomad-software/screensaver/output"
)

// signal is the signal sent on the channel when input is detected.
type signal struct{}

// GetInput checks for keyboard and mouse events and when detected sends a
// signal on the returned channel.
func GetInput() chan signal {
	c := make(chan signal, 256)

	go getInput(c)

	return c
}

// getInput gets the idle duration from mutter and if it's below the specified
// duration, the system is not idle so send a signal to say we have input.
func getInput(c chan signal) {
	for {
		t := getIdleTimeFromMutter()

		if t < time.Second*5 {
			c <- signal{}
		}

		time.Sleep(5 * time.Second)
	}
}

// getIdleTimeFromMutter connects to dbus and checks mutter for the idle
// monitor's idle time. This returns the duration that the system has been idle
// for.
func getIdleTimeFromMutter() time.Duration {
	conn, err := dbus.SessionBus()

	if err != nil {
		output.OnError(err, "couldn't establish connection to session bus")
		return 0
	}

	obj := conn.Object("org.gnome.Mutter.IdleMonitor", "/org/gnome/Mutter/IdleMonitor/Core")

	var idleTime uint64

	err = obj.Call("org.gnome.Mutter.IdleMonitor.GetIdletime", 0).Store(&idleTime)

	if err != nil {
		output.OnError(err, "couldn't get time from mutter idle monitor")
		return 0
	}

	return time.Duration(idleTime) * time.Millisecond
}
