package terminal

import (
	"fmt"
	"strings"
	"testing"

	xterm "github.com/eugenioenko/xterm-go"
)

// Narrowing with full scrollback used to panic in xterm-go's reflow with
// "index out of range [-1]" (fixed in eugenioenko/xterm-go v0.1.0). Resize is
// the door every caller comes through, including for the degenerate sizes a
// collapsed panel reports.
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
