package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func renderStatusBar(t *testing.T, sb *view.StatusBar, w int) (*StatusBarWidget, string) {
	t.Helper()
	grid := makeGrid(w, 1)
	widget := NewStatusBarWidget(sb)
	widget.SetRect(widgets.Rect{X: 0, Y: 0, W: w, H: 1})
	widget.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: w, H: 1}))
	return widget, gridText(grid)
}

func TestStatusBarAttachedSegment(t *testing.T) {
	sb := view.NewStatusBar()
	sb.SetSegment(view.StatusSegment{ID: "pos", Side: "right", Priority: 100, Text: "Ln 1"})
	sb.SetSegment(view.StatusSegment{ID: "language", Side: "right", Priority: 500, Text: "Go", OnClick: func() {}})
	sb.SetSegment(view.StatusSegment{ID: "lsp", Side: "right", Priority: 501, Text: "◉", Attached: true, OnClick: func() {}})

	widget, text := renderStatusBar(t, sb, 40)
	if !strings.HasSuffix(text, "Ln 1   Go ◉ ") {
		t.Fatalf("status bar = %q, want attached icon one column after the language", text)
	}

	langX := strings.Index(text, "Go")
	iconX := langX + 3
	if len(widget.spans) != 2 {
		t.Fatalf("spans = %v, want 2", widget.spans)
	}
	if s := widget.spans[0]; s.start != langX || s.end != langX+2 {
		t.Errorf("language span = [%d, %d), want [%d, %d)", s.start, s.end, langX, langX+2)
	}
	if s := widget.spans[1]; s.start != iconX || s.end != iconX+1 {
		t.Errorf("icon span = [%d, %d), want [%d, %d)", s.start, s.end, iconX, iconX+1)
	}
}

func TestStatusBarEmptyRightSegmentAddsNoGap(t *testing.T) {
	sb := view.NewStatusBar()
	sb.SetSegment(view.StatusSegment{ID: "pos", Side: "right", Priority: 100, Text: "Ln 1"})
	sb.SetSegment(view.StatusSegment{ID: "empty", Side: "right", Priority: 200, Text: ""})
	sb.SetSegment(view.StatusSegment{ID: "language", Side: "right", Priority: 500, Text: "Go"})
	sb.SetSegment(view.StatusSegment{ID: "lsp", Side: "right", Priority: 501, Text: "", Attached: true})

	_, text := renderStatusBar(t, sb, 40)
	if !strings.HasSuffix(text, " Ln 1   Go ") {
		t.Fatalf("status bar = %q, want empty segments to take no space", text)
	}
}
