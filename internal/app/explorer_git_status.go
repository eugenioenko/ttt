package app

import (
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
)

// explorerGitStyles maps every changed file, and each of its ancestor
// directories up to the repository root, to the color the file explorer
// sidebar should use. A directory takes the highest-priority status found
// among its descendants, mirroring how VSCode and Zed tint folders that
// contain changes.
func explorerGitStyles(groups []changesGroup) map[string]term.Style {
	styles := make(map[string]term.Style)
	merge := func(path string, style term.Style) {
		if existing, ok := styles[path]; !ok || gitStylePriority(style) > gitStylePriority(existing) {
			styles[path] = style
		}
	}
	apply := func(dir string, file git.FileStatus) {
		style := ui.StatusStyle(file.Status)
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

// gitStylePriority ranks statuses so a folder containing multiple kinds of
// changes shows the most attention-worthy one.
func gitStylePriority(style term.Style) int {
	switch style {
	case term.StyleGitConflict:
		return 4
	case term.StyleWarning:
		return 3
	case term.StyleDanger:
		return 2
	case term.StyleSuccess:
		return 1
	default:
		return 0
	}
}
