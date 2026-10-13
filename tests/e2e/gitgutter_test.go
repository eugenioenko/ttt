package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

func TestGitGutterSkipsIgnoredFiles(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	os.WriteFile(filepath.Join(h.dir, ".gitignore"), []byte("*.log\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	os.WriteFile(filepath.Join(h.dir, "app.log"), []byte("one\ntwo\n"), 0644)
	os.WriteFile(filepath.Join(h.dir, "new.txt"), []byte("one\ntwo\n"), 0644)

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "new.txt"))
	h.app.SyncRepositoryBranch()
	h.app.RequestGitGutterForActiveFile()
	h.redraw()
	changes := h.app.EditorGroup.Editor.LineChanges
	lines := h.app.EditorGroup.ActiveBuffer().Lines
	if len(changes) != len(lines) || slices.ContainsFunc(changes, func(c diff.LineChangeKind) bool { return c != diff.LineAdded }) {
		t.Fatalf("untracked file gutter = %v for %d lines, want every line added", changes, len(lines))
	}

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "app.log"))
	h.app.SyncRepositoryBranch()
	h.app.RequestGitGutterForActiveFile()
	h.redraw()
	if changes := h.app.EditorGroup.Editor.LineChanges; len(changes) != 0 {
		t.Fatalf("ignored file gutter = %v, want no markers", changes)
	}
}

func TestGitGutterUnchangedCommittedFiles(t *testing.T) {
	for _, tc := range []struct {
		name            string
		content         string
		trailingNewline bool
	}{
		{"crlf", "one\r\ntwo\r\nthree\r\n", true},
		{"crlf no trailing line", "one\r\ntwo\r\nthree\r\n", false},
		{"lf no trailing line", "one\ntwo\nthree\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t, 100, 30)
			defer h.stop()
			path := filepath.Join(h.dir, "file.txt")
			os.WriteFile(path, []byte(tc.content), 0644)
			os.WriteFile(filepath.Join(h.dir, ".gitattributes"), []byte("file.txt -text\n"), 0644)
			initializeHarnessRepository(t, h.dir)

			h.app.EditorGroup.ShowTrailingNewline = tc.trailingNewline
			h.app.EditorGroup.OpenFile(path)
			h.app.SyncRepositoryBranch()
			h.app.RequestGitGutterForActiveFile()
			h.redraw()
			changes := h.app.EditorGroup.Editor.LineChanges
			if want := len(h.app.EditorGroup.ActiveBuffer().Lines); len(changes) != want {
				t.Fatalf("unchanged file gutter has %d entries, want one per buffer line (%d)", len(changes), want)
			}
			if slices.ContainsFunc(changes, func(c diff.LineChangeKind) bool { return c != diff.LineUnchanged }) {
				t.Fatalf("unchanged file gutter = %v, want no markers", changes)
			}
		})
	}
}
