package ui

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
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

// DiffContextMode controls how much unchanged file content a diff surface
// shows. ChangesOnly keeps Git's compact hunks; FullFile includes every line.
type DiffContextMode uint8

const (
	DiffContextChangesOnly DiffContextMode = iota
	DiffContextFullFile
)

// DiffContextSurface is separate from DiffModeSurface because some diff
// readers can project split/unified and wrapping without owning full blobs.
type DiffContextSurface interface {
	ContextMode() DiffContextMode
	SetContextMode(DiffContextMode)
	ApplyDefaultContextMode(DiffContextMode)
}

// DiffModeSurface is implemented by every diff-reading surface so commands,
// menus, settings, and commands update the same presentation state.
// SetMode and SetWrapMode are reader overrides; ApplyDefaultMode and
// ApplyDefaultWrapMode only update properties that still inherit Options.
type DiffModeSurface interface {
	Mode() DiffMode
	SetMode(DiffMode)
	ApplyDefaultMode(DiffMode)
	WrapMode() DiffWrapMode
	SetWrapMode(DiffWrapMode)
	ApplyDefaultWrapMode(DiffWrapMode)
	SetDiffHighContrast(bool)
	SetDiffCollapsedEmphasis(bool)
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

func diffKindStyle(kind diff.LineKind) term.Style {
	switch kind {
	case diff.Added:
		return term.StyleDiffAdded
	case diff.Deleted:
		return term.StyleDiffDeleted
	case diff.Collapsed:
		return term.StyleDefault
	default:
		return term.StyleDefault
	}
}

func diffKindForeground(kind diff.LineKind, highContrast bool) term.Style {
	if kind == diff.Collapsed {
		return term.StyleMuted
	}
	if !highContrast {
		return term.StyleDefault
	}
	switch kind {
	case diff.Added:
		return term.StyleGutterAdded
	case diff.Deleted:
		return term.StyleGutterDeleted
	default:
		return term.StyleDefault
	}
}

func collapsedDiffRowStyle(kind diff.LineKind, emphasized, hovered bool) term.Style {
	if kind != diff.Collapsed {
		return diffKindStyle(kind)
	}
	if hovered {
		return term.StyleDiffCollapsedHover
	}
	if emphasized {
		return term.StyleDiffCollapsedEmphasis
	}
	return term.StyleDefault
}

func collapsedDiffGutterStyle(kind diff.LineKind, emphasized, hovered bool) term.Style {
	if kind != diff.Collapsed || !emphasized {
		return term.StyleDefault
	}
	return collapsedDiffRowStyle(kind, true, hovered)
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
