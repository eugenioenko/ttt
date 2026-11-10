package ui

import (
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/gdamore/tcell/v3"
)

// Clipped links get at most 2048 runes of context per side; targets crossing that bound are omitted.
const editorLinkLookaround = 2048

// FileLinkResolver checks paths off-thread and delivers apply on the UI goroutine.
type FileLinkResolver func(paths []string, dir string, apply func(map[string]string))

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
type editorLinkFrame struct {
	viewport view.Viewport
	rect     Rect
	offset   int
	wrap     bool
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
	detect          func(string) []linkSpan
	windows         []editorLinkWindow
	resolved        map[string]string
	generation      uint64
	pending         bool
	frame           editorLinkFrame
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
	if filepath.IsAbs(path) {
		dir = filepath.Dir(path)
	} else if g.LinkWorkDir != nil {
		dir = g.LinkWorkDir()
	}
	e.LinkWorkDir = dir
	e.ControlModifiers = g.ControlModifiers
	e.OnOpenURL, e.OnOpenFile, e.OnLinkHover = g.OnOpenURL, g.OnOpenFileLink, g.OnLinkHover
	e.ResolveFileLinks = g.ResolveFileLinks
}

func (r *Root) TrackEditorLinkHover(e *EditorPaneWidget) {
	if r.linkHover != nil && r.linkHover != e {
		r.linkHover.clearLinkHover()
	}
	r.linkHover = e
}

func (r *Root) ClearEditorLinkHover() {
	if e := r.linkHover; e != nil {
		e.clearLinkHover()
		// Keep the gesture's owner until release, even if opening a link switches tabs.
		if !e.links.pressed {
			r.linkHover = nil
		}
	}
}

func (e *EditorPaneWidget) clearLinkHover() {
	e.links.active = false
	e.links.generation++
	e.links.pending = false
	clear(e.links.hits)
}

func (e *EditorPaneWidget) observeLinkInput(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventMouse:
		x, y := ev.Position()
		if ev.Buttons()&tcell.Button1 == 0 {
			e.links.pressed = false
		}
		if !term.ControlModifierHeld(ev.Modifiers(), e.ControlModifiers) || ev.Modifiers()&tcell.ModAlt != 0 || !e.GetRect().Contains(x, y) {
			e.clearLinkHover()
		}
	case *tcell.EventKey:
		if !term.ControlModifierHeld(ev.Modifiers(), e.ControlModifiers) {
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
		l.resolved = nil
		l.generation++
		l.pending = false
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

func visibleLinkWindow(text string, start, end, col int) (string, int, bool, bool) {
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
	left, _ := utf8.DecodeLastRuneInString(text[:start])
	right, _ := utf8.DecodeRuneInString(text[end:])
	return strings.Clone(text[start:end]), col, start > 0 && !unicode.IsSpace(left), end < len(text) && !unicode.IsSpace(right)
}

func (e *EditorPaneWidget) linksForWindow(window editorLinkWindow) []linkSpan {
	l := &e.links
	if spans, ok := l.memo[window]; ok {
		return spans
	}
	if l.memo == nil || len(l.memo) >= linkMemoMax {
		l.memo = make(map[editorLinkWindow][]linkSpan)
	}
	text, col, clippedLeft, clippedRight := visibleLinkWindow(e.Buf.Lines[window.line], window.start, window.end, window.col)
	detect := l.detect
	if detect == nil {
		detect = detectLinkCandidates
	}
	spans := detect(text)
	complete := spans[:0]
	for _, span := range spans {
		// Trailing punctuation may have been trimmed by detection at a clipped edge.
		if (clippedLeft && span.StartCol == 0) || (clippedRight && strings.TrimRight(text[runeByteOffset(text, span.EndCol):], ".,;:") == "") {
			continue
		}
		span.StartCol += col
		span.EndCol += col
		complete = append(complete, span)
	}
	l.memo[window] = complete
	return complete
}

func (e *EditorPaneWidget) renderLinks(surface Surface) {
	l := &e.links
	e.resolveVisibleFileLinks()
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
					if span.IsFile {
						path := l.resolved[span.FilePath]
						if path == "" {
							continue
						}
						resolved := *span
						resolved.FilePath = path
						span = &resolved
					}
					c.cell.Underline = c.cell.UlStyle == 0
					surface.SetCell(c.point.x, c.point.y, c.cell)
					l.hits[c.point] = span
					break
				}
			}
		}
	}
}

func (e *EditorPaneWidget) linkFrame() editorLinkFrame {
	return editorLinkFrame{*e.Viewport, e.GetRect(), e.topRowOffset, e.WordWrap}
}

func (e *EditorPaneWidget) resolveVisibleFileLinks() {
	l := &e.links
	windows := make([]editorLinkWindow, len(l.lines))
	for i, line := range l.lines {
		windows[i] = line.window
	}
	frame := e.linkFrame()
	if frame != l.frame || !slices.Equal(windows, l.windows) {
		l.windows, l.frame = windows, frame
		l.generation++
		l.pending = false
	}
	files := make(map[string]bool)
	for _, line := range l.lines {
		for _, span := range e.linksForWindow(line.window) {
			if span.IsFile && e.OnOpenFile != nil && span.StartCol <= l.cells[line.last-1].col && span.EndCol > l.cells[line.first].col {
				files[span.FilePath] = true
			}
		}
	}
	for path := range l.resolved {
		if !files[path] {
			delete(l.resolved, path)
		}
	}
	var paths []string
	for path := range files {
		if _, ok := l.resolved[path]; !ok {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 || l.pending || e.ResolveFileLinks == nil {
		return
	}
	l.pending = true
	generation, buf, version, dir := l.generation, l.buf, l.version, l.dir
	e.ResolveFileLinks(paths, dir, func(resolved map[string]string) {
		if l.generation != generation {
			return
		}
		l.pending = false
		if resolved == nil || !l.active || e.Buf != buf || e.Buf.Version() != version || e.LinkWorkDir != dir || e.linkFrame() != frame {
			return
		}
		if l.resolved == nil {
			l.resolved = make(map[string]string)
		}
		for _, path := range paths {
			l.resolved[path] = resolved[path]
		}
	})
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
	if !term.ControlModifierHeld(ev.Modifiers(), e.ControlModifiers) || ev.Modifiers()&tcell.ModAlt != 0 ||
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
