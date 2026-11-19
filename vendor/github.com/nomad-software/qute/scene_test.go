package qute_test

import (
	"strings"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// Items built by QGraphicsScene.AddX are tracked: however they die, using
// them afterwards is a clear panic, not a crash.
func TestSceneItemsReportDeath(t *testing.T) {
	qutetest.Run(t, func() {
		r := qtcore.QRectF{Width: 10, Height: 10}

		sc := qtwidgets.NewQGraphicsScene(nil)
		a := sc.AddRect(r)
		b := sc.AddEllipse(r)
		c := sc.AddSimpleText("hi")
		sc.Delete()
		for i, it := range []interface{ Alive() bool }{a, b, c} {
			if it.Alive() {
				t.Errorf("item %d alive after its scene was deleted", i)
			}
		}

		sc2 := qtwidgets.NewQGraphicsScene(nil)
		defer sc2.Delete()
		d := sc2.AddRect(r)
		sc2.Clear()
		if d.Alive() {
			t.Error("item alive after scene.Clear()")
		}

		parent := sc2.AddRect(r)
		child := sc2.AddEllipse(r)
		child.SetParentItem(parent)
		parent.Delete()
		if child.Alive() {
			t.Error("child item alive after its parent item was deleted")
		}

		defer func() {
			msg, _ := recover().(string)
			if !strings.Contains(msg, "object destroyed") {
				t.Errorf("want a dead-handle panic, got %q", msg)
			}
		}()
		a.Rect()
	})
}
