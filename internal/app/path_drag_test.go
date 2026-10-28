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
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestShellQuote(t *testing.T) {
	tests := []struct{ path, program, want string }{
		{"/home/u/src/main.go", "/bin/bash", "/home/u/src/main.go"},
		{"/home/u/my file.txt", "/usr/bin/fish", "'/home/u/my file.txt'"},
		{"/home/u/it's.txt", "/bin/zsh", `'/home/u/it'\''s.txt'`},
		{"/home/u/$HOME/a", "/bin/bash", "'/home/u/$HOME/a'"},
		{`/home/u/a\b`, "/bin/bash", `'/home/u/a\b'`},
		{"/home/u/v1.2_final-draft+", "/bin/sh", "/home/u/v1.2_final-draft+"},
		{`C:\Users\u\main.go`, "pwsh.exe", `C:\Users\u\main.go`},
		{`C:\Users\u\$(calc).txt`, "powershell.exe", `'C:\Users\u\$(calc).txt'`},
		{`C:\Users\u\it's a,b.txt`, "pwsh", `'C:\Users\u\it''s a,b.txt'`},
		{`C:\Users\u\my file.txt`, `C:\Windows\System32\cmd.exe`, `"C:\Users\u\my file.txt"`},
	}
	for _, tc := range tests {
		if runtime.GOOS != "windows" && strings.Contains(tc.program, `\`) {
			continue
		}
		if got := shellQuote(tc.path, tc.program); got != tc.want {
			t.Errorf("shellQuote(%q, %q) = %s, want %s", tc.path, tc.program, got, tc.want)
		}
	}
}

// While a path is dragged the pointer stays a closed hand until release, and
// the status bar says what letting go will do.
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
	// A press alone is a click: the pointer must not flicker.
	if got := a.pointerShapeAt(5, rowY); got != "default" {
		t.Errorf("pointer on the pressed row = %q, want default", got)
	}
	// Moving within the Explorer already starts the drag.
	if !mouse(5, rowY+1, tcell.Button1) {
		t.Fatal("moving off the pressed row did not start the drag")
	}
	if got := a.pointerShapeAt(5, rowY+1); got != "grabbing" {
		t.Errorf("pointer while dragging in the Explorer = %q, want grabbing", got)
	}
	mouse(overEditor[0], overEditor[1], tcell.Button1)
	if got := a.pointerShapeAt(overEditor[0], overEditor[1]); got != "grabbing" {
		t.Errorf("pointer over the editor = %q, want grabbing", got)
	}
	if !strings.Contains(frame(), "Dragging notes.txt") {
		t.Errorf("status bar does not show the drag:\n%s", frame())
	}

	mouse(overTerm[0], overTerm[1], tcell.Button1)
	if got := a.pointerShapeAt(overTerm[0], overTerm[1]); got != "grabbing" {
		t.Errorf("pointer over the terminal = %q, want grabbing", got)
	}
	if !strings.Contains(frame(), "Release to insert notes.txt") {
		t.Errorf("status bar does not say the drop target:\n%s", frame())
	}

	if !mouse(overTerm[0], overTerm[1], tcell.ButtonNone) {
		t.Fatal("the release that ends a drag must be consumed")
	}
	if a.pointerShapeAt(overTerm[0], overTerm[1]) == "grabbing" || strings.Contains(frame(), "notes.txt ·") || strings.Contains(frame(), "Release to insert") {
		t.Error("drag feedback left behind after the drop")
	}
}

func pathDragApp(t *testing.T) (*App, *ui.TerminalWidget, map[string]int) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.AppConfig{Keybindings: config.DefaultKeybindings(), Settings: config.DefaultSettings(), Theme: config.DefaultTheme()}
	borders := BuildBorderSet(cfg.Theme.Borders)
	a := BuildAppFromConfig(&cfg, &borders, workspace.New([]string{dir}), nil)
	a.Root.SetSize(100, 30)
	tw := ui.NewTerminalWidget(nil, nil)
	a.TerminalPanel.AddTerminal(tw)
	a.ContentSplit.ShowBottom = true
	a.BottomPanel.SetActivePanel("terminal")
	cells := make([][]term.Cell, 30)
	for y := range cells {
		cells[y] = make([]term.Cell, 100)
	}
	a.Root.Render(cells)
	rows := map[string]int{}
	for y, row := range cells {
		var b strings.Builder
		for _, c := range row {
			b.WriteRune(c.Ch)
		}
		for _, n := range []string{"a.txt", "b.txt"} {
			if strings.Contains(b.String(), n) {
				rows[n] = y
			}
		}
	}
	if len(rows) != 2 {
		t.Fatalf("explorer rows = %v", rows)
	}
	return a, tw, rows
}

// mouse routes ev the way the event loop does.
func mouse(a *App, x, y int, btn tcell.ButtonMask) {
	ev := tcell.NewEventMouse(x, y, btn, tcell.ModNone)
	if !a.handlePathDrag(ev) {
		a.Root.HandleEvent(ev)
	}
}

func TestClickAfterPathDropReachesExplorer(t *testing.T) {
	a, tw, rows := pathDragApp(t)
	r := tw.GetRect()
	mouse(a, 5, rows["a.txt"], tcell.Button1)
	mouse(a, 5, rows["a.txt"]+3, tcell.Button1)
	mouse(a, r.X+2, r.Y+2, tcell.Button1)
	mouse(a, r.X+2, r.Y+2, tcell.ButtonNone)

	mouse(a, 5, rows["b.txt"], tcell.Button1)
	mouse(a, 5, rows["b.txt"], tcell.ButtonNone)

	if n := a.Explorer.Tree.Selected(); n == nil || filepath.Base(n.ID) != "b.txt" {
		t.Fatalf("selected after the click = %v, want b.txt: the Explorer still held the drag's press", n)
	}
}

func TestPathDropUnderModalOverlayInsertsNothing(t *testing.T) {
	a, tw, rows := pathDragApp(t)
	r := tw.GetRect()
	mouse(a, 5, rows["a.txt"], tcell.Button1)
	mouse(a, r.X+2, r.Y+2, tcell.Button1)
	dialog := ui.NewWidgetAdapter(widgets.NewDialogWidget(30))
	a.ShowDialog(dialog)
	mouse(a, r.X+2, r.Y+2, tcell.ButtonNone)
	if a.Root.Focused != dialog {
		t.Fatalf("focus = %T after a drop under a dialog, want the dialog: the path was inserted", a.Root.Focused)
	}
}

func TestInsertPathIgnoresExplorerActions(t *testing.T) {
	a, _, _ := pathDragApp(t)
	a.ContentSplit.ShowBottom = false
	a.Terminals = append(a.Terminals, TerminalTab{})
	a.ExplorerContextNode = &widgets.TreeNode{ID: "command:workspace.openFolder"}
	a.InsertExplorerPathInTerminal()
	if a.ContentSplit.ShowBottom {
		t.Fatal("an Explorer action was inserted into the terminal as a path")
	}
}
