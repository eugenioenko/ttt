package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestShellQuote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix quoting")
	}
	tests := map[string]string{
		"/home/u/src/main.go":       "/home/u/src/main.go",
		"/home/u/my file.txt":       "'/home/u/my file.txt'",
		"/home/u/it's.txt":          `'/home/u/it'\''s.txt'`,
		"/home/u/$HOME/a":           "'/home/u/$HOME/a'",
		"/home/u/v1.2_final-draft+": "/home/u/v1.2_final-draft+",
	}
	for in, want := range tests {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

// While a path is dragged the pointer says so, and the terminal is marked as
// the drop target only while the drag hovers it.
func TestPathDragFeedback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.AppConfig{Keybindings: config.DefaultKeybindings(), Settings: config.DefaultSettings(), Theme: config.DefaultTheme()}
	borders := BuildBorderSet(cfg.Theme.Borders)
	a := BuildAppFromConfig(&cfg, &borders, workspace.New([]string{dir}), nil)
	a.Root.SetSize(100, 30)
	tw := ui.NewTerminalWidget(nil, nil)
	a.TerminalPanel.AddTerminal(tw)
	a.ContentSplit.ShowBottom = true
	a.BottomPanel.SetActivePanel("terminal")

	frame := func() string {
		cells := make([][]term.Cell, 30)
		for y := range cells {
			cells[y] = make([]term.Cell, 100)
		}
		a.Root.Render(cells)
		a.renderPathDrag(cells)
		var b strings.Builder
		for _, row := range cells {
			for _, c := range row {
				b.WriteRune(c.Ch)
			}
			b.WriteByte('\n')
		}
		return b.String()
	}

	rowY := -1
	for y, row := range strings.Split(frame(), "\n") {
		if strings.Contains(row, "notes.txt") {
			rowY = y
		}
	}
	if rowY < 0 {
		t.Fatal("notes.txt not in the Explorer")
	}
	mouse := func(x, y int, btn tcell.ButtonMask) bool {
		return a.handlePathDrag(tcell.NewEventMouse(x, y, btn, tcell.ModNone))
	}
	tr := tw.GetRect()
	overTerm := [2]int{tr.X + tr.W/2, tr.Y + tr.H/2}
	overEditor := [2]int{overTerm[0], 5}

	mouse(5, rowY, tcell.Button1)
	if !mouse(overEditor[0], overEditor[1], tcell.Button1) {
		t.Fatal("leaving the Explorer with the button held did not start the drag")
	}
	if got := a.pointerShapeAt(overEditor[0], overEditor[1]); got != "grabbing" {
		t.Errorf("pointer over the editor = %q, want grabbing", got)
	}
	if strings.Contains(frame(), "drop to insert") {
		t.Error("drop mark shown away from the terminal")
	}

	mouse(overTerm[0], overTerm[1], tcell.Button1)
	if got := a.pointerShapeAt(overTerm[0], overTerm[1]); got != "copy" {
		t.Errorf("pointer over the terminal = %q, want copy", got)
	}
	if !strings.Contains(frame(), "drop to insert notes.txt") {
		t.Errorf("terminal not marked as the drop target:\n%s", frame())
	}

	if !mouse(overTerm[0], overTerm[1], tcell.ButtonNone) {
		t.Fatal("the release that ends a drag must be consumed")
	}
	if a.pointerShapeAt(overTerm[0], overTerm[1]) == "copy" || strings.Contains(frame(), "drop to insert") {
		t.Error("drag feedback left behind after the drop")
	}
}
