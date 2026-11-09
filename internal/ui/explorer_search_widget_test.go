package ui

import (
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestExplorerSearchDebouncesAndCancelsStaleDelivery(t *testing.T) {
	tree := widgets.NewTreeWidget(widgets.TreeConfig{Items: []*widgets.TreeNode{{ID: "a", Label: "alpha.go"}, {ID: "b", Label: "beta.go"}}})
	s := NewExplorerSearchWidget(tree, "")
	var runs, deliveries []func()
	s.Debounce = asynctask.New[struct{}](asynctask.TaskOptions{
		Delay: time.Hour,
		Run:   func(fn func()) { runs = append(runs, fn) },
	}, func(fn func()) error { deliveries = append(deliveries, fn); return nil })
	t.Cleanup(s.Debounce.Close)
	finish := func() {
		t.Helper()
		s.Debounce.Flush()
		if len(runs) != 1 {
			t.Fatalf("scheduled runs = %d, want 1", len(runs))
		}
		runs[0]()
		runs = nil
		for _, deliver := range deliveries {
			deliver()
		}
		deliveries = nil
	}
	s.ShowSearch()
	s.Input.SetText("alpha")
	s.Input.SetText("beta")
	if len(runs) != 0 || tree.ItemCount() != 2 {
		t.Fatal("filter ran on a keystroke before debounce")
	}
	finish()
	if tree.ItemCount() != 1 || tree.Selected().ID != "b" {
		t.Fatal("latest debounced query was not applied")
	}
	s.Input.SetText("alpha")
	s.Debounce.Flush()
	runs[0]()
	runs = nil
	s.Input.SetText("missing")
	for _, deliver := range deliveries {
		deliver()
	}
	deliveries = nil
	if tree.ItemCount() != 1 || tree.Selected().ID != "b" {
		t.Fatal("stale queued result applied")
	}
	finish()
	if tree.ItemCount() != 0 {
		t.Fatal("no-match query retained old rows")
	}
	s.Input.SetText("alpha")
	s.Debounce.Flush()
	runs[0]()
	runs = nil
	s.HideSearch()
	for _, deliver := range deliveries {
		deliver()
	}
	if s.Debounce.Pending() || tree.ItemCount() != 2 || s.visible {
		t.Fatal("closing did not immediately clear or reject stale results")
	}
}

func TestExplorerSearchEnterAppliesPendingQueryBeforeOpening(t *testing.T) {
	opened := ""
	tree := widgets.NewTreeWidget(widgets.TreeConfig{Items: []*widgets.TreeNode{{ID: "a", Label: "alpha"}, {ID: "b", Label: "beta"}}, OnCommand: func(_ string, node *widgets.TreeNode) { opened = node.ID }})
	s := NewExplorerSearchWidget(tree, "")
	s.Debounce = asynctask.New[struct{}](asynctask.TaskOptions{Delay: time.Hour}, func(func()) error { t.Fatal("pending debounce should be cancelled"); return nil })
	t.Cleanup(s.Debounce.Close)
	s.ShowSearch()
	s.Input.SetText("beta")
	s.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if opened != "b" || s.Debounce.Pending() {
		t.Fatalf("Enter opened %q, pending = %v", opened, s.Debounce.Pending())
	}
}

func TestExplorerSearchNavigationSkipsAncestorsAndEmptyResults(t *testing.T) {
	opened := ""
	tree := widgets.NewTreeWidget(widgets.TreeConfig{Items: []*widgets.TreeNode{
		{Label: "first", Children: []*widgets.TreeNode{{ID: "a", Label: "日本.go"}}},
		{Label: "second", Children: []*widgets.TreeNode{{ID: "b", Label: "日本.txt"}}},
	}, OnCommand: func(_ string, node *widgets.TreeNode) { opened = node.ID }})
	s := NewExplorerSearchWidget(tree, "")
	s.ShowSearch()
	s.Input.SetText("日本")
	key := func(key tcell.Key) { s.HandleEvent(tcell.NewEventKey(key, "", tcell.ModNone)) }
	if tree.Selected().ID != "a" {
		t.Fatal("first match not selected")
	}
	key(tcell.KeyDown)
	if tree.Selected().ID != "b" {
		t.Fatal("Down did not skip ancestor")
	}
	key(tcell.KeyUp)
	if tree.Selected().ID != "a" {
		t.Fatal("Up did not skip ancestor")
	}
	key(tcell.KeyEnter)
	if opened != "a" {
		t.Fatal("Enter did not open Unicode match")
	}
	s.Input.SetText("absent")
	opened = ""
	key(tcell.KeyDown)
	key(tcell.KeyUp)
	key(tcell.KeyEnter)
	if opened != "" || tree.Selected() != nil {
		t.Fatal("empty search activated a stale selection")
	}
}
