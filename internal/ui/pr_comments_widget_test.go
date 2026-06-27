package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/github"
	"github.com/gdamore/tcell/v2"
)

func TestPRCommentsWidgetEmpty(t *testing.T) {
	w := NewPRCommentsWidget()
	if !w.Focusable() {
		t.Error("PRCommentsWidget should be focusable")
	}
	// Empty widget should have one item (the input row)
	w.SetComments(nil)
	if len(w.items) != 1 {
		t.Errorf("expected 1 item (input row) with no comments, got %d", len(w.items))
	}
}

func TestPRCommentsWidgetSetComments(t *testing.T) {
	w := NewPRCommentsWidget()
	comments := []github.PRComment{
		{ID: 1, Body: "LGTM", User: "alice", IsInline: false},
		{ID: 2, Body: "Fix typo", User: "bob", Path: "main.go", Line: 10, IsInline: true},
		{ID: 3, Body: "Needs test", User: "carol", Path: "main.go", Line: 20, IsInline: true},
		{ID: 4, Body: "Good pattern", User: "dave", Path: "lib.go", Line: 5, IsInline: true},
	}
	w.SetComments(comments)

	// Expected items:
	// 1. "General Comments" section header
	// 2. LGTM comment
	// 3. "main.go" section header
	// 4. Fix typo comment
	// 5. Needs test comment
	// 6. "lib.go" section header
	// 7. Good pattern comment
	// 8. Input row
	if len(w.items) != 8 {
		t.Fatalf("expected 8 items, got %d", len(w.items))
	}

	if w.items[0].kind != commentItemSection {
		t.Error("first item should be a section header")
	}
	if w.items[0].label != "General Comments" {
		t.Errorf("expected 'General Comments', got %q", w.items[0].label)
	}
	if w.items[1].kind != commentItemComment {
		t.Error("second item should be a comment")
	}
	if w.items[2].kind != commentItemSection {
		t.Error("third item should be a section header")
	}
	if w.items[2].label != "main.go" {
		t.Errorf("expected 'main.go', got %q", w.items[2].label)
	}
	if w.items[7].kind != commentItemInputRow {
		t.Error("last item should be input row")
	}
}

func TestPRCommentsWidgetOnlyInline(t *testing.T) {
	w := NewPRCommentsWidget()
	comments := []github.PRComment{
		{ID: 1, Body: "Review comment", User: "alice", Path: "foo.go", Line: 5, IsInline: true},
	}
	w.SetComments(comments)

	// Expected: section "foo.go", comment, input
	if len(w.items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(w.items))
	}
	if w.items[0].kind != commentItemSection || w.items[0].label != "foo.go" {
		t.Error("expected foo.go section header")
	}
}

func TestPRCommentsWidgetOnlyGeneral(t *testing.T) {
	w := NewPRCommentsWidget()
	comments := []github.PRComment{
		{ID: 1, Body: "Looks good", User: "alice", IsInline: false},
		{ID: 2, Body: "Ship it", User: "bob", IsInline: false},
	}
	w.SetComments(comments)

	// Expected: section "General Comments", 2 comments, input
	if len(w.items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(w.items))
	}
}

func TestPRCommentsWidgetKeyNavigation(t *testing.T) {
	w := NewPRCommentsWidget()
	w.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})
	comments := []github.PRComment{
		{ID: 1, Body: "General", User: "alice", IsInline: false},
		{ID: 2, Body: "Inline", User: "bob", Path: "a.go", Line: 1, IsInline: true},
	}
	w.SetComments(comments)

	// Navigate down
	res := w.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if res != EventConsumed {
		t.Error("Down should be consumed")
	}
	if w.selected != 1 {
		t.Errorf("expected selected=1, got %d", w.selected)
	}

	// Navigate up
	res = w.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, 0))
	if res != EventConsumed {
		t.Error("Up should be consumed")
	}
	if w.selected != 0 {
		t.Errorf("expected selected=0, got %d", w.selected)
	}

	// Up at top stays at 0
	w.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, 0))
	if w.selected != 0 {
		t.Errorf("expected selected=0 at top, got %d", w.selected)
	}
}

func TestPRCommentsWidgetActivateInlineComment(t *testing.T) {
	w := NewPRCommentsWidget()
	w.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})
	comments := []github.PRComment{
		{ID: 1, Body: "Fix this", User: "alice", Path: "app.go", Line: 42, IsInline: true},
	}
	w.SetComments(comments)

	var openedPath string
	var openedLine int
	w.OnOpenFile = func(path string, line int) {
		openedPath = path
		openedLine = line
	}

	// Select the comment (index 1, after section header)
	w.selected = 1
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if openedPath != "app.go" {
		t.Errorf("expected path 'app.go', got %q", openedPath)
	}
	if openedLine != 42 {
		t.Errorf("expected line 42, got %d", openedLine)
	}
}

func TestPRCommentsWidgetActivateInputRow(t *testing.T) {
	w := NewPRCommentsWidget()
	w.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})
	comments := []github.PRComment{
		{ID: 1, Body: "Comment", User: "alice", IsInline: false},
	}
	w.SetComments(comments)

	// Select the input row (last item)
	w.selected = len(w.items) - 1
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if !w.inputFocused {
		t.Error("input should be focused after activating input row")
	}
}

func TestPRCommentsWidgetInputSubmit(t *testing.T) {
	w := NewPRCommentsWidget()
	w.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})
	w.SetComments(nil) // just the input row

	var submitted string
	w.OnSubmitComment = func(body string) {
		submitted = body
	}

	// Focus input
	w.selected = 0 // input row
	w.inputFocused = true

	// Type some text
	w.Input.Text = "Great PR!"
	w.Input.CursorPos = 9

	// Press Enter to submit
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if submitted != "Great PR!" {
		t.Errorf("expected submitted 'Great PR!', got %q", submitted)
	}
	if w.Input.Text != "" {
		t.Error("input should be cleared after submit")
	}
}

func TestPRCommentsWidgetInputEscape(t *testing.T) {
	w := NewPRCommentsWidget()
	w.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})
	w.SetComments(nil)
	w.inputFocused = true

	w.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, 0, 0))

	if w.inputFocused {
		t.Error("input should be unfocused after Escape")
	}
}

func TestPRCommentsWidgetCursorPosition(t *testing.T) {
	w := NewPRCommentsWidget()
	w.SetRect(Rect{X: 5, Y: 10, W: 80, H: 24})
	w.SetComments(nil) // just input row

	// Not focused - no cursor
	_, _, visible := w.CursorPosition()
	if visible {
		t.Error("cursor should not be visible when input not focused")
	}

	// Focus input
	w.inputFocused = true
	_, _, visible = w.CursorPosition()
	if !visible {
		t.Error("cursor should be visible when input is focused")
	}
}

func TestPRCommentsWidgetFocusedInput(t *testing.T) {
	w := NewPRCommentsWidget()
	w.SetComments(nil)

	if inp := w.FocusedInput(); inp != nil {
		t.Error("FocusedInput should return nil when not focused")
	}

	w.inputFocused = true
	if inp := w.FocusedInput(); inp == nil {
		t.Error("FocusedInput should return input when focused")
	}
}
