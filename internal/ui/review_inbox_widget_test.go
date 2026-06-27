package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/github"
)

func makeTestComments() []github.PRComment {
	return []github.PRComment{
		{ID: 1, Body: "Fix this bug", User: "alice", Path: "main.go", Line: 42, IsInline: true, CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: 2, Body: "Why is this needed?", User: "bob", Path: "main.go", Line: 78, IsInline: true, CreatedAt: "2024-01-01T01:00:00Z"},
		{ID: 3, Body: "Nit: typo", User: "alice", Path: "main.go", Line: 95, IsInline: true, CreatedAt: "2024-01-01T02:00:00Z"},
		{ID: 4, Body: "Bug in utils", User: "carol", Path: "util.go", Line: 12, IsInline: true, CreatedAt: "2024-01-01T03:00:00Z"},
		{ID: 5, Body: "Typo fix", User: "dave", Path: "util.go", Line: 30, IsInline: true, CreatedAt: "2024-01-01T04:00:00Z"},
		{ID: 6, Body: "LGTM with minor changes", User: "alice", IsInline: false, CreatedAt: "2024-01-01T05:00:00Z"},
		{ID: 7, Body: "Ship it", User: "bob", IsInline: false, CreatedAt: "2024-01-01T06:00:00Z"},
	}
}

func TestReviewInboxSetComments(t *testing.T) {
	w := NewReviewInboxWidget()
	state := github.NewReviewState("owner", "repo", 1)
	comments := makeTestComments()

	w.SetComments(comments, state)

	if len(w.FileGroups) != 2 {
		t.Fatalf("expected 2 file groups, got %d", len(w.FileGroups))
	}

	if w.FileGroups[0].Path != "main.go" {
		t.Errorf("expected first group to be main.go, got %s", w.FileGroups[0].Path)
	}
	if len(w.FileGroups[0].Comments) != 3 {
		t.Errorf("expected 3 comments in main.go, got %d", len(w.FileGroups[0].Comments))
	}

	if w.FileGroups[1].Path != "util.go" {
		t.Errorf("expected second group to be util.go, got %s", w.FileGroups[1].Path)
	}
	if len(w.FileGroups[1].Comments) != 2 {
		t.Errorf("expected 2 comments in util.go, got %d", len(w.FileGroups[1].Comments))
	}

	if len(w.GeneralComments) != 2 {
		t.Errorf("expected 2 general comments, got %d", len(w.GeneralComments))
	}

	if w.TotalComments() != 7 {
		t.Errorf("expected 7 total comments, got %d", w.TotalComments())
	}
}

func TestReviewInboxStateTransitions(t *testing.T) {
	w := NewReviewInboxWidget()
	state := github.NewReviewState("owner", "repo", 1)
	comments := makeTestComments()

	w.SetComments(comments, state)

	// All should be open initially
	progress := w.ProgressText()
	if progress != "0/7 resolved" {
		t.Errorf("expected '0/7 resolved', got %q", progress)
	}

	// Mark comment 1 as verified
	w.UpdateCommentState(1, github.StateVerified)
	progress = w.ProgressText()
	if progress != "1/7 resolved" {
		t.Errorf("expected '1/7 resolved', got %q", progress)
	}

	// Mark comment 3 as dismissed
	w.UpdateCommentState(3, github.StateDismissed)
	progress = w.ProgressText()
	if progress != "2/7 resolved" {
		t.Errorf("expected '2/7 resolved', got %q", progress)
	}

	// Check that state is persisted in the ReviewState object
	if state.GetState(1) != github.StateVerified {
		t.Error("expected comment 1 state to be verified in ReviewState")
	}
	if state.GetState(3) != github.StateDismissed {
		t.Error("expected comment 3 state to be dismissed in ReviewState")
	}
}

func TestReviewInboxUnresolvedComments(t *testing.T) {
	w := NewReviewInboxWidget()
	state := github.NewReviewState("owner", "repo", 1)
	comments := makeTestComments()

	w.SetComments(comments, state)

	unresolved := w.UnresolvedComments()
	if len(unresolved) != 5 {
		t.Fatalf("expected 5 unresolved inline comments, got %d", len(unresolved))
	}

	// Mark 2 as verified
	w.UpdateCommentState(1, github.StateVerified)
	w.UpdateCommentState(2, github.StateDismissed)

	// Re-query (need to re-set comments since UpdateCommentState changes state in-place)
	unresolved = w.UnresolvedComments()
	if len(unresolved) != 3 {
		t.Fatalf("expected 3 unresolved inline comments after marking 2, got %d", len(unresolved))
	}
}

func TestReviewInboxOpenCount(t *testing.T) {
	w := NewReviewInboxWidget()
	state := github.NewReviewState("owner", "repo", 1)
	state.SetState(3, github.StateVerified) // pre-set one as verified

	comments := makeTestComments()
	w.SetComments(comments, state)

	// main.go: 3 comments, 1 verified = 2 open
	if w.FileGroups[0].OpenCount != 2 {
		t.Errorf("expected main.go open count = 2, got %d", w.FileGroups[0].OpenCount)
	}

	// util.go: 2 comments, all open
	if w.FileGroups[1].OpenCount != 2 {
		t.Errorf("expected util.go open count = 2, got %d", w.FileGroups[1].OpenCount)
	}
}

func TestReviewInboxSelectedComment(t *testing.T) {
	w := NewReviewInboxWidget()
	state := github.NewReviewState("owner", "repo", 1)
	comments := makeTestComments()

	w.SetComments(comments, state)

	// First item is a file header (main.go), not a comment
	w.Selected = 0
	if sel := w.SelectedComment(); sel != nil {
		t.Error("expected no selected comment on file header")
	}

	// Second item is the first comment
	w.Selected = 1
	sel := w.SelectedComment()
	if sel == nil {
		t.Fatal("expected a selected comment at index 1")
	}
	if sel.Comment.ID != 1 {
		t.Errorf("expected comment ID 1, got %d", sel.Comment.ID)
	}
}

func TestReviewInboxCommentMarkersForFile(t *testing.T) {
	w := NewReviewInboxWidget()
	state := github.NewReviewState("owner", "repo", 1)
	comments := makeTestComments()

	w.SetComments(comments, state)

	markers := w.CommentMarkersForFile("main.go")
	if len(markers) != 3 {
		t.Fatalf("expected 3 markers for main.go, got %d", len(markers))
	}

	// Line 42 -> index 41 (0-based)
	if _, ok := markers[41]; !ok {
		t.Error("expected marker at line 41 (0-based for line 42)")
	}
	if _, ok := markers[77]; !ok {
		t.Error("expected marker at line 77 (0-based for line 78)")
	}
	if _, ok := markers[94]; !ok {
		t.Error("expected marker at line 94 (0-based for line 95)")
	}

	// Check state
	if markers[41].State != github.StateOpen {
		t.Errorf("expected marker state Open, got %v", markers[41].State)
	}
}

func TestReviewInboxEmptyState(t *testing.T) {
	w := NewReviewInboxWidget()

	if w.HasData() {
		t.Error("expected HasData() = false for empty inbox")
	}
	if w.TotalComments() != 0 {
		t.Error("expected 0 total comments")
	}
	if w.ProgressText() != "" {
		t.Error("expected empty progress text")
	}
	if w.SelectedComment() != nil {
		t.Error("expected nil selected comment")
	}
}

func TestReviewInboxToggleExpansion(t *testing.T) {
	w := NewReviewInboxWidget()
	state := github.NewReviewState("owner", "repo", 1)
	comments := makeTestComments()

	w.SetComments(comments, state)

	// Count initial items (all groups expanded)
	initialCount := len(w.items)

	// Collapse first group (main.go - 3 comments)
	w.FileGroups[0].Expanded = false
	w.buildItems()

	collapsedCount := len(w.items)
	if collapsedCount != initialCount-3 {
		t.Errorf("expected %d items after collapse, got %d", initialCount-3, collapsedCount)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a very long string", 10, "this is .."},
		{"hello\nworld", 20, "hello world"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestStateIndicator(t *testing.T) {
	tests := []struct {
		state github.CommentState
		ch    rune
	}{
		{github.StateOpen, '●'},
		{github.StateAddressed, '~'},
		{github.StateVerified, '✓'},
		{github.StateDismissed, '✗'},
	}

	for _, tt := range tests {
		ch, _ := stateIndicator(tt.state)
		if ch != tt.ch {
			t.Errorf("stateIndicator(%v) = %c, want %c", tt.state, ch, tt.ch)
		}
	}
}
