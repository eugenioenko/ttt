package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestMergeGroupsKeepsEditedTabsAfterAnUntouchedPlaceholder(t *testing.T) {
	source := NewEditorGroupWidget(nil, 4, true, "relative")
	target := NewEditorGroupWidget(nil, 4, true, "relative")
	source.NewFile()
	source.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", 0))
	draft := source.ActiveBuffer()
	target.MergeTabsFrom(source)
	if target.TabCount() != 1 || target.ActiveBuffer() != draft || !draft.Dirty || draft.Lines[0] != "x" {
		t.Fatal("merging a group skipped the edited document behind its empty placeholder")
	}
}

func TestMergeGroupsPreservesPinnedTargetPlaceholder(t *testing.T) {
	source := NewEditorGroupWidget(nil, 4, true, "relative")
	target := NewEditorGroupWidget(nil, 4, true, "relative")
	target.TogglePinTab()
	pinned := target.ActiveBuffer()
	source.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", 0))
	target.MergeTabsFrom(source)
	if target.TabCount() != 2 || target.tabs[0].Buf != pinned || target.pinnedCount != 1 {
		t.Fatal("merging discarded the pinned target placeholder")
	}
}

type closeCountContent struct {
	BinaryFileWidget
	closed int
}

func (w *closeCountContent) Close() { w.closed++ }

func TestContentViewsReleaseEachWidgetAndNotifyLastClose(t *testing.T) {
	left := NewEditorGroupWidget(nil, 4, true, "relative")
	right := NewEditorGroupWidget(nil, 4, true, "relative")
	groups := func() []*EditorGroupWidget { return []*EditorGroupWidget{left, right} }
	first, second := &closeCountContent{}, &closeCountContent{}
	closed := 0
	for _, group := range groups() {
		group.RelatedGroups = groups
		group.OnContentTabClose = func(string) { closed++ }
	}
	left.OpenPluginTab("handled", "Handled", first)
	right.OpenPluginTab("handled", "Handled", second)
	left.CloseTab()
	if first.closed != 1 || closed != 0 {
		t.Fatal("closing one view released the document or retained its widget")
	}
	right.CloseTab()
	if second.closed != 1 || closed != 1 {
		t.Fatal("last view did not release its widget and document exactly once")
	}
}

func TestMovedTabsKeepStateAndReceiveUniqueDestinationIDs(t *testing.T) {
	source := NewEditorGroupWidget(nil, 4, true, "compact")
	target := NewEditorGroupWidget(nil, 4, true, "compact")
	source.NewFileFromText("source")
	target.NewFileFromText("target")
	source.TogglePinTab()
	buf, stack, cur := source.ActiveBuffer(), source.Editor.Undo, source.Editor.Cursor
	source.MoveActiveTabTo(target)
	if target.ActiveBuffer() != buf || target.Editor.Undo != stack || target.Editor.Cursor != cur || !target.IsActiveTabPinned() {
		t.Fatal("moving the tab lost its document, view or pin")
	}
	if target.TabCount() != 2 || target.tabs[0].ID == target.tabs[1].ID {
		t.Fatal("moved tabs have conflicting destination identities")
	}
}

func TestMovingPinnedViewPinsExistingDestinationView(t *testing.T) {
	source, target := sharedTestGroups(t)
	source.TogglePinTab()
	buf := target.ActiveBuffer()
	source.MoveActiveTabTo(target)
	if target.TabCount() != 1 || target.ActiveBuffer() != buf || !target.IsActiveTabPinned() {
		t.Fatal("deduplicating a moved view lost its document or pin")
	}
}
