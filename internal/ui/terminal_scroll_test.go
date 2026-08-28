package ui

import (
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

// Guards against dragging to the bottom resolving against a stale TotalItems
// snapshot instead of going live against the streaming PTY.
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

	// Stale snapshot: only 3 lines of scrollback existed at last Render().
	staleRange := widgets.NewScrollRange(5, 8, 0) // sbLen(3) + rows(5)
	tw.scrollbarMaxTop = staleRange.MaxOffset()
	tw.scrollbar.Render(
		NewRenderSurface(makeGrid(20, 5), Rect{W: 20, H: 5}),
		widgets.NewScrollbarGeometry(
			Rect{X: 19, Y: 0, W: 1, H: 5},
			Rect{X: 19, Y: 0, W: 1, H: 5},
		),
		staleRange,
	)
	tw.scrollOffset = 3

	ev := tcell.NewEventMouse(19, 4, tcell.Button1, tcell.ModNone)
	if got := tw.HandleEvent(ev); got != EventCaptured {
		t.Fatalf("scrollbar press = %v, want captured", got)
	}

	if tw.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0 after dragging to the scrollbar's bottom", tw.scrollOffset)
	}
	if got := tw.HandleEvent(tcell.NewEventMouse(50, 50, tcell.ButtonNone, 0)); got != EventConsumed {
		t.Fatalf("owned off-widget release = %v, want consumed", got)
	}

	tw.scrollOffset = 3
	if got := tw.HandleEvent(ev); got != EventCaptured {
		t.Fatalf("second scrollbar press = %v, want captured", got)
	}
	invalidations := 0
	widgets.SetPointerCaptureInvalidated(tw, func() { invalidations++ })
	if !widgets.InvalidatePointerInteraction(tw) {
		t.Fatal("terminal did not invalidate scrollbar capture")
	}
	if invalidations != 1 {
		t.Fatalf("terminal invalidations = %d, want 1", invalidations)
	}
}
