package app

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/ui"
)

func splitResultApp() (*App, *ui.EditorGroupWidget, *ui.EditorGroupWidget) {
	owner := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	other := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	layout := ui.NewEditorLayoutWidget(owner, nil)
	layout.SplitGroup(owner, other, ui.SplitRight)
	return &App{EditorGroup: other, EditorSplit: layout}, owner, other
}

func TestSplitDiffContentFollowsOwningTab(t *testing.T) {
	for _, action := range []string{"switch", "move", "reopen"} {
		t.Run(action, func(t *testing.T) {
			a, owner, other := splitResultApp()
			owner.OpenDiff("file.txt", diff.FileDiff{}, nil, nil, false)
			target := owner.ActiveDiffWidget()
			target.Loading = true
			if action == "move" {
				owner.MoveActiveTabTo(other)
				a.EditorGroup = owner
			} else {
				other.OpenDiff("file.txt", diff.FileDiff{}, nil, nil, false)
				other.ActiveDiffWidget().Loading = true
			}
			if action == "reopen" {
				owner.CloseTab()
				owner.OpenDiff("file.txt", diff.FileDiff{}, nil, nil, false)
				owner.ActiveDiffWidget().Loading = true
			}
			a.applyDiffContent(&DiffContentResult{Widget: target, TabName: "file.txt (diff)", OldLines: []string{"old"}, NewLines: []string{"new"}})
			if action == "reopen" {
				if !owner.ActiveDiffWidget().Loading {
					t.Fatal("closed-tab result completed a replacement tab")
				}
			} else if target.Loading {
				t.Fatal("owning tab remained loading after group navigation")
			}
			if action != "move" && !other.ActiveDiffWidget().Loading {
				t.Fatal("result completed a different group's same-name tab")
			}
		})
	}
}

func TestSplitCommitResultsFollowOwningIncarnation(t *testing.T) {
	for _, moved := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch", true: "move"}[moved], func(t *testing.T) {
			a, owner, other := splitResultApp()
			ref := strings.Repeat("a", 40)
			tabID := commitDetailTabID("/repo", ref)
			detail := ui.NewCommitDetailWidget("/repo", ref, "aaaaaaa", false)
			detail.Incarnation = 1
			owner.OpenPluginTab(tabID, "Commit", detail)
			if moved {
				owner.MoveActiveTabTo(other)
				a.EditorGroup = owner
			}
			distractor := ui.NewCommitDetailWidget("/repo", ref, "aaaaaaa", false)
			distractor.Incarnation = 2
			a.EditorGroup.OpenPluginTab(tabID, "Commit", distractor)
			a.ApplyCommitDetail(&CommitDetailResult{Incarnation: 1, Dir: "/repo", Ref: ref, Message: "loaded", Files: []ui.CommitDetailFile{{Path: "file.txt"}}})
			if detail.Message != "loaded" || distractor.Message != "" {
				t.Fatal("commit result did not reach only its owning incarnation")
			}
			a.ApplyCommitDetailContext(&CommitDetailContextResult{
				Incarnation: 1, TabID: tabID, Dir: "/repo", Ref: ref, FileIndex: 0,
				FileKey: ui.CommitDetailContextKey(detail.Files[0]), OldLines: []string{"old"}, NewLines: []string{"new"},
			})
			if detail.Files[0].FullFileState != ui.CommitDetailFullFileLoaded {
				t.Fatal("context result did not reach its owning incarnation")
			}
		})
	}
}

func TestOpenCommitDetailReusesInactiveGroupRequest(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	owner := a.EditorGroup
	ref := strings.Repeat("a", 40)
	detail := ui.NewCommitDetailWidget("/repo", ref, "aaaaaaa", false)
	owner.OpenPluginTab(commitDetailTabID("/repo", ref), "Commit", detail)
	a.SplitEditor()
	a.OpenCommitDetail("/repo", ref, "aaaaaaa")
	if a.EditorGroup != owner || a.EditorGroup.CommitDetailWidgetByTab(commitDetailTabID("/repo", ref)) != detail || len(a.commitDetailRequests) != 0 {
		t.Fatal("opening an existing commit started another group's request")
	}
}
