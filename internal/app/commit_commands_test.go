package app

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/gdamore/tcell/v3"
)

func commitCommandApp(t *testing.T, stageWorkingFile bool) (string, *ChangesPanel, *App, string) {
	t.Helper()
	dir := dirtyRepo(t)
	if stageWorkingFile {
		if err := git.Stage(dir, "a.txt"); err != nil {
			t.Fatal(err)
		}
	}
	cp := newRefreshedPanel(dir)
	commitID := expandFirstCommit(t, cp)
	commitNode := cp.commitNode(commitID)
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)
	if len(commitNode.Children) == 0 {
		t.Fatal("commit has no file children")
	}
	childID := commitNode.Children[0].ID
	if !cp.selectLogNode(childID) {
		t.Fatalf("could not select commit file %q", childID)
	}
	// Keep a valid but stale working-tree selection to prove commands do not
	// silently fall through to the other tree.
	for i, node := range cp.Tree.FlatList() {
		if _, _, _, ok := cp.parseFileNode(node); ok {
			cp.Tree.SetSelectedIndex(i)
			break
		}
	}

	a := buildTestApp(t, config.DefaultSettings())
	a.Changes = cp
	a.Reg = command.NewRegistry()
	RegisterCommands(a)
	a.Root.SetFocus(cp.Adapter)
	for i := 0; i < 10 && !cp.CommitLog.IsFocused(); i++ {
		cp.Adapter.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	}
	if !cp.CommitLog.IsFocused() {
		t.Fatal("could not focus commit log through the changes panel")
	}
	return dir, cp, a, childID
}

// The changes panel has two independent selections. Registered commands are
// also the surface used by ttt.exec_command and --exec, so they must follow the
// tree the reader last acted in rather than a stale selection in the other one.
func TestRegisteredOpenCommandsUseRememberedCommitFile(t *testing.T) {
	for _, tc := range []struct {
		command  string
		extended bool
	}{
		{command: "changes.openDiff"},
		{command: "changes.openExtendedDiff", extended: true},
		{command: "changes.openFile"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			_, cp, a, childID := commitCommandApp(t, false)
			if !a.Reg.Execute(tc.command) {
				t.Fatalf("command %q was not registered", tc.command)
			}

			file := cp.logFiles[childID]
			want := fmt.Sprintf("%s:%s (diff)", file.Ref, file.Status.Path)
			if got := a.EditorGroup.ActiveFilePath(); got != want {
				t.Fatalf("%s opened %q from the working tree, want commit diff %q", tc.command, got, want)
			}
			if dv := a.EditorGroup.ActiveDiffWidget(); dv == nil || dv.IsExtended() != tc.extended {
				t.Fatalf("%s extended mode = %v, want %v", tc.command, dv != nil && dv.IsExtended(), tc.extended)
			}
		})
	}
}

// The palette takes focus before it executes a command. That transient modal
// state must not erase which changes-panel tree the reader was acting in.
func TestPaletteOpenDiffUsesRememberedCommitFile(t *testing.T) {
	_, cp, a, childID := commitCommandApp(t, false)
	a.OpenCommandPalette(false, ">Git: Open Compact Diff")
	a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	file := cp.logFiles[childID]
	want := fmt.Sprintf("%s:%s (diff)", file.Ref, file.Status.Path)
	if got := a.EditorGroup.ActiveFilePath(); got != want {
		t.Fatalf("palette command opened %q from the working tree, want commit diff %q", got, want)
	}
}

func TestRegisteredWorkingTreeCommandsNoOpInCommitContext(t *testing.T) {
	t.Run("stage", func(t *testing.T) {
		dir, _, a, _ := commitCommandApp(t, false)
		if !a.Reg.Execute("changes.stage") {
			t.Fatal("changes.stage was not registered")
		}
		files, err := git.StatusFiles(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if file.Path == "a.txt" && file.Staged {
				t.Fatal("history-focused changes.stage staged stale working-tree selection a.txt")
			}
		}
	})

	t.Run("unstage", func(t *testing.T) {
		dir, _, a, _ := commitCommandApp(t, true)
		if !a.Reg.Execute("changes.unstage") {
			t.Fatal("changes.unstage was not registered")
		}
		files, err := git.StatusFiles(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if file.Path == "a.txt" && file.Staged {
				return
			}
		}
		t.Fatal("history-focused changes.unstage unstaged stale working-tree selection a.txt")
	})

	t.Run("discard", func(t *testing.T) {
		_, _, a, _ := commitCommandApp(t, false)
		if !a.Reg.Execute("changes.discard") {
			t.Fatal("changes.discard was not registered")
		}
		if a.Root.HasModalOverlay() {
			t.Fatal("history-focused changes.discard opened a dialog for the stale working-tree selection")
		}
	})
}
