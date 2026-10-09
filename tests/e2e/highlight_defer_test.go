package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cellStyleAt returns the style of the first cell of label on screen.
func cellStyleAt(t *testing.T, h *testHarness, label string) string {
	t.Helper()
	cells, w, ht := h.screen.GetContents()
	for y := range ht {
		row := h.screenRow(y)
		if x := displayColumnOf(row, label); x >= 0 {
			return fmt.Sprint(cells[y*w+x].Style)
		}
	}
	t.Fatalf("%q not on screen:\n%s", label, h.screenText())
	return ""
}

// Jumping to the end of a large file draws it at once and colors it once the
// deferred steps run, instead of tokenizing every line above in that frame.
func TestJumpIntoLargeFileDefersHighlighting(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	var src strings.Builder
	src.WriteString("package big\n\n")
	for i := range 3000 {
		fmt.Fprintf(&src, "var value%d = %d\n", i, i)
	}
	path := filepath.Join(h.dir, "big.go")
	if err := os.WriteFile(path, []byte(src.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	ed := h.app.EditorGroup.Editor
	ed.Cursor.Line = len(ed.Buf.Lines) - 2
	ed.EnsureCursorVisible()
	h.redraw()

	h.assertContains("var value2999 = 2999")
	if keyword, name := cellStyleAt(t, h, "var value2999"), cellStyleAt(t, h, "value2999 ="); keyword != name {
		t.Fatalf("line was highlighted in the jump frame (var %s, name %s)", keyword, name)
	}
	if len(h.highlightSteps) == 0 {
		t.Fatal("no highlighting step was scheduled for the deferred lines")
	}

	h.settle()

	if keyword, name := cellStyleAt(t, h, "var value2999"), cellStyleAt(t, h, "value2999 ="); keyword == name {
		t.Fatalf("line still uncolored after the deferred steps (both %s)", keyword)
	}
}
