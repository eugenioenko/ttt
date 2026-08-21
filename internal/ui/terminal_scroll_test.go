package ui

import (
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/gdamore/tcell/v3"
)

// TestScrollbarDragToBottomGoesLiveDespiteStaleTotalItems guards against a
// regression where dragging the terminal scrollbar to the end of the track
// left scrollOffset slightly above zero whenever content streamed in after
// the widget's last Render() call — the drag math resolved against a
// TotalItems snapshot that was already behind the live PTY, so the user
// could never precisely reach the bottom while output kept arriving.
func TestScrollbarDragToBottomGoesLiveDespiteStaleTotalItems(t *testing.T) {
	term, err := terminal.New("/bin/cat", 20, 5, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	term.Run()

	for i := 0; i < 10; i++ {
		term.WriteString("line\n")
	}
	deadline := time.Now().Add(2 * time.Second)
	for term.ScrollbackLen() <= 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if term.ScrollbackLen() <= 3 {
		t.Fatalf("setup: expected live ScrollbackLen() > 3, got %d", term.ScrollbackLen())
	}

	tw := NewTerminalWidget(term, nil)
	tw.SetRect(Rect{X: 0, Y: 0, W: 20, H: 5})

	// Simulate a stale Render() snapshot from before the lines above arrived:
	// only 3 lines of scrollback existed then, vs. the live count checked above.
	tw.scrollbar.X = 19
	tw.scrollbar.Y = 0
	tw.scrollbar.Height = 5
	tw.scrollbar.TotalItems = 8 // sbLen(3) + rows(5)
	tw.scrollbar.TopItem = 0
	tw.scrollOffset = 3

	ev := tcell.NewEventMouse(19, 4, tcell.Button1, tcell.ModNone)
	tw.HandleEvent(ev)

	if tw.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0 after dragging to the scrollbar's bottom", tw.scrollOffset)
	}
}
