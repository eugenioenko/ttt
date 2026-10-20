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

func TestSplitMultiFileDiffGitPaths(t *testing.T) {
	for _, tc := range []struct{ name, header, extra, want string }{
		{"quoted spaces", `"a/my file.txt" "b/my file.txt"`, "", "my file.txt"},
		{"spaces", "a/my file.txt b/my file.txt", "", "my file.txt"},
		{"custom prefixes", "old/file.txt new/file.txt", "", "new/file.txt"},
		{"embedded prefix", "a/tricky b/part b/name.txt b/tricky b/part b/name.txt", "", "tricky b/part b/name.txt"},
		{"quoted tab", `"a/tab\tname.txt" "b/tab\tname.txt"`, "", "tab\tname.txt"},
		{"quoted unicode", `"a/\347\225\214.txt" "b/\347\225\214.txt"`, "", "界.txt"},
		{"rename", "a/old name.txt b/new b/name.txt", "rename from old name.txt\nrename to new b/name.txt", "new b/name.txt"},
		{"quoted rename", `a/old.txt "b/new\tname.txt"`, "rename from old.txt\nrename to \"new\\tname.txt\"", "new\tname.txt"},
		{"deletion", "a/my file.txt b/my file.txt", "deleted file mode 100644\n--- a/my file.txt\t\n+++ /dev/null", "my file.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extra := tc.extra
			if extra == "" {
				extra = "@@ -1 +1 @@\n-old\n+new"
			}
			text := "diff --git " + tc.header + "\n" + extra
			files := SplitMultiFileDiff(text)
			if len(files) != 1 || files[tc.want] != text {
				t.Fatalf("split=%v, want path %q", files, tc.want)
			}
		})
	}
}

func TestSplitMultiFileDiffMalformedHeaderDoesNotOverwritePreviousFile(t *testing.T) {
	text := "diff --git a/valid.txt b/valid.txt\n@@ -1 +1 @@\n-old\n+new\ndiff --git invalid\nignored"
	files := SplitMultiFileDiff(text)
	if len(files) != 1 || files["valid.txt"] != "diff --git a/valid.txt b/valid.txt\n@@ -1 +1 @@\n-old\n+new" {
		t.Fatalf("split = %#v", files)
	}
}

func TestSplitMultiFileDiffSkipsIncompleteBlocks(t *testing.T) {
	valid := "diff --git a/valid.txt b/valid.txt\n@@ -1 +1 @@\n-old\n+new"
	for _, malformed := range []string{
		"diff --git a/other.txt b/other.txt",
		"diff --git a/other.txt b/other.txt\nindex 1111111..2222222 100644",
		"diff --git a/other.txt b/other.txt\nindex 1111111..2222222 100644\n+++ b/valid.txt",
		"diff --git a/other.txt b/other.txt\nsimilarity index 100%",
		"diff --git a/other.txt b/other.txt\n+++ b/valid.txt\nignored",
	} {
		files := SplitMultiFileDiff(valid + "\n" + malformed)
		if len(files) != 1 || files["valid.txt"] != valid {
			t.Fatalf("incomplete block replaced a valid file: %#v", files)
		}
		if files := SplitMultiFileDiff(malformed); len(files) != 0 {
			t.Fatalf("incomplete block was accepted: %#v", files)
		}
	}
}

func TestSplitMultiFileDiffKeepsMetadataOnlyChanges(t *testing.T) {
	for _, metadata := range []string{
		"old mode 100644\nnew mode 100755",
		"new file mode 100644\nindex 0000000..e69de29",
		"deleted file mode 100644\nindex e69de29..0000000",
		"similarity index 100%\nrename from old.txt\nrename to file.txt",
		"similarity index 100%\ncopy from old.txt\ncopy to file.txt",
		"Binary files a/file.txt and b/file.txt differ",
		"GIT binary patch\nliteral 1\nIc${MZ000310RR91",
	} {
		text := "diff --git a/file.txt b/file.txt\n" + metadata
		files := SplitMultiFileDiff(text)
		if len(files) != 1 || files["file.txt"] != text {
			t.Fatalf("metadata-only change was lost: %#v", files)
		}
	}
}
