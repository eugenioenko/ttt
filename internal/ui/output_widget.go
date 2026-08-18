package ui

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

// A chatty producer (an LSP server dumping stderr) would otherwise grow the log
// without bound. Lines are dropped in chunks so the O(n) rebuild that a trim
// forces is amortised across outputTrimChunk appends.
const (
	outputMaxLines  = 5000
	outputTrimChunk = 500
)

type OutputLine struct {
	Time       string
	PluginName string
	Level      string
	Message    string
}

type OutputWidget struct {
	Lines      []OutputLine
	nodes      []*widgets.TreeNode
	seq        int
	autoScroll bool
	tree       *widgets.TreeWidget
}

func NewOutputWidget() *OutputWidget {
	o := &OutputWidget{autoScroll: true}
	o.tree = widgets.NewListWidgetFromConfig(widgets.ListConfig{
		EmptyText: "No output",
		RenderItem: func(surface widgets.Surface, node *widgets.TreeNode, idx, y, w int, selected bool) {
			o.renderItem(surface, idx, y, w, selected)
		},
	})
	return o
}

func (o *OutputWidget) Focusable() bool                 { return true }
func (o *OutputWidget) SetFocused(f bool)               { o.tree.SetFocused(f) }
func (o *OutputWidget) IsFocused() bool                 { return o.tree.IsFocused() }
func (o *OutputWidget) GetRect() Rect                   { return Rect(o.tree.GetRect()) }
func (o *OutputWidget) SetRect(r Rect)                  { o.tree.SetRect(widgets.Rect(r)) }
func (o *OutputWidget) Height() int                     { return 0 }
func (o *OutputWidget) Width() int                      { return 0 }
func (o *OutputWidget) SetBoxModel(bm widgets.BoxModel) { o.tree.SetBoxModel(bm) }
func (o *OutputWidget) Render(surface Surface)          { o.tree.Render(surface) }

func (o *OutputWidget) HandleEvent(ev tcell.Event) EventResult {
	if kev, ok := ev.(*tcell.EventKey); ok && kev.Key() == tcell.KeyCtrlC {
		if o.CopySelected() {
			return EventConsumed
		}
	}

	prevSel := o.tree.SelectedIndex()
	result := EventResult(o.tree.HandleEvent(ev))
	if result != EventIgnored && o.tree.SelectedIndex() != prevSel {
		if o.tree.SelectedIndex() == o.tree.ItemCount()-1 {
			o.autoScroll = true
		} else {
			o.autoScroll = false
		}
	}
	return result
}

// CopySelected puts the selected line on the clipboard as it reads on screen.
func (o *OutputWidget) CopySelected() bool {
	idx := o.tree.SelectedIndex()
	if idx < 0 || idx >= len(o.Lines) {
		return false
	}
	line := o.Lines[idx]
	clipboard.Set(fmt.Sprintf("%s [%s] %s", line.Time, line.PluginName, line.Message))
	return true
}

func (o *OutputWidget) AddLine(line OutputLine) {
	o.Lines = append(o.Lines, line)
	o.nodes = append(o.nodes, &widgets.TreeNode{
		ID:    fmt.Sprintf("output-%d", o.seq),
		Label: line.Message,
	})
	o.seq++

	if len(o.Lines) > outputMaxLines {
		drop := len(o.Lines) - (outputMaxLines - outputTrimChunk)
		o.Lines = append([]OutputLine(nil), o.Lines[drop:]...)
		o.nodes = append([]*widgets.TreeNode(nil), o.nodes[drop:]...)
		o.tree.SetItems(o.nodes)
	} else {
		o.tree.AppendItem(o.nodes[len(o.nodes)-1])
	}

	if o.autoScroll {
		o.tree.SetSelectedIndex(len(o.Lines) - 1)
	}
}

func (o *OutputWidget) Clear() {
	o.Lines = nil
	o.nodes = nil
	o.tree.SetItems(nil)
	o.autoScroll = true
}

func (o *OutputWidget) renderItem(surface widgets.Surface, idx, y, w int, selected bool) {
	if idx >= len(o.Lines) {
		return
	}
	line := o.Lines[idx]

	style := term.StyleDefault
	if selected {
		style = term.StyleSidebarSelected
	}

	for x := 0; x < w; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
	}

	// Selection owns the whole row: a style carries its own background, so
	// keeping the muted prefix or the level color on a selected line would
	// punch holes in the selection bar.
	prefixStyle, levelStyle := style, style
	if !selected {
		prefixStyle = term.StyleMuted
		switch line.Level {
		case "error":
			levelStyle = term.StyleDanger
		case "warn":
			levelStyle = term.StyleWarning
		}
	}

	prefix := fmt.Sprintf("%s [%s]", line.Time, line.PluginName)
	x := surface.DrawText(1, y, prefix, w, prefixStyle)
	surface.DrawText(x+1, y, line.Message, w, levelStyle)
}
