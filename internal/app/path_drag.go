package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
)

// pathDrag tracks a file or folder dragged out of the Explorer. The press
// stays with the Explorer as usual; once the pointer moves off the pressed
// spot with the button held the drag starts, and from then on the mouse
// belongs to the drag until the button is released.
type pathDrag struct {
	pressed bool
	path    string
	active  bool
	px, py  int // where the press started
}

const pathDragSegment = "path-drag"

// handlePathDrag reports whether it consumed the event.
func (a *App) handlePathDrag(ev *tcell.EventMouse) bool {
	mx, my := ev.Position()
	d := &a.pathDrag
	if ev.Buttons()&tcell.Button1 == 0 {
		wasActive := d.active
		if wasActive {
			a.Status.RemoveSegment(pathDragSegment)
			if tw := a.terminalAt(mx, my); tw != nil {
				tw.PasteText(shellQuote(d.path) + " ")
				a.Root.SetFocus(a.TerminalPanel)
			}
		}
		*d = pathDrag{}
		return wasActive
	}
	switch {
	case !d.pressed:
		d.pressed = true
		d.path = a.explorerPathAt(mx, my)
		d.px, d.py = mx, my
	case d.active || d.path != "" && (my != d.py || mx-d.px >= 2 || d.px-mx >= 2):
		d.active = true
		a.showPathDragStatus(a.terminalAt(mx, my) != nil)
		return true
	}
	return false
}

// showPathDragStatus keeps the drag in the status bar, in the accent color,
// saying what letting go will do.
func (a *App) showPathDragStatus(overTerminal bool) {
	name := filepath.Base(a.pathDrag.path)
	text := " ⇢ Dragging " + name + " · drop it on the terminal "
	if overTerminal {
		text = " ⇣ Release to insert " + name + " "
	}
	a.Status.SetSegment(view.StatusSegment{ID: pathDragSegment, Side: "left", Priority: 300,
		Text: text, Style: term.StylePaletteSelected})
}

func (a *App) explorerVisible() bool {
	return a.Sidebar.Visible && a.Sidebar.ActivePanel == "explorer"
}

// explorerPathAt is the file or folder on the Explorer row under the pointer.
func (a *App) explorerPathAt(mx, my int) string {
	if !a.explorerVisible() {
		return ""
	}
	node := a.Explorer.Tree.NodeAt(mx, my)
	if node == nil || !filepath.IsAbs(node.ID) {
		return ""
	}
	if _, err := os.Stat(node.ID); err != nil {
		return ""
	}
	return node.ID
}

// terminalAt is the terminal shown under the pointer, if any.
func (a *App) terminalAt(mx, my int) *ui.TerminalWidget {
	if !a.ContentSplit.ShowBottom || a.BottomPanel.ActivePanel != "terminal" {
		return nil
	}
	tw, ok := a.TerminalPanel.ActiveWidget().(*ui.TerminalWidget)
	if !ok {
		return nil
	}
	r := tw.GetRect()
	if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
		return nil
	}
	return tw
}

// InsertExplorerPathInTerminal is the keyboard path to the same result: the
// selected (or right-clicked) Explorer entry goes into the active terminal.
func (a *App) InsertExplorerPathInTerminal() {
	path := a.explorerNodePath()
	a.ExplorerContextNode = nil
	if path == "" {
		a.StatusWarn("No file selected")
		return
	}
	tw, ok := a.TerminalPanel.ActiveWidget().(*ui.TerminalWidget)
	if !ok || len(a.Terminals) == 0 {
		a.StatusWarn("No terminal open")
		return
	}
	a.showTerminalPanel()
	tw.PasteText(shellQuote(path) + " ")
}

// shellQuote leaves plain paths alone and quotes the rest for the shell:
// single quotes on Unix, double quotes on Windows.
func shellQuote(path string) string {
	if strings.IndexFunc(path, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-+:@,=~\\", r))
	}) < 0 {
		return path
	}
	if runtime.GOOS == "windows" {
		return `"` + path + `"`
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
