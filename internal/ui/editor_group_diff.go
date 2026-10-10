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
	lines        []diff.DiffLine
	headActive   bool
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
	s.lines = lines
	s.unified = NewDiffOverlay(lines)
	s.left, s.right = NewSplitDiffOverlays(lines)
	if s.head != nil {
		s.head.Buf.Lines = base
		s.head.Cursor.Line = s.head.Buf.ClampLine(s.head.Cursor.Line)
		s.head.Selection.Clear()
		if s.head.Highlighter != nil {
			s.head.Highlighter.ClearCache()
		}
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
	h.WordWrap = g.Editor.WordWrap
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
	s := t.InlineDiff
	head := g.headPane(t)
	s.headRect = Rect{X: rect.X, Y: rect.Y, W: leftW, H: h}
	head.SetRect(s.headRect)
	g.Editor.SetRect(Rect{X: rect.X + rightX, Y: rect.Y, W: w - rightX, H: h})
	for pass := 0; pass < 2; pass++ {
		widths := [2]int{head.Viewport.Width, g.Editor.Viewport.Width}
		s.left.Fillers, s.right.Fillers = alignSplitFillers(s.lines, head, g.Editor)
		head.SetDiffOverlay(s.left)
		g.Editor.SetDiffOverlay(s.right)
		g.Editor.Render(surface.Sub(Rect{X: rightX, Y: 0, W: w - rightX, H: h}))
		syncFollower(g.Editor, head)
		head.Render(surface.Sub(Rect{X: 0, Y: 0, W: leftW, H: h}))
		if !g.Editor.WordWrap || widths == [2]int{head.Viewport.Width, g.Editor.Viewport.Width} {
			break
		}
	}
	for y := 0; y < h; y++ {
		surface.SetCell(leftW, y, term.Cell{Ch: '│', Style: term.StyleBorder})
	}
}

// splitHeadEvent routes pointer input over the read-only HEAD pane to it, so
// its text can be selected and copied; wheel scrolling still drives the
// editor so both panes stay in step.
func (g *EditorGroupWidget) splitHeadEvent(t *editorTab, ev tcell.Event) (EventResult, bool) {
	s := t.InlineDiff
	if !s.split() || g.Editor.mouseDown {
		return EventIgnored, false
	}
	if _, ok := ev.(*tcell.EventKey); ok {
		s.headActive = false
		return EventIgnored, false
	}
	mev, ok := ev.(*tcell.EventMouse)
	if !ok || s.head == nil {
		return EventIgnored, false
	}
	x, y := mev.Position()
	r := s.headRect
	inside := x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
	if !inside && !s.head.mouseDown {
		if mev.Buttons()&tcell.Button1 != 0 {
			s.headActive = false
			s.head.Selection.Clear()
		}
		return EventIgnored, false
	}
	if mev.Buttons()&(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) != 0 {
		return g.Editor.HandleEvent(ev), true
	}
	if mev.Buttons()&tcell.Button1 != 0 {
		s.headActive = true
	}
	result := s.head.HandleEvent(ev)
	if result == EventIgnored {
		result = EventConsumed
	}
	return result, true
}

func (g *EditorGroupWidget) inlineHeadSelection() (string, bool) {
	t := g.activeTab()
	if t == nil || t.Content != nil || !t.InlineDiff.split() {
		return "", false
	}
	s := t.InlineDiff
	if !s.headActive || s.head == nil || !s.head.Selection.Active {
		return "", false
	}
	h := s.head
	return h.Selection.Text(h.Buf.Lines, h.Cursor.Line, h.Cursor.Col), true
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
