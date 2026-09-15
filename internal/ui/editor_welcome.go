package ui

import (
	"fmt"
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/widgets"
)

// isPristineUntitled reports whether a tab is an untouched scratch buffer:
// virtual, empty and clean. Only such tabs make room for the welcome screen;
// anything the user typed or opened renders normally.
func isPristineUntitled(t *editorTab) bool {
	return t != nil && t.Virtual && t.Content == nil && t.Buf != nil && !t.Buf.Dirty &&
		len(t.Buf.Lines) <= 1 && (len(t.Buf.Lines) == 0 || t.Buf.Lines[0] == "")
}

// showingWelcome reports whether the editor area currently shows the welcome
// placeholder instead of a buffer: exactly one tab, and it is a pristine
// scratch buffer. An explicitly created empty tab next to other tabs is a
// real (if empty) file, so it renders normally.
func (g *EditorGroupWidget) showingWelcome() bool {
	return len(g.tabs) == 1 && isPristineUntitled(g.activeTab())
}

type welcomeLine struct {
	text  string
	style term.Style
}

func (g *EditorGroupWidget) welcomeLines() []welcomeLine {
	title := "ttt"
	if g.WelcomeVersion != "" {
		title += "  " + g.WelcomeVersion
	}
	lines := []welcomeLine{
		{title, term.StyleDefault},
		{"", term.StyleDefault},
	}
	hints := [][2]string{
		{"Ctrl+P", "Command palette"},
		{"Ctrl+K P", "Go to file"},
		{"Ctrl+N", "New file"},
		{"Ctrl+Q", "Quit"},
	}
	// Align the key column, then pad every hint to the same width so the
	// centered block keeps aligned columns instead of ragged per-line
	// centering.
	width := 0
	rendered := make([]string, 0, len(hints))
	for _, h := range hints {
		text := fmt.Sprintf("  %-10s  %s", h[0], h[1])
		rendered = append(rendered, text)
		if w := textwidth.String(text); w > width {
			width = w
		}
	}
	for _, text := range rendered {
		text += strings.Repeat(" ", width-textwidth.String(text))
		lines = append(lines, welcomeLine{text, term.StyleMuted})
	}
	return lines
}

func (g *EditorGroupWidget) renderWelcome(surface widgets.Surface) {
	w, h := surface.Size()
	lines := g.welcomeLines()
	if h < len(lines) {
		return
	}
	surface.Fill(term.Cell{Ch: ' '})
	y := (h - len(lines)) / 2
	for i, ln := range lines {
		if ln.text == "" {
			continue
		}
		x := (w - textwidth.String(ln.text)) / 2
		if x < 0 {
			x = 0
		}
		surface.DrawText(x, y+i, ln.text, 0, ln.style)
	}
}
