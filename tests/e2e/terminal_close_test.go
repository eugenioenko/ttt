package e2e

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func openPanelTerminals(t *testing.T, h *testHarness, n int) []string {
	t.Helper()
	h.app.Settings.Terminal.Shell = "/bin/cat"
	for i := 0; i < n; i++ {
		h.exec("terminal.new")
	}
	if len(h.app.Terminals) != n {
		t.Fatalf("panel terminals = %d, want %d", len(h.app.Terminals), n)
	}
	ids := make([]string, n)
	for i, tt := range h.app.Terminals {
		ids[i] = tt.ID
		t.Cleanup(tt.Term.Close)
	}
	return ids
}

func TestCloseTerminalTargetsFocusedLocation(t *testing.T) {
	for _, location := range []string{"editor", "panel"} {
		for _, palette := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/palette=%v", location, palette), func(t *testing.T) {
				h := newTestHarness(t, 100, 30)
				defer h.stop()
				panelID := openPanelTerminals(t, h, 1)[0]
				h.app.HideBottomPanel()
				h.exec("terminal.newInEditor")
				editor := h.app.EditorTerminals[0]
				t.Cleanup(editor.Term.Close)
				closed, remaining := editor.ID, panelID
				if location == "panel" {
					if err := h.app.FocusTerminal(panelID); err != nil {
						t.Fatal(err)
					}
					closed, remaining = panelID, editor.ID
				}
				if palette {
					h.app.OpenCommandPalette(false)
					dialog := h.app.Root.TopOverlayWidget().(*ui.SelectDialogWidget)
					dialog.OnExecute("terminal.close")
				} else {
					h.exec("terminal.close")
				}
				terms := h.app.ListTerminals()
				if len(terms) != 1 || terms[0].ID != remaining {
					t.Fatalf("closed wrong terminal: remaining=%+v, want %s", terms, remaining)
				}
				if err := h.app.SendToTerminal(closed, "closed"); err == nil {
					t.Error("closed terminal still accepts input")
				}
				if err := h.app.SendToTerminal(remaining, "still-running\n"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestPanelMenuClosesPanelTerminalFromEditorFocus(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	panelID := openPanelTerminals(t, h, 1)[0]
	h.exec("terminal.newInEditor")
	editor := h.app.EditorTerminals[0]
	t.Cleanup(editor.Term.Close)
	h.app.BottomPanel.Tabs.Config.Actions[1].OnClick(80, 15)
	menu := focusedContextMenu(t, h)
	for i, item := range menu.Items {
		if item.Label == i18n.Text("menu.closeTerminal") {
			menu.Selected = i
			break
		}
	}
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	terms := h.app.ListTerminals()
	if len(terms) != 1 || terms[0].ID != editor.ID {
		t.Fatalf("panel menu closed wrong terminal: %+v", terms)
	}
	if err := h.app.SendToTerminal(panelID, "closed"); err == nil {
		t.Error("closed panel terminal still accepts input")
	}
}

func terminalIDs(h *testHarness) []string {
	var ids []string
	for _, tt := range h.app.Terminals {
		ids = append(ids, tt.ID)
	}
	return ids
}

func TestRightClickTerminalTabClosesOnlyThatTerminal(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	ids := openPanelTerminals(t, h, 3)

	strip := h.app.TerminalPanel.TabBar.GetRect()
	h.rightClick(strip.X+1, strip.Y+1)
	if got := h.app.TerminalPanel.ActiveIndex(); got != 1 {
		t.Fatalf("right-click selected tab %d, want 1", got)
	}
	menu := focusedContextMenu(t, h)
	found := false
	for i, item := range menu.Items {
		if item.Command == "terminal.close" {
			menu.Selected = i
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("terminal tab menu has no Close Terminal item: %+v", menu.Items)
	}
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	got := terminalIDs(h)
	if len(got) != 2 || got[0] != ids[0] || got[1] != ids[2] {
		t.Fatalf("terminals after close = %v, want [%s %s]", got, ids[0], ids[2])
	}
	if err := h.app.SendToTerminal(ids[1], "x"); err == nil {
		t.Fatal("the closed terminal still accepts input")
	}
	if got := h.app.TerminalPanel.ActiveIndex(); got != 1 {
		t.Errorf("active tab after close = %d, want 1", got)
	}
	for _, tt := range h.app.Terminals {
		if raw := tt.Term.RawTail(); len(raw) != 0 {
			t.Errorf("terminal %s received %q from the menu interaction", tt.ID, raw)
		}
	}
	if !h.app.ContentSplit.ShowBottom {
		t.Error("closing one of several terminals hid the panel")
	}
}

func TestCloseTerminalCommandClosesActiveTerminal(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	ids := openPanelTerminals(t, h, 2)

	h.app.TerminalPanel.SetActive(0)
	h.exec("terminal.close")
	if got := terminalIDs(h); len(got) != 1 || got[0] != ids[1] {
		t.Fatalf("terminals after close = %v, want [%s]", got, ids[1])
	}

	h.exec("terminal.close")
	if len(h.app.Terminals) != 0 {
		t.Fatalf("terminals after closing the last = %d, want 0", len(h.app.Terminals))
	}
	if h.app.ContentSplit.ShowBottom {
		t.Error("closing the last terminal must hide the panel")
	}

	h.exec("terminal.close")
}
