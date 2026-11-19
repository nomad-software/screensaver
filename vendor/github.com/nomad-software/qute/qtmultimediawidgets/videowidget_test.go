package qtmultimediawidgets_test

import (
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtmultimedia"
	"github.com/nomad-software/qute/qtmultimediawidgets"
	"github.com/nomad-software/qute/qutetest"
)

// A video widget's own sink takes frames, and the widget's sink handle is
// the widget's: it dies with the widget.
func TestVideoWidgetSink(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtmultimediawidgets.NewQVideoWidget(nil)
		w.Resize(64, 32)
		sink := w.VideoSink()
		frames := 0
		sink.OnVideoFrameChanged(func(*qtmultimedia.QVideoFrame) { frames++ })
		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 64, Height: 32}, qtgui.QImageFormat_ARGB32)
		img.FillColor(qtgui.RGB(0, 0, 255))
		sink.SetVideoFrame(qtmultimedia.NewQVideoFrameWithImage(img))
		qutetest.Wait(10)
		if frames != 1 {
			t.Errorf("handler saw %d frames, want 1", frames)
		}
		if got := sink.VideoSize(); got.Width != 64 || got.Height != 32 {
			t.Errorf("VideoSize = %v, want 64x32", got)
		}
		w.Delete()
		if sink.Alive() {
			t.Error("the widget's sink outlived the widget")
		}
	})
}

// A player keeps the video widget it plays into through a collection.
func TestPlayerKeepsVideoWidget(t *testing.T) {
	var player *qtmultimedia.QMediaPlayer
	qutetest.Run(t, func() {
		player = qtmultimedia.NewQMediaPlayer(nil)
		player.SetVideoOutput(qtmultimediawidgets.NewQVideoWidget(nil))
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		out := player.VideoOutput()
		if out == nil || !out.Alive() {
			t.Fatal("the video widget was freed while the player uses it")
		}
		if qtmultimediawidgets.QVideoWidgetCast(out) == nil {
			t.Error("the video output is not a QVideoWidget")
		}
		out.Delete()
		player.Delete()
	})
}
