package github

import "testing"

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

func TestParseReviewComments(t *testing.T) {
	data := []byte(`[
		{
			"id": 101,
			"body": "This needs a nil check",
			"user": {"login": "reviewer1"},
			"created_at": "2024-01-15T10:00:00Z",
			"updated_at": "2024-01-15T10:00:00Z",
			"path": "main.go",
			"line": 42,
			"original_line": 40,
			"in_reply_to_id": 0
		},
		{
			"id": 103,
			"body": "Good point, fixed",
			"user": {"login": "author1"},
			"created_at": "2024-01-15T11:00:00Z",
			"updated_at": "2024-01-15T11:00:00Z",
			"path": "main.go",
			"line": 42,
			"original_line": 40,
			"in_reply_to_id": 101
		},
		{
			"id": 105,
			"body": "Outdated comment",
			"user": {"login": "reviewer2"},
			"created_at": "2024-01-15T12:00:00Z",
			"updated_at": "2024-01-15T12:00:00Z",
			"path": "utils.go",
			"line": null,
			"original_line": 10,
			"in_reply_to_id": 0
		}
	]`)

	comments, err := parseReviewComments(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(comments))
	}

	// First comment: inline review comment with line set
	c := comments[0]
	if c.ID != 101 {
		t.Errorf("expected ID 101, got %d", c.ID)
	}
	if c.Body != "This needs a nil check" {
		t.Errorf("unexpected body: %s", c.Body)
	}
	if c.User != "reviewer1" {
		t.Errorf("expected user reviewer1, got %s", c.User)
	}
	if c.CreatedAt != "2024-01-15T10:00:00Z" {
		t.Errorf("unexpected created_at: %s", c.CreatedAt)
	}
	if c.Path != "main.go" {
		t.Errorf("expected path main.go, got %s", c.Path)
	}
	if c.Line != 42 {
		t.Errorf("expected line 42, got %d", c.Line)
	}
	if !c.IsInline {
		t.Error("expected IsInline to be true")
	}
	if c.InReplyTo != 0 {
		t.Errorf("expected InReplyTo 0, got %d", c.InReplyTo)
	}

	// Reply comment
	c = comments[1]
	if c.ID != 103 {
		t.Errorf("expected ID 103, got %d", c.ID)
	}
	if c.InReplyTo != 101 {
		t.Errorf("expected InReplyTo 101, got %d", c.InReplyTo)
	}

	// Comment with null line (should fallback to original_line)
	c = comments[2]
	if c.ID != 105 {
		t.Errorf("expected ID 105, got %d", c.ID)
	}
	if c.Line != 10 {
		t.Errorf("expected line 10 (from original_line fallback), got %d", c.Line)
	}
	if c.Path != "utils.go" {
		t.Errorf("expected path utils.go, got %s", c.Path)
	}
}

func TestParseReviewCommentsEmpty(t *testing.T) {
	data := []byte(`[]`)
	comments, err := parseReviewComments(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(comments))
	}
}

func TestParseReviewCommentsInvalidJSON(t *testing.T) {
	data := []byte(`not json`)
	_, err := parseReviewComments(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseIssueComments(t *testing.T) {
	data := []byte(`[
		{
			"id": 200,
			"body": "Looks good overall",
			"user": {"login": "reviewer1"},
			"created_at": "2024-01-15T09:00:00Z",
			"updated_at": "2024-01-15T09:30:00Z"
		},
		{
			"id": 202,
			"body": "Please add tests",
			"user": {"login": "reviewer2"},
			"created_at": "2024-01-15T13:00:00Z",
			"updated_at": "2024-01-15T13:00:00Z"
		}
	]`)

	comments, err := parseIssueComments(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	// First comment: general issue comment
	c := comments[0]
	if c.ID != 200 {
		t.Errorf("expected ID 200, got %d", c.ID)
	}
	if c.Body != "Looks good overall" {
		t.Errorf("unexpected body: %s", c.Body)
	}
	if c.User != "reviewer1" {
		t.Errorf("expected user reviewer1, got %s", c.User)
	}
	if c.CreatedAt != "2024-01-15T09:00:00Z" {
		t.Errorf("unexpected created_at: %s", c.CreatedAt)
	}
	if c.UpdatedAt != "2024-01-15T09:30:00Z" {
		t.Errorf("unexpected updated_at: %s", c.UpdatedAt)
	}
	if c.Path != "" {
		t.Errorf("expected empty path for general comment, got %s", c.Path)
	}
	if c.Line != 0 {
		t.Errorf("expected line 0 for general comment, got %d", c.Line)
	}
	if c.IsInline {
		t.Error("expected IsInline to be false for general comment")
	}
	if c.InReplyTo != 0 {
		t.Errorf("expected InReplyTo 0, got %d", c.InReplyTo)
	}

	// Second comment
	c = comments[1]
	if c.ID != 202 {
		t.Errorf("expected ID 202, got %d", c.ID)
	}
	if c.Body != "Please add tests" {
		t.Errorf("unexpected body: %s", c.Body)
	}
	if c.User != "reviewer2" {
		t.Errorf("expected user reviewer2, got %s", c.User)
	}
	if c.IsInline {
		t.Error("expected IsInline to be false")
	}
}

func TestParseIssueCommentsEmpty(t *testing.T) {
	data := []byte(`[]`)
	comments, err := parseIssueComments(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(comments))
	}
}

func TestParseIssueCommentsInvalidJSON(t *testing.T) {
	data := []byte(`{broken}`)
	_, err := parseIssueComments(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
