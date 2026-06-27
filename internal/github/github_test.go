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

func TestPRCommentType(t *testing.T) {
	// Verify PRComment fields can be set for both inline and general comments
	inline := PRComment{
		ID:        1,
		Body:      "Needs a nil check here",
		User:      "reviewer",
		CreatedAt: "2024-01-15T10:30:00Z",
		Path:      "internal/app/app.go",
		Line:      42,
		IsInline:  true,
	}
	if !inline.IsInline {
		t.Error("expected inline comment")
	}
	if inline.Path != "internal/app/app.go" {
		t.Errorf("expected path internal/app/app.go, got %s", inline.Path)
	}
	if inline.Line != 42 {
		t.Errorf("expected line 42, got %d", inline.Line)
	}

	general := PRComment{
		ID:        2,
		Body:      "LGTM",
		User:      "approver",
		CreatedAt: "2024-01-15T11:00:00Z",
		IsInline:  false,
	}
	if general.IsInline {
		t.Error("expected general comment")
	}
	if general.Path != "" {
		t.Errorf("expected empty path for general comment, got %s", general.Path)
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
