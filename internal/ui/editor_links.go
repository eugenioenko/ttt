package ui

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

// Recover clipped targets with at most 2048 runes of context on either side.
// A token that still crosses that boundary is omitted, never opened truncated.
const editorLinkLookaround = 2048

type linkPoint struct{ x, y int }

type editorLinkCell struct {
	// position uses the rendered text grid's frame.width, excluding the gutter.
	position, col int
}

type editorLinkWindow struct {
	line, col, start, end int
}

type editorLinkLine struct {
	window      editorLinkWindow
	first, last int
}

type editorLinkFrame struct {
	buf                 *buffer.Buffer
	version             uint64
	dir                 string
	rect                Rect
	top, offset, left   int
	width, height, tabW int
	wrap                bool
	overlay             *DiffOverlay
	phantomGen          uint64
}

type editorLinks struct {
	active, pressed bool
	frame           editorLinkFrame
	hits            map[linkPoint]*linkSpan
	// Readiness is separate from allocation so empty results can reuse the map.
	hitsReady bool
	memo      map[editorLinkWindow][]linkSpan
	// A first Ctrl-click uses rendered geometry instead of rescanning hidden text.
	cells  []editorLinkCell
	lines  []editorLinkLine
	styles []term.Cell
	detect func(string, string) []linkSpan
}

func (t *editorTab) diffWidget() *DiffEditorWidget {
	if t.Diff != nil {
		return t.Diff
	}
	d, _ := t.Content.(*DiffEditorWidget)
	return d
}

func (g *EditorGroupWidget) configurePaneLinks(e *EditorPaneWidget, path string) {
	dir := ""
	if path != "" {
		dir = filepath.Dir(path)
	} else if g.LinkWorkDir != nil {
		dir = g.LinkWorkDir()
	}
	e.LinkWorkDir = dir
	e.OnOpenURL, e.OnOpenFile, e.OnLinkHover = g.OnOpenURL, g.OnOpenFileLink, g.OnLinkHover
}

func (r *Root) TrackEditorLinkHover(e *EditorPaneWidget) {
	if r.linkHover != nil && r.linkHover != e {
		r.linkHover.clearLinkHover()
	}
	r.linkHover = e
}

func (e *EditorPaneWidget) clearLinkHover() {
	e.links.active = false
	e.links.invalidateHits()
}

func (l *editorLinks) invalidateHits() {
	l.hitsReady = false
	clear(l.hits)
}

func (e *EditorPaneWidget) observeLinkInput(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventMouse:
		x, y := ev.Position()
		if ev.Buttons()&tcell.Button1 == 0 {
			e.links.pressed = false
		}
		if ev.Modifiers()&tcell.ModCtrl == 0 || ev.Modifiers()&tcell.ModAlt != 0 || !e.GetRect().Contains(x, y) {
			e.clearLinkHover()
		}
	case *tcell.EventKey:
		if ev.Modifiers()&tcell.ModCtrl == 0 {
			e.links.pressed = false
			e.clearLinkHover()
		}
	case *tcell.EventFocus:
		if !ev.Focused {
			e.links.pressed = false
			e.clearLinkHover()
		}
	}
}

func (e *EditorPaneWidget) linkFrame() editorLinkFrame {
	return editorLinkFrame{
		buf: e.Buf, version: e.Buf.Version(), dir: e.LinkWorkDir, rect: e.GetRect(),
		top: e.Viewport.TopLine, offset: e.topOffset(), left: e.Viewport.LeftCol,
		width: e.Viewport.Width, height: e.Viewport.Height, tabW: e.resolveTabSize(),
		wrap: e.WordWrap, overlay: e.DiffOverlay, phantomGen: e.phantomGen,
	}
}

func (e *EditorPaneWidget) beginLinkFrame() {
	l := &e.links
	l.cells = l.cells[:0]
	l.lines = l.lines[:0]
	l.styles = l.styles[:0]
	l.invalidateHits()
	frame := e.linkFrame()
	if frame.buf != l.frame.buf || frame.version != l.frame.version || frame.dir != l.frame.dir {
		l.memo = nil
	}
	l.frame = frame
}

func (l *editorLinks) finishRow(window editorLinkWindow, first int) {
	last := len(l.cells)
	if first == last {
		return
	}
	if n := len(l.lines); n > 0 && l.lines[n-1].window.line == window.line {
		l.lines[n-1].window.end = window.end
		l.lines[n-1].last = last
	} else {
		l.lines = append(l.lines, editorLinkLine{window, first, last})
	}
}

func visibleLinkWindow(text string, start, end, col int) (string, int) {
	for n := 0; n < editorLinkLookaround && start > 0; n++ {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if unicode.IsSpace(r) {
			break
		}
		start -= size
		col--
	}
	for n := 0; n < editorLinkLookaround && end < len(text); n++ {
		r, size := utf8.DecodeRuneInString(text[end:])
		if unicode.IsSpace(r) {
			break
		}
		end += size
	}
	if r, _ := utf8.DecodeLastRuneInString(text[:start]); start > 0 && !unicode.IsSpace(r) {
		for start < end {
			r, size := utf8.DecodeRuneInString(text[start:end])
			if unicode.IsSpace(r) {
				break
			}
			start += size
			col++
		}
	}
	if r, _ := utf8.DecodeRuneInString(text[end:]); end < len(text) && !unicode.IsSpace(r) {
		for end > start {
			r, size := utf8.DecodeLastRuneInString(text[start:end])
			if unicode.IsSpace(r) {
				break
			}
			end -= size
		}
	}
	// Cached targets must not retain a very long buffer line through a substring.
	return strings.Clone(text[start:end]), col
}

func (e *EditorPaneWidget) linksForWindow(window editorLinkWindow) []linkSpan {
	l := &e.links
	if spans, ok := l.memo[window]; ok {
		return spans
	}
	if l.memo == nil || len(l.memo) >= linkMemoMax {
		l.memo = make(map[editorLinkWindow][]linkSpan)
	}
	text, col := visibleLinkWindow(e.Buf.Lines[window.line], window.start, window.end, window.col)
	detect := l.detect
	if detect == nil {
		detect = detectLinks
	}
	spans := detect(text, e.LinkWorkDir)
	for i := range spans {
		spans[i].StartCol += col
		spans[i].EndCol += col
	}
	l.memo[window] = spans
	return spans
}

func (e *EditorPaneWidget) linkHits() map[linkPoint]*linkSpan {
	l := &e.links
	if !l.active || l.frame != e.linkFrame() {
		return nil
	}
	if l.hitsReady {
		return l.hits
	}
	if l.hits == nil {
		l.hits = make(map[linkPoint]*linkSpan)
	}
	gutterW := e.GutterWidth()
	for _, line := range l.lines {
		spans := e.linksForWindow(line.window)
		for _, c := range l.cells[line.first:line.last] {
			for i := range spans {
				span := &spans[i]
				if c.col >= span.StartCol && c.col < span.EndCol &&
					((span.IsFile && e.OnOpenFile != nil) || (!span.IsFile && e.OnOpenURL != nil)) {
					point := linkPoint{gutterW + c.position%l.frame.width, c.position / l.frame.width}
					l.hits[point] = span
					break
				}
			}
		}
	}
	l.hitsReady = true
	return l.hits
}

func (e *EditorPaneWidget) renderLinks(surface Surface) {
	hits := e.linkHits()
	if len(hits) == 0 {
		return
	}
	gutterW := e.GutterWidth()
	for i, cell := range e.links.styles {
		pos := e.links.cells[i].position
		point := linkPoint{gutterW + pos%e.links.frame.width, pos / e.links.frame.width}
		if hits[point] != nil {
			cell.Underline = cell.UlStyle == 0
			surface.SetCell(point.x, point.y, cell)
		}
	}
}

func (e *EditorPaneWidget) editorLinkAt(mx, my int) (linkSpan, bool) {
	r := e.GetRect()
	if span := e.linkHits()[linkPoint{mx - r.X, my - r.Y}]; span != nil {
		return *span, true
	}
	return linkSpan{}, false
}

func (e *EditorPaneWidget) handleLinkMouse(ev *tcell.EventMouse) bool {
	x, y := ev.Position()
	if e.links.pressed {
		return true
	}
	if e.OnOpenURL == nil && e.OnOpenFile == nil {
		e.clearLinkHover()
		return false
	}
	if ev.Modifiers()&tcell.ModCtrl == 0 || ev.Modifiers()&tcell.ModAlt != 0 ||
		!e.GetRect().Contains(x, y) || e.mouseDown {
		return false
	}
	e.links.active = true
	if e.OnLinkHover != nil {
		e.OnLinkHover(e)
	}
	if ev.Buttons()&tcell.Button1 == 0 {
		return false
	}
	span, ok := e.editorLinkAt(x, y)
	if !ok {
		return false
	}
	e.links.pressed = true
	if span.IsFile && e.OnOpenFile != nil {
		e.OnOpenFile(span.FilePath, span.Line, span.Col)
	} else if !span.IsFile && e.OnOpenURL != nil {
		e.OnOpenURL(span.URL)
	}
	return true
}
