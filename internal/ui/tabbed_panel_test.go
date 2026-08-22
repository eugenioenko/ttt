package ui

import (
	"slices"
	"testing"
)

func TestTabbedPanelMergesSavedAndLatePanelOrder(t *testing.T) {
	tp := NewTabbedPanel()
	tp.InitTabClick()
	tp.SetPanelOrder([]string{"changes", "plugin.todo", "explorer"})
	tp.AddPanel("explorer", "Explore", &mockWidget{})
	tp.AddPanel("search", "Find", &mockWidget{})
	tp.AddPanel("changes", "Changes", &mockWidget{})
	if got := tp.PanelIDs(); !slices.Equal(got, []string{"changes", "explorer", "search"}) {
		t.Fatalf("core panel order = %v", got)
	}
	var reported []string
	tp.OnPanelReorder = func(ids []string) { reported = ids }
	if !tp.MovePanel(0, 1) {
		t.Fatal("MovePanel returned false")
	}
	wantPreference := []string{"explorer", "plugin.todo", "changes", "search"}
	if !slices.Equal(reported, wantPreference) {
		t.Fatalf("reported preference = %v, want %v", reported, wantPreference)
	}
	tp.AddPanel("plugin.todo", "Todo", &mockWidget{})
	if got := tp.PanelIDs(); !slices.Equal(got, wantPreference) {
		t.Fatalf("late plugin panel order = %v, want %v", got, wantPreference)
	}
}

func TestTabbedPanelMovePreservesActiveIdentity(t *testing.T) {
	tp := NewTabbedPanel()
	tp.InitTabClick()
	for _, id := range []string{"explorer", "search", "changes"} {
		tp.AddPanel(id, id, &mockWidget{})
	}
	tp.SetActivePanel("changes")
	if !tp.MovePanel(2, 0) {
		t.Fatal("MovePanel returned false")
	}
	if tp.ActivePanel != "changes" || tp.ActivePanelIndex() != 0 {
		t.Fatalf("active panel = %q at %d", tp.ActivePanel, tp.ActivePanelIndex())
	}
}

func TestTabbedPanelDuplicateAddIsIdempotent(t *testing.T) {
	tp := NewTabbedPanel()
	tp.InitTabClick()
	original := &mockWidget{}
	replacement := &mockWidget{}
	tp.AddPanel("explorer", "Explore", original)
	tp.AddPanel("search", "Find", &mockWidget{})
	tp.SetActivePanel("explorer")

	tp.AddPanel("explorer", "Replacement", replacement)

	if got := tp.PanelIDs(); !slices.Equal(got, []string{"explorer", "search"}) {
		t.Fatalf("panel IDs = %v, want unique original order", got)
	}
	if tp.ActivePanel != "explorer" || tp.ActiveWidget() != original {
		t.Fatal("duplicate add replaced the active panel surface")
	}
	if got := tp.PanelEntries()[0].Title; got != "Explore" {
		t.Fatalf("duplicate add changed title to %q", got)
	}
}

func TestTabbedPanelLateDuplicatePreservesPersistedOrder(t *testing.T) {
	tp := NewTabbedPanel()
	tp.InitTabClick()
	tp.SetPanelOrder([]string{"changes", "plugin.todo", "plugin.todo", "explorer", ""})
	tp.AddPanel("explorer", "Explore", &mockWidget{})
	tp.AddPanel("changes", "Changes", &mockWidget{})
	original := &mockWidget{}
	tp.AddPanel("plugin.todo", "Todo", original)
	tp.SetActivePanel("plugin.todo")

	tp.AddPanel("plugin.todo", "Replacement", &mockWidget{})

	want := []string{"changes", "plugin.todo", "explorer"}
	if got := tp.PanelIDs(); !slices.Equal(got, want) {
		t.Fatalf("late plugin panel order = %v, want %v", got, want)
	}
	if tp.ActivePanel != "plugin.todo" || tp.ActiveWidget() != original {
		t.Fatal("late duplicate changed active plugin identity")
	}
}
