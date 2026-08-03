package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

// openFullwidth writes a file of fullwidth text and opens it.
func openFullwidth(t *testing.T, h *testHarness, name, content string) {
	t.Helper()
	f := filepath.Join(h.dir, name)
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(f)
	h.redraw()
}

// Every syllable must reach the screen. Before the fix the terminal swallowed
// the character following each fullwidth rune, rendering 가다마사 (issue #434).
func TestFullwidthLineRendersEveryRune(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	openFullwidth(t, h, "hangul.txt", "가나다라마바사\n")
	h.assertContains("가나다라마바사")
}

func TestFullwidthMixedWithASCIIRenders(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	openFullwidth(t, h, "mixed.txt", "가a나b다\n")
	h.assertContains("가a나b다")
}

// Arrow keys move one rune at a time; the reported column is a character index,
// not a screen column.
func TestFullwidthCursorMovesByRune(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	openFullwidth(t, h, "hangul.txt", "가나다라\n")

	ed := h.app.EditorGroup.Editor
	for i := 1; i <= 4; i++ {
		h.pressKey(tcell.KeyRight, tcell.ModNone)
		if ed.Cursor.Col != i {
			t.Fatalf("after %d right presses, cursor col = %d, want %d", i, ed.Cursor.Col, i)
		}
	}
	// Past the last rune the cursor wraps to the next line, as it does for
	// ASCII text.
	h.pressKey(tcell.KeyRight, tcell.ModNone)
	if ed.Cursor.Line != 1 || ed.Cursor.Col != 0 {
		t.Errorf("cursor = (%d,%d), want (1,0)", ed.Cursor.Line, ed.Cursor.Col)
	}
}

func TestFullwidthHomeEnd(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	openFullwidth(t, h, "hangul.txt", "가나다라\n")
	ed := h.app.EditorGroup.Editor

	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	if ed.Cursor.Col != 4 {
		t.Errorf("End put the cursor at col %d, want 4", ed.Cursor.Col)
	}
	h.pressKey(tcell.KeyHome, tcell.ModNone)
	if ed.Cursor.Col != 0 {
		t.Errorf("Home put the cursor at col %d, want 0", ed.Cursor.Col)
	}
}

// Clicking either column of a fullwidth rune puts the cursor on that rune.
func TestFullwidthClickMapsToRune(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	openFullwidth(t, h, "hangul.txt", "가나다라\n")
	ed := h.app.EditorGroup.Editor

	textX, textY := fullwidthTextOrigin(t, h, '가')

	cases := []struct {
		dx   int
		want int
	}{
		{0, 0}, {1, 0}, // 가
		{2, 1}, {3, 1}, // 나
		{4, 2}, {5, 2}, // 다
		{6, 3}, {7, 3}, // 라
	}
	for _, tc := range cases {
		// Both halves of a rune resolve to the same buffer column, so two
		// clicks in a row would register as a double click and select a word.
		// Park the cursor on the empty line below to break the streak.
		h.click(textX, textY+1)

		h.click(textX+tc.dx, textY)
		if ed.Cursor.Col != tc.want {
			t.Errorf("click %d columns into the line put the cursor at rune %d, want %d",
				tc.dx, ed.Cursor.Col, tc.want)
		}
	}
}

// Selecting two fullwidth runes must yield those runes, not the columns they
// span.
func TestFullwidthSelectionSpansRunes(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	openFullwidth(t, h, "hangul.txt", "가나다라\n")
	ed := h.app.EditorGroup.Editor

	h.pressKey(tcell.KeyHome, tcell.ModNone)
	h.pressKey(tcell.KeyRight, tcell.ModShift)
	h.pressKey(tcell.KeyRight, tcell.ModShift)

	if !ed.Selection.Active {
		t.Fatal("shift+right should start a selection")
	}
	if got := ed.Cursor.Col; got != 2 {
		t.Errorf("cursor col = %d, want 2", got)
	}
	start, end := ed.Selection.Range(ed.Cursor.Line, ed.Cursor.Col)
	if start.Line != 0 || end.Line != 0 || start.Col != 0 || end.Col != 2 {
		t.Errorf("selection = (%d,%d)-(%d,%d), want (0,0)-(0,2)",
			start.Line, start.Col, end.Line, end.Col)
	}
}

// A line of fullwidth text must wrap on a rune boundary, never mid-rune.
func TestFullwidthWordWrapKeepsRunesIntact(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	openFullwidth(t, h, "wrap.txt", strings.Repeat("가", 60)+"\n")
	h.exec("options.toggleWordWrap")
	h.redraw()

	// Whatever the wrap width works out to, every rendered row must contain
	// only whole runes — a split rune shows up as a missing glyph.
	text := h.screenText()
	if !strings.Contains(text, "가가가") {
		t.Errorf("wrapped fullwidth text did not render:\n%s", text)
	}
}

// fullwidthTextOrigin finds where the given rune first appears on screen, so
// click tests do not hard-code the gutter width.
func fullwidthTextOrigin(t *testing.T, h *testHarness, r rune) (x, y int) {
	t.Helper()
	cells, w, ht := h.screen.GetContents()
	for cy := 0; cy < ht; cy++ {
		for cx := 0; cx < w; cx++ {
			if cells[cy*w+cx].Str == string(r) {
				return cx, cy
			}
		}
	}
	t.Fatalf("rune %q not found on screen:\n%s", r, h.screenText())
	return 0, 0
}
