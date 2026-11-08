package e2e

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/gdamore/tcell/v3"
)

func waitForRawTail(t *testing.T, term *terminal.Terminal, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(string(term.RawTail()), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("PTY output never contained %q: %q", want, term.RawTail())
}

func TestEditorTerminalTabTakesKeysAndFollowsItsTab(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.app.Settings.Terminal.Shell = "/bin/cat"

	h.exec("terminal.newInEditor")
	if len(h.app.EditorTerminals) != 1 || len(h.app.Terminals) != 0 {
		t.Fatalf("editor terminals = %d, panel terminals = %d", len(h.app.EditorTerminals), len(h.app.Terminals))
	}
	if h.app.ContentSplit.ShowBottom {
		t.Fatal("an editor terminal must not open the bottom panel")
	}
	entry := h.app.EditorTerminals[0]
	defer entry.Term.Close()
	if h.app.EditorGroup.ActiveTerminalTab() != entry.Tab {
		t.Fatal("the new terminal tab is not active")
	}

	cols, rows := entry.Term.Size()
	if c := entry.Tab.ContentRect(); c.W != cols || c.H != rows {
		t.Fatalf("PTY is %dx%d, tab content is %dx%d", cols, rows, c.W, c.H)
	}

	// ctrl+k starts a chord in the editor; the terminal must get it raw instead.
	h.pressCtrl(tcell.KeyCtrlK)
	for _, r := range "typed" {
		h.pressRune(r)
	}
	waitForRawTail(t, entry.Term, "typed")
	if len(h.app.Terminals) != 0 {
		t.Fatal("ctrl+k t reached the chord table instead of the PTY")
	}

	h.pressCtrl(tcell.KeyCtrlT)
	if !h.app.ContentSplit.ShowBottom || len(h.app.Terminals) != 1 {
		t.Fatal("the terminal.toggle force key must still work from an editor terminal")
	}
	defer h.app.Terminals[0].Term.Close()

	var info *plugin.TerminalInfo
	for _, ti := range h.app.ListTerminals() {
		if ti.ID == entry.ID {
			info = &ti
		}
	}
	if info == nil || info.Location != plugin.TerminalLocationEditor {
		t.Fatalf("ListTerminals entry = %+v, want location editor", info)
	}
	if err := h.app.SendToTerminal(entry.ID, "sent_by_plugin\n"); err != nil {
		t.Fatal(err)
	}
	waitForRawTail(t, entry.Term, "sent_by_plugin")

	if err := h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	if err := h.app.FocusTerminal(entry.ID); err != nil {
		t.Fatal(err)
	}
	h.redraw()
	if h.app.Root.Focused != h.app.EditorGroup || h.app.EditorGroup.ActiveTerminalTab() != entry.Tab {
		t.Fatal("FocusTerminal must select and focus the editor tab")
	}

	h.app.CloseTerminal(entry.ID)
	h.redraw()
	if len(h.app.EditorTerminals) != 0 || h.app.EditorGroup.ActiveTerminalTab() != nil {
		t.Fatal("closing the terminal must close its editor tab")
	}
	if len(h.app.Terminals) != 1 {
		t.Fatal("closing an editor terminal must leave panel terminals alone")
	}
}

func TestClosingEditorTerminalTabReleasesTerminal(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.app.Settings.Terminal.Shell = "/bin/cat"

	h.exec("terminal.newInEditor")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatalf("editor terminals = %d, want 1", len(h.app.EditorTerminals))
	}
	id := h.app.EditorTerminals[0].ID

	h.app.EditorGroup.CloseTab()
	h.redraw()
	if len(h.app.EditorTerminals) != 0 {
		t.Fatal("closing the tab must release the terminal")
	}
	if err := h.app.SendToTerminal(id, "x"); err == nil {
		t.Fatal("a released terminal must not accept input")
	}
	// The child's exit posts a close for the same id; it must be a no-op.
	h.app.CloseTerminal(id)
}
