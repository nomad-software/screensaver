package output

import (
	"fmt"
	"os"
)

// ScreenInfo prints information about a screensaver.
func ScreenInfo(format string, args ...any) {
	Info("screensaver: "+format, args...)
}

// ScreenErr prints an error about a screensaver.
func ScreenErr(format string, args ...any) {
	Error("screensaver: "+format, args...)
}

// LaunchInfo prints information about the launcher.
func LaunchInfo(format string, args ...any) {
	Info("launcher: "+format, args...)
}

// LaunchErr prints an error about a screensaver.
func LaunchErr(format string, args ...any) {
	Error("launcher: "+format, args...)
}

// OnError prints an error if err is not nil and exits the program.
func OnError(err error, text string) {
	if err != nil {
		Fatal("error: %s: %s", text, err.Error())
	}
}

// Info prints information.
func Info(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format+"\n", args...)
}

// Error prints an error.
func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Fatal prints an error and exits the program.
func Fatal(format string, args ...any) {
	Error(format, args...)
	os.Exit(1)
}
