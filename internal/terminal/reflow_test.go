package terminal

import (
	"fmt"
	"strings"
	"testing"

	xterm "github.com/gitpod-io/xterm-go"
)

// Narrowing a terminal whose scrollback is full makes xterm-go's reflow produce
// more lines than the circular list can hold, which used to panic with
// "index out of range [-1]". See trimForReflow.
func TestTrimForReflowSurvivesOverflowingNarrow(t *testing.T) {
	for _, newCols := range []int{2, 5, 20, 40, 79} {
		t.Run(fmt.Sprintf("cols=%d", newCols), func(t *testing.T) {
			term := xterm.New(xterm.WithCols(80), xterm.WithRows(24), xterm.WithScrollback(1000))
			for i := range 200 {
				term.WriteString(fmt.Sprintf("%d:%s\r\n", i, strings.Repeat("x", 300)))
			}

			trimForReflow(term.NormalBuffer(), 80, 24, newCols, 35)
			term.Resize(newCols, 35)

			if got := term.Cols(); got != newCols {
				t.Fatalf("Cols() = %d, want %d", got, newCols)
			}
			buf := term.NormalBuffer()
			if buf.Lines.Length() > buf.Lines.MaxLength() {
				t.Fatalf("buffer overflowed: length %d > max %d", buf.Lines.Length(), buf.Lines.MaxLength())
			}
			for i := range buf.Lines.Length() {
				if buf.Lines.Get(i) == nil {
					t.Fatalf("line %d is nil after reflow", i)
				}
			}
		})
	}
}

// The trim must only ever drop scrollback that has scrolled off, never lines the
// viewport still shows.
func TestTrimForReflowKeepsViewport(t *testing.T) {
	term := xterm.New(xterm.WithCols(80), xterm.WithRows(24), xterm.WithScrollback(1000))
	for i := range 200 {
		term.WriteString(fmt.Sprintf("%d:%s\r\n", i, strings.Repeat("x", 300)))
	}
	term.WriteString("last line marker")

	trimForReflow(term.NormalBuffer(), 80, 24, 20, 24)
	term.Resize(20, 24)

	if !strings.Contains(term.String(), "last line marker") {
		t.Fatal("viewport content was trimmed away")
	}
}

// A widening resize reflows in the other direction and must not trim anything.
func TestTrimForReflowIgnoresWidening(t *testing.T) {
	term := xterm.New(xterm.WithCols(40), xterm.WithRows(24), xterm.WithScrollback(1000))
	for i := range 100 {
		term.WriteString(fmt.Sprintf("%d:%s\r\n", i, strings.Repeat("y", 100)))
	}

	before := term.NormalBuffer().Lines.Length()
	trimForReflow(term.NormalBuffer(), 40, 24, 120, 24)
	if after := term.NormalBuffer().Lines.Length(); after != before {
		t.Fatalf("widening trimmed the buffer: %d lines before, %d after", before, after)
	}
}

// Resize is the door every caller comes through, so the guard has to hold there
// too, including for the degenerate sizes a collapsed panel reports.
func TestResizeSurvivesNarrowingWithFullScrollback(t *testing.T) {
	term := newTestTerminal(t)
	defer term.Close()

	term.Snapshot(func(x *xterm.Terminal) {
		for i := range 200 {
			x.WriteString(fmt.Sprintf("%d:%s\r\n", i, strings.Repeat("x", 300)))
		}
	})

	for _, size := range [][2]int{{40, 35}, {2, 35}, {1, 1}, {0, 0}, {-5, -5}, {80, 24}} {
		term.Resize(size[0], size[1])
	}

	if cols, rows := term.Size(); cols != 80 || rows != 24 {
		t.Fatalf("Size() = %d,%d, want 80,24", cols, rows)
	}
}
