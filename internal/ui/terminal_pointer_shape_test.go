package ui

import (
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/xterm-go"
	"github.com/gdamore/tcell/v3"
)

func TestTerminalWidgetPointerShape_LinksAndModifiers(t *testing.T) {
	tw := NewTerminalWidget(nil, nil)
	tw.SetRect(Rect{X: 10, Y: 5, W: 40, H: 10})
	tw.linkCache = map[int][]linkSpan{
		1: {
			{StartCol: 5, EndCol: 25, URL: "https://example.com"},
			{StartCol: 28, EndCol: 38, IsFile: true, FilePath: "main.go"},
		},
	}

	tests := []struct {
		name     string
		x, y     int
		ctrlHeld bool
		want     string
	}{
		{"plain text without ctrl", 12, 6, false, term.PointerDefault},
		{"plain text with ctrl", 12, 6, true, term.PointerDefault},
		{"url link without ctrl", 16, 6, false, term.PointerDefault},
		{"url link with ctrl", 16, 6, true, term.PointerHand},
		{"file link without ctrl", 39, 6, false, term.PointerDefault},
		{"file link with ctrl", 39, 6, true, term.PointerHand},
		{"between links with ctrl", 36, 6, true, term.PointerDefault},
		{"outside widget bounds", 5, 5, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tw.ctrlHeld = tt.ctrlHeld
			if got := tw.PointerShapeAt(tt.x, tt.y); got != tt.want {
				t.Errorf("PointerShapeAt(%d, %d, ctrl=%v) = %q, want %q", tt.x, tt.y, tt.ctrlHeld, got, tt.want)
			}
		})
	}
}

func TestTerminalWidgetPointerShape_Scrollbar(t *testing.T) {
	tw := NewTerminalWidget(nil, nil)
	tw.SetRect(Rect{X: 0, Y: 0, W: 30, H: 10})
	tw.scrollbar.X = 29
	tw.scrollbar.Y = 0
	tw.scrollbar.Height = 10
	tw.scrollbar.TotalItems = 20
	tw.scrollbar.TopItem = 0

	tw.linkCache = map[int][]linkSpan{
		2: {{StartCol: 25, EndCol: 30, URL: "https://example.com"}},
	}

	// Scrollbar area retains neutral default shape even if link spans reach or ctrl is held
	tw.ctrlHeld = true
	if got := tw.PointerShapeAt(29, 2); got != term.PointerDefault {
		t.Errorf("over scrollbar with ctrl = %q, want %q", got, term.PointerDefault)
	}

	tw.ctrlHeld = false
	if got := tw.PointerShapeAt(29, 2); got != term.PointerDefault {
		t.Errorf("over scrollbar without ctrl = %q, want %q", got, term.PointerDefault)
	}

	// While dragging, pointer remains default even if cursor moves outside bounds
	tw.scrollbar.HandleEvent(tcell.NewEventMouse(29, 2, tcell.Button1, 0))
	if !tw.scrollbar.IsDragging() {
		// If HandleEvent didn't start drag due to geometry, set directly for test
		tw.scrollbar.dragging = true
	}
	if got := tw.PointerShapeAt(50, 50); got != term.PointerDefault {
		t.Errorf("while scrollbar dragging outside = %q, want %q", got, term.PointerDefault)
	}
}

func TestVerticalTabBarPointerShape(t *testing.T) {
	v := NewVerticalTabBar()
	v.Count = 3
	v.SetRect(Rect{X: 0, Y: 0, W: 5, H: 10})

	// Tab rows 0, 1, 2 show hand pointer
	for y := 0; y < 3; y++ {
		if got := v.PointerShapeAt(2, y); got != term.PointerHand {
			t.Errorf("tab row %d = %q, want %q", y, got, term.PointerHand)
		}
	}

	// Empty tab bar space below tabs shows default pointer
	for y := 3; y < 10; y++ {
		if got := v.PointerShapeAt(2, y); got != term.PointerDefault {
			t.Errorf("empty row %d = %q, want %q", y, got, term.PointerDefault)
		}
	}

	// Outside tab bar bounds returns empty string for parent routing
	if got := v.PointerShapeAt(10, 2); got != "" {
		t.Errorf("outside tab bar = %q, want empty string", got)
	}
}

func TestTerminalPanelWidgetPointerShape_Routing(t *testing.T) {
	tp := NewTerminalPanelWidget()
	tp.SetRect(Rect{X: 0, Y: 0, W: 60, H: 20})

	tw := NewTerminalWidget(nil, nil)
	tw.SetRect(Rect{X: 6, Y: 0, W: 50, H: 20})
	tw.scrollbar.X = 59
	tw.scrollbar.Y = 0
	tw.scrollbar.Height = 20
	tw.scrollbar.TotalItems = 30
	tw.scrollbar.TopItem = 0
	tw.linkCache = map[int][]linkSpan{
		3: {{StartCol: 4, EndCol: 24, URL: "https://github.com"}},
	}

	tp.AddTerminal(tw)
	tp.syncTabBar()

	// 1. Over vertical tab bar (x in 0..4, y=0 is tab 1)
	if got := tp.PointerShapeAt(2, 0); got != term.PointerHand {
		t.Errorf("tab 1 = %q, want %q", got, term.PointerHand)
	}
	if got := tp.PointerShapeAt(2, 5); got != term.PointerDefault {
		t.Errorf("empty tab area = %q, want %q", got, term.PointerDefault)
	}

	// 2. Over scrollbar (x=59)
	tw.ctrlHeld = true
	if got := tp.PointerShapeAt(59, 3); got != term.PointerDefault {
		t.Errorf("scrollbar with ctrl = %q, want %q", got, term.PointerDefault)
	}

	// 3. Over terminal link (tw.X=6, col=4 -> x=10, y=3)
	tw.ctrlHeld = false
	if got := tp.PointerShapeAt(10, 3); got != term.PointerDefault {
		t.Errorf("terminal link without ctrl = %q, want %q", got, term.PointerDefault)
	}

	tw.ctrlHeld = true
	if got := tp.PointerShapeAt(10, 3); got != term.PointerHand {
		t.Errorf("terminal link with ctrl = %q, want %q", got, term.PointerHand)
	}

	// 4. Over terminal non-link text (tw.X=6, col=0 -> x=6, y=3)
	if got := tp.PointerShapeAt(6, 3); got != term.PointerDefault {
		t.Errorf("terminal plain text with ctrl = %q, want %q", got, term.PointerDefault)
	}

	// 5. Over margin / padding (x=5)
	if got := tp.PointerShapeAt(5, 3); got != term.PointerDefault {
		t.Errorf("margin padding = %q, want %q", got, term.PointerDefault)
	}

	// 6. Outside panel rect
	if got := tp.PointerShapeAt(70, 5); got != "" {
		t.Errorf("outside panel = %q, want empty string", got)
	}
}

type captureShapeStubWidget struct {
	captureStubWidget
	shape string
}

func (s *captureShapeStubWidget) PointerShapeAt(mx, my int) string {
	return s.shape
}

func TestTerminalPanelWidgetPointerShape_CaptureFollowsActiveWidget(t *testing.T) {
	tp := NewTerminalPanelWidget()
	tp.SetRect(Rect{X: 0, Y: 0, W: 60, H: 20})

	stub := &captureShapeStubWidget{
		captureStubWidget: captureStubWidget{owns: true},
		shape:             term.PointerDefault,
	}
	tp.AddTerminal(stub)

	if got := tp.PointerShapeAt(100, 100); got != term.PointerDefault {
		t.Errorf("captured terminal outside bounds = %q, want %q", got, term.PointerDefault)
	}
}

func TestRootPointerShapeWithTerminalPanelAndOverlay(t *testing.T) {
	tp := NewTerminalPanelWidget()
	tp.SetRect(Rect{X: 0, Y: 0, W: 60, H: 20})

	tw := NewTerminalWidget(nil, nil)
	tw.SetRect(Rect{X: 6, Y: 0, W: 50, H: 20})
	tw.linkCache = map[int][]linkSpan{
		2: {{StartCol: 5, EndCol: 25, URL: "https://example.com"}},
	}
	tw.ctrlHeld = true
	tp.AddTerminal(tw)

	root := NewRoot(tp)

	// Link with Ctrl shows hand pointer through Root
	if got := root.PointerShapeAt(11, 2); got != term.PointerHand {
		t.Fatalf("root over link with ctrl = %q, want %q", got, term.PointerHand)
	}

	// Plain text without link shows default through Root
	if got := root.PointerShapeAt(7, 2); got != term.PointerDefault {
		t.Fatalf("root over plain text = %q, want %q", got, term.PointerDefault)
	}

	// Overlay suppresses terminal underneath
	root.PushOverlay(Overlay{Widget: &shapeWidget{shape: term.PointerText}, Modal: true})
	if got := root.PointerShapeAt(11, 2); got != term.PointerText {
		t.Fatalf("root covered by shaped overlay = %q, want %q", got, term.PointerText)
	}

	root.PushOverlay(Overlay{Widget: &mockWidget{}, Modal: false})
	if got := root.PointerShapeAt(11, 2); got != term.PointerDefault {
		t.Fatalf("root covered by shapeless overlay = %q, want %q", got, term.PointerDefault)
	}
}

func TestTerminalWidgetRenderDoesNotTakeNestedSnapshot(t *testing.T) {
	termDev, err := terminal.New("", 40, 10, 1000, nil, "")
	if err != nil {
		t.Skipf("skipping terminal test: %v", err)
	}
	defer termDev.Close()

	termDev.Snapshot(func(xt *xterm.Terminal) {
		xt.WriteString("https://example.com/test\r\nplain text\r\n")
	})

	tw := NewTerminalWidget(termDev, &TerminalColorPalette{})
	tw.SetRect(Rect{W: 40, H: 10})
	tw.ctrlHeld = false

	cells := make([][]term.Cell, 10)
	for y := range cells {
		cells[y] = make([]term.Cell, 40)
	}
	surface := NewRenderSurface(cells, Rect{W: 40, H: 10})

	done := make(chan struct{})
	go func() {
		tw.Render(surface)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("tw.Render deadlocked, likely taking nested terminal Snapshot")
	}

	if cells[0][0].Attrs&term.CellAttrUnderline != 0 {
		t.Error("expected no underline when ctrlHeld is false")
	}
}
