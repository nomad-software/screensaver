package qute_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// The application event filter re-anchors a subtree's Go state on every
// reparenting, walking the subtree. These measure that walk: moving a
// populated subtree between two roots, and building a tree by adding
// populated subtrees, at a few sizes.
func BenchmarkReparentSubtree(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			qutetest.Run(b, func() {
				a, c := qtcore.NewQObject(nil), qtcore.NewQObject(nil)
				sub := qtcore.NewQObject(a)
				for i := 0; i < n; i++ {
					o := qtcore.NewQObject(sub)
					o.OnObjectNameChanged(func(string) {}) // anchored Go state
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if i%2 == 0 {
						sub.SetParent(c)
					} else {
						sub.SetParent(a)
					}
				}
				b.StopTimer()
				a.Delete()
				c.Delete()
			})
		})
	}
}

func BenchmarkBuildTreeFromSubtrees(b *testing.B) {
	for _, n := range []int{10, 100} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			qutetest.Run(b, func() {
				for i := 0; i < b.N; i++ {
					root := qtcore.NewQObject(nil)
					for j := 0; j < n; j++ {
						sub := qtcore.NewQObject(nil)
						for k := 0; k < 10; k++ {
							qtcore.NewQObject(sub).OnObjectNameChanged(func(string) {})
						}
						sub.SetParent(root)
					}
					root.Delete()
				}
			})
		})
	}
}

// benchModel is a Go table model of the shape an application has: Qt asks it,
// through its overrides, for counts, flags and data, each call with an index.
type benchModel struct {
	qtcore.QAbstractTableModelBase
	rows int
	text *qtcore.QVariant
}

func (m *benchModel) RowCount(parent *qtcore.QModelIndex) int {
	if parent.IsValid() {
		return 0
	}
	return m.rows
}

func (m *benchModel) ColumnCount(parent *qtcore.QModelIndex) int {
	if parent.IsValid() {
		return 0
	}
	return 4
}

func (m *benchModel) Flags(index *qtcore.QModelIndex) qtcore.ItemFlags {
	return qtcore.ItemIsEnabled | qtcore.ItemIsSelectable
}

func (m *benchModel) Data(index *qtcore.QModelIndex, role int) *qtcore.QVariant {
	if role != int(qtcore.DisplayRole) {
		return nil
	}
	return m.text
}

func withBenchView(b *testing.B, rows int, f func(v *qtwidgets.QTableView, m *benchModel)) {
	qutetest.Run(b, func() {
		v := qtwidgets.NewQTableView(nil)
		defer v.Delete()
		m := &benchModel{rows: rows, text: qtcore.NewQVariantOf("cell")}
		qtcore.NewQAbstractTableModelWith(m, v)
		v.SetModel(m)
		v.SetSelectionBehavior(qtwidgets.QAbstractItemViewSelectRows)
		v.Resize(800, 600)
		f(v, m)
	})
}

// BenchmarkModelSelectAll selects every row of a 100,000-row Go model and
// asks the selection about each row, which has Qt call the model's overrides
// (RowCount, ColumnCount, Flags) several times a row.
func BenchmarkModelSelectAll(b *testing.B) {
	withBenchView(b, 100000, func(v *qtwidgets.QTableView, m *benchModel) {
		for b.Loop() {
			v.SelectAll()
			sm := v.SelectionModel()
			n := 0
			for i := range m.rows {
				if sm.RowIntersectsSelection(i) {
					n++
				}
			}
			if n != m.rows {
				b.Fatalf("%d selected", n)
			}
			v.ClearSelection()
		}
	})
}

// BenchmarkModelPaint draws a view of a Go model: Qt asks Data in each role
// for every visible cell.
func BenchmarkModelPaint(b *testing.B) {
	withBenchView(b, 100000, func(v *qtwidgets.QTableView, m *benchModel) {
		for b.Loop() {
			runtime.KeepAlive(v.Viewport().Grab())
		}
	})
}

// BenchmarkListRows fills a list with rows the way an application shows a
// file list (an item with text, an icon and a tooltip per row), then clears
// it: the per-object cost of making many small objects and seeing them die.
func BenchmarkListRows(b *testing.B) {
	for _, added := range []bool{false, true} {
		name := "parented"
		if added {
			name = "added" // made Owned, then handed to the list
		}
		b.Run(name, func(b *testing.B) {
			qutetest.Run(b, func() {
				list := qtwidgets.NewQListWidget(nil)
				defer list.Delete()
				pm := qtgui.NewQPixmapWithW(16, 16)
				pm.Fill()
				icon := qtgui.NewQIconWithPixmap(pm)
				b.ReportAllocs()
				for b.Loop() {
					for i := range 1200 {
						if added {
							it := qtwidgets.NewQListWidgetItemWithIcon(icon, "file.go", nil)
							it.SetToolTip(fmt.Sprint("path/to/file", i))
							list.AddItem(it)
						} else {
							it := qtwidgets.NewQListWidgetItemWithIcon(icon, "file.go", list)
							it.SetToolTip(fmt.Sprint("path/to/file", i))
						}
					}
					list.Clear()
				}
			})
		})
	}
}

// BenchmarkWidgetRows makes a row of widgets per entry (a container, a layout
// and two labels) under one parent, then deletes the parent: the per-object
// cost of QObjects, whose death Qt reports.
func BenchmarkWidgetRows(b *testing.B) {
	qutetest.Run(b, func() {
		b.ReportAllocs()
		for b.Loop() {
			top := qtwidgets.NewQWidget(nil)
			lay := qtwidgets.NewQVBoxLayout(top)
			for range 300 {
				row := qtwidgets.NewQWidget(nil)
				rl := qtwidgets.NewQHBoxLayout(row)
				rl.AddWidget(qtwidgets.NewQLabelWithText("M", nil))
				rl.AddWidget(qtwidgets.NewQLabelWithText("file.go", nil))
				lay.AddWidget(row)
			}
			top.Delete()
		}
	})
}

// BenchmarkPainter makes, ends and frees a painter: on an image, and in a
// widget's paint event (with an empty paint event for the cost of the event
// itself), with NewQPainterOn/End/Delete and with the scoped PaintOn.
func BenchmarkPainter(b *testing.B) {
	for _, way := range []string{"new", "paint-on"} {
		b.Run("image/"+way, func(b *testing.B) {
			qutetest.Run(b, func() {
				img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 64, Height: 64}, qtgui.QImageFormat_ARGB32_Premultiplied)
				b.ReportAllocs()
				for b.Loop() {
					if way == "new" {
						p := qtgui.NewQPainterOn(img)
						p.End()
						p.Delete()
					} else {
						qtgui.PaintOn(img, func(*qtgui.QPainter) {})
					}
				}
			})
		})
	}
	for _, way := range []string{"none", "new", "paint-on"} {
		b.Run("widget/"+way, func(b *testing.B) {
			qutetest.Run(b, func() {
				w := qtwidgets.NewQWidgetWith(&benchCanvas{way: way}, nil)
				defer w.Delete()
				w.Resize(200, 100)
				w.Show()
				img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 200, Height: 100}, qtgui.QImageFormat_ARGB32_Premultiplied)
				b.ReportAllocs()
				for b.Loop() {
					w.Render(img)
				}
			})
		})
	}
}

// benchCanvas paints in the way named: not at all, or with a painter.
type benchCanvas struct {
	qtwidgets.QWidgetBase
	way string
}

func (c *benchCanvas) PaintEvent(e *qtgui.QPaintEvent) {
	switch c.way {
	case "new":
		p := qtgui.NewQPainterOn(c)
		defer p.Delete()
		defer p.End()
	case "paint-on":
		qtgui.PaintOn(c, func(*qtgui.QPainter) {})
	}
}

// BenchmarkTreeRows fills a tree the way a process list does (an item of
// eight cells per row, each cell aligned), then clears it.
func BenchmarkTreeRows(b *testing.B) {
	qutetest.Run(b, func() {
		tree := qtwidgets.NewQTreeWidget(nil)
		defer tree.Delete()
		tree.SetColumnCount(8)
		cells := []string{"1234", "name", "user", "1.0", "2.0", "3 MiB", "S", "cmd --flag"}
		b.ReportAllocs()
		for b.Loop() {
			tree.Clear()
			for range 400 {
				it := qtwidgets.NewQTreeWidgetItemWithStrings(cells)
				for col := range 8 {
					it.SetTextAlignment(col, qtcore.AlignRight)
				}
				tree.AddTopLevelItem(it)
			}
		}
	})
}
