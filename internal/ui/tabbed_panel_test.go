package ui

import (
	"slices"
	"testing"
)

func TestTabbedPanelAppliesSavedOrderAsPanelsArrive(t *testing.T) {
	tp := NewTabbedPanel()
	tp.InitTabClick()
	tp.SetPanelOrder([]string{"changes", "plugin.todo", "explorer"})
	tp.AddPanel("explorer", "Explore", &mockWidget{})
	tp.AddPanel("search", "Find", &mockWidget{})
	tp.AddPanel("changes", "Changes", &mockWidget{})
	if got := tp.PanelIDs(); !slices.Equal(got, []string{"changes", "explorer", "search"}) {
		t.Fatalf("core panel order = %v", got)
	}
	tp.AddPanel("plugin.todo", "Todo", &mockWidget{})
	if got := tp.PanelIDs(); !slices.Equal(got, []string{"changes", "plugin.todo", "explorer", "search"}) {
		t.Fatalf("late plugin panel order = %v", got)
	}
	if tp.ActivePanel != "explorer" {
		t.Fatalf("active panel changed to %q", tp.ActivePanel)
	}
}

func TestTabbedPanelMovePreservesActiveIdentity(t *testing.T) {
	tp := NewTabbedPanel()
	tp.InitTabClick()
	for _, id := range []string{"explorer", "search", "changes"} {
		tp.AddPanel(id, id, &mockWidget{})
	}
	tp.SetActivePanel("changes")
	var reported []string
	tp.OnPanelReorder = func(ids []string) { reported = ids }
	if !tp.MovePanel(2, 0) {
		t.Fatal("MovePanel returned false")
	}
	want := []string{"changes", "explorer", "search"}
	if !slices.Equal(tp.PanelIDs(), want) || !slices.Equal(reported, want) {
		t.Fatalf("order = %v, reported = %v, want %v", tp.PanelIDs(), reported, want)
	}
	if tp.ActivePanel != "changes" || tp.ActivePanelIndex() != 0 {
		t.Fatalf("active panel = %q at %d", tp.ActivePanel, tp.ActivePanelIndex())
	}
}
