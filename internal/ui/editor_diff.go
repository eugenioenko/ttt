package ui

import (
	"strconv"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
)

// DiffOverlay decorates an editor buffer with a diff against an older
// version: Kinds is indexed by buffer line, Deleted holds removed old lines
// and Fillers blank alignment rows, both keyed by the buffer line they precede
// (len(Kinds) for end of file). A block's deleted rows come before its fillers.
type DiffOverlay struct {
	Kinds   []diff.LineKind
	Deleted map[int][]diff.SideLine
	Fillers map[int]int
}

// NewDiffOverlay projects full-file diff lines onto the new side. A change
// block shows all of its removals before its first addition, matching the
// unified diff view.
func NewDiffOverlay(lines []diff.DiffLine) *DiffOverlay {
	o := &DiffOverlay{Deleted: make(map[int][]diff.SideLine)}
	for i := 0; i < len(lines); {
		dl := lines[i]
		if dl.Left.Kind != diff.Deleted && dl.Right.Kind != diff.Added {
			if dl.Right.Kind != diff.Blank {
				o.Kinds = append(o.Kinds, diff.Context)
			}
			i++
			continue
		}
		anchor := len(o.Kinds)
		end := i
		for end < len(lines) && (lines[end].Left.Kind == diff.Deleted || lines[end].Right.Kind == diff.Added) {
			if lines[end].Left.Kind == diff.Deleted {
				o.Deleted[anchor] = append(o.Deleted[anchor], lines[end].Left)
			}
			end++
		}
		for ; i < end; i++ {
			if lines[i].Right.Kind == diff.Added {
				o.Kinds = append(o.Kinds, diff.Added)
			}
		}
	}
	return o
}

// NewSplitDiffOverlays aligns both sides of a split diff: each diff row
// yields exactly one screen row per side, so equal top rows stay in step.
func NewSplitDiffOverlays(lines []diff.DiffLine) (left, right *DiffOverlay) {
	left = &DiffOverlay{Fillers: make(map[int]int)}
	right = &DiffOverlay{Fillers: make(map[int]int)}
	for _, dl := range lines {
		if dl.Left.Kind == diff.Blank {
			left.Fillers[len(left.Kinds)]++
		} else {
			left.Kinds = append(left.Kinds, dl.Left.Kind)
		}
		if dl.Right.Kind == diff.Blank {
			right.Fillers[len(right.Kinds)]++
		} else {
			right.Kinds = append(right.Kinds, dl.Right.Kind)
		}
	}
	return left, right
}

func (o *DiffOverlay) kind(line int) diff.LineKind {
	if o == nil || line < 0 || line >= len(o.Kinds) {
		return diff.Context
	}
	return o.Kinds[line]
}

func (o *DiffOverlay) deletedLine(row editorRow) (diff.SideLine, bool) {
	if o == nil {
		return diff.SideLine{}, false
	}
	block := o.Deleted[row.bufLine]
	if row.phantom < 0 || row.phantom >= len(block) {
		return diff.SideLine{}, false
	}
	return block[row.phantom], true
}

func (e *EditorPaneWidget) SetDiffOverlay(o *DiffOverlay) {
	e.DiffOverlay = o
	e.phantoms = nil
	if o == nil {
		return
	}
	e.phantoms = make(map[int]int, len(o.Deleted)+len(o.Fillers))
	for anchor, block := range o.Deleted {
		e.phantoms[anchor] += len(block)
	}
	for anchor, n := range o.Fillers {
		e.phantoms[anchor] += n
	}
	for anchor, n := range e.phantoms {
		if n == 0 {
			delete(e.phantoms, anchor)
		}
	}
}

func (e *EditorPaneWidget) diffLineBg(line int) term.Style {
	switch e.DiffOverlay.kind(line) {
	case diff.Added:
		return term.StyleDiffAdded
	case diff.Deleted:
		return term.StyleDiffDeleted
	}
	return 0
}

func (e *EditorPaneWidget) renderPhantomRow(surface Surface, y, gutterW, editorW int, row editorRow) {
	old, ok := e.DiffOverlay.deletedLine(row)
	if !ok {
		for x := 0; x < gutterW+editorW; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StyleLineNumber})
		}
		return
	}
	if gutterW > 0 {
		padded := []rune(e.gutterNumber("", gutterW))
		if e.LineNumbers && old.Num > 0 {
			padded = []rune(e.gutterNumber(strconv.Itoa(old.Num), gutterW))
		}
		for i := 0; i < gutterW; i++ {
			ch := ' '
			if i < len(padded) {
				ch = padded[i]
			}
			surface.SetCell(i, y, term.Cell{Ch: ch, Style: term.StyleLineNumber, BgStyle: term.StyleDiffDeleted})
		}
		surface.SetCell(0, y, term.Cell{Ch: '−', Style: term.StyleGutterDeleted, BgStyle: term.StyleDiffDeleted})
	}
	var spans []highlight.Span
	if e.Highlighter != nil && old.Text != "" {
		spans = e.Highlighter.HighlightLine(old.Text)
	}
	leftCol := e.Viewport.LeftCol
	if e.WordWrap {
		leftCol = 0
	}
	cells := e.renderLineToScreen([]rune(old.Text), spans, false, nil, e.resolveTabSize(), leftCol, editorW)
	for x := 0; x < editorW; x++ {
		surface.SetCell(gutterW+x, y, term.Cell{Ch: cells[x].ch, Style: cells[x].style, BgStyle: term.StyleDiffDeleted})
	}
}
