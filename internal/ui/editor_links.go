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

// Clipped targets get at most 2048 runes of context per side; longer tokens are omitted.
const editorLinkLookaround = 2048

type linkPoint struct{ x, y int }
type editorLinkCell struct {
	point linkPoint
	col   int
	cell  term.Cell
}
type editorLinkWindow struct{ line, col, start, end int }
type editorLinkLine struct {
	window      editorLinkWindow
	first, last int
}
type editorLinks struct {
	active, pressed bool
	cells           []editorLinkCell
	lines           []editorLinkLine
	hits            map[linkPoint]*linkSpan
	memo            map[editorLinkWindow][]linkSpan
	buf             *buffer.Buffer
	version         uint64
	dir             string
	detect          func(string, string) []linkSpan
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
	clear(e.links.hits)
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

func (e *EditorPaneWidget) beginLinkFrame() bool {
	l := &e.links
	l.cells, l.lines = l.cells[:0], l.lines[:0]
	clear(l.hits)
	if l.buf != e.Buf || l.version != e.Buf.Version() || l.dir != e.LinkWorkDir {
		l.memo = nil
		l.buf, l.version, l.dir = e.Buf, e.Buf.Version(), e.LinkWorkDir
	}
	return l.active && (e.OnOpenURL != nil || e.OnOpenFile != nil)
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
		if skip := strings.IndexFunc(text[start:end], unicode.IsSpace); skip >= 0 {
			col += utf8.RuneCountInString(text[start : start+skip])
			start += skip
		} else {
			return "", col
		}
	}
	if r, _ := utf8.DecodeRuneInString(text[end:]); end < len(text) && !unicode.IsSpace(r) {
		if stop := strings.LastIndexFunc(text[start:end], unicode.IsSpace); stop >= 0 {
			end = start + stop
		} else {
			return "", col
		}
	}
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

func (e *EditorPaneWidget) renderLinks(surface Surface) {
	l := &e.links
	if l.hits == nil {
		l.hits = make(map[linkPoint]*linkSpan)
	}
	for _, line := range l.lines {
		spans := e.linksForWindow(line.window)
		for _, c := range l.cells[line.first:line.last] {
			for i := range spans {
				span := &spans[i]
				if c.col >= span.StartCol && c.col < span.EndCol &&
					((span.IsFile && e.OnOpenFile != nil) || (!span.IsFile && e.OnOpenURL != nil)) {
					c.cell.Underline = c.cell.UlStyle == 0
					surface.SetCell(c.point.x, c.point.y, c.cell)
					l.hits[c.point] = span
					break
				}
			}
		}
	}
}

func (e *EditorPaneWidget) editorLinkAt(mx, my int) (linkSpan, bool) {
	r := e.GetRect()
	span := e.links.hits[linkPoint{mx - r.X, my - r.Y}]
	if span != nil {
		return *span, true
	}
	return linkSpan{}, false
}

func (e *EditorPaneWidget) handleLinkMouse(ev *tcell.EventMouse) bool {
	if e.links.pressed {
		return true
	}
	x, y := ev.Position()
	if ev.Modifiers()&tcell.ModCtrl == 0 || ev.Modifiers()&tcell.ModAlt != 0 ||
		!e.GetRect().Contains(x, y) || e.mouseDown {
		return false
	}
	e.links.active = true
	if e.OnLinkHover != nil {
		e.OnLinkHover(e)
	}
	span, ok := e.editorLinkAt(x, y)
	if !ok || ev.Buttons()&tcell.Button1 == 0 {
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
