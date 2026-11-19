package qtmultimedia_test

import (
	"testing"
	"unsafe"

	"github.com/nomad-software/qute/internal/rt"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtmultimedia"
	"github.com/nomad-software/qute/qutetest"
)

// A player does not own its audio output or video sink; with no Go variable
// left, only the player's kept references hold them through a collection.
func TestPlayerKeepsOutputsThroughGC(t *testing.T) {
	var player *qtmultimedia.QMediaPlayer
	qutetest.Run(t, func() {
		player = qtmultimedia.NewQMediaPlayer(nil)
		player.SetAudioOutput(qtmultimedia.NewQAudioOutput(nil))
		player.SetVideoSink(qtmultimedia.NewQVideoSink(nil))
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		if out := player.AudioOutput(); out == nil || !out.Alive() {
			t.Error("audio output freed while the player uses it")
		}
		if sink := player.VideoSink(); sink == nil || !sink.Alive() {
			t.Error("video sink freed while the player uses it")
		}
		player.Delete()
	})
}

// ~QVideoSink drops its own connections before ~QObject emits destroyed; the
// runtime's death notice must survive that, or the handle would dangle.
func TestVideoSinkDeathSeen(t *testing.T) {
	qutetest.Run(t, func() {
		root := qtcore.NewQObject(nil)
		sink := qtmultimedia.NewQVideoSink(root)
		root.Delete()
		if sink.Alive() {
			t.Error("video sink handle alive after its parent deleted it")
		}
	})
}

// Video frames hold pixel memory outside the Go heap; each frame counts it.
func TestVideoFrameMemoryCounted(t *testing.T) {
	qutetest.Run(t, func() {
		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 100, Height: 100}, qtgui.QImageFormat_ARGB32)
		before := rt.MemoryLive()
		f := qtmultimedia.NewQVideoFrameWithImage(img)
		if got := rt.MemoryLive() - before; got != 100*100*4 {
			t.Errorf("a 100x100 frame counts %d bytes, want %d", got, 100*100*4)
		}
		f.Delete()
		if got := rt.MemoryLive() - before; got != 0 {
			t.Errorf("%d bytes still counted after Delete", got)
		}
	})
}

// SetVideoOutput and SetVideoSink replace each other in Qt, so they share one
// kept reference: the sink set first is no longer held.
func TestVideoOutputReplacesSink(t *testing.T) {
	qutetest.Run(t, func() {
		player := qtmultimedia.NewQMediaPlayer(nil)
		first := qtmultimedia.NewQVideoSink(nil)
		second := qtmultimedia.NewQVideoSink(nil)
		player.SetVideoSink(first)
		player.SetVideoOutput(second)
		if rt.KeepsArg(unsafe.Pointer(player), unsafe.Pointer(first)) {
			t.Error("the replaced sink is still kept")
		}
		if !rt.KeepsArg(unsafe.Pointer(player), unsafe.Pointer(second)) {
			t.Error("the new output is not kept")
		}
		player.Delete()
		first.Delete()
		second.Delete()
	})
}

// A sink hands each frame to its handler, with the frame's real size and
// pixels; the frame argument is valid only inside the handler.
func TestVideoSinkDeliversFrames(t *testing.T) {
	qutetest.Run(t, func() {
		sink := qtmultimedia.NewQVideoSink(nil)
		var sizes []qtcore.QSize
		var centre qtgui.QColor
		sink.OnVideoFrameChanged(func(f *qtmultimedia.QVideoFrame) {
			sizes = append(sizes, f.Size())
			centre = f.ToImage().PixelColor(f.Size().Width/2, f.Size().Height/2)
		})
		for _, w := range []int{16, 32} {
			img := qtgui.NewQImageWithSize(qtcore.QSize{Width: w, Height: 8}, qtgui.QImageFormat_ARGB32)
			img.FillColor(qtgui.RGB(0, 200, 0))
			sink.SetVideoFrame(qtmultimedia.NewQVideoFrameWithImage(img))
		}
		qutetest.Wait(10)
		if len(sizes) != 2 || sizes[0].Width != 16 || sizes[1].Width != 32 {
			t.Errorf("frames seen %v, want widths 16 and 32", sizes)
		}
		if centre.G < 190 || centre.R > 10 {
			t.Errorf("frame pixel %v, want green", centre)
		}
		if got := sink.VideoSize(); got.Width != 32 || got.Height != 8 {
			t.Errorf("VideoSize = %v, want 32x8", got)
		}
		sink.Delete()
	})
}

// A capture session's video output and sink share one kept slot too.
func TestCaptureSessionVideoOutputReplacesSink(t *testing.T) {
	qutetest.Run(t, func() {
		session := qtmultimedia.NewQMediaCaptureSession(nil)
		first := qtmultimedia.NewQVideoSink(nil)
		second := qtmultimedia.NewQVideoSink(nil)
		session.SetVideoSink(first)
		session.SetVideoOutput(second)
		if rt.KeepsArg(unsafe.Pointer(session), unsafe.Pointer(first)) {
			t.Error("the replaced sink is still kept")
		}
		if !rt.KeepsArg(unsafe.Pointer(session), unsafe.Pointer(second)) {
			t.Error("the new output is not kept")
		}
		session.Delete()
		first.Delete()
		second.Delete()
	})
}
