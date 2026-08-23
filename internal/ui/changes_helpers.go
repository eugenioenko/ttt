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
