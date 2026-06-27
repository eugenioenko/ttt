package github

import (
	"encoding/json"
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

func TestPRCommentJSONParsing(t *testing.T) {
	// Test parsing inline review comments JSON.
	reviewJSON := `[
		{
			"id": 101,
			"body": "Looks good",
			"user": {"login": "alice"},
			"created_at": "2025-01-15T10:30:00Z",
			"path": "main.go",
			"line": 42
		},
		{
			"id": 102,
			"body": "Needs a test",
			"user": {"login": "bob"},
			"created_at": "2025-01-16T08:00:00Z",
			"path": "utils.go",
			"line": null
		}
	]`

	var reviewComments []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt string `json:"created_at"`
		Path      string `json:"path"`
		Line      *int   `json:"line"`
	}
	if err := json.Unmarshal([]byte(reviewJSON), &reviewComments); err != nil {
		t.Fatalf("failed to parse review comments JSON: %v", err)
	}
	if len(reviewComments) != 2 {
		t.Fatalf("expected 2 review comments, got %d", len(reviewComments))
	}

	// First comment has a line number.
	if reviewComments[0].ID != 101 {
		t.Errorf("expected ID 101, got %d", reviewComments[0].ID)
	}
	if reviewComments[0].User.Login != "alice" {
		t.Errorf("expected user alice, got %q", reviewComments[0].User.Login)
	}
	if reviewComments[0].Line == nil || *reviewComments[0].Line != 42 {
		t.Error("expected line 42 for first comment")
	}

	// Second comment has null line.
	if reviewComments[1].Line != nil {
		t.Error("expected nil line for second comment")
	}

	// Test parsing issue comments JSON.
	issueJSON := `[
		{
			"id": 201,
			"body": "General feedback",
			"user": {"login": "charlie"},
			"created_at": "2025-01-17T12:00:00Z"
		}
	]`

	var issueComments []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal([]byte(issueJSON), &issueComments); err != nil {
		t.Fatalf("failed to parse issue comments JSON: %v", err)
	}
	if len(issueComments) != 1 {
		t.Fatalf("expected 1 issue comment, got %d", len(issueComments))
	}
	if issueComments[0].User.Login != "charlie" {
		t.Errorf("expected user charlie, got %q", issueComments[0].User.Login)
	}
}

func TestPRCommentType(t *testing.T) {
	// Test that the PRComment struct stores data correctly.
	c := PRComment{
		ID:        1,
		Body:      "Test comment",
		User:      "testuser",
		CreatedAt: "2025-06-18T10:00:00Z",
		Path:      "src/main.go",
		Line:      15,
		IsInline:  true,
	}
	if c.ID != 1 {
		t.Errorf("expected ID 1, got %d", c.ID)
	}
	if c.User != "testuser" {
		t.Errorf("expected user testuser, got %q", c.User)
	}
	if !c.IsInline {
		t.Error("expected IsInline to be true")
	}
	if c.Path != "src/main.go" {
		t.Errorf("expected path src/main.go, got %q", c.Path)
	}
	if c.Line != 15 {
		t.Errorf("expected line 15, got %d", c.Line)
	}
}
