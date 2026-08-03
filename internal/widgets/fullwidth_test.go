package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

// rowText reads a surface row back the way the terminal displays it: the column
// covered by a fullwidth rune is skipped, since the terminal paints it from the
// rune itself and never draws whatever the cell holds. This mirrors what
// app.Screenshot does with tcell's GetContent.
func rowText(s *testSurface, y int) string {
	var b strings.Builder
	for x := 0; x < s.w; x++ {
		ch := s.cells[y][x].Ch
		if ch == 0 {
			ch = ' '
		}
		b.WriteRune(ch)
		if textwidth.Rune(ch) > 1 {
			x++
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// defaultPrefixW is the width of the " ❯ " prefix NewInputWidget applies to
// borderless inputs.
const defaultPrefixW = 3

func TestDrawRunesClippedFullwidth(t *testing.T) {
	s := newTestSurface(20, 1)
	end := drawRunesClipped(s, 0, 0, 20, []rune("가a나"), term.StyleDefault)
	if end != 5 {
		t.Fatalf("end = %d, want 5", end)
	}
	for x, want := range map[int]rune{0: '가', 2: 'a', 3: '나'} {
		if s.cells[0][x].Ch != want {
			t.Errorf("cell %d = %q, want %q", x, s.cells[0][x].Ch, want)
		}
	}
}

func TestDrawRunesClippedFullwidthEllipsis(t *testing.T) {
	s := newTestSurface(20, 1)
	// 가나다 needs 6 columns but only 4 are available.
	drawRunesClipped(s, 0, 0, 4, []rune("가나다"), term.StyleDefault)
	if s.cells[0][0].Ch != '가' {
		t.Errorf("cell 0 = %q, want 가", s.cells[0][0].Ch)
	}
	if s.cells[0][2].Ch != '…' {
		t.Errorf("cell 2 = %q, want an ellipsis", s.cells[0][2].Ch)
	}
	if s.cells[0][4].Ch != 0 {
		t.Error("drawing ran past maxX")
	}
}

func TestTruncateRunesLeftFullwidth(t *testing.T) {
	// 5 columns available: the ellipsis takes 1, leaving 4 for two runes.
	got := string(truncateRunesLeft([]rune("가나다라"), 5))
	if got != "…다라" {
		t.Errorf("got %q, want %q", got, "…다라")
	}
	// Fits already: returned unchanged.
	if got := string(truncateRunesLeft([]rune("가나"), 4)); got != "가나" {
		t.Errorf("got %q, want %q", got, "가나")
	}
}

func TestInputRendersFullwidthText(t *testing.T) {
	s := newTestSurface(20, 1)
	inp := NewInputWidget(InputConfig{})
	inp.SetRect(Rect{X: 0, Y: 0, W: 20, H: 1})
	inp.SetText("가나다")
	inp.Render(s)

	if got := rowText(s, 0); !strings.Contains(got, "가나다") {
		t.Errorf("row = %q, want it to contain 가나다", got)
	}
}

// The caret sits after the text in columns, not in runes: three fullwidth runes
// are six columns wide, not three.
func TestInputCursorPositionFullwidth(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetRect(Rect{X: 0, Y: 0, W: 20, H: 1})
	inp.SetFocused(true)
	inp.SetText("가나다")

	x, _, visible := inp.CursorPosition()
	if !visible {
		t.Fatal("cursor should be visible when focused")
	}
	if want := defaultPrefixW + 6; x != want {
		t.Errorf("cursor x = %d, want %d", x, want)
	}
}

func TestInputClickMapsColumnToRune(t *testing.T) {
	cases := []struct {
		col  int
		want int
	}{
		{0, 0}, // first half of 가
		{1, 0}, // second half of 가 selects the same rune
		{2, 1}, // first half of 나
		{3, 1},
		{4, 2},
		{9, 3}, // past the end clamps to the rune count
	}
	for _, tc := range cases {
		inp := NewInputWidget(InputConfig{})
		inp.SetRect(Rect{X: 0, Y: 0, W: 20, H: 1})
		inp.SetFocused(true)
		inp.SetText("가나다")

		ev := tcell.NewEventMouse(defaultPrefixW+tc.col, 0, tcell.Button1, 0)
		if res := inp.HandleEvent(ev); res != EventConsumed {
			t.Fatalf("column %d: click not consumed", tc.col)
		}
		if inp.cursorPos != tc.want {
			t.Errorf("click at column %d put the cursor at rune %d, want %d", tc.col, inp.cursorPos, tc.want)
		}
	}
}

// A field narrower than its text must scroll by columns; a fullwidth rune that
// does not fit in the remaining columns is dropped rather than half-drawn.
func TestInputScrollsByColumns(t *testing.T) {
	// The widget fills the surface it is handed: prefix plus 6 columns of text,
	// which the 8-column string cannot fit.
	s := newTestSurface(defaultPrefixW+6, 1)
	inp := NewInputWidget(InputConfig{})
	inp.SetRect(Rect{X: 0, Y: 0, W: defaultPrefixW + 6, H: 1})
	inp.SetText("가나다라")
	inp.Render(s)

	row := rowText(s, 0)
	if strings.Contains(row, "가") {
		t.Errorf("row = %q: the field should have scrolled past the first rune", row)
	}
	if !strings.Contains(row, "라") {
		t.Errorf("row = %q, want the rune at the cursor to stay visible", row)
	}
}
