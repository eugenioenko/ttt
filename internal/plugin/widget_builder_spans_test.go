package plugin

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func reconcileButton(t *testing.T, descs []WidgetDesc) (*WidgetState, *Plugin, widgets.Widget) {
	t.Helper()
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	t.Cleanup(p.State.Close)
	root := ws.Reconcile(descs, p)
	if root == nil || len(root.Children) != 1 {
		t.Fatal("expected one child")
	}
	return ws, p, root.Children[0]
}

func TestReconcileButtonSpansUpdateKeepsFocusAndCallback(t *testing.T) {
	clicked := 0
	onClick := func() { clicked++ }

	ws, p, w := reconcileButton(t, []WidgetDesc{
		{Kind: WidgetButton, Key: "button:0", Label: "CI", OnClick: onClick},
	})
	btn := w.(*widgets.ButtonWidget)
	btn.SetRect(widgets.Rect{X: 2, Y: 3, W: 12, H: 1})
	// Re-reconcile so the FocusManager collects the button with its geometry,
	// then focus it as the panel would (pointer identity drives the restore).
	ws.Reconcile([]WidgetDesc{
		{Kind: WidgetButton, Key: "button:0", Label: "CI", OnClick: onClick},
	}, p)
	ws.focus.SetActive(true)
	ws.focus.FocusNext()
	if !btn.IsFocused() {
		t.Fatal("expected the button focused before the update")
	}

	root := ws.Reconcile([]WidgetDesc{
		{Kind: WidgetButton, Key: "button:0", Label: "CI",
			Spans: []widgets.ButtonSpan{
				{Text: "2", Style: term.StyleSuccess},
				{Text: " 1", Style: term.StyleWarning},
				{Text: " 3", Style: term.StyleDanger},
			}, OnClick: onClick},
	}, p)

	if root.Children[0] != widgets.Widget(btn) {
		t.Fatal("a spans update should keep the widget instance")
	}
	if !btn.IsFocused() {
		t.Error("button should keep focus across a spans update")
	}
	if r := btn.GetRect(); r.X != 2 || r.Y != 3 {
		t.Errorf("button should keep its rect across a spans update, got %v", r)
	}
	if !btn.HasSpans() || btn.Width() != 5+2 {
		t.Errorf("expected 3 spans with width 7, got spans=%v width=%d", btn.Config.Spans, btn.Width())
	}
	// A click inside the rect triggers the re-wired callback without focus.
	btn.HandleEvent(tcell.NewEventMouse(4, 3, tcell.Button1, tcell.ModNone))
	if clicked != 1 {
		t.Errorf("callback should survive the update, got %d clicks", clicked)
	}
}

func TestReconcileButtonUnchangedSpansKeepWidget(t *testing.T) {
	spans := []widgets.ButtonSpan{{Text: "ok", Style: term.StyleSuccess}}
	ws, p, w := reconcileButton(t, []WidgetDesc{
		{Kind: WidgetButton, Key: "button:0", Label: "", Spans: spans},
	})
	btn := w.(*widgets.ButtonWidget)

	ws.Reconcile([]WidgetDesc{
		{Kind: WidgetButton, Key: "button:0", Label: "", Spans: spans},
	}, p)

	root := ws.Reconcile([]WidgetDesc{
		{Kind: WidgetButton, Key: "button:0", Label: "", Spans: spans},
	}, p)
	if root.Children[0] != widgets.Widget(btn) {
		t.Error("unchanged spans should keep the same widget instance")
	}
}

func TestParseButtonSpans(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	if err := L.DoString(`t = {
		{text = "12", style = "success"},
		{text = " 1", style = "warning"},
		{text = " 3", style = "unknown-name"},
		{text = ""},
		"not a table",
	}`); err != nil {
		t.Fatal(err)
	}
	spans := parseButtonSpans(L, L.GetGlobal("t").(*lua.LTable))
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans, got %d", len(spans))
	}
	if spans[0].Text != "12" || spans[0].Style != term.StyleSuccess {
		t.Errorf("unexpected first span: %+v", spans[0])
	}
	if spans[2].Style != 0 {
		t.Errorf("unknown style name should keep the button style, got %v", spans[2].Style)
	}
}
