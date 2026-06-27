package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func makeReviewComments() []ReviewComment {
	return []ReviewComment{
		{ID: 1, Body: "Fix this bug", User: "alice", CreatedAt: "2024-01-15T10:00:00Z", Path: "main.go", Line: 10, IsInline: true},
		{ID: 2, Body: "Looks good", User: "bob", CreatedAt: "2024-01-15T11:00:00Z", Path: "main.go", Line: 20, IsInline: true},
		{ID: 3, Body: "Check error handling", User: "alice", CreatedAt: "2024-01-15T12:00:00Z", Path: "util.go", Line: 5, IsInline: true},
		{ID: 4, Body: "LGTM overall", User: "carol", CreatedAt: "2024-01-16T09:00:00Z", IsInline: false},
	}
}

func TestNewReviewHubWidget(t *testing.T) {
	comments := makeReviewComments()
	w := NewReviewHubWidget(comments, "Fix bugs", 42)

	if w.PRTitle != "Fix bugs" {
		t.Errorf("expected title 'Fix bugs', got %q", w.PRTitle)
	}
	if w.PRNumber != 42 {
		t.Errorf("expected number 42, got %d", w.PRNumber)
	}
	if len(w.Comments) != 4 {
		t.Errorf("expected 4 comments, got %d", len(w.Comments))
	}
	if !w.Focusable() {
		t.Error("expected Focusable() to return true")
	}
}

func TestReviewHubBuildRows(t *testing.T) {
	comments := makeReviewComments()
	w := NewReviewHubWidget(comments, "Fix bugs", 42)
	w.buildRows()

	if len(w.rows) == 0 {
		t.Fatal("expected rows to be built")
	}

	// Should have at least: file header for main.go, comments, file header for util.go, comments, general header, comment
	hasMainHeader := false
	hasUtilHeader := false
	hasGeneralHeader := false
	bodyCount := 0
	userCount := 0

	for _, row := range w.rows {
		switch row.kind {
		case rowHeader:
			if row.text == " main.go" {
				hasMainHeader = true
			}
			if row.text == " util.go" {
				hasUtilHeader = true
			}
		case rowGeneral:
			hasGeneralHeader = true
		case rowBody:
			bodyCount++
		case rowUser:
			userCount++
		}
	}

	if !hasMainHeader {
		t.Error("expected main.go file header")
	}
	if !hasUtilHeader {
		t.Error("expected util.go file header")
	}
	if !hasGeneralHeader {
		t.Error("expected General Comments header")
	}
	// 3 inline + 1 general = 4 user rows
	if userCount != 4 {
		t.Errorf("expected 4 user rows, got %d", userCount)
	}
	// Each comment has 1 body line = 4 body rows
	if bodyCount != 4 {
		t.Errorf("expected 4 body rows, got %d", bodyCount)
	}
}

func TestReviewHubEmptyComments(t *testing.T) {
	w := NewReviewHubWidget(nil, "Empty PR", 1)
	w.buildRows()

	if len(w.rows) != 1 {
		t.Fatalf("expected 1 row for empty comments, got %d", len(w.rows))
	}
	if w.rows[0].text != "  No comments on this PR" {
		t.Errorf("unexpected empty text: %q", w.rows[0].text)
	}
}

func TestReviewHubRender(t *testing.T) {
	comments := makeReviewComments()
	w := NewReviewHubWidget(comments, "Fix bugs", 42)
	surface := makeSurface(80, 24)
	w.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})
	w.Render(surface)

	// After render, layout values should be set
	if w.boxW <= 0 || w.boxH <= 0 {
		t.Errorf("expected positive box dimensions, got %dx%d", w.boxW, w.boxH)
	}
	if w.visibleRows <= 0 {
		t.Errorf("expected positive visibleRows, got %d", w.visibleRows)
	}
}

func TestReviewHubKeyboardNav(t *testing.T) {
	comments := makeReviewComments()
	w := NewReviewHubWidget(comments, "Fix bugs", 42)
	w.buildRows()

	initial := w.list.Selected

	// Down
	w.HandleEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if w.list.Selected != initial+1 {
		t.Errorf("down: expected %d, got %d", initial+1, w.list.Selected)
	}

	// Up
	w.HandleEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if w.list.Selected != initial {
		t.Errorf("up: expected %d, got %d", initial, w.list.Selected)
	}

	// j/k vim-style navigation
	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	if w.list.Selected != initial+1 {
		t.Errorf("j: expected %d, got %d", initial+1, w.list.Selected)
	}
	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if w.list.Selected != initial {
		t.Errorf("k: expected %d, got %d", initial, w.list.Selected)
	}

	// Home
	w.list.Selected = 5
	w.HandleEvent(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if w.list.Selected != 0 {
		t.Errorf("home: expected 0, got %d", w.list.Selected)
	}

	// End
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if w.list.Selected != len(w.rows)-1 {
		t.Errorf("end: expected %d, got %d", len(w.rows)-1, w.list.Selected)
	}
}

func TestReviewHubEscapeDismisses(t *testing.T) {
	w := NewReviewHubWidget(nil, "Test", 1)
	w.buildRows()

	dismissed := false
	w.OnDismiss = func() { dismissed = true }

	w.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if !dismissed {
		t.Error("escape should call OnDismiss")
	}
}

func TestReviewHubEnterNavigates(t *testing.T) {
	comments := []ReviewComment{
		{ID: 1, Body: "Fix", User: "alice", Path: "main.go", Line: 10, IsInline: true},
	}
	w := NewReviewHubWidget(comments, "Test", 1)
	w.buildRows()

	var navPath string
	var navLine int
	w.OnNavigate = func(path string, line int) {
		navPath = path
		navLine = line
	}

	// Move to the user row (first selectable row with a comment)
	for i, row := range w.rows {
		if row.commentIdx >= 0 {
			w.list.Selected = i
			break
		}
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if navPath != "main.go" {
		t.Errorf("expected navigate to main.go, got %q", navPath)
	}
	if navLine != 10 {
		t.Errorf("expected navigate to line 10, got %d", navLine)
	}
}

func TestReviewHubAddCommentShortcut(t *testing.T) {
	w := NewReviewHubWidget(nil, "Test", 1)
	w.buildRows()

	addCalled := false
	w.OnAddComment = func() { addCalled = true }

	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	if !addCalled {
		t.Error("'a' should call OnAddComment")
	}
}

func TestReviewHubSectionJump(t *testing.T) {
	comments := makeReviewComments()
	w := NewReviewHubWidget(comments, "Fix bugs", 42)
	w.buildRows()

	// Start at first row (should be a header)
	w.list.Selected = 0

	// Jump to next section
	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if w.list.Selected == 0 {
		t.Error("'n' should move to next section")
	}
	row := w.rows[w.list.Selected]
	if row.kind != rowHeader && row.kind != rowGeneral {
		t.Errorf("expected header row, got kind %d", row.kind)
	}

	// Jump back
	savedPos := w.list.Selected
	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))
	if w.list.Selected >= savedPos {
		t.Error("'p' should move to previous section")
	}
}

func TestReviewHubMultilineComment(t *testing.T) {
	comments := []ReviewComment{
		{ID: 1, Body: "Line 1\nLine 2\nLine 3", User: "alice", Path: "f.go", Line: 1, IsInline: true},
	}
	w := NewReviewHubWidget(comments, "Test", 1)
	w.buildRows()

	bodyCount := 0
	for _, row := range w.rows {
		if row.kind == rowBody {
			bodyCount++
		}
	}
	if bodyCount != 3 {
		t.Errorf("expected 3 body rows for multiline comment, got %d", bodyCount)
	}
}

func TestReviewHubAllEventsConsumed(t *testing.T) {
	w := NewReviewHubWidget(nil, "Test", 1)
	w.buildRows()

	// All key events should be consumed (modal widget)
	result := w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if result != EventConsumed {
		t.Error("unhandled key should still return EventConsumed")
	}

	result = w.HandleEvent(tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone))
	if result != EventConsumed {
		t.Error("unhandled special key should still return EventConsumed")
	}
}

func TestFormatCommentDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2024-01-15T10:00:00Z", "2024-01-15"},
		{"2024-12-31", "2024-12-31"},
		{"short", "short"},
		{"", ""},
	}
	for _, tt := range tests {
		got := formatCommentDate(tt.input)
		if got != tt.want {
			t.Errorf("formatCommentDate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestReviewHubReplyInline(t *testing.T) {
	comments := []ReviewComment{
		{ID: 1, Body: "Fix", User: "alice", Path: "main.go", Line: 10, IsInline: true},
	}
	w := NewReviewHubWidget(comments, "Test", 1)
	w.buildRows()

	var replyPath string
	var replyLine int
	w.OnAddInlineComment = func(path string, line int) {
		replyPath = path
		replyLine = line
	}

	// Move to a row with a comment
	for i, row := range w.rows {
		if row.commentIdx >= 0 {
			w.list.Selected = i
			break
		}
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
	if replyPath != "main.go" {
		t.Errorf("expected reply to main.go, got %q", replyPath)
	}
	if replyLine != 10 {
		t.Errorf("expected reply to line 10, got %d", replyLine)
	}
}
