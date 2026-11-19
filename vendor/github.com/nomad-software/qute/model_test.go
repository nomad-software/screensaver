package qute_test

import (
	"testing"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// fruitModel is a list model implemented in Go.
type fruitModel struct {
	qtcore.QAbstractListModelBase
	items []string
}

func (m *fruitModel) RowCount(parent *qtcore.QModelIndex) int { return len(m.items) }

func (m *fruitModel) Data(index *qtcore.QModelIndex, role int) *qtcore.QVariant {
	if role != int(qtcore.DisplayRole) || index.Row() >= len(m.items) {
		return nil
	}
	return qtcore.NewQVariantOf(m.items[index.Row()])
}

func (m *fruitModel) add(s string) {
	n := len(m.items)
	m.BeginInsertRows(nil, n, n)
	m.items = append(m.items, s)
	m.EndInsertRows()
}

func TestGoListModel(t *testing.T) {
	qutetest.Run(t, func() {
		view := qtwidgets.NewQListView(nil)
		defer view.Delete()
		m := &fruitModel{items: []string{"apple", "pear"}}
		qtcore.NewQAbstractListModelWith(m, view) // the view owns the model
		view.SetModel(m)
		if got := m.RowCount(nil); got != 2 {
			t.Fatalf("RowCount = %d", got)
		}
		inserted := 0
		m.OnRowsInserted(func(parent *qtcore.QModelIndex, first, last int) { inserted++ })
		m.add("plum")
		if inserted != 1 {
			t.Errorf("rowsInserted fired %d times", inserted)
		}
		idx := m.Index(2, 0)
		if got := m.Data(idx, int(qtcore.DisplayRole)).Value(); got != "plum" {
			t.Errorf("Data = %v", got)
		}
		// Qt calls back into Go for data: ask through the C++ side.
		if got := view.Model().Data(idx).Value(); got != "plum" {
			t.Errorf("Model().Data via C++ = %v", got)
		}
		if view.Model().RowCount() != 3 {
			t.Errorf("C++ RowCount = %d", view.Model().RowCount())
		}
		view.Resize(120, 100)
		qute.Screenshot(view)
	})
}
