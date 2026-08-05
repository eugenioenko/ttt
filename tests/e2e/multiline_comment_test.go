package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

// findRow returns the screen row whose text contains want, or -1.
func findRow(h *testHarness, want string) int {
	_, _, ht := h.screen.GetContents()
	for y := 0; y < ht; y++ {
		if containsSubstring(h.screenRow(y), want) {
			return y
		}
	}
	return -1
}

func containsSubstring(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// styleOfRune returns the style of the first cell on row y holding r.
func styleOfRune(h *testHarness, y int, r rune) (tcell.Style, bool) {
	cells, w, _ := h.screen.GetContents()
	for x := 0; x < w; x++ {
		if runeAt(cells, y*w+x) == r {
			return cells[y*w+x].Style, true
		}
	}
	return tcell.StyleDefault, false
}

// TestMultilineCommentIsStyledAcrossLines is the regression test for multiline
// block comments being ignored: only the line holding the opener was styled as
// a comment, because each line was lexed in isolation.
func TestMultilineCommentIsStyledAcrossLines(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	fp := filepath.Join(h.dir, "sample.js")
	content := `const before = 1;
/* opener
   MIDDLEWORD
*/
const after = 2;
`
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(fp)
	h.redraw()

	h.assertContains("MIDDLEWORD")

	// Reference: the styling of the opener line, which worked before the fix.
	openRow := findRow(h, "/* opener")
	if openRow < 0 {
		t.Fatal("opener line not on screen")
	}
	commentStyle, ok := styleOfRune(h, openRow, 'p') // from "opener"
	if !ok {
		t.Fatal("could not sample opener line style")
	}

	// The interior line must carry that same comment style.
	midRow := findRow(h, "MIDDLEWORD")
	if midRow < 0 {
		t.Fatal("interior line not on screen")
	}
	midStyle, ok := styleOfRune(h, midRow, 'W')
	if !ok {
		t.Fatal("could not sample interior line style")
	}
	if midStyle != commentStyle {
		t.Errorf("interior comment line not styled as a comment: got %v want %v", midStyle, commentStyle)
	}

	// The closer line too.
	closeRow := findRow(h, "*/")
	if closeRow < 0 {
		t.Fatal("closer line not on screen")
	}
	closeStyle, ok := styleOfRune(h, closeRow, '/')
	if !ok {
		t.Fatal("could not sample closer line style")
	}
	if closeStyle != commentStyle {
		t.Errorf("closer line not styled as a comment: got %v want %v", closeStyle, commentStyle)
	}

	// Code after the comment closes must NOT be comment styled.
	afterRow := findRow(h, "const after")
	if afterRow < 0 {
		t.Fatal("trailing code line not on screen")
	}
	afterStyle, ok := styleOfRune(h, afterRow, 'c')
	if !ok {
		t.Fatal("could not sample trailing code style")
	}
	if afterStyle == commentStyle {
		t.Error("code after the comment closed is still styled as a comment")
	}
}

// TestCommentStateUpdatesAfterEdit checks the state table is rebuilt when the
// buffer changes: deleting the closer should extend the comment downward.
func TestCommentStateUpdatesAfterEdit(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	fp := filepath.Join(h.dir, "edit.js")
	content := `/* opener
*/
const TAILWORD = 2;
`
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(fp)
	h.redraw()

	openRow := findRow(h, "/* opener")
	commentStyle, ok := styleOfRune(h, openRow, 'p')
	if !ok {
		t.Fatal("could not sample comment style")
	}

	tailRow := findRow(h, "TAILWORD")
	before, ok := styleOfRune(h, tailRow, 'T')
	if !ok {
		t.Fatal("could not sample tail style")
	}
	if before == commentStyle {
		t.Fatal("precondition: tail should not start as a comment")
	}

	// Delete the closer line so the comment never closes.
	ed := h.app.EditorGroup.Editor
	ed.Cursor.Line = 1
	ed.Cursor.Col = 0
	h.exec("editor.deleteLine")
	h.flushOnChange()
	h.redraw()

	tailRow = findRow(h, "TAILWORD")
	if tailRow < 0 {
		t.Fatal("tail line not on screen after edit")
	}
	after, ok := styleOfRune(h, tailRow, 'T')
	if !ok {
		t.Fatal("could not sample tail style after edit")
	}
	if after != commentStyle {
		t.Errorf("removing the closer should extend the comment: got %v want %v", after, commentStyle)
	}
}
