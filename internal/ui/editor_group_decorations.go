package ui

import (
	"math"
	"slices"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/term"
)

// Decoration tints a range of buffer lines or highlights text. Lines and
// columns are 0-based; columns are runes. Without Text it tints whole lines
// StartLine..EndLine; with Text it highlights from StartCol on StartLine to
// EndCol (exclusive) on EndLine. With Note it draws the note under
// StartLine instead, wrapped to the pane and indented like the line. Lines
// past the end of the file are ignored.
type Decoration struct {
	StartLine, StartCol int
	EndLine, EndCol     int
	Text                bool
	Note                string
	Style               term.Style
}

// noteMaxWidth keeps notes readable in a wide pane.
const noteMaxWidth = 80

// decoSource is one source's decorations for one file. overlay holds them
// on the file's open buffer, moved by the edits made since; it is built
// from decos when the file opens.
type decoSource struct {
	decos   []Decoration
	overlay *DiffOverlay
}

// SetDecorations replaces source's decorations for path; none removes them.
// Decorations of a file that is not open apply when it opens. They move with
// edits; a highlight on a line that is edited is dropped, as is a tint on a
// line that is deleted. They are dropped when the file's tab closes with
// unsaved edits, or when one action makes more edits than the buffer's change
// log keeps. A tab showing an inline diff hides them until the diff closes.
func (g *EditorGroupWidget) SetDecorations(source, path string, decos []Decoration) {
	if g.decoSources == nil {
		g.decoSources = make(map[string]map[string]*decoSource)
	}
	byPath := g.decoSources[source]
	if byPath == nil {
		byPath = make(map[string]*decoSource)
		g.decoSources[source] = byPath
	}
	g.catchUpDecorations(path)
	if len(decos) == 0 {
		delete(byPath, path)
	} else {
		byPath[path] = &decoSource{decos: decos}
	}
	g.applyDecorations(path)
}

// ClearDecorationsSource removes every decoration source set.
func (g *EditorGroupWidget) ClearDecorationsSource(source string) {
	byPath, ok := g.decoSources[source]
	if !ok {
		return
	}
	for path := range byPath {
		g.catchUpDecorations(path)
	}
	delete(g.decoSources, source)
	for path := range byPath {
		g.applyDecorations(path)
	}
}

// moveDecorations keeps decorations with their file when remap gives it a
// new path.
func (g *EditorGroupWidget) moveDecorations(remap func(string) (string, bool)) {
	for _, byPath := range g.decoSources {
		moved := map[string]*decoSource{}
		for p, src := range byPath {
			if np, ok := remap(p); ok {
				delete(byPath, p)
				moved[np] = src
			}
		}
		for p, src := range moved {
			byPath[p] = src
		}
	}
}

// decorationOverlay builds the overlay for decos on lines.
func decorationOverlay(decos []Decoration, lines []string, tabSize int) *DiffOverlay {
	o := NewAnnotationOverlay()
	n := len(lines)
	for _, d := range decos {
		if d.StartLine < 0 || d.EndLine < d.StartLine || d.StartLine >= n {
			continue
		}
		endLine := min(d.EndLine, n-1)
		if d.Note != "" {
			o.NoteTexts[d.StartLine] = append(o.NoteTexts[d.StartLine], NoteText{
				Text:     d.Note,
				Indent:   leadingIndent(lines[d.StartLine], tabSize),
				Style:    d.Style,
				MaxWidth: noteMaxWidth,
			})
			continue
		}
		if !d.Text {
			for line := d.StartLine; line <= endLine; line++ {
				o.LineBg[line] = d.Style
			}
			continue
		}
		for line := d.StartLine; line <= endLine; line++ {
			sp := ColSpan{Start: 0, End: math.MaxInt, Style: d.Style}
			if line == d.StartLine {
				sp.Start = d.StartCol
			}
			if line == d.EndLine {
				sp.End = d.EndCol
			}
			if sp.End > sp.Start {
				o.Spans[line] = append(o.Spans[line], sp)
			}
		}
	}
	return o
}

// catchUpDecorations moves path's source overlays across the edits its tab
// has seen since they were last merged. The merged overlay carries them along
// while the tab is shown; this covers edits made since its last render.
func (g *EditorGroupWidget) catchUpDecorations(path string) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].Buf == nil {
		return
	}
	if o := g.tabs[i].Annotations; o != nil {
		o.catchUp(g.tabs[i].Buf)
	}
}

// applyDecorations merges every source's decorations for path, in source
// order, into the overlay its tab shows. Later sources win where they overlap.
func (g *EditorGroupWidget) applyDecorations(path string) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].Buf == nil {
		return
	}
	t := &g.tabs[i]
	t.Annotations = g.mergedDecorations(path, t.Buf, t.TabSize)
	if i == g.active && t.Diff == nil {
		g.Editor.SetDiffOverlay(t.Annotations)
	}
}

// mergedDecorations returns the overlay for path on buf, or nil when no
// source decorates it.
func (g *EditorGroupWidget) mergedDecorations(path string, buf *buffer.Buffer, tabSize int) *DiffOverlay {
	sources := make([]string, 0, len(g.decoSources))
	for source, byPath := range g.decoSources {
		if byPath[path] != nil {
			sources = append(sources, source)
		}
	}
	if len(sources) == 0 {
		return nil
	}
	slices.Sort(sources)
	merged := NewAnnotationOverlay()
	merged.buf, merged.ver = buf, buf.Version()
	for _, source := range sources {
		part := g.bindDecorations(g.decoSources[source][path], buf, tabSize)
		if part == nil {
			delete(g.decoSources[source], path)
			continue
		}
		for line, style := range part.LineBg {
			merged.LineBg[line] = style
		}
		for line, spans := range part.Spans {
			merged.Spans[line] = append(merged.Spans[line], spans...)
		}
		for line, notes := range part.NoteTexts {
			merged.NoteTexts[line] = append(merged.NoteTexts[line], notes...)
		}
		merged.parts = append(merged.parts, part)
	}
	if len(merged.parts) == 0 {
		return nil
	}
	return merged
}

// bindDecorations returns src's overlay brought up to date on buf, or nil
// when its positions no longer match the text: it fell too far behind buf's
// edits, or it belonged to a buffer that was closed with unsaved edits.
func (g *EditorGroupWidget) bindDecorations(src *decoSource, buf *buffer.Buffer, tabSize int) *DiffOverlay {
	if o := src.overlay; o != nil && o.buf != buf {
		if o.buf.Dirty {
			return nil
		}
		o.buf, o.ver, o.stale = buf, buf.Version(), false
	}
	if src.overlay == nil {
		src.overlay = decorationOverlay(src.decos, buf.Lines, tabSize)
		src.overlay.buf, src.overlay.ver = buf, buf.Version()
	}
	if !src.overlay.catchUp(buf) {
		return nil
	}
	return src.overlay
}

// catchUp replays buf's edits since o was last brought up to date, reporting
// whether o is current.
func (o *DiffOverlay) catchUp(buf *buffer.Buffer) bool {
	if o.buf != buf || o.stale {
		return false
	}
	if o.ver == buf.Version() {
		return true
	}
	if !buf.ChangesSince(o.ver, o.splice) {
		o.stale = true
	}
	o.ver = buf.Version()
	o.syncParts()
	return !o.stale
}

// syncParts records o's version on its parts. An annotation overlay that went
// stale can no longer say where its decorations belong, so it drops them.
func (o *DiffOverlay) syncParts() {
	if o.stale && o.annotate {
		clear(o.LineBg)
		clear(o.Spans)
		clear(o.NoteTexts)
		o.notesMoved = len(o.Notes) > 0
	}
	for _, part := range o.parts {
		part.ver, part.stale = o.ver, o.stale
		part.syncParts()
	}
}
