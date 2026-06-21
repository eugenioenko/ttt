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

func TestFormatCommentTime(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2024-01-15T10:30:00Z", "2024-01-15"},
		{"2024-12-01", "2024-12-01"},
		{"short", "short"},
		{"", ""},
	}
	for _, tt := range tests {
		got := FormatCommentTime(tt.input)
		if got != tt.want {
			t.Errorf("FormatCommentTime(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCommentsForFile(t *testing.T) {
	comments := []PRComment{
		{ID: 1, Body: "inline on main.go", Path: "main.go", Line: 10, IsInline: true},
		{ID: 2, Body: "inline on util.go", Path: "util.go", Line: 5, IsInline: true},
		{ID: 3, Body: "general comment", IsInline: false},
		{ID: 4, Body: "another on main.go", Path: "main.go", Line: 20, IsInline: true},
	}

	mainComments := CommentsForFile(comments, "main.go")
	if len(mainComments) != 2 {
		t.Errorf("expected 2 comments for main.go, got %d", len(mainComments))
	}
	if mainComments[0].ID != 1 || mainComments[1].ID != 4 {
		t.Errorf("wrong comments returned for main.go")
	}

	utilComments := CommentsForFile(comments, "util.go")
	if len(utilComments) != 1 {
		t.Errorf("expected 1 comment for util.go, got %d", len(utilComments))
	}

	noComments := CommentsForFile(comments, "nonexistent.go")
	if len(noComments) != 0 {
		t.Errorf("expected 0 comments for nonexistent.go, got %d", len(noComments))
	}
}

func TestGeneralComments(t *testing.T) {
	comments := []PRComment{
		{ID: 1, Body: "inline", Path: "main.go", Line: 10, IsInline: true},
		{ID: 2, Body: "general 1", IsInline: false},
		{ID: 3, Body: "general 2", IsInline: false},
		{ID: 4, Body: "inline 2", Path: "util.go", Line: 5, IsInline: true},
	}

	general := GeneralComments(comments)
	if len(general) != 2 {
		t.Fatalf("expected 2 general comments, got %d", len(general))
	}
	if general[0].ID != 2 || general[1].ID != 3 {
		t.Errorf("wrong general comments returned")
	}
}

func TestFileCommentCounts(t *testing.T) {
	comments := []PRComment{
		{ID: 1, Path: "main.go", Line: 10, IsInline: true},
		{ID: 2, Path: "main.go", Line: 20, IsInline: true},
		{ID: 3, Path: "util.go", Line: 5, IsInline: true},
		{ID: 4, IsInline: false}, // general comment - no path
	}

	counts := FileCommentCounts(comments)
	if counts["main.go"] != 2 {
		t.Errorf("expected 2 for main.go, got %d", counts["main.go"])
	}
	if counts["util.go"] != 1 {
		t.Errorf("expected 1 for util.go, got %d", counts["util.go"])
	}
	if counts["nonexistent.go"] != 0 {
		t.Errorf("expected 0 for nonexistent.go, got %d", counts["nonexistent.go"])
	}
}

func TestGeneralCommentsEmpty(t *testing.T) {
	var comments []PRComment
	general := GeneralComments(comments)
	if len(general) != 0 {
		t.Errorf("expected 0 general comments from nil slice, got %d", len(general))
	}
}

func TestFileCommentCountsEmpty(t *testing.T) {
	var comments []PRComment
	counts := FileCommentCounts(comments)
	if len(counts) != 0 {
		t.Errorf("expected empty map from nil slice, got %d entries", len(counts))
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
