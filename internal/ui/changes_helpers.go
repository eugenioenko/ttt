package ui

import (
	"slices"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/term"
)

type ChangesGroup struct {
	Dir       string
	Name      string
	Staged    []git.FileStatus
	Unstaged  []git.FileStatus
	IsPR      bool
	PRURL     string
	PRBody    string
	PRDiffs   map[string]string
	PROwner   string
	PRRepo    string
	PRBaseSHA string
	PRHeadSHA string
}

type changeDecoration struct {
	kinds  []scm.ChangeKind
	live   term.Style
	staged term.Style
}

// changeDecorations is ordered least to most attention-worthy. The position also
// sets the rank, so the color a folder inherits from its children stays
// consistent with the colors themselves without a second table to keep in sync.
var changeDecorations = []changeDecoration{
	{kinds: []scm.ChangeKind{scm.Added, scm.Untracked, scm.Renamed, scm.Copied}, live: term.StyleSuccess, staged: term.StyleSuccessStaged},
	{kinds: []scm.ChangeKind{scm.Deleted}, live: term.StyleDanger, staged: term.StyleDangerStaged},
	{kinds: []scm.ChangeKind{scm.Modified}, live: term.StyleWarning, staged: term.StyleWarningStaged},
	{kinds: []scm.ChangeKind{scm.Conflicted}, live: term.StyleGitConflict, staged: term.StyleGitConflictStaged},
}

func StatusStyle(status string) term.Style {
	return GitDecorationStyle(status, false)
}

func GitDecorationStyle(status string, staged bool) term.Style {
	return ChangeStyle(git.ToChange(git.FileStatus{Status: status}).Kind, staged)
}

func ChangeStyle(kind scm.ChangeKind, staged bool) term.Style {
	for _, decoration := range changeDecorations {
		if !slices.Contains(decoration.kinds, kind) {
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
	for _, decoration := range changeDecorations {
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
	for i, decoration := range changeDecorations {
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
	return ChangeBadge(git.ToChange(git.FileStatus{Status: status}))
}

func ChangeBadge(change scm.Change) string {
	switch change.Kind {
	case scm.Modified:
		return "M"
	case scm.Added:
		return "A"
	case scm.Deleted:
		return "D"
	case scm.Renamed:
		return "R"
	case scm.Copied:
		return "C"
	case scm.Untracked:
		return "U"
	case scm.TypeChanged:
		return "T"
	case scm.Conflicted:
		if change.Detail != "" {
			return change.Detail
		}
		return "U"
	default:
		return change.Detail
	}
}
