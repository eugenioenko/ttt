package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func fooMatch(path string) []ui.SearchMatch {
	return []ui.SearchMatch{{FilePath: path, LineNum: 2, ColStart: 0, ColEnd: 3, LineText: "foo"}}
}

func TestSearchReplaceKeepsUnsavedEdits(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "edited.txt")
	if err := os.WriteFile(path, []byte("alpha\nfoo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.Root.SetFocus(h.app.EditorGroup)
	h.redraw()
	h.pressRune('Z')
	unsaved := editorText(h)

	h.app.ApplySearchReplace(path, fooMatch(path), "bar", ui.SearchOptions{})
	h.redraw()

	if editorText(h) != unsaved || !h.app.EditorGroup.IsDirtyPath(path) {
		t.Errorf("unsaved edits were discarded: buffer %q", editorText(h))
	}
	if got := readFile(t, path); got != "alpha\nfoo\n" {
		t.Errorf("file with unsaved edits was rewritten: %q", got)
	}
}

func TestSearchReplaceSkipsFileChangedSinceSearch(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "moved.txt")
	if err := os.WriteFile(path, []byte("alpha\nXXfoo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	h.app.ApplySearchReplace(path, fooMatch(path), "bar", ui.SearchOptions{})

	if got := readFile(t, path); got != "alpha\nXXfoo\n" {
		t.Errorf("replace wrote at stale columns: %q", got)
	}
}

func TestSearchReplaceAllSkipsUnsavedAndChangedFiles(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	edited := filepath.Join(h.dir, "edited.txt")
	moved := filepath.Join(h.dir, "moved.txt")
	plain := filepath.Join(h.dir, "plain.txt")
	crlf := filepath.Join(h.dir, "crlf.txt")
	for path, text := range map[string]string{
		edited: "alpha\nfoo\n",
		moved:  "alpha\nXXfoo\n",
		plain:  "alpha\nfoo\n",
		crlf:   "alpha\r\nfoo\r\n",
	} {
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	h.app.EditorGroup.OpenFile(edited)
	h.app.Root.SetFocus(h.app.EditorGroup)
	h.redraw()
	h.pressRune('Z')
	unsaved := editorText(h)

	h.app.ApplySearchReplaceAll(map[string][]ui.SearchMatch{
		edited: fooMatch(edited),
		moved:  fooMatch(moved),
		plain:  fooMatch(plain),
		crlf:   fooMatch(crlf),
	}, "bar", ui.SearchOptions{})
	h.redraw()
	h.pressKey(tcell.KeyRight, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	if got := readFile(t, plain); got != "alpha\nbar\n" {
		t.Errorf("plain.txt = %q, want the match replaced", got)
	}
	if got := readFile(t, crlf); got != "alpha\r\nbar\r\n" {
		t.Errorf("crlf.txt = %q, want the match replaced", got)
	}
	if got := readFile(t, moved); got != "alpha\nXXfoo\n" {
		t.Errorf("moved.txt = %q, want it left alone", got)
	}
	if got := readFile(t, edited); got != "alpha\nfoo\n" {
		t.Errorf("edited.txt = %q, want it left alone", got)
	}
	if editorText(h) != unsaved {
		t.Errorf("unsaved edits were discarded: buffer %q", editorText(h))
	}
}
