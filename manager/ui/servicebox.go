package ui

import (
	"fmt"
	"html"
	"slices"
	"strconv"
	"time"

	qt "github.com/mappu/miqt/qt6"

	"github.com/nomad-software/screensaver/manager/service"
)

// formSpacing is the room between the labels and fields of the form, and
// between its rows.
const formSpacing = 12

// timerWidth is the least width of the box of the timer, which is otherwise
// only as wide as its longest value.
const timerWidth = 140

// timerSettle is how long the timer is left unchanged before it is saved, so
// that the launcher is not restarted for every step of the spin box.
const timerSettle = 800 * time.Millisecond

// serviceBox shows the launcher's service, with the buttons that install and
// control it, and the timer it waits for.
type serviceBox struct {
	*qt.QWidget
	w         *Window
	state     *qt.QLabel
	login     *qt.QLabel
	stray     *qt.QLabel
	timer     *qt.QSpinBox
	install   *qt.QPushButton
	start     *qt.QPushButton
	stop      *qt.QPushButton
	uninstall *qt.QPushButton
}

// newServiceBox shows the service, as found at the first refresh.
func newServiceBox(w *Window) *serviceBox {
	b := &serviceBox{QWidget: qt.NewQWidget2(), w: w}
	layout := qt.NewQVBoxLayout(b.QWidget)
	note := qt.NewQLabel3("The launcher runs as a systemd user service, " + service.Name +
		", which starts when you log in. It shows the saver once there has been no input for the time below.")
	note.SetWordWrap(true)
	layout.AddWidget(note.QWidget)

	// The state and the timer are a form below the note, its labels lined
	// up on the right and its fields at their own widths.
	layout.AddSpacing(paneMargin)
	form := qt.NewQFormLayout2()
	form.SetFieldGrowthPolicy(qt.QFormLayout__FieldsStayAtSizeHint)
	form.SetLabelAlignment(qt.AlignRight | qt.AlignVCenter)
	form.SetHorizontalSpacing(formSpacing)
	form.SetVerticalSpacing(formSpacing)
	b.state = qt.NewQLabel2()
	b.state.SetTextInteractionFlags(qt.TextSelectableByMouse)
	form.AddRow3("Status:", b.state.QWidget)
	b.login = qt.NewQLabel2()
	form.AddRow3("Starts at login:", b.login.QWidget)
	b.timer = qt.NewQSpinBox2()
	b.timer.SetRange(1, 24*60)
	b.timer.SetSuffix(" minutes")
	b.timer.SetValue(minutes(w.settings.Timer))
	b.timer.SetMinimumWidth(timerWidth)
	form.AddRow3("Show after:", b.timer.QWidget)
	layout.AddLayout(form.QLayout)

	settle := qt.NewQTimer2(b.QObject)
	settle.SetSingleShot(true)
	settle.OnTimeout(func() {
		w.settings.Timer = strconv.Itoa(b.timer.Value()) + "m"
		w.saveAndReconfigure()
	})
	b.timer.OnValueChanged(func(int) { settle.Start(int(timerSettle / time.Millisecond)) })

	// A warning stands out in a box of its own below the form.
	layout.AddSpacing(paneMargin)
	b.stray = qt.NewQLabel2()
	b.stray.SetWordWrap(true)
	b.stray.SetStyleSheet("QLabel { color: " + colourFailed + "; border: 1px solid " + colourFailed +
		"; border-radius: 4px; padding: 10px; }")
	b.stray.Hide()
	layout.AddWidget(b.stray.QWidget)
	layout.AddStretch()

	// The buttons are at their own sizes at the bottom right, as on the
	// Screensavers tab.
	buttons := qt.NewQHBoxLayout2()
	buttons.AddStretch()
	b.install = qt.NewQPushButton3("Install")
	b.install.SetToolTip("Writes the service, with the launcher found now, and starts it with every login and now.")
	b.install.OnClicked(b.installService)
	b.start = qt.NewQPushButton3("Start")
	b.start.OnClicked(func() { w.inBackground(w.manager.Start) })
	b.stop = qt.NewQPushButton3("Stop")
	b.stop.SetToolTip("Stops the launcher until it is started again, or the next login.")
	b.stop.OnClicked(func() { w.inBackground(w.manager.Stop) })
	b.uninstall = qt.NewQPushButton3("Uninstall")
	b.uninstall.SetToolTip("Stops the launcher and removes the service, so that it no longer starts at login.")
	b.uninstall.OnClicked(b.uninstallService)
	for _, button := range []*qt.QPushButton{b.install, b.start, b.stop, b.uninstall} {
		buttons.AddWidget(button.QWidget)
	}
	layout.AddLayout(buttons.QLayout)
	b.show(state{})
	return b
}

// launcherTimer is the timer the launcher has when it is given none, which
// is shown until one is chosen.
const launcherTimer = 15 * time.Minute

// minutes returns the timer in whole minutes, at least one, for a duration
// that cannot be shown as minutes (as one written by hand might be), or the
// launcher's own for none.
func minutes(timer string) int {
	d, err := time.ParseDuration(timer)
	if err != nil {
		d = launcherTimer
	}
	return max(1, int((d+time.Minute-1)/time.Minute))
}

// installService writes and starts the service, for the launcher installed.
func (b *serviceBox) installService() {
	// The launcher will not start without a saver to show.
	if b.w.settings.Saver == "" {
		warning(b.w.QWidget, "Start Up", "Choose a saver first, with Use This Saver on the Screensavers tab.")
		return
	}
	path, ok := b.w.launcherPath()
	if !ok {
		return
	}
	args := b.w.settings.LauncherArgs()
	b.w.inBackground(func() error { return b.w.manager.Install(path, args) })
}

// uninstallService stops and removes the service, once the user agrees.
func (b *serviceBox) uninstallService() {
	if !confirm(b.w.QWidget, "Start Up", "Stop the launcher and remove the service? No saver will be shown until it is installed again.") {
		return
	}
	b.w.inBackground(b.w.manager.Uninstall)
}

// show shows the state of the service, and of a launcher running apart
// from it.
func (b *serviceBox) show(s state) {
	st := s.status
	var colour, text string
	switch {
	case s.statusErr != nil:
		colour, text = colourFailed, "Unknown: "+s.statusErr.Error()
	case !st.Installed:
		colour, text = colourOff, "Not installed"
	case st.Running():
		colour, text = colourOn, fmt.Sprintf("Running (process %d)", st.PID)
	case st.State == "failed":
		colour, text = colourFailed, "Failed; see journalctl --user -u "+service.Name
	case st.State == "activating" && st.Sub == "auto-restart":
		colour, text = colourWaiting, "Restarting after it stopped; see journalctl --user -u "+service.Name
	case st.State == "activating":
		colour, text = colourWaiting, "Starting"
	case st.State == "deactivating":
		colour, text = colourWaiting, "Stopping"
	default:
		colour, text = colourOff, "Stopped"
	}
	b.state.SetText(dot(colour) + " " + html.EscapeString(text))
	switch {
	case !st.Installed:
		b.login.SetText("No")
	case st.Enabled:
		b.login.SetText("Yes")
	default:
		b.login.SetText("No (the service is disabled)")
	}

	// A launcher started some other way, as by a script or a startup
	// application, takes the port the service's needs, which then cannot
	// start, and shows a saver of its own.
	var stray []int
	for _, pid := range s.running[launcher] {
		if pid != st.PID {
			stray = append(stray, pid)
		}
	}
	slices.Sort(stray)
	if len(stray) > 0 {
		b.stray.SetText(fmt.Sprintf("A launcher not started by the service is running (process %s), "+
			"perhaps from a script or a startup application. Stop it and remove it from startup, "+
			"as it shows its own saver and keeps the service's launcher from starting.", pidList(stray)))
		b.stray.Show()
	} else {
		b.stray.Hide()
	}

	b.install.SetText("Install")
	if st.Installed {
		b.install.SetText("Reinstall")
	}
	active := st.State == "active" || st.State == "activating"
	b.start.SetEnabled(st.Installed && !active)
	b.stop.SetEnabled(st.Installed && active)
	b.uninstall.SetEnabled(st.Installed)
}

// pidList writes the ids of processes as a list.
func pidList(pids []int) string {
	text := ""
	for i, pid := range pids {
		if i > 0 {
			text += ", "
		}
		text += strconv.Itoa(pid)
	}
	return text
}
