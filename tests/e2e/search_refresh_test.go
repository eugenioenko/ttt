package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

func openSearchFixture(t *testing.T, h *testHarness) {
	t.Helper()
	path := filepath.Join(h.dir, "search.txt")
	if err := os.WriteFile(path, []byte("foo bar\nfoo baz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()
}

func TestSearchHighlightsFollowEdits(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openSearchFixture(t, h)

	opts := ui.SearchOptions{CaseSensitive: true}
	matches, _ := ui.FindInLines(h.app.EditorGroup.Editor.Buf.Lines, "foo", opts)
	h.app.EditorGroup.SetSearch("foo", opts, matches)
	h.app.Root.SetFocus(h.app.EditorGroup)

	for range 3 {
		h.pressKey(tcell.KeyDelete, tcell.ModNone)
	}

	got := h.app.EditorGroup.Editor.SearchMatches
	if len(got) != 1 || got[0].Line != 1 || got[0].Col != 0 {
		t.Fatalf("matches after deleting the first foo = %+v, want one match at 1:0", got)
	}
}

func TestReplaceBarMatchesFollowEdits(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openSearchFixture(t, h)

	h.app.OpenFindReplace()
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatal("replace bar did not open")
	}
	for _, r := range "foo" {
		h.pressRune(r)
	}
	if len(bar.Matches) != 2 {
		t.Fatalf("replace bar matches = %d, want 2", len(bar.Matches))
	}

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 0
	editor.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	h.flushOnChange()

	if len(bar.Matches) != 2 || bar.Matches[0].Line != 1 || bar.Matches[1].Line != 2 {
		t.Fatalf("replace bar matches after inserting a line = %+v, want lines 1 and 2", bar.Matches)
	}
}
