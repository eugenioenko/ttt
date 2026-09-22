package app

import (
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
)

// explorerGitStyles also colors every ancestor directory up to the repo root
// with the highest-priority status among its descendants, mirroring how
// VSCode and Zed tint folders that contain changes.
func explorerGitStyles(groups []changesGroup) map[string]term.Style {
	styles := make(map[string]term.Style)
	merge := func(path string, style term.Style) {
		if existing, ok := styles[path]; !ok || gitStylePriority(style) > gitStylePriority(existing) {
			styles[path] = style
		}
	}
	apply := func(dir string, file git.FileStatus) {
		style := ui.GitDecorationStyle(file.Status, file.Staged)
		if style == term.StyleDefault {
			return
		}
		abs := filepath.Join(dir, file.Path)
		merge(abs, style)
		parent := filepath.Dir(abs)
		for {
			merge(parent, style)
			next := filepath.Dir(parent)
			if next == parent {
				break
			}
			parent = next
		}
	}
	for _, group := range groups {
		for _, file := range group.Staged {
			apply(group.Dir, file)
		}
		for _, file := range group.Unstaged {
			apply(group.Dir, file)
		}
	}
	return styles
}

// gitStylePriority ranks a live status above its dimmed staged variant, so a
// folder with both a staged and a pending change shows the pending one.
func gitStylePriority(style term.Style) int {
	switch style {
	case term.StyleGitConflict:
		return 8
	case term.StyleGitConflictStaged:
		return 7
	case term.StyleWarning:
		return 6
	case term.StyleWarningStaged:
		return 5
	case term.StyleDanger:
		return 4
	case term.StyleDangerStaged:
		return 3
	case term.StyleSuccess:
		return 2
	case term.StyleSuccessStaged:
		return 1
	default:
		return 0
	}
}
