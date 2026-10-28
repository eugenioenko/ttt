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
// stays with the Explorer as usual; once the pointer moves to another row, or
// two columns along the pressed one, with the button held the drag starts, and
// from then on the mouse belongs to the drag until the button is released. A
// one-column wobble while clicking stays a click.
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
			// The Explorer saw the press; give it the release where the press
			// was, or it keeps the button held and drops the next click.
			a.Root.HandleEvent(tcell.NewEventMouse(d.px, d.py, tcell.ButtonNone, tcell.ModNone))
			if tw := a.terminalAt(mx, my); tw != nil && !a.Root.HasModalOverlay() {
				tw.PasteText(terminalQuote(d.path, tw) + " ")
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
	if !filepath.IsAbs(path) {
		a.StatusWarn("No file selected")
		return
	}
	tw, ok := a.TerminalPanel.ActiveWidget().(*ui.TerminalWidget)
	if !ok || len(a.Terminals) == 0 {
		a.StatusWarn("No terminal open")
		return
	}
	a.showTerminalPanel()
	tw.PasteText(terminalQuote(path, tw) + " ")
}

// terminalQuote quotes path for the shell running in tw.
func terminalQuote(path string, tw *ui.TerminalWidget) string {
	program := ""
	if tw != nil && tw.Term != nil {
		program = tw.Term.Program()
	}
	return shellQuote(path, program)
}

// shellQuote leaves plain paths alone and quotes the rest so the shell named
// by program reads them literally. PowerShell expands $(...) inside double
// quotes, so it gets single quotes like a POSIX shell; cmd has no literal
// quoting and gets double quotes.
func shellQuote(path, program string) string {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(program), ".exe"))
	if program == "" && runtime.GOOS == "windows" {
		name = "cmd"
	}
	plain := "/._-+:@,=~"
	if name == "pwsh" || name == "powershell" || name == "cmd" {
		plain = `\/._-:`
	}
	if strings.IndexFunc(path, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(plain, r))
	}) < 0 {
		return path
	}
	switch name {
	case "pwsh", "powershell":
		return "'" + strings.ReplaceAll(path, "'", "''") + "'"
	case "cmd":
		return `"` + path + `"`
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
