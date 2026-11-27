package ui

import (
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func newTestSurface(w, h int) Surface {
	return NewRenderSurface(makeGrid(w, h), Rect{X: 0, Y: 0, W: w, H: h})
}

func TestStatusBarPointerShapeSegments(t *testing.T) {
	sb := view.NewStatusBar()
	clickedBranch := false
	clickedPlugin := false
	clickedRight := false

	sb.SetSegment(view.StatusSegment{
		ID:       "branch",
		Side:     "left",
		Priority: 100,
		Text:     "main",
		OnClick: func() {
			clickedBranch = true
		},
	})
	sb.SetSegment(view.StatusSegment{
		ID:       "read-only-left",
		Side:     "left",
		Priority: 90,
		Text:     "ReadOnly",
	})
	sb.SetSegment(view.StatusSegment{
		ID:       "plugin-item",
		Side:     "left",
		Priority: 80,
		Text:     "PluginAction",
		Owner:    "plugin:custom",
		OnClick: func() {
			clickedPlugin = true
		},
	})
	sb.SetSegment(view.StatusSegment{
		ID:       "pos",
		Side:     "right",
		Priority: 100,
		Text:     "Ln 1, Col 1",
		OnClick: func() {
			clickedRight = true
		},
	})
	sb.SetSegment(view.StatusSegment{
		ID:       "encoding",
		Side:     "right",
		Priority: 90,
		Text:     "UTF-8",
	})

	widget := NewStatusBarWidget(sb)
	widget.SetRect(widgets.Rect{X: 0, Y: 0, W: 80, H: 1})
	widget.Render(newTestSurface(80, 1))

	// Find the clickable spans recorded by Render
	if len(widget.spans) < 3 {
		t.Fatalf("expected at least 3 clickable spans, got %d", len(widget.spans))
	}

	// Verify hand pointer on clickable spans
	for _, span := range widget.spans {
		mid := (span.start + span.end) / 2
		if got := widget.PointerShapeAt(mid, 0); got != term.PointerHand {
			t.Errorf("midpoint %d of span [%d, %d) = %q, want %q", mid, span.start, span.end, got, term.PointerHand)
		}
	}

	// Leading space (x=0) and gaps between segments should have no hand pointer
	if got := widget.PointerShapeAt(0, 0); got != "" {
		t.Errorf("leading space (0, 0) = %q, want empty", got)
	}

	// Verify points outside clickable spans return ""
	for x := 0; x < 80; x++ {
		isClickable := false
		for _, s := range widget.spans {
			if x >= s.start && x < s.end {
				isClickable = true
				break
			}
		}
		got := widget.PointerShapeAt(x, 0)
		if isClickable && got != term.PointerHand {
			t.Errorf("clickable x=%d = %q, want %q", x, got, term.PointerHand)
		} else if !isClickable && got != "" {
			t.Errorf("unclickable x=%d = %q, want empty", x, got)
		}
	}

	// Queries must not invoke click callbacks
	if clickedBranch || clickedPlugin || clickedRight {
		t.Fatalf("PointerShapeAt triggered click callbacks")
	}
}

func TestStatusBarPointerShapeNonzeroCoordinatesAndBounds(t *testing.T) {
	sb := view.NewStatusBar()
	sb.SetSegment(view.StatusSegment{
		ID:       "branch",
		Side:     "left",
		Priority: 100,
		Text:     "feat/branch",
		OnClick:  func() {},
	})

	widget := NewStatusBarWidget(sb)
	// Nonzero screen coordinates: X=10, Y=24, W=60, H=1
	widget.SetRect(widgets.Rect{X: 10, Y: 24, W: 60, H: 1})
	widget.Render(NewRenderSurface(makeGrid(60, 1), Rect{X: 10, Y: 24, W: 60, H: 1}))

	if len(widget.spans) == 0 {
		t.Fatal("expected at least 1 clickable span")
	}
	span := widget.spans[0]
	if span.start < 10 {
		t.Fatalf("span start %d should be >= 10 for nonzero rect", span.start)
	}

	// Hit test on the status bar row
	if got := widget.PointerShapeAt(span.start+1, 24); got != term.PointerHand {
		t.Errorf("inside nonzero segment = %q, want %q", got, term.PointerHand)
	}

	// Points outside the status row (Y != 24)
	if got := widget.PointerShapeAt(span.start+1, 23); got != "" {
		t.Errorf("above status row (Y=23) = %q, want empty", got)
	}
	if got := widget.PointerShapeAt(span.start+1, 25); got != "" {
		t.Errorf("below status row (Y=25) = %q, want empty", got)
	}

	// Points left or right of widget rect bounds
	if got := widget.PointerShapeAt(9, 24); got != "" {
		t.Errorf("left of rect (X=9) = %q, want empty", got)
	}
	if got := widget.PointerShapeAt(70, 24); got != "" {
		t.Errorf("right of rect (X=70) = %q, want empty", got)
	}
}

func TestStatusBarPointerShapeFullwidthRuneWidth(t *testing.T) {
	sb := view.NewStatusBar()
	// Fullwidth label: each character is 2 terminal columns
	sb.SetSegment(view.StatusSegment{
		ID:       "japanese",
		Side:     "left",
		Priority: 100,
		Text:     "チェック", // 4 fullwidth runes = 8 terminal columns
		OnClick:  func() {},
	})

	widget := NewStatusBarWidget(sb)
	widget.SetRect(widgets.Rect{X: 0, Y: 0, W: 40, H: 1})
	widget.Render(newTestSurface(40, 1))

	if len(widget.spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(widget.spans))
	}
	span := widget.spans[0]
	// Starts after leading space (x=1), width is 8 columns, so span is [1, 9)
	if width := span.end - span.start; width != 8 {
		t.Fatalf("expected fullwidth span width of 8 columns, got %d [%d, %d)", width, span.start, span.end)
	}

	for x := span.start; x < span.end; x++ {
		if got := widget.PointerShapeAt(x, 0); got != term.PointerHand {
			t.Errorf("fullwidth rune column %d = %q, want %q", x, got, term.PointerHand)
		}
	}
	if got := widget.PointerShapeAt(span.end, 0); got != "" {
		t.Errorf("immediately after fullwidth span %d = %q, want empty", span.end, got)
	}
}

func TestStatusBarPointerShapeNarrowClipping(t *testing.T) {
	sb := view.NewStatusBar()
	sb.SetSegment(view.StatusSegment{
		ID:       "long-branch",
		Side:     "left",
		Priority: 100,
		Text:     "very-long-branch-name-that-gets-clipped",
		OnClick:  func() {},
	})
	sb.SetSegment(view.StatusSegment{
		ID:       "right-item",
		Side:     "right",
		Priority: 100,
		Text:     "RightItem",
		OnClick:  func() {},
	})

	widget := NewStatusBarWidget(sb)
	// Narrow width of 10 columns: left segment will be clipped at width 10, right segment won't fit
	widget.SetRect(widgets.Rect{X: 0, Y: 0, W: 10, H: 1})
	widget.Render(newTestSurface(10, 1))

	for _, span := range widget.spans {
		if span.end > 10 {
			t.Errorf("span [%d, %d) extends beyond narrow surface width 10", span.start, span.end)
		}
	}
	if got := widget.PointerShapeAt(10, 0); got != "" {
		t.Errorf("outside narrow width (X=10) = %q, want empty", got)
	}
}

func TestStatusBarPointerShapeNotifications(t *testing.T) {
	sb := view.NewStatusBar()
	sb.SetSegment(view.StatusSegment{
		ID:       "branch",
		Side:     "left",
		Priority: 100,
		Text:     "main",
		OnClick:  func() {},
	})

	actionRun := false
	secActionRun := false

	sb.SetNotificationWithAction("Build completed", view.NotifyInfo, time.Minute, "View Logs", func() {
		actionRun = true
	})
	sb.SecondaryLabel = "Dismiss"
	sb.SecondaryAction = func() {
		secActionRun = true
	}

	widget := NewStatusBarWidget(sb)
	widget.SetRect(widgets.Rect{X: 0, Y: 0, W: 80, H: 1})
	widget.Render(newTestSurface(80, 1))

	// Notification replaces segments: action and secondary spans should be populated
	if widget.actionSpan.start == widget.actionSpan.end || widget.secondarySpan.start == widget.secondarySpan.end {
		t.Fatal("expected rendered action and secondary spans")
	}

	// Verify hand pointer on action and secondary buttons
	actionMid := (widget.actionSpan.start + widget.actionSpan.end) / 2
	if got := widget.PointerShapeAt(actionMid, 0); got != term.PointerHand {
		t.Errorf("action button mid=%d = %q, want %q", actionMid, got, term.PointerHand)
	}

	secMid := (widget.secondarySpan.start + widget.secondarySpan.end) / 2
	if got := widget.PointerShapeAt(secMid, 0); got != term.PointerHand {
		t.Errorf("secondary button mid=%d = %q, want %q", secMid, got, term.PointerHand)
	}

	// Neutral notification text (e.g. x=5) should not show hand pointer
	if got := widget.PointerShapeAt(5, 0); got != "" {
		t.Errorf("notification message text = %q, want empty", got)
	}

	// Verify default [OK] notification
	sb2 := view.NewStatusBar()
	sb2.SetNotification("Disk space low", view.NotifyWarning, time.Minute)
	w2 := NewStatusBarWidget(sb2)
	w2.SetRect(widgets.Rect{X: 0, Y: 0, W: 80, H: 1})
	w2.Render(newTestSurface(80, 1))

	if w2.okSpan.start == w2.okSpan.end {
		t.Fatal("expected [OK] span to be rendered")
	}
	okMid := (w2.okSpan.start + w2.okSpan.end) / 2
	if got := w2.PointerShapeAt(okMid, 0); got != term.PointerHand {
		t.Errorf("[OK] mid=%d = %q, want %q", okMid, got, term.PointerHand)
	}

	// Pointer query must not dismiss notification or invoke actions
	if actionRun || secActionRun {
		t.Fatal("PointerShapeAt triggered notification actions or dismiss")
	}
}

func TestStatusBarPointerShapeRenderTransitionsAndEchoText(t *testing.T) {
	sb := view.NewStatusBar()
	clicked := false
	sb.SetSegment(view.StatusSegment{
		ID:       "target",
		Side:     "left",
		Priority: 100,
		Text:     "ClickMe",
		OnClick: func() {
			clicked = true
		},
	})

	widget := NewStatusBarWidget(sb)
	widget.SetRect(widgets.Rect{X: 0, Y: 0, W: 60, H: 1})

	// 1. Normal mode
	widget.Render(newTestSurface(60, 1))
	if got := widget.PointerShapeAt(2, 0); got != term.PointerHand {
		t.Fatalf("normal segment at x=2 = %q, want %q", got, term.PointerHand)
	}

	// 2. Transition to EchoText
	sb.EchoText = "Find: pattern"
	widget.Render(newTestSurface(60, 1))

	// Old clickable segment should no longer supply a hand pointer
	if got := widget.PointerShapeAt(2, 0); got != "" {
		t.Errorf("under EchoText, x=2 = %q, want empty", got)
	}
	// Clicking during EchoText must not trigger the old segment callback
	widget.HandleEvent(tcell.NewEventMouse(2, 0, tcell.Button1, 0))
	if clicked {
		t.Fatal("clicking under EchoText invoked hidden segment callback")
	}

	// 3. Transition to Notification
	sb.EchoText = ""
	sb.SetNotification("Info notification", view.NotifyInfo, time.Minute)
	widget.Render(newTestSurface(60, 1))

	if got := widget.PointerShapeAt(2, 0); got != "" {
		t.Errorf("under Notification, x=2 = %q, want empty", got)
	}
	okMid := (widget.okSpan.start + widget.okSpan.end) / 2
	if got := widget.PointerShapeAt(okMid, 0); got != term.PointerHand {
		t.Errorf("notification [OK] at %d = %q, want %q", okMid, got, term.PointerHand)
	}

	// 4. Dismiss notification back to normal
	sb.DismissNotification()
	widget.Render(newTestSurface(60, 1))
	if got := widget.PointerShapeAt(2, 0); got != term.PointerHand {
		t.Errorf("back to normal, x=2 = %q, want %q", got, term.PointerHand)
	}
	widget.HandleEvent(tcell.NewEventMouse(2, 0, tcell.Button1, 0))
	if !clicked {
		t.Fatal("clicking normal segment should invoke callback")
	}
}

func TestStatusBarPointerShapeComposedRootAndOverlayPrecedence(t *testing.T) {
	sb := view.NewStatusBar()
	sb.SetSegment(view.StatusSegment{
		ID:       "branch",
		Side:     "left",
		Priority: 100,
		Text:     "main",
		OnClick:  func() {},
	})

	statusBar := NewStatusBarWidget(sb)
	statusBar.SetRect(widgets.Rect{X: 0, Y: 10, W: 50, H: 1})
	statusBar.Render(NewRenderSurface(makeGrid(50, 1), Rect{X: 0, Y: 10, W: 50, H: 1}))

	content := &shapeWidget{shape: term.PointerDefault}
	content.SetRect(widgets.Rect{X: 0, Y: 0, W: 50, H: 10})

	rootBox := widgets.NewVStackWidget(content, statusBar)
	root := NewRoot(rootBox)

	// Over clickable status bar segment through Root routing -> term.PointerHand
	spanMid := (statusBar.spans[0].start + statusBar.spans[0].end) / 2
	if got := root.PointerShapeAt(spanMid, 10); got != term.PointerHand {
		t.Fatalf("root.PointerShapeAt over status bar = %q, want %q", got, term.PointerHand)
	}

	// Neutral point on status bar through Root routing -> term.PointerDefault
	if got := root.PointerShapeAt(40, 10); got != term.PointerDefault {
		t.Fatalf("root.PointerShapeAt over neutral status bar = %q, want %q", got, term.PointerDefault)
	}

	// Covering overlay on Root takes precedence and blocks status bar underneath
	root.PushOverlay(Overlay{Widget: &shapeWidget{shape: term.PointerText}, Modal: true})
	if got := root.PointerShapeAt(spanMid, 10); got != term.PointerText {
		t.Fatalf("root.PointerShapeAt with covering overlay = %q, want %q", got, term.PointerText)
	}

	// Shapeless overlay suppresses status bar beneath, falling back to default
	root.PushOverlay(Overlay{Widget: &mockWidget{}, Modal: false})
	if got := root.PointerShapeAt(spanMid, 10); got != term.PointerDefault {
		t.Fatalf("root.PointerShapeAt with shapeless overlay = %q, want %q", got, term.PointerDefault)
	}
}
