package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type selectionPointerStep struct {
	name string
	x, y int
	btn  tcell.ButtonMask
	want string
}

func runSelectionPointerSteps(t *testing.T, h *testHarness, steps []selectionPointerStep) {
	t.Helper()
	for _, s := range steps {
		h.app.Root.HandleEvent(tcell.NewEventMouse(s.x, s.y, s.btn, tcell.ModNone))
		h.redraw()
		if got := h.app.Root.PointerShapeAt(s.x, s.y); got != s.want {
			t.Fatalf("%s (%d,%d) = %q, want %q", s.name, s.x, s.y, got, s.want)
		}
	}
}

func screenRowContaining(h *testHarness, text string) int {
	for y, row := range strings.Split(h.screenText(), "\n") {
		if strings.Contains(row, text) {
			return y
		}
	}
	return -1
}

func TestEditorSelectionKeepsTextPointerThroughDrag(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.ShowBottomPanel()
	h.redraw()

	e := h.app.EditorGroup.Editor
	r := e.GetRect()
	x, y := r.X+e.GutterWidth()+2, r.Y
	tabsY := h.app.EditorGroup.TabBar.GetRect().Y + 1
	tabX := displayColumnOf(h.screenRow(tabsY), "alpha.txt")
	if tabX < 0 {
		t.Fatalf("tab label not visible in %q", h.screenRow(tabsY))
	}

	runSelectionPointerSteps(t, h, []selectionPointerStep{
		{"press", x, y, tcell.Button1, term.PointerText},
		{"drag over text", x + 3, y, tcell.Button1, term.PointerText},
		{"drag over gutter", r.X, y + 1, tcell.Button1, term.PointerText},
		{"drag over tab", tabX + 1, tabsY, tcell.Button1, term.PointerText},
		{"drag over sidebar", 1, y + 1, tcell.Button1, term.PointerText},
		{"drag over bottom panel", x, h.app.ContentSplit.DividerScreenY() + 2, tcell.Button1, term.PointerText},
		{"drag back", x + 4, y + 1, tcell.Button1, term.PointerText},
		{"release over text", x + 4, y + 1, tcell.ButtonNone, term.PointerText},
		{"hover tab after release", tabX + 1, tabsY, tcell.ButtonNone, term.PointerHand},
	})
	if !e.Selection.Active {
		t.Fatal("drag did not select text")
	}
}

func TestDiffSelectionKeepsTextPointerOverTabBar(t *testing.T) {
	h := newTestHarness(t, 100, 45)
	defer h.stop()
	setupPartiallyStagedFile(t, h)
	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M", Staged: true}, true, false)
	h.redraw()
	d := h.app.EditorGroup.ActiveDiffWidget()
	if d == nil {
		t.Fatal("staged entry did not open a diff")
	}

	y := screenRowContaining(h, "row 02")
	if y < 0 {
		t.Fatalf("diff text not visible:\n%s", h.screenText())
	}
	x := displayColumnOf(h.screenRow(y), "row 02")
	tabsY := h.app.EditorGroup.TabBar.GetRect().Y + 1
	tabX := displayColumnOf(h.screenRow(tabsY), "staged.txt")
	if tabX < 0 {
		t.Fatalf("tab label not visible in %q", h.screenRow(tabsY))
	}
	tabs := len(h.app.EditorGroup.TabBar.Tabs)

	runSelectionPointerSteps(t, h, []selectionPointerStep{
		{"press", x, y, tcell.Button1, term.PointerText},
		{"drag over text", x + 3, y, tcell.Button1, term.PointerText},
		{"drag over tab", tabX + 1, tabsY, tcell.Button1, term.PointerText},
		{"drag back", x + 4, y + 1, tcell.Button1, term.PointerText},
		{"release over text", x + 4, y + 1, tcell.ButtonNone, term.PointerText},
	})
	if d.OwnsPointerCapture() {
		t.Fatal("release did not end the diff selection capture")
	}
	if got := d.SelectedText(); got == "" {
		t.Fatal("drag did not select diff text")
	}
	if got := len(h.app.EditorGroup.TabBar.Tabs); got != tabs || h.app.EditorGroup.ActiveDiffWidget() != d {
		t.Fatalf("selection drag over the tab bar changed tabs: %d tabs, active diff %v", got, h.app.EditorGroup.ActiveDiffWidget() == d)
	}
	runSelectionPointerSteps(t, h, []selectionPointerStep{
		{"hover tab after release", tabX + 1, tabsY, tcell.ButtonNone, term.PointerHand},
	})
}
