package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"testing"

	"github.com/gdamore/tcell/v3"
)

type mockWidget struct {
	BaseWidget
	lastEvent  tcell.Event
	eventCount int
	rendered   bool
	focusable  bool
	renderChar rune
}

func (m *mockWidget) HandleEvent(ev tcell.Event) EventResult {
	m.lastEvent = ev
	m.eventCount++
	return EventConsumed
}

func (m *mockWidget) Render(surface Surface) {
	m.rendered = true
	if m.renderChar != 0 {
		surface.Fill(term.Cell{Ch: m.renderChar})
	}
}

func (m *mockWidget) Focusable() bool { return m.focusable }

type passThroughWidget struct {
	BaseWidget
	eventCount int
}

func (m *passThroughWidget) HandleEvent(ev tcell.Event) EventResult {
	m.eventCount++
	return EventIgnored
}

func (m *passThroughWidget) Render(surface Surface) {}
func (m *passThroughWidget) Focusable() bool        { return true }

func makeKeyEvent(key tcell.Key, mod tcell.ModMask) *tcell.EventKey {
	return tcell.NewEventKey(key, "", mod)
}

func TestRootRoutesToFocused(t *testing.T) {
	main := &mockWidget{}
	focused := &mockWidget{focusable: true}

	root := NewRoot(main)
	root.SetFocus(focused)
	root.SetSize(80, 24)

	ev := makeKeyEvent(tcell.KeyUp, 0)
	root.HandleEvent(ev)

	if focused.eventCount != 1 {
		t.Fatal("event not routed to focused widget")
	}
	if main.eventCount != 0 {
		t.Fatal("event incorrectly routed to main widget")
	}
}

func TestRootSetSizeClearsBothDividerCaptures(t *testing.T) {
	content := NewContentSplitWidget()
	content.Top = &mockWidget{}
	content.Bottom = &mockWidget{}
	content.dragging = true
	split := NewSplitPanelWidget()
	split.Left = &mockWidget{}
	split.Right = content
	split.dragging = true
	main := widgets.NewVStackWidget(split)
	root := NewRoot(main)
	root.Width = 80
	root.Height = 24
	root.capturedWidget = main

	root.SetSize(100, 30)
	if split.dragging || content.dragging {
		t.Fatalf("resize retained divider capture: split=%v content=%v", split.dragging, content.dragging)
	}
	if root.PointerCaptureActive() {
		t.Fatal("resize retained Root capture")
	}
}

func TestRootModalOverlayCaptures(t *testing.T) {
	main := &mockWidget{}
	focused := &mockWidget{focusable: true}
	modal := &mockWidget{focusable: true}

	root := NewRoot(main)
	root.SetFocus(focused)
	root.PushOverlay(Overlay{Widget: modal, Modal: true})

	ev := makeKeyEvent(tcell.KeyUp, 0)
	root.HandleEvent(ev)

	if modal.eventCount != 1 {
		t.Fatal("modal overlay did not receive event")
	}
	if focused.eventCount != 0 {
		t.Fatal("focused widget received event despite modal overlay")
	}
}

func TestRootModalOverlayPropagatesPointerCapture(t *testing.T) {
	main := &mockWidget{}
	overlay := &notificationCaptureProbe{}
	root := NewRoot(main)
	root.SetSize(80, 24)
	root.PushOverlay(Overlay{Widget: overlay, Modal: true})

	root.HandleEvent(tcell.NewEventMouse(10, 10, tcell.Button1, 0))
	if !root.PointerCaptureActive() || !overlay.OwnsPointerCapture() {
		t.Fatal("modal overlay capture did not reach Root")
	}

	root.SetSize(100, 30)
	if root.PointerCaptureActive() || overlay.OwnsPointerCapture() {
		t.Fatal("resize retained modal overlay capture")
	}
}

func TestRootGlobalKeysFire(t *testing.T) {
	main := &mockWidget{}
	focused := &passThroughWidget{}
	fired := false

	root := NewRoot(main)
	root.SetFocus(focused)
	root.AddGlobalKey(tcell.KeyCtrlB, tcell.ModCtrl, 0, func() { fired = true })

	ev := makeKeyEvent(tcell.KeyCtrlB, tcell.ModCtrl)
	root.HandleEvent(ev)

	if !fired {
		t.Fatal("global key handler did not fire")
	}
}

func TestRootFocusedWidgetBlocksGlobalKey(t *testing.T) {
	main := &mockWidget{}
	focused := &mockWidget{focusable: true}
	fired := false

	root := NewRoot(main)
	root.SetFocus(focused)
	root.AddGlobalKey(tcell.KeyCtrlB, tcell.ModCtrl, 0, func() { fired = true })

	ev := makeKeyEvent(tcell.KeyCtrlB, tcell.ModCtrl)
	root.HandleEvent(ev)

	if fired {
		t.Fatal("global key fired despite focused widget consuming the event")
	}
	if focused.eventCount != 1 {
		t.Fatal("focused widget did not receive the event")
	}
}

func TestRootOverlayRendersOnTop(t *testing.T) {
	main := &mockWidget{renderChar: 'M'}
	overlay := &mockWidget{renderChar: 'O'}

	root := NewRoot(main)
	root.SetSize(10, 5)
	root.PushOverlay(Overlay{Widget: overlay, Modal: true})

	grid := makeGrid(10, 5)
	root.Render(grid)

	// Overlay renders last, so its character should be on top
	if grid[0][0].Ch != 'O' {
		t.Fatalf("expected overlay char 'O', got '%c'", grid[0][0].Ch)
	}
}

func TestRootPushPopOverlay(t *testing.T) {
	root := NewRoot(&mockWidget{})
	root.SetSize(80, 24)

	root.PushOverlay(Overlay{Widget: &mockWidget{}, Modal: true})
	if len(root.Overlays) != 1 {
		t.Fatal("expected 1 overlay")
	}

	root.PopOverlay()
	if len(root.Overlays) != 0 {
		t.Fatal("expected 0 overlays after pop")
	}

	// Pop on empty should not panic
	root.PopOverlay()
}

func TestChordKeysFire(t *testing.T) {
	main := &mockWidget{}
	root := NewRoot(main)
	root.SetSize(80, 24)
	root.SetFocus(&mockWidget{focusable: true})

	fired := false
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyCtrlC, Mod: tcell.ModCtrl},
	}, func() { fired = true })

	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlK, tcell.ModCtrl))
	if fired {
		t.Fatal("chord should not fire after first key")
	}

	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlC, tcell.ModCtrl))
	if !fired {
		t.Fatal("chord should fire after second key")
	}
}

func TestChordResetsOnMismatch(t *testing.T) {
	main := &mockWidget{}
	focused := &mockWidget{focusable: true}
	root := NewRoot(main)
	root.SetSize(80, 24)
	root.SetFocus(focused)

	fired := false
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyCtrlC, Mod: tcell.ModCtrl},
	}, func() { fired = true })

	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlK, tcell.ModCtrl))
	// Wrong second key
	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlX, tcell.ModCtrl))

	if fired {
		t.Fatal("chord should not fire on mismatch")
	}
	// After mismatch, the wrong key falls through to focused widget
	if focused.eventCount != 1 {
		t.Fatalf("expected mismatched key to reach focused widget, got eventCount=%d", focused.eventCount)
	}
}

func TestChordSharedPrefix(t *testing.T) {
	root := NewRoot(&mockWidget{})
	root.SetSize(80, 24)
	root.SetFocus(&mockWidget{focusable: true})

	firedA := false
	firedB := false
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyCtrlC, Mod: tcell.ModCtrl},
	}, func() { firedA = true })
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyCtrlU, Mod: tcell.ModCtrl},
	}, func() { firedB = true })

	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlK, tcell.ModCtrl))
	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlU, tcell.ModCtrl))

	if firedA {
		t.Fatal("chord A should not have fired")
	}
	if !firedB {
		t.Fatal("chord B should have fired")
	}
}

func TestChordMatchesCaseInsensitive(t *testing.T) {
	root := NewRoot(&mockWidget{})
	root.SetSize(80, 24)
	root.SetFocus(&mockWidget{focusable: true})

	fired := false
	// Register chord with lowercase rune second key: ctrl+k, j
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyRune, Rune: 'j'},
	}, func() { fired = true })

	// First step: ctrl+k
	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlK, tcell.ModCtrl))
	if fired {
		t.Fatal("chord should not fire after first key")
	}

	// Second step: uppercase J (simulating caps lock)
	ev := tcell.NewEventKey(tcell.KeyRune, "J", tcell.ModNone)
	root.HandleEvent(ev)
	if !fired {
		t.Fatal("chord should fire with uppercase second key (caps lock)")
	}
}

func TestChordMatchesCaseInsensitiveReverse(t *testing.T) {
	root := NewRoot(&mockWidget{})
	root.SetSize(80, 24)
	root.SetFocus(&mockWidget{focusable: true})

	fired := false
	// Register chord with uppercase rune second key: ctrl+k, J
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyRune, Rune: 'J'},
	}, func() { fired = true })

	// First step: ctrl+k
	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlK, tcell.ModCtrl))

	// Second step: lowercase j (caps lock off)
	ev := tcell.NewEventKey(tcell.KeyRune, "j", tcell.ModNone)
	root.HandleEvent(ev)
	if !fired {
		t.Fatal("chord should fire with lowercase second key when registered as uppercase")
	}
}

func TestModalOverlayBlocksGlobalKeysWhenUnhandled(t *testing.T) {
	modal := &passThroughWidget{}
	root := NewRoot(&mockWidget{})
	root.SetSize(80, 24)
	root.SetFocus(&mockWidget{focusable: true})
	root.PushOverlay(Overlay{Widget: modal, Modal: true})

	globalFired := false
	root.AddGlobalKey(tcell.KeyCtrlP, tcell.ModCtrl, 0, func() { globalFired = true })

	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlP, tcell.ModCtrl))

	if globalFired {
		t.Fatal("global key should not fire when modal overlay is active, even if overlay ignores the event")
	}
	if modal.eventCount != 1 {
		t.Fatalf("modal overlay should have received the event, got %d", modal.eventCount)
	}
}

func TestModalOverlayBlocksChord(t *testing.T) {
	modal := &mockWidget{focusable: true}
	root := NewRoot(&mockWidget{})
	root.SetSize(80, 24)
	root.PushOverlay(Overlay{Widget: modal, Modal: true})

	fired := false
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyCtrlC, Mod: tcell.ModCtrl},
	}, func() { fired = true })

	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlK, tcell.ModCtrl))
	root.HandleEvent(makeKeyEvent(tcell.KeyCtrlC, tcell.ModCtrl))

	if fired {
		t.Fatal("chord should not fire when modal overlay is active")
	}
	if modal.eventCount != 2 {
		t.Fatalf("modal should have received both events, got %d", modal.eventCount)
	}
}
