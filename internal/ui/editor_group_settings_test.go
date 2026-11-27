package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestEditorGroupShebangHighlighting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy")
	text := "#!/usr/bin/env python3\nprint(\"deploy\")\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"open", "readonly", "buffer", "save-as", "rename", "refresh", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			g := NewEditorGroupWidget(nil, 4, false, "extended")
			g.SyntaxHighlight = mode != "disabled"
			switch mode {
			case "readonly":
				if err := g.OpenFileReadOnly(path, "deploy"); err != nil {
					t.Fatal(err)
				}
			case "buffer":
				g.OpenBufferReadOnly("deploy", path, []string{"#!/usr/bin/env python3", `print("deploy")`})
			default:
				requireNoError(t, g.OpenFile(path))
			}
			switch mode {
			case "save-as":
				if !g.SaveAs(filepath.Join(t.TempDir(), "renamed")) {
					t.Fatal("SaveAs failed")
				}
			case "rename":
				g.RenamePath(path, filepath.Join(t.TempDir(), "renamed"))
			case "refresh":
				g.RefreshHighlighters()
			}
			h := g.Editor.Highlighter
			if mode == "disabled" {
				if h != nil {
					t.Fatal("highlighting should remain disabled")
				}
				return
			}
			if h == nil || h.Language() != "Python" {
				t.Fatalf("highlighter = %v, want Python", h)
			}
			for _, span := range h.HighlightLineAt(g.Editor.Buf.Lines, 1) {
				if span.Style == term.StyleSyntaxString {
					return
				}
			}
			t.Fatal("script string is not highlighted")
		})
	}
}

func TestEditorGroupWordWrapMaxBytes(t *testing.T) {
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.txt")
	smallPath := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(largePath, []byte("01234567890123456789"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(smallPath, []byte("12345"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewEditorGroupWidget(nil, 4, false, "compact")
	g.WordWrap = true
	g.WordWrapMaxBytes = 10

	// 1. A file larger than the limit opens with g.Editor.WordWrap == false.
	requireNoError(t, g.OpenFile(largePath))
	g.CommitActiveTab()
	if g.Editor.WordWrap {
		t.Errorf("large file opened with WordWrap = true, want false")
	}
	if g.ActiveFileWordWrap() {
		t.Errorf("large file ActiveFileWordWrap() = true, want false")
	}

	// 2. A file at or under the limit opens with g.Editor.WordWrap == true.
	requireNoError(t, g.OpenFile(smallPath))
	g.CommitActiveTab()
	if !g.Editor.WordWrap {
		t.Errorf("small file opened with WordWrap = false, want true")
	}
	if !g.ActiveFileWordWrap() {
		t.Errorf("small file ActiveFileWordWrap() = false, want true")
	}

	// 3. Switching between those two tabs flips g.Editor.WordWrap each time.
	g.SwitchTab(0)
	if g.Editor.WordWrap {
		t.Errorf("switching to large file: WordWrap = true, want false")
	}
	g.SwitchTab(1)
	if !g.Editor.WordWrap {
		t.Errorf("switching back to small file: WordWrap = false, want true")
	}
	g.SwitchTab(0)
	if g.Editor.WordWrap {
		t.Errorf("switching to large file again: WordWrap = true, want false")
	}

	// 4. With g.WordWrapMaxBytes = 0, the large file wraps.
	g.WordWrapMaxBytes = 0
	g.syncTabs()
	if !g.Editor.WordWrap {
		t.Errorf("with WordWrapMaxBytes = 0, large file WordWrap = false, want true")
	}
	if !g.ActiveFileWordWrap() {
		t.Errorf("with WordWrapMaxBytes = 0, ActiveFileWordWrap() = false, want true")
	}

	// 5. With g.WordWrap = false, neither file wraps.
	g.WordWrap = false
	g.syncTabs()
	if g.Editor.WordWrap {
		t.Errorf("with WordWrap = false, large file WordWrap = true, want false")
	}
	if g.ActiveFileWordWrap() {
		t.Errorf("with WordWrap = false, large file ActiveFileWordWrap() = true, want false")
	}
	g.SwitchTab(1)
	if g.Editor.WordWrap {
		t.Errorf("with WordWrap = false, small file WordWrap = true, want false")
	}
	if g.ActiveFileWordWrap() {
		t.Errorf("with WordWrap = false, small file ActiveFileWordWrap() = true, want false")
	}

	// 6. Saving a file that grows past the limit disables wrap immediately.
	g.WordWrap = true
	g.WordWrapMaxBytes = 10
	g.syncTabs()
	if !g.Editor.WordWrap {
		t.Fatalf("expected small file to have WordWrap = true")
	}
	g.Editor.Buf.Lines = []string{"01234567890123456789"}
	if !g.Save() {
		t.Fatalf("Save() failed")
	}
	if g.Editor.WordWrap {
		t.Errorf("after saving large content: WordWrap = true, want false")
	}
}
