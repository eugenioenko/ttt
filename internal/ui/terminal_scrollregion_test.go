package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/vt10x"
)

// TestScrolledRenderKeepsRowsOutsideScrollRegionLive guards against a
// regression where scrolling the terminal widget into history treated the
// whole screen as one flat scrollable sequence. Full-screen apps that pin a
// status/input bar outside their DECSTBM scroll region (chat CLIs, htop-style
// tools) rely on those rows never scrolling -- vt10x already respects the
// region internally, but the widget ignored it once scrollOffset was
// nonzero, leaving the fixed row blank instead of showing its live content.
func TestScrolledRenderKeepsRowsOutsideScrollRegionLive(t *testing.T) {
	cols, rows := 20, 6
	tm, err := terminal.New("/bin/cat", cols, rows, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer tm.Close()
	tm.Run()

	// Scroll region rows 1-5 (0-indexed 0-4), leaving row 6 (index 5) fixed.
	tm.WriteString("\x1b[1;5r\x1b[2J\x1b[6;1HFIXED")
	for i := 0; i < 20; i++ {
		tm.WriteString(fmt.Sprintf("\x1b[5;1Hline %02d\n", i))
	}
	tm.WriteString("\x1b[6;1HFIXED")

	deadline := time.Now().Add(2 * time.Second)
	for !strings.HasPrefix(liveRowText(tm, rows-1, cols), "FIXED") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := liveRowText(tm, rows-1, cols); !strings.HasPrefix(got, "FIXED") {
		t.Fatalf("setup: fixed row never settled to FIXED, got %q", got)
	}
	if tm.ScrollbackLen() == 0 {
		t.Fatal("setup: expected scrollback content from the scroll-region loop")
	}

	tw := NewTerminalWidget(tm, &TerminalColorPalette{})
	tw.SetRect(Rect{X: 0, Y: 0, W: cols, H: rows})
	tw.scrollOffset = tm.ScrollbackLen()

	cells := make([][]term.Cell, rows)
	for y := range cells {
		cells[y] = make([]term.Cell, cols)
	}
	surface := NewRenderSurface(cells, Rect{X: 0, Y: 0, W: cols, H: rows})
	tw.Render(surface)

	got := cellsToString(cells[rows-1][:5])
	if got != "FIXED" {
		t.Errorf("fixed row (outside scroll region) while scrolled = %q, want %q", got, "FIXED")
	}
}

func liveRowText(tm *terminal.Terminal, row, cols int) string {
	var s string
	tm.Snapshot(func(view vt10x.View) {
		runes := make([]rune, cols)
		for x := 0; x < cols; x++ {
			ch := view.Cell(x, row).Char
			if ch == 0 {
				ch = ' '
			}
			runes[x] = ch
		}
		s = string(runes)
	})
	return s
}

func cellsToString(cells []term.Cell) string {
	runes := make([]rune, len(cells))
	for i, c := range cells {
		ch := c.Ch
		if ch == 0 {
			ch = ' '
		}
		runes[i] = ch
	}
	return string(runes)
}
