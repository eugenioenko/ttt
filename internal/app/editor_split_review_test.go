package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func splitReviewApp(t *testing.T) (*App, string) {
	t.Helper()
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	a.wireEditorGroupActivation(a.EditorGroup)
	a.Root.SetFocus(a.EditorGroup)
	t.Cleanup(a.CloseTasks)
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("working document\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return a, path
}

func TestCodeActionSnapshotUsesSelectedBufferAcrossGroups(t *testing.T) {
	a, path := splitReviewApp(t)
	if err := a.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	working := a.EditorGroup
	workingBuffer := working.ActiveBuffer()
	a.SplitEditor()
	a.EditorGroup.OpenFileReadOnly(path, "Snapshot")
	if a.EditorGroup.ActiveBuffer() == workingBuffer {
		t.Fatal("read-only peer must have a separate snapshot buffer")
	}
	a.Root.SetFocus(working)
	if got := a.snapshotCodeActionBuffers()[path].Buffer; got != workingBuffer {
		t.Fatal("snapshot did not retain the independently captured working buffer")
	}
}

func TestOpenWorkingFileDoesNotShareReadOnlyPeer(t *testing.T) {
	for _, memory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "memory"}[memory], func(t *testing.T) {
			a, path := splitReviewApp(t)
			if memory {
				a.EditorGroup.OpenBufferReadOnly("Snapshot", path, []string{"historical"})
			} else {
				a.EditorGroup.OpenFileReadOnly(path, "Snapshot")
			}
			readOnly := a.EditorGroup.ActiveBuffer()
			a.SplitEditor()
			if err := a.EditorGroup.OpenFile(path); err != nil {
				t.Fatal(err)
			}
			if a.EditorGroup.ActiveBuffer() == readOnly || a.EditorGroup.Editor.ReadOnly || a.EditorGroup.ActiveBuffer().Lines[0] != "working document" {
				t.Fatal("working-file open reused the read-only snapshot")
			}
		})
	}
}

func TestInactiveGroupRemovalPreservesActiveGroupAndFocus(t *testing.T) {
	for _, sidebar := range []bool{false, true} {
		t.Run(map[bool]string{false: "editor", true: "sidebar"}[sidebar], func(t *testing.T) {
			a, _ := splitReviewApp(t)
			a.SplitEditor()
			closing := a.EditorGroup
			closing.OpenPluginTab("temporary", "Temporary", widgets.NewInputWidget(widgets.InputConfig{}))
			closing.CloseOtherTabs()
			a.SplitEditorDown()
			active := a.EditorGroup
			if sidebar {
				a.FocusSidebar()
			}
			focused := a.Root.Focused
			closing.ClosePluginTab("temporary")
			if len(a.editorGroups()) != 2 || a.EditorGroup != active || a.Root.Focused != focused {
				t.Fatal("removing an inactive group moved the active group or keyboard focus")
			}
		})
	}
}

func TestMergedSameNameDiffsReceiveTheirOwnResults(t *testing.T) {
	a, owner, other := splitResultApp()
	owner.OpenDiff("file.txt", diff.FileDiff{}, nil, nil, false)
	other.OpenDiff("file.txt", diff.FileDiff{}, nil, nil, false)
	target := other.ActiveDiffWidget()
	target.Loading = true
	owner.MergeTabsFrom(other)
	a.applyDiffContent(&DiffContentResult{Widget: target, TabName: "file.txt (diff)", OldLines: []string{"old"}, NewLines: []string{"new"}})
	if target.Loading {
		t.Fatal("same-name tab shadowed the retained widget's result")
	}
}

func TestDiffSearchIncludesInactiveGroupsAndDeduplicatesTabs(t *testing.T) {
	a, _ := splitReviewApp(t)
	a.EditorGroup.OpenDiff("first.txt", diff.FileDiff{}, []string{"old"}, []string{"new"}, false)
	owner := a.EditorGroup
	a.SplitEditor()
	a.EditorGroup.OpenDiff("first.txt", diff.FileDiff{}, []string{"old"}, []string{"new"}, false)
	a.EditorGroup.OpenDiff("second.txt", diff.FileDiff{}, []string{"old"}, []string{"new"}, false)
	a.SplitEditorDown()
	if got := a.DiffSearchSources(); len(got) != 2 {
		t.Fatalf("diff search sources = %d, want 2", len(got))
	}
	a.NavigateToSearchMatch("first.txt (diff)", 1, 1)
	if a.EditorGroup != owner || a.EditorGroup.ActiveDiffWidget() == nil {
		t.Fatal("search navigation did not focus the owning diff group")
	}
}

func TestHandledFileOwnerSurvivesClosingOneView(t *testing.T) {
	a, path := splitReviewApp(t)
	left := a.EditorGroup
	left.OnContentTabClose = a.closeFileHandlerTab
	left.OpenPluginTab(path, "Handled", widgets.NewInputWidget(widgets.InputConfig{}))
	a.SplitEditor()
	right := a.EditorGroup
	right.OpenPluginTab(path, "Handled", widgets.NewInputWidget(widgets.InputConfig{}))
	a.fileHandlerTabs = map[string]*plugin.Plugin{path: {Name: "handler"}}
	left.ClosePluginTab(path)
	if a.fileHandlerTabs[path] == nil {
		t.Fatal("closing one view forgot the surviving handler owner")
	}
	a.SyncFileHandlerTabs()
	if _, ok := right.TabContent(right.ActiveTabIndex()).(*ui.BinaryFileWidget); !ok {
		t.Fatal("unloaded handler left its surviving view mounted")
	}
	if a.fileHandlerTabs[path] != nil {
		t.Fatal("unloaded handler ownership remained")
	}
}
