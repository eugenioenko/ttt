package ui

import (
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

type inlineDiffState struct {
	Base         []string
	Mode         DiffMode
	modeExplicit bool
	unified      *DiffOverlay
	left         *DiffOverlay
	right        *DiffOverlay
	head         *EditorPaneWidget
	headRect     Rect
}

func (s *inlineDiffState) setLines(base []string, lines []diff.DiffLine) {
	s.Base = base
	s.unified = NewDiffOverlay(lines)
	s.left, s.right = NewSplitDiffOverlays(lines)
	if s.head != nil {
		s.head.Buf.Lines = base
		s.head.SetDiffOverlay(s.left)
	}
}

func (s *inlineDiffState) overlay() *DiffOverlay {
	if s == nil {
		return nil
	}
	if s.Mode == DiffModeSplit {
		return s.right
	}
	return s.unified
}

func (s *inlineDiffState) split() bool {
	return s != nil && s.Mode == DiffModeSplit
}

func (g *EditorGroupWidget) tabIndexByPath(path string) int {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].Content == nil && g.tabs[i].Buf != nil {
			return i
		}
	}
	return -1
}

func (g *EditorGroupWidget) EnableInlineDiff(path string, base []string) bool {
	i := g.tabIndexByPath(path)
	if i < 0 {
		return false
	}
	t := &g.tabs[i]
	t.Preview = false
	t.InlineDiff = &inlineDiffState{Mode: g.DiffMode}
	t.InlineDiff.setLines(base, diff.FullDiffLines(base, t.Buf.Lines))
	if i == g.active {
		g.syncTabs()
	}
	return true
}

func (g *EditorGroupWidget) DisableInlineDiff(path string) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].InlineDiff == nil {
		return
	}
	g.tabs[i].InlineDiff = nil
	if i == g.active {
		g.syncTabs()
	}
}

func (g *EditorGroupWidget) SetInlineDiff(path string, base []string, lines []diff.DiffLine) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].InlineDiff == nil {
		return
	}
	g.tabs[i].InlineDiff.setLines(base, lines)
	if i == g.active {
		g.Editor.SetDiffOverlay(g.tabs[i].InlineDiff.overlay())
	}
}

func (g *EditorGroupWidget) IsInlineDiffPath(path string) bool {
	i := g.tabIndexByPath(path)
	return i >= 0 && g.tabs[i].InlineDiff != nil
}

func (g *EditorGroupWidget) IsInlineDiffActive() bool {
	t := g.activeTab()
	return t != nil && t.Content == nil && t.InlineDiff != nil
}

func (g *EditorGroupWidget) headPane(t *editorTab) *EditorPaneWidget {
	s := t.InlineDiff
	if s.head == nil {
		buf := &buffer.Buffer{Lines: s.Base}
		head := NewEditorPaneWidget(buf, &cursor.Cursor{}, &view.Viewport{})
		head.Undo = g.newUndoStack()
		head.Selection = &selection.Selection{}
		head.ReadOnly = true
		if g.SyntaxHighlight {
			head.Highlighter = highlight.New(t.FilePath)
		}
		head.SetDiffOverlay(s.left)
		s.head = head
	}
	h := s.head
	h.LineNumbers = g.Editor.LineNumbers
	h.GutterStyle = g.Editor.GutterStyle
	h.TabSize = g.Editor.TabSize
	h.WordWrap = false
	h.Passive = true
	return h
}

func (g *EditorGroupWidget) renderSplitDiff(t *editorTab, surface Surface, rect Rect) {
	w, h := surface.Size()
	leftW := (w - 1) / 2
	if leftW < 1 {
		g.Editor.SetRect(rect)
		g.Editor.Render(surface)
		return
	}
	rightX := leftW + 1
	g.Editor.SetRect(Rect{X: rect.X + rightX, Y: rect.Y, W: w - rightX, H: h})
	g.Editor.Render(surface.Sub(Rect{X: rightX, Y: 0, W: w - rightX, H: h}))

	head := g.headPane(t)
	t.InlineDiff.headRect = Rect{X: rect.X, Y: rect.Y, W: leftW, H: h}
	head.SetRect(t.InlineDiff.headRect)
	head.Viewport.Width = g.Editor.Viewport.Width
	head.Viewport.Height = g.Editor.Viewport.Height
	head.setTopRow(head.layout(), g.Editor.topRow(g.Editor.layout()))
	head.Viewport.LeftCol = g.Editor.Viewport.LeftCol
	head.Render(surface.Sub(Rect{X: 0, Y: 0, W: leftW, H: h}))
	for y := 0; y < h; y++ {
		surface.SetCell(leftW, y, term.Cell{Ch: '│', Style: term.StyleBorder})
	}
}

// splitHeadEvent keeps the read-only HEAD pane passive: wheel scrolling is
// forwarded to the editor so both panes stay in step.
func (g *EditorGroupWidget) splitHeadEvent(t *editorTab, ev tcell.Event) (EventResult, bool) {
	if !t.InlineDiff.split() || g.Editor.mouseDown {
		return EventIgnored, false
	}
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored, false
	}
	x, y := mev.Position()
	r := t.InlineDiff.headRect
	if x < r.X || x >= r.X+r.W || y < r.Y || y >= r.Y+r.H {
		return EventIgnored, false
	}
	if mev.Buttons()&(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) != 0 {
		return g.Editor.HandleEvent(ev), true
	}
	return EventConsumed, true
}

type inlineDiffSurface struct {
	g *EditorGroupWidget
	s *inlineDiffState
}

func (g *EditorGroupWidget) activeInlineDiffSurface() DiffModeSurface {
	t := g.activeTab()
	if t == nil || t.Content != nil || t.InlineDiff == nil {
		return nil
	}
	return inlineDiffSurface{g: g, s: t.InlineDiff}
}

func (d inlineDiffSurface) Mode() DiffMode { return d.s.Mode }

func (d inlineDiffSurface) SetMode(mode DiffMode) {
	d.s.modeExplicit = true
	d.applyMode(mode)
}

func (d inlineDiffSurface) ApplyDefaultMode(mode DiffMode) {
	if !d.s.modeExplicit {
		d.applyMode(mode)
	}
}

func (d inlineDiffSurface) applyMode(mode DiffMode) {
	if d.s.Mode == mode {
		return
	}
	d.s.Mode = mode
	if t := d.g.activeTab(); t != nil && t.InlineDiff == d.s {
		d.g.Editor.SetDiffOverlay(d.s.overlay())
	}
}

func (d inlineDiffSurface) WrapMode() DiffWrapMode            { return DiffWrapOff }
func (d inlineDiffSurface) SetWrapMode(DiffWrapMode)          {}
func (d inlineDiffSurface) ApplyDefaultWrapMode(DiffWrapMode) {}
func (d inlineDiffSurface) SetDiffHighContrast(bool)          {}
func (d inlineDiffSurface) SetDiffCollapsedEmphasis(bool)     {}
