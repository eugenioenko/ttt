package ui

import (
	"slices"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/term"
)

type ChangesGroup struct {
	Dir       string
	Name      string
	Staged    []git.FileStatus
	Unstaged  []git.FileStatus
	IsPR      bool
	PRURL     string
	PRDiffs   map[string]string
	PROwner   string
	PRRepo    string
	PRBaseSHA string
	PRHeadSHA string
}

type gitDecoration struct {
	statuses []string
	live     term.Style
	staged   term.Style
}

// gitDecorations is ordered least to most attention-worthy. The position also
// sets the rank, so the color a folder inherits from its children stays
// consistent with the colors themselves without a second table to keep in sync.
var gitDecorations = []gitDecoration{
	{statuses: []string{"A", "?", "R", "C"}, live: term.StyleSuccess, staged: term.StyleSuccessStaged},
	{statuses: []string{"D"}, live: term.StyleDanger, staged: term.StyleDangerStaged},
	{statuses: []string{"M"}, live: term.StyleWarning, staged: term.StyleWarningStaged},
	{statuses: []string{"U"}, live: term.StyleGitConflict, staged: term.StyleGitConflictStaged},
}

func StatusStyle(status string) term.Style {
	return GitDecorationStyle(status, false)
}

func GitDecorationStyle(status string, staged bool) term.Style {
	for _, decoration := range gitDecorations {
		if !slices.Contains(decoration.statuses, status) {
			continue
		}
		if staged {
			return decoration.staged
		}
		return decoration.live
	}
	return term.StyleDefault
}

// GitDecorationLive undoes the staged dimming, for callers that show staged and
// pending changes in the same color. Category ordering does not depend on
// staged-ness, so collapsing after a folder aggregation keeps its result.
func GitDecorationLive(style term.Style) term.Style {
	for _, decoration := range gitDecorations {
		if style == decoration.staged {
			return decoration.live
		}
	}
	return style
}

// GitDecorationRank orders the statuses a folder can inherit from its children.
// A live status outranks its dimmed staged variant, so a folder holding both a
// staged and a pending change shows the pending one.
func GitDecorationRank(style term.Style) int {
	for i, decoration := range gitDecorations {
		switch style {
		case decoration.live:
			return 2*i + 2
		case decoration.staged:
			return 2*i + 1
		}
	}
	return 0
}

func StatusBadge(status string) string {
	switch status {
	case "M":
		return "M"
	case "A":
		return "A"
	case "D":
		return "D"
	case "R":
		return "R"
	case "C":
		return "C"
	case "?":
		return "U"
	default:
		return status
	}
}
