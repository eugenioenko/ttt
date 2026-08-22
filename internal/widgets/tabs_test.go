package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func tabsRowText(s *testSurface, y int) string {
	if y < 0 || y >= len(s.cells) {
		return ""
	}
	runes := make([]rune, len(s.cells[y]))
	for x, c := range s.cells[y] {
		if c.Ch == 0 {
			runes[x] = ' '
		} else {
			runes[x] = c.Ch
		}
	}
	return string(runes)
}

func TestTabsDragCapturesOnlyAfterThreshold(t *testing.T) {
	var from, to = -1, -1
	tw := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "explorer", Label: "Explore", Active: true},
			{ID: "search", Label: "Find"},
			{ID: "changes", Label: "Changes"},
		},
		Reorderable: true,
		OnReorder: func(f, target int) {
			from, to = f, target
		},
	})
	s := renderWidget(tw, 0, 0, 40, 1)
	start := tw.tabSpans[0][0] + 2
	end := tw.tabSpans[2][0] + 2
	if got := tw.HandleEvent(tcell.NewEventMouse(start, 0, tcell.Button1, 0)); got != EventConsumed {
		t.Fatalf("mouse down result = %v, want EventConsumed", got)
	}
	if got := tw.HandleEvent(tcell.NewEventMouse(start+1, 0, tcell.Button1, 0)); got != EventConsumed {
		t.Fatalf("jitter result = %v, want EventConsumed", got)
	}
	if got := tw.HandleEvent(tcell.NewEventMouse(end, 0, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("drag result = %v, want EventCaptured", got)
	}
	tw.Render(s)
	foundIndicator := false
	for _, cell := range s.cells[0] {
		if cell.Ch == '│' && cell.Style == term.StyleBorderActive {
			foundIndicator = true
		}
	}
	if !foundIndicator {
		t.Fatal("drag should render a themed drop indicator")
	}
	tw.HandleEvent(tcell.NewEventMouse(end, 0, tcell.ButtonNone, 0))
	if from != 0 || to != 2 {
		t.Fatalf("reorder = %d -> %d, want 0 -> 2", from, to)
	}
}

func TestTabsClickJitterActivatesWithoutReorder(t *testing.T) {
	clicked, reordered := -1, false
	tw := NewTabsWidget(TabsConfig{
		Items:       []TabItem{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}},
		Reorderable: true,
		OnTabClick:  func(i int) { clicked = i },
		OnReorder:   func(_, _ int) { reordered = true },
	})
	renderWidget(tw, 0, 0, 30, 1)
	x := tw.tabSpans[0][0] + 2
	tw.HandleEvent(tcell.NewEventMouse(x, 0, tcell.Button1, 0))
	tw.HandleEvent(tcell.NewEventMouse(x+1, 0, tcell.Button1, 0))
	tw.HandleEvent(tcell.NewEventMouse(x+1, 0, tcell.ButtonNone, 0))
	if clicked != 0 {
		t.Fatalf("clicked = %d, want 0", clicked)
	}
	if reordered {
		t.Fatal("one-column jitter reordered a tab")
	}
}

func TestTabsWideLabelsStoreDisplayColumnSpans(t *testing.T) {
	clicked := -1
	tw := NewTabsWidget(TabsConfig{
		Items:      []TabItem{{ID: "search", Label: "検索"}, {ID: "changes", Label: "Changes"}},
		OnTabClick: func(i int) { clicked = i },
	})
	renderWidget(tw, 0, 0, 30, 1)
	if got := tw.tabSpans[1][0]; got != 6 {
		t.Fatalf("second tab starts at column %d, want 6 after two fullwidth runes", got)
	}
	tw.HandleEvent(tcell.NewEventMouse(7, 0, tcell.Button1, 0))
	if clicked != 1 {
		t.Fatalf("wide-label hit test activated %d, want 1", clicked)
	}
}

func TestTabsAllFit(t *testing.T) {
	tw := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "a", Label: "Explorer", Active: true},
			{ID: "b", Label: "Search"},
			{ID: "c", Label: "Changes"},
		},
	})
	s := renderWidget(tw, 0, 0, 40, 1)
	row := tabsRowText(s, 0)
	if !strings.Contains(row, "Explorer") || !strings.Contains(row, "Search") || !strings.Contains(row, "Changes") {
		t.Fatalf("all tabs should be visible, got: %q", row)
	}
	if len(tw.HiddenTabs()) != 0 {
		t.Fatal("no tabs should be hidden")
	}
}

func TestTabsOverflowActiveFirst(t *testing.T) {
	tw := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "a", Label: "Explorer"},
			{ID: "b", Label: "Search"},
			{ID: "c", Label: "Changes"},
			{ID: "d", Label: "Docker", Active: true},
		},
	})
	// Width too narrow for all four tabs
	s := renderWidget(tw, 0, 0, 30, 1)
	row := tabsRowText(s, 0)
	if !strings.Contains(row, "Docker") {
		t.Fatalf("active tab 'Docker' should always be visible, got: %q", row)
	}
	if !strings.Contains(row, "»") {
		t.Fatalf("should have overflow chevron, got: %q", row)
	}
}

func TestTabsOverflowPreservesOrder(t *testing.T) {
	tw := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "a", Label: "Explorer"},
			{ID: "b", Label: "Search"},
			{ID: "c", Label: "Changes", Active: true},
			{ID: "d", Label: "Docker"},
		},
	})
	s := renderWidget(tw, 0, 0, 30, 1)
	row := tabsRowText(s, 0)
	if !strings.Contains(row, "Changes") {
		t.Fatalf("active tab should be visible, got: %q", row)
	}
	// Visible tabs should maintain their original order
	explorerPos := strings.Index(row, "Explorer")
	changesPos := strings.Index(row, "Changes")
	if explorerPos >= 0 && changesPos >= 0 && explorerPos > changesPos {
		t.Fatalf("visible tabs should maintain original order, got: %q", row)
	}
}

func TestTabsOverflowOnlyActiveFits(t *testing.T) {
	tw := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "a", Label: "Explorer"},
			{ID: "b", Label: "Search"},
			{ID: "c", Label: "Docker", Active: true},
		},
	})
	// Very narrow: only active tab + chevron should fit
	s := renderWidget(tw, 0, 0, 12, 1)
	row := tabsRowText(s, 0)
	if !strings.Contains(row, "Docker") {
		t.Fatalf("active tab should be visible even when very narrow, got: %q", row)
	}
	if len(tw.HiddenTabs()) != 2 {
		t.Fatalf("expected 2 hidden tabs, got %d", len(tw.HiddenTabs()))
	}
}

func TestTabsHiddenTabsNotClickable(t *testing.T) {
	tw := NewTabsWidget(TabsConfig{
		Items: []TabItem{
			{ID: "a", Label: "Explorer"},
			{ID: "b", Label: "Search"},
			{ID: "c", Label: "Changes"},
			{ID: "d", Label: "Docker", Active: true},
		},
	})
	renderWidget(tw, 0, 0, 30, 1)
	for _, idx := range tw.HiddenTabs() {
		span := tw.tabSpans[idx]
		if span[0] != 0 || span[1] != 0 {
			t.Fatalf("hidden tab %d should have zero span, got %v", idx, span)
		}
	}
}
