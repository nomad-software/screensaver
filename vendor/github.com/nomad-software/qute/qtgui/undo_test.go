package qtgui_test

import (
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qutetest"
)

// setValue is an undoable assignment; consecutive ones to the same target merge.
type setValue struct {
	qtgui.QUndoCommandBase
	target   *int
	from, to int
}

func (c *setValue) Redo()   { *c.target = c.to }
func (c *setValue) Undo()   { *c.target = c.from }
func (c *setValue) Id() int { return 1 }
func (c *setValue) MergeWith(other *qtgui.QUndoCommand) bool {
	o, ok := other.GoValue().(*setValue)
	if !ok || o.target != c.target {
		return false
	}
	c.to = o.to
	return true
}

func newSet(target *int, to int) *qtgui.QUndoCommand {
	cmd := qtgui.NewQUndoCommandWith(&setValue{target: target, from: *target, to: to}, nil)
	cmd.SetText("set")
	return cmd
}

func TestUndoStack(t *testing.T) {
	qutetest.Run(t, func() {
		v := 0
		stack := qtgui.NewQUndoStack(nil)
		first := newSet(&v, 1)
		stack.Push(first)
		if v != 1 || stack.Count() != 1 || !stack.CanUndo() || stack.UndoText() != "set" || first.Ownership() != qtcore.Parented {
			t.Fatalf("after push: v %d, count %d, %v", v, stack.Count(), first.Ownership())
		}
		second := newSet(&v, 2)
		stack.Push(second) // merged into the first and deleted
		if v != 2 || stack.Count() != 1 || second.Alive() {
			t.Errorf("after merge: v %d, count %d, second alive %v", v, stack.Count(), second.Alive())
		}
		if stack.Command(0) != first {
			t.Error("Command(0) is not the first command")
		}

		stack.SetClean()
		cleanChanges := 0
		stack.OnCleanChanged(func(bool) { cleanChanges++ })
		stack.Undo()
		if v != 0 || stack.IsClean() || !stack.CanRedo() || cleanChanges != 1 {
			t.Errorf("after undo: v %d, clean %v, changes %d", v, stack.IsClean(), cleanChanges)
		}
		stack.Redo()
		if v != 2 || !stack.IsClean() {
			t.Errorf("after redo: v %d", v)
		}

		// a macro is one entry; a parent command owns its children
		stack.BeginMacro("macro")
		stack.Push(qtgui.NewQUndoCommandWith(&setValue{target: new(int), to: 5}, nil))
		stack.Push(qtgui.NewQUndoCommandWith(&setValue{target: new(int), to: 6}, nil))
		stack.EndMacro()
		if stack.Count() != 2 || stack.Command(1).ChildCount() != 2 || stack.Text(1) != "macro" {
			t.Errorf("macro: count %d, children %d", stack.Count(), stack.Command(1).ChildCount())
		}
		parent := qtgui.NewQUndoCommandWithText("parent", nil)
		child := qtgui.NewQUndoCommandWith(&setValue{target: new(int), to: 1}, parent)
		if parent.ChildCount() != 1 || parent.Child(0) != child || child.Ownership() != qtcore.Parented {
			t.Errorf("child: %d children, %v", parent.ChildCount(), child.Ownership())
		}
		parent.Delete()
		if child.Alive() {
			t.Error("child command outlived its parent")
		}

		undo := stack.CreateUndoAction(nil)
		if undo.Ownership() != qtcore.Owned || !undo.IsEnabled() {
			t.Errorf("undo action: %v, enabled %v", undo.Ownership(), undo.IsEnabled())
		}
		holder := qtgui.NewQAction(nil)
		redo := stack.CreateRedoAction(holder)
		if redo.Ownership() != qtcore.Parented {
			t.Errorf("redo action with a parent: %v", redo.Ownership())
		}
		undo.Trigger() // the macro
		undo.Trigger() // the merged assignment
		if v != 0 || stack.Index() != 0 {
			t.Errorf("undo action: v %d, index %d", v, stack.Index())
		}
		undo.Delete()
		holder.Delete()

		stack.Delete()
		if first.Alive() {
			t.Error("command outlived its stack")
		}
	})
}

func TestUndoGroup(t *testing.T) {
	qutetest.Run(t, func() {
		v := 0
		g := qtgui.NewQUndoGroup(nil)
		a, b := qtgui.NewQUndoStack(nil), qtgui.NewQUndoStack(nil)
		g.AddStack(a)
		g.AddStack(b)
		var active *qtgui.QUndoStack
		g.OnActiveStackChanged(func(s *qtgui.QUndoStack) { active = s })
		g.SetActiveStack(b)
		if active != b || g.ActiveStack() != b || len(g.Stacks()) != 2 {
			t.Errorf("active %v, stacks %d", active, len(g.Stacks()))
		}
		b.Push(newSet(&v, 3))
		if !g.CanUndo() || g.UndoText() != "set" {
			t.Error("group does not follow its active stack")
		}
		g.Undo()
		if v != 0 {
			t.Errorf("group undo: v %d", v)
		}
		redo := g.CreateRedoAction(nil)
		redo.Trigger()
		if v != 3 {
			t.Errorf("group redo action: v %d", v)
		}
		redo.Delete()
		g.RemoveStack(a)
		if len(g.Stacks()) != 1 {
			t.Errorf("after RemoveStack: %d stacks", len(g.Stacks()))
		}
		g.Delete()
		if !a.Alive() || !b.Alive() {
			t.Error("the group deleted its stacks")
		}
		a.Delete()
		b.Delete()
	})
}
