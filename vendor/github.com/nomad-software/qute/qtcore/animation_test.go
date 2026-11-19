package qtcore_test

import (
	"testing"
	"time"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qutetest"
)

// waitFor runs the event loop until done reports true or about two seconds pass.
func waitFor(done func() bool) bool {
	for range 200 {
		if done() {
			return true
		}
		qutetest.Wait(10)
	}
	return done()
}

func TestPropertyAnimation(t *testing.T) {
	qutetest.Run(t, func() {
		tm := qtcore.NewQTimer(nil)
		a := qtcore.NewQPropertyAnimationWithObject(tm, []byte("interval"), nil)
		if a.TargetObject() != tm.AsQObject() || string(a.PropertyName()) != "interval" {
			t.Errorf("target %v, property %q", a.TargetObject(), a.PropertyName())
		}
		a.SetStartValue(qtcore.NewQVariantOf(10))
		a.SetEndValue(qtcore.NewQVariantOf(110))
		a.SetDuration(40)
		finished := false
		a.OnFinished(func() { finished = true })
		a.Start()
		if a.State() != qtcore.QAbstractAnimationRunning {
			t.Errorf("state %v, want Running", a.State())
		}
		if !waitFor(func() bool { return finished }) {
			t.Fatal("animation did not finish")
		}
		if tm.Interval() != 110 || a.CurrentValue().ToInt() != 110 {
			t.Errorf("interval %d, current %d; want 110", tm.Interval(), a.CurrentValue().ToInt())
		}
		a.Delete()
		tm.Delete()
	})
}

func TestAnimationGroups(t *testing.T) {
	qutetest.Run(t, func() {
		seq := qtcore.NewQSequentialAnimationGroup(nil)
		pause := seq.AddPause(20)
		va := qtcore.NewQVariantAnimation(nil)
		va.SetStartValue(qtcore.NewQVariantOf(0))
		va.SetKeyValueAt(0.5, qtcore.NewQVariantOf(50))
		va.SetEndValue(qtcore.NewQVariantOf(100))
		va.SetDuration(20)
		var values []int
		va.OnValueChanged(func(v *qtcore.QVariant) { values = append(values, v.ToInt()) })
		seq.AddAnimation(va)
		if seq.AnimationCount() != 2 || seq.IndexOfAnimation(va) != 1 || seq.AnimationAt(0) != pause.AsQAbstractAnimation() {
			t.Errorf("sequence: count %d, index %d", seq.AnimationCount(), seq.IndexOfAnimation(va))
		}
		if pause.Group() != seq.AsQAnimationGroup() || pause.Ownership() != qtcore.Parented {
			t.Errorf("pause: group %v, %v", pause.Group(), pause.Ownership())
		}
		if va.KeyValueAt(0.5).ToInt() != 50 {
			t.Errorf("key value at 0.5: %d", va.KeyValueAt(0.5).ToInt())
		}
		extra := seq.InsertPause(0, 5)
		if seq.AnimationCount() != 3 || extra.Duration() != 5 {
			t.Errorf("after InsertPause: count %d, duration %d", seq.AnimationCount(), extra.Duration())
		}

		par := qtcore.NewQParallelAnimationGroup(nil)
		par.AddAnimation(seq)
		par.AddAnimation(qtcore.NewQPauseAnimationWithMsecs(30, nil))
		if par.Duration() != 45 {
			t.Errorf("parallel duration %d, want 45 (the longest child)", par.Duration())
		}
		finished := false
		par.OnFinished(func() { finished = true })
		par.Start()
		if !waitFor(func() bool { return finished }) {
			t.Fatal("group did not finish")
		}
		if seq.CurrentAnimation() != va.AsQAbstractAnimation() {
			t.Errorf("current animation at the end: %v, want the variant animation", seq.CurrentAnimation())
		}
		if len(values) == 0 || values[len(values)-1] != 100 {
			t.Errorf("values %v, want them to end at 100", values)
		}
		par.Clear()
		if seq.Alive() || va.Alive() {
			t.Error("Clear did not delete the group's animations")
		}
		par.Delete()
	})
}

func TestTimeLineAndChronoTimer(t *testing.T) {
	qutetest.Run(t, func() {
		tl := qtcore.NewQTimeLineWithDuration(40, nil)
		tl.SetFrameRange(0, 10)
		tl.SetUpdateInterval(5)
		var frames []int
		tl.OnFrameChanged(func(f int) { frames = append(frames, f) })
		done := false
		tl.OnFinished(func() { done = true })
		tl.Start()
		if !waitFor(func() bool { return done }) {
			t.Fatal("time line did not finish")
		}
		if tl.CurrentFrame() != 10 || len(frames) == 0 || frames[len(frames)-1] != 10 {
			t.Errorf("frame %d, frames %v", tl.CurrentFrame(), frames)
		}
		if tl.FrameForTime(40) != 10 || tl.ValueForTime(20) < 0.45 || tl.ValueForTime(20) > 0.55 {
			t.Errorf("frame for 40: %d, value for 20: %v", tl.FrameForTime(40), tl.ValueForTime(20))
		}
		tl.Delete()

		ct := qtcore.NewQChronoTimerWithDuration(5*time.Millisecond, nil)
		ct.SetSingleShot(true)
		fired := 0
		ct.OnTimeout(func() { fired++ })
		ct.Start()
		if !ct.IsActive() || ct.Interval() != 5*time.Millisecond {
			t.Errorf("active %v, interval %v", ct.IsActive(), ct.Interval())
		}
		waitFor(func() bool { return fired > 0 })
		qutetest.Wait(20)
		if fired != 1 || ct.IsActive() {
			t.Errorf("fired %d times, active %v; want once, then inactive", fired, ct.IsActive())
		}
		ct.Delete()
	})
}
