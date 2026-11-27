package app

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type stagingFolderRepository struct {
	*testSCMStaging
	changes []scm.Change
}

func (r *stagingFolderRepository) Changes(context.Context) ([]scm.Change, error) {
	return r.changes, nil
}

func TestSCMFolderActionsUseProviderForCollapsedSubtree(t *testing.T) {
	for _, group := range []scm.ChangeGroup{scm.WorkingTree, scm.Staged} {
		t.Run(string(group), func(t *testing.T) {
			root := t.TempDir()
			newFileKind := scm.Untracked
			if group == scm.Staged {
				newFileKind = scm.Added
			}
			repo := &stagingFolderRepository{testSCMStaging: &testSCMStaging{testSCMRepository: testSCMRepository{root: root}}, changes: []scm.Change{
				{Path: "src/a.go", Kind: scm.Modified, Group: group},
				{Path: "src/deep/b.go", Kind: scm.Modified, Group: group},
				{Path: "src/untracked.go", Kind: newFileKind, Group: group},
				{Path: "docs/outside.md", Kind: scm.Modified, Group: group},
			}}
			cp := NewChangesPanelWithProvider(&testSCMProvider{repo: repo}, root)
			cp.Tree.Config.OnSelect = nil
			cp.OnStatusChanged = func() {}
			cp.OnError = func(message string) { t.Errorf("folder operation failed: %s", message) }
			cp.groups = readChangesGroups(cp.provider, []string{root})
			cp.SetFileView(config.GitFileViewTree)
			folder := nodeWithLabel(cp.Tree.Config.Items, "src")
			if folder == nil || !revealTreeSelection(cp.Tree, folder.ID) {
				t.Fatal("missing folder")
			}
			folder.Expanded = false
			cp.Tree.SetItems(cp.Tree.Config.Items)
			inline, palette := "stageFolder", "changes.stageFolder"
			calls := &repo.staged
			if group == scm.Staged {
				inline, palette = "unstageFolder", "changes.unstageFolder"
				calls = &repo.unstaged
			}
			cp.handleCommand(inline, folder)
			cp.ToggleStageSelected()
			a := &App{Changes: cp, Reg: command.NewRegistry()}
			registerGitCommands(a)
			a.Reg.Execute(palette)
			if len(*calls) != 3 {
				t.Fatalf("provider calls: %v", *calls)
			}
			want := []string{"src/a.go", "src/deep/b.go", "src/untracked.go"}
			for _, paths := range *calls {
				sort.Strings(paths)
				if !reflect.DeepEqual(paths, want) {
					t.Fatalf("folder scope: %v", paths)
				}
			}
		})
	}
}

func TestSCMFolderControlsHonorOptionalStaging(t *testing.T) {
	root := t.TempDir()
	repo := &modelRepository{testSCMRepository: testSCMRepository{root: root}, changes: []scm.Change{
		{Path: "src/live.go", Kind: scm.Modified, Group: scm.WorkingTree},
		{Path: "docs/staged.md", Kind: scm.Modified, Group: scm.Staged},
	}}
	cp := NewChangesPanelWithProvider(&testSCMProvider{repo: repo}, root)
	cp.Tree.Config.OnSelect = nil
	cp.groups = readChangesGroups(cp.provider, []string{root})
	cp.SetFileView(config.GitFileViewTree)
	var visit func([]*widgets.TreeNode)
	visit = func(nodes []*widgets.TreeNode) {
		for _, node := range nodes {
			for _, action := range node.Actions {
				if action.Command == "stageFolder" || action.Command == "unstageFolder" {
					t.Errorf("unsupported folder action: %s", action.Command)
				}
			}
			visit(node.Children)
		}
	}
	visit(cp.Tree.Config.Items)
	settings := config.DefaultSettings()
	a := &App{Changes: cp, Root: ui.NewRoot(nil), Reg: command.NewRegistry(), Settings: &settings}
	registerGitCommands(a)
	cp.OnStatusChanged = func() {}
	var failures []string
	cp.OnError = func(message string) { failures = append(failures, message) }
	for _, staged := range []bool{false, true} {
		a.ShowChangesFolderContextMenu(root, "src", staged, 0, 0)
		menu, ok := a.Root.TopOverlayWidget().(*ui.ContextMenuWidget)
		if !ok {
			t.Fatal("folder menu missing")
		}
		for _, item := range menu.Items {
			if item.Command == "changes.stageFolder" || item.Command == "changes.unstageFolder" {
				t.Errorf("unsupported folder menu item: %s", item.Command)
			}
		}
		if len(menu.Items) == 0 || menu.Items[0].IsSep {
			t.Fatal("folder menu starts with a separator")
		}
		a.Root.PopOverlay()
		path, action := "src", "changes.stageFolder"
		if staged {
			path, action = "docs", "changes.unstageFolder"
		}
		node := nodeWithLabel(cp.Tree.Config.Items, path)
		if node == nil || !revealTreeSelection(cp.Tree, node.ID) {
			t.Fatal("folder selection failed")
		}
		a.Reg.Execute(action)
	}
	if want := []string{errors.ErrUnsupported.Error(), errors.ErrUnsupported.Error()}; !reflect.DeepEqual(failures, want) {
		t.Fatalf("unsupported folder operations: %v", failures)
	}
}
