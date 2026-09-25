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

// While a path is dragged its name follows the pointer, and letting go
// anywhere but the terminal clears it.
func TestPathDragLabelFollowsPointer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.AppConfig{Keybindings: config.DefaultKeybindings(), Settings: config.DefaultSettings(), Theme: config.DefaultTheme()}
	borders := BuildBorderSet(cfg.Theme.Borders)
	a := BuildAppFromConfig(&cfg, &borders, workspace.New([]string{dir}), nil)
	a.Root.SetSize(100, 30)

	frame := func() []string {
		cells := make([][]term.Cell, 30)
		for y := range cells {
			cells[y] = make([]term.Cell, 100)
		}
		a.Root.Render(cells)
		a.renderPathDrag(cells)
		rows := make([]string, 30)
		for y, row := range cells {
			rs := make([]rune, 0, 100)
			for _, c := range row {
				rs = append(rs, c.Ch)
			}
			rows[y] = string(rs)
		}
		return rows
	}

	rowY := -1
	for y, row := range frame() {
		if strings.Contains(row, "notes.txt") {
			rowY = y
		}
	}
	if rowY < 0 {
		t.Fatal("notes.txt not in the Explorer")
	}
	press := func(x, y int, btn tcell.ButtonMask) bool {
		return a.handlePathDrag(tcell.NewEventMouse(x, y, btn, tcell.ModNone))
	}
	press(5, rowY, tcell.Button1)
	if !press(60, 8, tcell.Button1) {
		t.Fatal("leaving the Explorer with the button held did not start the drag")
	}
	if !strings.Contains(frame()[8], "⇢ notes.txt") {
		t.Fatalf("no label next to the pointer:\n%s", strings.Join(frame(), "\n"))
	}
	if !press(60, 8, tcell.ButtonNone) {
		t.Fatal("the release that ends a drag must be consumed")
	}
	for _, row := range frame() {
		if strings.Contains(row, "⇢") {
			t.Fatalf("label left behind after the drop:\n%s", strings.Join(frame(), "\n"))
		}
	}
}
