package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestNewReviewsWidget(t *testing.T) {
	w := NewReviewsWidget()
	if w == nil {
		t.Fatal("NewReviewsWidget returned nil")
	}
	if !w.Focusable() {
		t.Error("ReviewsWidget should be focusable")
	}
	if w.Input == nil {
		t.Error("ReviewsWidget.Input should not be nil")
	}
}

func TestReviewsWidgetSetComments(t *testing.T) {
	w := NewReviewsWidget()
	comments := []github.PRComment{
		{ID: 1, Body: "Inline comment", User: "alice", Path: "main.go", Line: 10, IsInline: true},
		{ID: 2, Body: "General comment", User: "bob", IsInline: false},
		{ID: 3, Body: "Another inline", User: "alice", Path: "main.go", Line: 20, IsInline: true},
		{ID: 4, Body: "Other file", User: "charlie", Path: "utils.go", Line: 5, IsInline: true},
	}
	w.SetComments(comments)

	if len(w.Comments) != 4 {
		t.Fatalf("expected 4 comments, got %d", len(w.Comments))
	}

	// Verify items were built: should have headers, comments, and spacers.
	// Expected structure:
	//   header: "main.go"
	//   comment: inline #1
	//   comment: inline #3
	//   spacer
	//   header: "utils.go"
	//   comment: inline #4
	//   spacer
	//   header: "General"
	//   comment: general #2
	//   spacer
	if len(w.items) != 10 {
		t.Fatalf("expected 10 items, got %d", len(w.items))
	}

	// First item should be a header for "main.go".
	if w.items[0].kind != reviewItemHeader {
		t.Errorf("item 0: expected header, got %d", w.items[0].kind)
	}

	// Items 1 and 2 should be comments.
	if w.items[1].kind != reviewItemComment {
		t.Errorf("item 1: expected comment, got %d", w.items[1].kind)
	}
	if w.items[2].kind != reviewItemComment {
		t.Errorf("item 2: expected comment, got %d", w.items[2].kind)
	}

	// Item 3 should be a spacer.
	if w.items[3].kind != reviewItemSpacer {
		t.Errorf("item 3: expected spacer, got %d", w.items[3].kind)
	}
}

func TestReviewsWidgetEmptyRender(t *testing.T) {
	w := NewReviewsWidget()
	w.SetRect(Rect{X: 0, Y: 0, W: 40, H: 10})

	grid := makeGrid(40, 10)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 10})
	w.Render(surface)

	// Should render "No comments" in the first row.
	var text []rune
	for x := 0; x < 40; x++ {
		c := grid[0][x]
		if c.Ch != '.' && c.Ch != ' ' {
			text = append(text, c.Ch)
		}
	}
	got := string(text)
	if got != "Nocomments" {
		t.Errorf("expected 'Nocomments' text (without spaces), got %q", got)
	}
}

func TestReviewsWidgetRenderWithComments(t *testing.T) {
	w := NewReviewsWidget()
	w.SetRect(Rect{X: 0, Y: 0, W: 60, H: 15})
	w.SetComments([]github.PRComment{
		{ID: 1, Body: "Great work!", User: "alice", CreatedAt: "2025-01-15T10:30:00Z", Path: "main.go", Line: 10, IsInline: true},
		{ID: 2, Body: "Needs docs", User: "bob", CreatedAt: "2025-01-16T08:00:00Z", IsInline: false},
	})

	grid := makeGrid(60, 15)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 60, H: 15})
	w.Render(surface)

	// The first row should contain the file header "main.go".
	// Extract text by checking style (header uses StyleMuted, default grid is StyleDefault with '.')
	var row0 []rune
	for x := 0; x < 60; x++ {
		c := grid[0][x]
		if c.Style != term.StyleDefault || c.Ch != '.' {
			if c.Ch != ' ' {
				row0 = append(row0, c.Ch)
			}
		}
	}
	headerText := string(row0)
	if headerText != "main.go" {
		t.Errorf("expected file header 'main.go', got %q", headerText)
	}
}

func TestReviewsWidgetSetPR(t *testing.T) {
	w := NewReviewsWidget()
	w.SetPR("owner", "repo", 42)
	if w.Owner != "owner" {
		t.Errorf("expected owner 'owner', got %q", w.Owner)
	}
	if w.Repo != "repo" {
		t.Errorf("expected repo 'repo', got %q", w.Repo)
	}
	if w.Number != 42 {
		t.Errorf("expected number 42, got %d", w.Number)
	}
}

func TestReviewsWidgetSelectedStyle(t *testing.T) {
	w := NewReviewsWidget()
	w.SetRect(Rect{X: 0, Y: 0, W: 60, H: 15})
	w.SetComments([]github.PRComment{
		{ID: 1, Body: "Comment 1", User: "alice", CreatedAt: "2025-01-15", Path: "file.go", Line: 1, IsInline: true},
		{ID: 2, Body: "Comment 2", User: "bob", CreatedAt: "2025-01-16", Path: "file.go", Line: 2, IsInline: true},
	})

	// Select the second item (first comment).
	w.Selected = 1

	grid := makeGrid(60, 15)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 60, H: 15})
	w.Render(surface)

	// The selected row (y=1) should use StyleSidebarSelected.
	found := false
	for x := 0; x < 60; x++ {
		c := grid[1][x]
		if c.Style == term.StyleSidebarSelected {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected selected row to use StyleSidebarSelected")
	}
}

func TestReviewsWidgetOnOpenFileCallback(t *testing.T) {
	w := NewReviewsWidget()
	w.SetComments([]github.PRComment{
		{ID: 1, Body: "Review", User: "alice", Path: "main.go", Line: 42, IsInline: true},
	})

	var openedPath string
	var openedLine int
	w.OnOpenFile = func(path string, line int) {
		openedPath = path
		openedLine = line
	}

	// Select the comment (item index 1, after the header at 0).
	w.Selected = 1
	w.activateSelected()

	if openedPath != "main.go" {
		t.Errorf("expected path 'main.go', got %q", openedPath)
	}
	if openedLine != 42 {
		t.Errorf("expected line 42, got %d", openedLine)
	}
}

func TestReviewsWidgetGeneralCommentNoCallback(t *testing.T) {
	w := NewReviewsWidget()
	w.SetComments([]github.PRComment{
		{ID: 1, Body: "General feedback", User: "bob", IsInline: false},
	})

	called := false
	w.OnOpenFile = func(path string, line int) {
		called = true
	}

	// Select the general comment (item index 1, after "General" header at 0).
	w.Selected = 1
	w.activateSelected()

	if called {
		t.Error("OnOpenFile should not be called for general comments")
	}
}

func TestReviewsWidgetLoadingState(t *testing.T) {
	w := NewReviewsWidget()
	w.Loading = true
	w.SetRect(Rect{X: 0, Y: 0, W: 40, H: 10})

	grid := makeGrid(40, 10)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 10})
	w.Render(surface)

	// Should render "Loading..." when Loading=true and no comments.
	// Extract styled text (non-default-styled cells).
	var text []rune
	for x := 0; x < 40; x++ {
		c := grid[0][x]
		if c.Style != term.StyleDefault || c.Ch != '.' {
			if c.Ch != ' ' {
				text = append(text, c.Ch)
			}
		}
	}
	got := string(text)
	if got != "Loading..." {
		t.Errorf("expected 'Loading...' text, got %q", got)
	}
}
