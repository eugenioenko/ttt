package github

import (
	"testing"
)

func TestParsePRURL(t *testing.T) {
	tests := []struct {
		url    string
		owner  string
		repo   string
		number int
		err    bool
	}{
		{"https://github.com/owner/repo/pull/123", "owner", "repo", 123, false},
		{"https://github.com/owner/repo/pull/123/", "owner", "repo", 123, false},
		{"https://github.com/owner/repo/pull/123/files", "owner", "repo", 123, false},
		{"https://github.com/owner/repo/pull/456?tab=commits", "owner", "repo", 456, false},
		{"https://github.com/owner/repo/pull/789#discussion", "owner", "repo", 789, false},
		{"https://github.com/owner/repo", "", "", 0, true},
		{"not-a-url", "", "", 0, true},
		{"https://github.com/owner/repo/pull/abc", "", "", 0, true},
	}
	for _, tt := range tests {
		owner, repo, number, err := ParsePRURL(tt.url)
		if tt.err {
			if err == nil {
				t.Errorf("ParsePRURL(%q) expected error", tt.url)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePRURL(%q) unexpected error: %v", tt.url, err)
			continue
		}
		if owner != tt.owner || repo != tt.repo || number != tt.number {
			t.Errorf("ParsePRURL(%q) = (%q, %q, %d), want (%q, %q, %d)",
				tt.url, owner, repo, number, tt.owner, tt.repo, tt.number)
		}
	}
}

func TestSplitMultiFileDiff(t *testing.T) {
	unified := `diff --git a/file1.go b/file1.go
--- a/file1.go
+++ b/file1.go
@@ -1,3 +1,3 @@
 line1
-old
+new
 line3
diff --git a/pkg/file2.go b/pkg/file2.go
--- a/pkg/file2.go
+++ b/pkg/file2.go
@@ -1,2 +1,2 @@
-removed
+added`

	result := SplitMultiFileDiff(unified)
	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}
	if _, ok := result["file1.go"]; !ok {
		t.Error("missing file1.go")
	}
	if _, ok := result["pkg/file2.go"]; !ok {
		t.Error("missing pkg/file2.go")
	}
}

func TestSortCommentsByTime(t *testing.T) {
	comments := []PRComment{
		{ID: 3, CreatedAt: "2024-03-15T10:00:00Z"},
		{ID: 1, CreatedAt: "2024-01-10T10:00:00Z"},
		{ID: 2, CreatedAt: "2024-02-20T10:00:00Z"},
	}
	sortCommentsByTime(comments)
	if comments[0].ID != 1 || comments[1].ID != 2 || comments[2].ID != 3 {
		t.Errorf("comments not sorted by time: got IDs %d, %d, %d",
			comments[0].ID, comments[1].ID, comments[2].ID)
	}
}

func TestSortCommentsByTimeAlreadySorted(t *testing.T) {
	comments := []PRComment{
		{ID: 1, CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: 2, CreatedAt: "2024-02-01T00:00:00Z"},
		{ID: 3, CreatedAt: "2024-03-01T00:00:00Z"},
	}
	sortCommentsByTime(comments)
	if comments[0].ID != 1 || comments[1].ID != 2 || comments[2].ID != 3 {
		t.Errorf("already sorted comments reordered: got IDs %d, %d, %d",
			comments[0].ID, comments[1].ID, comments[2].ID)
	}
}

func TestSortCommentsByTimeEmpty(t *testing.T) {
	var comments []PRComment
	sortCommentsByTime(comments) // should not panic
}

func TestPRCommentTypes(t *testing.T) {
	// Verify PRComment struct fields work correctly
	c := PRComment{
		ID:        42,
		Body:      "looks good",
		User:      "reviewer",
		CreatedAt: "2024-06-15T12:00:00Z",
		Path:      "main.go",
		Line:      10,
		IsInline:  true,
		InReplyTo: 0,
	}
	if c.ID != 42 {
		t.Error("unexpected ID")
	}
	if c.User != "reviewer" {
		t.Error("unexpected User")
	}
	if !c.IsInline {
		t.Error("expected inline comment")
	}
	if c.Path != "main.go" {
		t.Error("unexpected Path")
	}
}
