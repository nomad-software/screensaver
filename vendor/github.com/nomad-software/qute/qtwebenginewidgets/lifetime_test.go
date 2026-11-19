package qtwebenginewidgets_test

import (
	"testing"
	"unsafe"

	"github.com/nomad-software/qute/internal/rt"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtwebenginecore"
	"github.com/nomad-software/qute/qtwebenginewidgets"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// Qt only accepts custom schemes registered before the application exists;
// init runs before TestMain creates it, on the main thread.
func init() {
	s := qtwebenginecore.NewQWebEngineUrlSchemeWithBytes([]byte("qute"))
	s.SetSyntax(qtwebenginecore.QWebEngineUrlSchemeSyntaxPath)
	qtwebenginecore.QWebEngineUrlSchemeRegisterScheme(s)
}

// replyHandler answers every request with a page read from a parentless
// buffer, which only the job's kept reference holds.
type replyHandler struct {
	qtwebenginecore.QWebEngineUrlSchemeHandlerBase
	kept bool
}

func (h *replyHandler) RequestStarted(job *qtwebenginecore.QWebEngineUrlRequestJob) {
	buf := qtcore.NewQBuffer(nil)
	buf.SetData([]byte("<html><body>hello from go</body></html>"))
	buf.Open(qtcore.QIODeviceBaseReadOnly)
	job.Reply([]byte("text/html"), buf)
	h.kept = rt.KeepsArg(unsafe.Pointer(job), unsafe.Pointer(buf))
}

// QWebEngineUrlRequestJob::reply reads the device after the handler returns:
// the job's keep rule must hold it through a garbage collection.
func TestSchemeReplyDeviceSurvivesGC(t *testing.T) {
	var view *qtwebenginewidgets.QWebEngineView
	var handler *qtwebenginecore.QWebEngineUrlSchemeHandler
	var prof *qtwebenginecore.QWebEngineProfile
	h := &replyHandler{}
	loaded, ok := false, false
	qutetest.Run(t, func() {
		prof = qtwebenginecore.NewQWebEngineProfile(nil)
		handler = qtwebenginecore.NewQWebEngineUrlSchemeHandlerWith(h, nil)
		prof.InstallUrlSchemeHandler([]byte("qute"), handler)
		view = qtwebenginewidgets.NewQWebEngineViewWithProfile(prof, nil)
		view.OnLoadFinished(func(success bool) { loaded, ok = true, success })
		view.SetUrl(qtcore.NewQUrlWithUrl("qute:page"))
		for i := 0; i < 100 && !h.kept; i++ {
			qutetest.Wait(20)
		}
	})
	qutetest.CollectGarbage() // the buffer has no Go variable left
	var text string
	qutetest.Run(t, func() {
		for i := 0; i < 250 && !loaded; i++ {
			qutetest.Wait(20)
		}
		if !h.kept {
			t.Error("the job did not keep the reply device")
		}
		if !loaded || !ok {
			t.Errorf("page load finished=%v ok=%v, want true true", loaded, ok)
		}
		done := false
		view.Page().ToPlainText(func(s string) { text, done = s, true })
		for i := 0; i < 100 && !done; i++ {
			qutetest.Wait(20)
		}
		if text != "hello from go" {
			t.Errorf("page text %q, want %q", text, "hello from go")
		}
		view.Delete()
		handler.Delete()
		prof.Delete()
	})
}

// A window whose page keeps a parentless profile is collected together with
// it. Qt requires the profile to outlive the page, so the cleanup queue must
// free the window (and with it the page) before the profile, whatever order
// the GC queued them in.
func TestPageAndProfileCollectedTogether(t *testing.T) {
	before := qutetest.Freed()
	qutetest.Run(t, func() {
		for i := 0; i < 3; i++ {
			win := qtwidgets.NewQWidget(nil)
			prof := qtwebenginecore.NewQWebEngineProfile(nil)
			page := qtwebenginecore.NewQWebEnginePageWithProfile(prof, win)
			qtwebenginewidgets.NewQWebEngineViewWithPage(page, win)
		}
	})
	qutetest.CollectGarbage()
	if got := qutetest.Freed() - before; got < 6 {
		t.Errorf("freed %d objects, want at least 6 (3 windows, 3 profiles)", got)
	}
	qutetest.Run(t, func() { qutetest.Wait(50) }) // let Chromium finish tearing down
}
