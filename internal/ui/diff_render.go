package ui

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

const diffTabWidth = 4

// DiffMode is the reader-selected projection for every diff surface. Keeping
// this as a value rather than a boolean lets menus and other controls inspect
// and set a specific mode without reverse-engineering toggle state.
type DiffMode uint8

const (
	DiffModeSplit DiffMode = iota
	DiffModeUnified
)

func (m DiffMode) Toggle() DiffMode {
	if m == DiffModeUnified {
		return DiffModeSplit
	}
	return DiffModeUnified
}

// DiffWrapMode is the explicit line-wrapping state shared by diff surfaces.
type DiffWrapMode uint8

const (
	DiffWrapOff DiffWrapMode = iota
	DiffWrapOn
)

func (m DiffWrapMode) Toggle() DiffWrapMode {
	if m == DiffWrapOn {
		return DiffWrapOff
	}
	return DiffWrapOn
}

// DiffModeSurface is implemented by every diff-reading surface so commands,
// menus, and the tab control all inspect and update the same presentation
// state. SetMode and SetWrapMode are reader overrides; ApplyDefaultMode and
// ApplyDefaultWrapMode only update properties that still inherit Options.
type DiffModeSurface interface {
	Mode() DiffMode
	SetMode(DiffMode)
	ApplyDefaultMode(DiffMode)
	WrapMode() DiffWrapMode
	SetWrapMode(DiffWrapMode)
	ApplyDefaultWrapMode(DiffWrapMode)
}

// diffUnifiedLine points back to its source split row so search and selection
// can keep using their original indexes after projection.
type diffUnifiedLine struct {
	sourceLine int
	right      bool
	side       diff.SideLine
}

// buildUnifiedDiffLines is shared by file diffs and each file section in a
// commit detail. A contiguous change block is projected as all removals then
// all additions; unchanged and collapsed rows appear once.
func buildUnifiedDiffLines(lines []diff.DiffLine) []diffUnifiedLine {
	var unified []diffUnifiedLine
	for i := 0; i < len(lines); {
		if lines[i].Left.Kind == diff.Deleted || lines[i].Right.Kind == diff.Added {
			end := i
			for end < len(lines) && (lines[end].Left.Kind == diff.Deleted || lines[end].Right.Kind == diff.Added) {
				end++
			}
			for source := i; source < end; source++ {
				if lines[source].Left.Kind == diff.Deleted {
					unified = append(unified, diffUnifiedLine{sourceLine: source, side: lines[source].Left})
				}
			}
			for source := i; source < end; source++ {
				if lines[source].Right.Kind == diff.Added {
					unified = append(unified, diffUnifiedLine{sourceLine: source, right: true, side: lines[source].Right})
				}
			}
			i = end
			continue
		}

		side := lines[i].Left
		if side.Text == "" && lines[i].Right.Text != "" {
			side = lines[i].Right
		}
		unified = append(unified, diffUnifiedLine{sourceLine: i, side: side})
		i++
	}
	return unified
}

type diffCellDecorator func(runeIndex int, cell term.Cell) term.Cell

func diffKindStyle(kind diff.LineKind) term.Style {
	switch kind {
	case diff.Added:
		return term.StyleDiffAdded
	case diff.Deleted:
		return term.StyleDiffDeleted
	case diff.Collapsed:
		return term.StyleDiffCollapsed
	default:
		return term.StyleDefault
	}
}

func diffLineVisualWidth(text string) int {
	width := 0
	for _, ch := range text {
		if ch == '\t' {
			width = ((width / diffTabWidth) + 1) * diffTabWidth
		} else {
			width += textwidth.Rune(ch)
		}
	}
	return width
}

func diffWrapStarts(text string, width int) []int {
	return wrapLineSegments([]rune(text), width, diffTabWidth)
}

func renderDiffGutter(surface Surface, x, y, width int, line diff.SideLine, baseStyle term.Style) {
	number := ""
	if line.Num > 0 {
		number = fmt.Sprintf("%d", line.Num)
	}
	text := " " + fmt.Sprintf("%*s", width-3, number) + "  "
	style := term.StyleLineNumber
	if baseStyle == term.StyleDiffCollapsed {
		style = baseStyle
	}
	for column, ch := range []rune(text) {
		if column >= width {
			break
		}
		surface.SetCell(x+column, y, term.Cell{Ch: ch, Style: style})
	}
}

func renderDiffText(surface Surface, x, y, width int, text string, baseStyle term.Style, spans []highlight.Span, segmentStart, leftVisualCol int, decorate diffCellDecorator) {
	fullBaseStyle := baseStyle == term.StyleDiffCollapsed
	blank := term.Cell{Ch: ' '}
	if fullBaseStyle {
		blank.Style = baseStyle
	} else if baseStyle != term.StyleDefault {
		blank.BgStyle = baseStyle
	}

	drawTextSegment(surface, x, y, width, text, segmentStart, leftVisualCol, blank, func(runeIndex int, ch rune) term.Cell {
		style := term.StyleDefault
		if fullBaseStyle {
			style = baseStyle
		}
		for _, span := range spans {
			if runeIndex >= span.Start && runeIndex < span.End {
				style = span.Style
				break
			}
		}
		cell := term.Cell{Ch: ch, Style: style}
		if !fullBaseStyle && baseStyle != term.StyleDefault {
			cell.BgStyle = baseStyle
		}
		if decorate != nil {
			cell = decorate(runeIndex, cell)
		}
		return cell
	})
}

// drawTextSegment draws one horizontally clipped or wrapped segment. Rune
// indexes remain indexes into the original text so syntax, search, and
// selection spans do not need their own wrapping logic.
func drawTextSegment(surface Surface, x, y, width int, text string, segmentStart, leftVisualCol int, blank term.Cell, cellAt func(runeIndex int, ch rune) term.Cell) {
	for column := 0; column < width; column++ {
		surface.SetCell(x+column, y, blank)
	}
	if segmentStart < 0 {
		return
	}

	runes := []rune(text)
	visualColumn := 0
	for runeIndex := segmentStart; runeIndex < len(runes); runeIndex++ {
		ch := runes[runeIndex]
		cell := cellAt(runeIndex, ch)
		if ch == '\t' {
			nextStop := ((visualColumn / diffTabWidth) + 1) * diffTabWidth
			for tabColumn := visualColumn; tabColumn < nextStop; tabColumn++ {
				drawColumn := tabColumn - leftVisualCol
				if drawColumn >= 0 && drawColumn < width {
					cell.Ch = ' '
					surface.SetCell(x+drawColumn, y, cell)
				}
			}
			visualColumn = nextStop
		} else {
			runeWidth := textwidth.Rune(ch)
			drawColumn := visualColumn - leftVisualCol
			if drawColumn >= 0 && drawColumn < width {
				if runeWidth > 1 && drawColumn == width-1 {
					cell.Ch = ' '
				}
				surface.SetCell(x+drawColumn, y, cell)
			}
			visualColumn += runeWidth
		}
		if visualColumn-leftVisualCol >= width {
			break
		}
	}
}
