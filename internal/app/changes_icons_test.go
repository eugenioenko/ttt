package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/fileicons"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
)

func changesIconFixture(view string) *ChangesPanel {
	cp := NewChangesPanel("/repo")
	cp.groups = []changesGroup{{
		Dir:      "/repo",
		Name:     "repo",
		Staged:   []git.FileStatus{{Status: "A", Path: "web/app.ts"}},
		Unstaged: []git.FileStatus{{Status: "M", Path: "cmd/main.go"}},
	}}
	cp.SetFileView(view)
	return cp
}

func TestChangesIconsKeepStatusLetterAndAddFileIcon(t *testing.T) {
	for _, view := range []string{config.GitFileViewList, config.GitFileViewTree} {
		cp := changesIconFixture(view)
		cp.SetIcons(config.IconsNerdFont)

		mainGo := nodeWithID(cp.Tree.Config.Items, workingNodeID(workNodeFile, "/repo", "cmd/main.go", false))
		if mainGo == nil {
			t.Fatalf("%s view: cmd/main.go missing", view)
		}
		want := fileicons.ForFile("main.go")
		if mainGo.Icon != "M" || mainGo.IconStyle != ui.StatusStyle("M") {
			t.Errorf("%s view: status letter replaced: icon %q style %v", view, mainGo.Icon, mainGo.IconStyle)
		}
		if mainGo.LabelIcon != want.Glyph || mainGo.LabelIconStyle != fileIconStyle(want.Color) {
			t.Errorf("%s view: file icon = %q style %v, want %q", view, mainGo.LabelIcon, mainGo.LabelIconStyle, want.Glyph)
		}
	}

	cp := changesIconFixture(config.GitFileViewTree)
	cp.SetIcons(config.IconsNerdFont)
	folder := nodeWithID(cp.Tree.Config.Items, workingNodeID(workNodeFolder, "/repo", "cmd", false))
	if folder == nil || folder.Icon != "" || folder.LabelIcon != "" {
		t.Fatalf("tree view folder should carry no icon: %+v", folder)
	}
}

func TestChangesIconsNoneLeavesRowsUndecorated(t *testing.T) {
	cp := changesIconFixture(config.GitFileViewTree)
	cp.SetIcons(config.IconsNerdFont)
	cp.SetIcons(config.IconsNone)
	for _, node := range []string{
		workingNodeID(workNodeFile, "/repo", "cmd/main.go", false),
		workingNodeID(workNodeFolder, "/repo", "cmd", false),
	} {
		got := nodeWithID(cp.Tree.Config.Items, node)
		if got == nil || got.LabelIcon != "" {
			t.Errorf("icons none left %q decorated: %+v", node, got)
		}
	}
}

func TestChangesIconSwitchPreservesSelection(t *testing.T) {
	cp := changesIconFixture(config.GitFileViewTree)
	fileID := workingNodeID(workNodeFile, "/repo", "cmd/main.go", false)
	if !revealTreeSelection(cp.Tree, fileID) {
		t.Fatal("tree did not contain cmd/main.go")
	}
	cp.SetIcons(config.IconsNerdFont)
	if got := cp.Tree.Selected(); got == nil || got.ID != fileID {
		t.Fatalf("icon switch lost the selection: %+v", got)
	}
}

func TestCommitFileNodesCarryFileIcons(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.SetIcons(config.IconsNerdFont)
	nodes := cp.commitFileNodes("/repo", "abc", "abc", "commit:abc", []git.FileStatus{{Status: "D", Path: "docs/guide.md"}})
	if len(nodes) != 1 || nodes[0].Icon != "D" || nodes[0].LabelIcon != fileicons.ForFile("guide.md").Glyph {
		t.Fatalf("commit file row = %+v, want status D with the markdown icon", nodes)
	}
}
