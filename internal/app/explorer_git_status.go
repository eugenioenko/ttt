package app

import (
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
)

func explorerGitStyles(groups []changesGroup) map[string]term.Style {
	styles := make(map[string]term.Style)
	for _, group := range groups {
		for _, file := range group.Changes {
			addExplorerGitStyle(styles, group.Dir, file)
		}
	}
	return styles
}

// addExplorerGitStyle colors the file and every ancestor up to the repository
// root, so a collapsed folder still shows that something under it changed.
func addExplorerGitStyle(styles map[string]term.Style, root string, file scm.Change) {
	style := ui.ChangeStyle(file.Kind, file.Group == scm.Staged)
	if style == term.StyleDefault {
		return
	}
	rank := ui.GitDecorationRank(style)
	path := filepath.Join(root, file.Path)
	for {
		if rank > ui.GitDecorationRank(styles[path]) {
			styles[path] = style
		}
		parent := filepath.Dir(path)
		if path == root || parent == path {
			return
		}
		path = parent
	}
}
