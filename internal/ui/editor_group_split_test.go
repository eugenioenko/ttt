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
