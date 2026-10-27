package wayland

//go:generate wayland-scanner client-header ext-idle-notify-v1.xml ext-idle-notify-v1-client-protocol.h
//go:generate wayland-scanner private-code ext-idle-notify-v1.xml ext-idle-notify-v1-protocol.c

/*
#cgo LDFLAGS: -lwayland-client

#include <string.h>
#include <time.h>
#include <wayland-client.h>
#include "ext-idle-notify-v1-client-protocol.h"

static struct wl_display* display;
static struct wl_seat* seat;
static struct ext_idle_notifier_v1* notifier;
static int idle;

static void registry_global(void* data, struct wl_registry* registry, uint32_t name, const char* interface, uint32_t version) {
	if (strcmp(interface, wl_seat_interface.name) == 0 && seat == NULL) {
		seat = wl_registry_bind(registry, name, &wl_seat_interface, 1);
	} else if (strcmp(interface, ext_idle_notifier_v1_interface.name) == 0) {
		notifier = wl_registry_bind(registry, name, &ext_idle_notifier_v1_interface, 1);
	}
}

static void registry_global_remove(void* data, struct wl_registry* registry, uint32_t name) {
}

static const struct wl_registry_listener registry_listener = {
	.global = registry_global,
	.global_remove = registry_global_remove,
};

static void notification_idled(void* data, struct ext_idle_notification_v1* notification) {
	idle = 1;
}

static void notification_resumed(void* data, struct ext_idle_notification_v1* notification) {
	idle = 0;
}

static const struct ext_idle_notification_v1_listener notification_listener = {
	.idled = notification_idled,
	.resumed = notification_resumed,
};

// idle_connect connects to the compositor and requests a notification when
// the seat has been idle for the timeout (in milliseconds). Returns -1 if the
// display can't be reached and -2 if the protocol isn't supported.
int idle_connect(uint32_t timeout) {
	display = wl_display_connect(NULL);
	if (display == NULL) {
		return -1;
	}

	struct wl_registry* registry = wl_display_get_registry(display);
	wl_registry_add_listener(registry, &registry_listener, NULL);
	wl_display_roundtrip(display);

	if (seat == NULL || notifier == NULL) {
		wl_display_disconnect(display);
		return -2;
	}

	struct ext_idle_notification_v1* notification = ext_idle_notifier_v1_get_idle_notification(notifier, timeout, seat);
	ext_idle_notification_v1_add_listener(notification, &notification_listener, NULL);
	wl_display_flush(display);

	return 0;
}

// idle_dispatch waits up to the timeout (in milliseconds) for events from the
// compositor and dispatches them. Returns -1 on error.
int idle_dispatch(int timeout) {
	struct timespec ts = {
		.tv_sec = timeout / 1000,
		.tv_nsec = (timeout % 1000) * 1000000,
	};
	return wl_display_dispatch_timeout(display, &ts);
}

int idle_is_idle() {
	return idle;
}
*/
import "C"
import (
	"time"

	"github.com/nomad-software/screensaver/output"
)

// signal is the signal sent on the channel when input is detected.
type signal = struct{}

// GetInput checks for keyboard and mouse events and when detected sends a
// signal on the returned channel.
func GetInput() chan signal {
	c := make(chan signal, 256)

	go getInput(c)

	return c
}

// getInput asks the compositor to notify us when the seat has been idle for
// the specified duration. While the seat is not idle there has been input
// within that duration, so send a signal to say we have input.
func getInput(c chan signal) {
	timeout := C.int((5 * time.Second).Milliseconds())

	switch C.idle_connect(C.uint32_t(timeout)) {
	case -1:
		output.Fatal("couldn't connect to wayland display")
	case -2:
		output.Fatal("wayland compositor doesn't support the ext-idle-notify protocol")
	}

	for {
		if C.idle_dispatch(timeout) == -1 {
			output.Fatal("couldn't dispatch wayland events")
		}

		if C.idle_is_idle() == 0 {
			c <- signal{}
		}
	}
}
