package ui

import (
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

func StatusStyle(status string) term.Style {
	switch status {
	case "U":
		return term.StyleGitConflict
	case "M":
		return term.StyleWarning
	case "A", "?", "R", "C":
		return term.StyleSuccess
	case "D":
		return term.StyleDanger
	default:
		return term.StyleDefault
	}
}

// GitDecorationStyle is StatusStyle, dimmed to the staged variant when staged.
func GitDecorationStyle(status string, staged bool) term.Style {
	style := StatusStyle(status)
	if !staged {
		return style
	}
	switch style {
	case term.StyleSuccess:
		return term.StyleSuccessStaged
	case term.StyleDanger:
		return term.StyleDangerStaged
	case term.StyleWarning:
		return term.StyleWarningStaged
	case term.StyleGitConflict:
		return term.StyleGitConflictStaged
	default:
		return style
	}
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
