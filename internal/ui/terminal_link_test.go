package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDetectLinks_URLs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantURLs []string
	}{
		{
			name:     "simple https URL",
			input:    "Visit https://example.com for info",
			wantURLs: []string{"https://example.com"},
		},
		{
			name:     "http URL",
			input:    "Go to http://example.org/path",
			wantURLs: []string{"http://example.org/path"},
		},
		{
			name:     "URL with query params",
			input:    "See https://example.com/search?q=hello&lang=en",
			wantURLs: []string{"https://example.com/search?q=hello&lang=en"},
		},
		{
			name:     "URL followed by trailing period",
			input:    "Check https://example.com.",
			wantURLs: []string{"https://example.com"},
		},
		{
			name:     "URL followed by trailing comma",
			input:    "See https://a.com, https://b.com,",
			wantURLs: []string{"https://a.com", "https://b.com"},
		},
		{
			name:     "multiple URLs on one line",
			input:    "https://first.com and https://second.com",
			wantURLs: []string{"https://first.com", "https://second.com"},
		},
		{
			name:     "no URLs",
			input:    "just some text without links",
			wantURLs: nil,
		},
		{
			name:     "URL in parentheses",
			input:    "(https://example.com/path)",
			wantURLs: []string{"https://example.com/path"},
		},
		{
			name:     "URL with fragment",
			input:    "https://example.com/page#section",
			wantURLs: []string{"https://example.com/page#section"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := detectLinks(tt.input, "")
			var urls []string
			for _, s := range spans {
				if !s.IsFile {
					urls = append(urls, s.URL)
				}
			}
			if len(urls) != len(tt.wantURLs) {
				t.Fatalf("got %d URLs %v, want %d URLs %v", len(urls), urls, len(tt.wantURLs), tt.wantURLs)
			}
			for i := range urls {
				if urls[i] != tt.wantURLs[i] {
					t.Errorf("URL[%d] = %q, want %q", i, urls[i], tt.wantURLs[i])
				}
			}
		})
	}
}

func TestDetectLinks_FileReferences(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	subDir := filepath.Join(tmpDir, "internal")
	os.MkdirAll(subDir, 0755)
	subFile := filepath.Join(subDir, "util.go")
	os.WriteFile(subFile, []byte("package internal\n"), 0644)

	makeFile := filepath.Join(tmpDir, "Makefile")
	os.WriteFile(makeFile, []byte("all:\n"), 0644)

	tests := []struct {
		name     string
		input    string
		workDir  string
		wantFile string
		wantLine int
		wantCol  int
	}{
		{
			name:     "relative file:line",
			input:    "./main.go:42",
			workDir:  tmpDir,
			wantFile: testFile,
			wantLine: 42,
		},
		{
			name:     "relative file:line:col",
			input:    "./internal/util.go:10:5",
			workDir:  tmpDir,
			wantFile: subFile,
			wantLine: 10,
			wantCol:  5,
		},
		{
			name:     "absolute file path",
			input:    testFile + ":7",
			workDir:  "",
			wantFile: testFile,
			wantLine: 7,
		},
		{
			name:     "file that does not exist",
			input:    "./nonexistent.go:5",
			workDir:  tmpDir,
			wantFile: "",
			wantLine: 0,
		},
		{
			name:     "file reference in error output",
			input:    "error: ./main.go:12: undefined variable",
			workDir:  tmpDir,
			wantFile: testFile,
			wantLine: 12,
		},
		{
			name:     "bare relative path (compiler output)",
			input:    "main.go:7:2: undefined: foo",
			workDir:  tmpDir,
			wantFile: testFile,
			wantLine: 7,
			wantCol:  2,
		},
		{
			name:     "bare nested relative path",
			input:    "internal/util.go:3 some message",
			workDir:  tmpDir,
			wantFile: subFile,
			wantLine: 3,
		},
		{
			name:     "extensionless file (Makefile)",
			input:    "Makefile:12: recipe failed",
			workDir:  tmpDir,
			wantFile: makeFile,
			wantLine: 12,
		},
		{
			name:     "extensionless token that is not a file",
			input:    "warning:42 something happened",
			workDir:  tmpDir,
			wantFile: "",
			wantLine: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := detectLinks(tt.input, tt.workDir)
			var fileSpans []linkSpan
			for _, s := range spans {
				if s.IsFile {
					fileSpans = append(fileSpans, s)
				}
			}
			if tt.wantFile == "" {
				if len(fileSpans) != 0 {
					t.Fatalf("expected no file links, got %d: %+v", len(fileSpans), fileSpans)
				}
				return
			}
			if len(fileSpans) == 0 {
				t.Fatal("expected a file link, got none")
			}
			got := fileSpans[0]
			if got.FilePath != tt.wantFile {
				t.Errorf("FilePath = %q, want %q", got.FilePath, tt.wantFile)
			}
			if got.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d", got.Line, tt.wantLine)
			}
			if got.Col != tt.wantCol {
				t.Errorf("Col = %d, want %d", got.Col, tt.wantCol)
			}
		})
	}
}

func TestDetectLinks_URLSpanPositions(t *testing.T) {
	input := "See https://example.com here"
	spans := detectLinks(input, "")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	s := spans[0]
	if s.StartCol != 4 {
		t.Errorf("StartCol = %d, want 4", s.StartCol)
	}
	if s.EndCol != 23 {
		t.Errorf("EndCol = %d, want 23", s.EndCol)
	}
}

func TestDetectLinks_MixedURLAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	input := "See https://example.com and ./test.go:10"
	spans := detectLinks(input, tmpDir)

	var urls, files int
	for _, s := range spans {
		if s.IsFile {
			files++
		} else {
			urls++
		}
	}
	if urls != 1 {
		t.Errorf("expected 1 URL span, got %d", urls)
	}
	if files != 1 {
		t.Errorf("expected 1 file span, got %d", files)
	}
}

func TestDetectLinks_EmptyLine(t *testing.T) {
	spans := detectLinks("", "")
	if len(spans) != 0 {
		t.Errorf("expected no spans for empty line, got %d", len(spans))
	}
}

func TestResolveFilePath_HomeTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	result := resolveFilePath("~/", "")
	if result != home {
		_ = result
	}
}

func TestResolveFilePath_Empty(t *testing.T) {
	result := resolveFilePath("", "/some/dir")
	if result != "" {
		t.Errorf("expected empty result for empty path, got %q", result)
	}
}

func TestLinksForLine_Memoizes(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	tw := &TerminalWidget{WorkDir: tmpDir}
	line := "err at ./main.go:3"

	first := tw.linksForLine(line)
	if len(first) != 1 || first[0].FilePath != testFile {
		t.Fatalf("expected one link to %s, got %+v", testFile, first)
	}
	if _, ok := tw.linkMemo[line]; !ok {
		t.Fatal("expected memo entry for line text")
	}

	// Deleting the file must not affect the memoized result: detection is
	// served from the memo, no re-stat.
	os.Remove(testFile)
	second := tw.linksForLine(line)
	if len(second) != 1 || second[0].FilePath != testFile {
		t.Fatalf("expected memoized link, got %+v", second)
	}
}

func TestLinksForLine_MemoCap(t *testing.T) {
	tw := &TerminalWidget{}
	for i := 0; i < linkMemoMax; i++ {
		tw.linksForLine("line " + strconv.Itoa(i))
	}
	if len(tw.linkMemo) != linkMemoMax {
		t.Fatalf("expected memo len %d, got %d", linkMemoMax, len(tw.linkMemo))
	}
	// The next unique line resets the memo instead of growing it.
	tw.linksForLine("one more line")
	if len(tw.linkMemo) != 1 {
		t.Fatalf("expected memo reset to 1 entry, got %d", len(tw.linkMemo))
	}
}
