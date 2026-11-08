package ui

import "testing"

func TestEditorGroupRoutesRawKeysToActiveTerminalTab(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.SetFocused(true)
	tab := NewTerminalTabWidget(NewTerminalWidget(nil, nil))
	closed := 0
	tab.OnClose = func() { closed++ }

	g.OpenPluginTab("terminal:t", "Terminal", tab)
	if !g.WantsRawKeys() || g.ActiveTerminalTab() != tab {
		t.Fatal("a focused terminal tab must take raw keys")
	}

	g.SwitchTab(0)
	if g.WantsRawKeys() || g.ActiveTerminalTab() != nil {
		t.Fatal("raw keys must stop once another tab is active")
	}

	if !g.FocusPluginTab("terminal:t") || !g.WantsRawKeys() {
		t.Fatal("FocusPluginTab must reselect and focus the terminal")
	}

	g.SetFocused(false)
	if g.WantsRawKeys() {
		t.Fatal("an unfocused group must not take raw keys")
	}

	if !g.ClosePluginTab("terminal:t") || closed != 1 {
		t.Fatalf("closing the tab ran OnClose %d times, want 1", closed)
	}
}

func TestTerminalTabContentRectMatchesRender(t *testing.T) {
	tab := NewTerminalTabWidget(NewTerminalWidget(nil, nil))
	r := Rect{X: 2, Y: 1, W: 20, H: 5}
	tab.SetRect(r)
	grid := makeGrid(r.X+r.W, r.Y+r.H)
	tab.Render(NewRenderSurface(grid, r))

	want := Rect{X: 3, Y: 1, W: 18, H: 5}
	if got := tab.ContentRect(); got != want {
		t.Fatalf("ContentRect = %+v, want %+v", got, want)
	}
	if got := tab.Terminal.GetRect(); got != want {
		t.Fatalf("terminal drawn at %+v, want %+v", got, want)
	}
}
