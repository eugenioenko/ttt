package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommentStateString(t *testing.T) {
	tests := []struct {
		state CommentState
		want  string
	}{
		{StateOpen, "open"},
		{StateAddressed, "addressed"},
		{StateVerified, "verified"},
		{StateDismissed, "dismissed"},
		{CommentState(99), "open"}, // unknown defaults to "open"
	}
	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("CommentState(%d).String() = %q, want %q", int(tt.state), got, tt.want)
		}
	}
}

func TestNewReviewState(t *testing.T) {
	rs := NewReviewState("octocat", "hello-world", 42)
	if rs.Owner != "octocat" {
		t.Errorf("Owner = %q, want %q", rs.Owner, "octocat")
	}
	if rs.Repo != "hello-world" {
		t.Errorf("Repo = %q, want %q", rs.Repo, "hello-world")
	}
	if rs.PRNumber != 42 {
		t.Errorf("PRNumber = %d, want %d", rs.PRNumber, 42)
	}
	if rs.Comments == nil {
		t.Fatal("Comments map should be initialized, got nil")
	}
	if len(rs.Comments) != 0 {
		t.Errorf("Comments should be empty, got %d entries", len(rs.Comments))
	}
}

func TestSetGetState(t *testing.T) {
	rs := NewReviewState("owner", "repo", 1)

	// Default state for unknown comment is StateOpen
	if got := rs.GetState(999); got != StateOpen {
		t.Errorf("GetState(unknown) = %v, want StateOpen", got)
	}

	// Set and get each state
	rs.SetState(1, StateAddressed)
	if got := rs.GetState(1); got != StateAddressed {
		t.Errorf("GetState(1) = %v, want StateAddressed", got)
	}

	rs.SetState(2, StateVerified)
	if got := rs.GetState(2); got != StateVerified {
		t.Errorf("GetState(2) = %v, want StateVerified", got)
	}

	rs.SetState(3, StateDismissed)
	if got := rs.GetState(3); got != StateDismissed {
		t.Errorf("GetState(3) = %v, want StateDismissed", got)
	}

	// Overwrite existing state
	rs.SetState(1, StateVerified)
	if got := rs.GetState(1); got != StateVerified {
		t.Errorf("GetState(1) after overwrite = %v, want StateVerified", got)
	}
}

func TestCountByState(t *testing.T) {
	rs := NewReviewState("owner", "repo", 1)

	// Empty state
	open, addressed, verified, dismissed := rs.CountByState()
	if open != 0 || addressed != 0 || verified != 0 || dismissed != 0 {
		t.Errorf("empty CountByState = (%d,%d,%d,%d), want (0,0,0,0)",
			open, addressed, verified, dismissed)
	}

	rs.SetState(1, StateOpen)
	rs.SetState(2, StateOpen)
	rs.SetState(3, StateAddressed)
	rs.SetState(4, StateVerified)
	rs.SetState(5, StateDismissed)
	rs.SetState(6, StateDismissed)

	open, addressed, verified, dismissed = rs.CountByState()
	if open != 2 {
		t.Errorf("open = %d, want 2", open)
	}
	if addressed != 1 {
		t.Errorf("addressed = %d, want 1", addressed)
	}
	if verified != 1 {
		t.Errorf("verified = %d, want 1", verified)
	}
	if dismissed != 2 {
		t.Errorf("dismissed = %d, want 2", dismissed)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	rs := NewReviewState("octocat", "hello-world", 42)
	rs.SetState(10, StateAddressed)
	rs.SetState(20, StateVerified)
	rs.SetState(30, StateDismissed)

	if err := rs.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, ".ttt-review-state.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file not found: %v", err)
	}

	loaded, err := LoadReviewState(dir)
	if err != nil {
		t.Fatalf("LoadReviewState failed: %v", err)
	}

	if loaded.Owner != rs.Owner {
		t.Errorf("loaded Owner = %q, want %q", loaded.Owner, rs.Owner)
	}
	if loaded.Repo != rs.Repo {
		t.Errorf("loaded Repo = %q, want %q", loaded.Repo, rs.Repo)
	}
	if loaded.PRNumber != rs.PRNumber {
		t.Errorf("loaded PRNumber = %d, want %d", loaded.PRNumber, rs.PRNumber)
	}
	if len(loaded.Comments) != len(rs.Comments) {
		t.Fatalf("loaded Comments length = %d, want %d", len(loaded.Comments), len(rs.Comments))
	}
	for id, state := range rs.Comments {
		if loaded.Comments[id] != state {
			t.Errorf("loaded Comments[%d] = %v, want %v", id, loaded.Comments[id], state)
		}
	}
}

func TestLoadReviewStateNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadReviewState(dir)
	if err == nil {
		t.Fatal("LoadReviewState should return error for missing file")
	}
}
