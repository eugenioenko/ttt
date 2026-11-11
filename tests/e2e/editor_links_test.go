package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func (h *testHarness) ctrlClickText(text string) {
	h.t.Helper()
	h.redraw()
	for y := 0; y < h.app.Root.Height; y++ {
		if col := displayColumnOf(h.screenRow(y), text); col >= 0 {
			x := col + 1
			h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
			h.redraw()
			if h.app.Root.PointerShapeAt(x, y) != term.PointerHand {
				h.t.Fatalf("missing link pointer for %q", text)
			}
			for range 2 {
				h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModCtrl))
				h.redraw()
			}
			h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
			return
		}
	}
	h.t.Fatalf("missing %q:\n%s", text, h.screenText())
}

func TestEditorLinkNavigationThroughApp(t *testing.T) {
	for _, untitled := range []bool{false, true} {
		h := newTestHarness(t, 100, 25)
		defer h.stop()
		g := h.app.EditorGroup
		target := filepath.Join(h.dir, "subdir", "nested.txt")
		if untitled {
			g.NewFile()
			h.redraw()
			h.app.PasteText("https://example.com\nsubdir/nested.txt:1:2")
		} else {
			source := filepath.Join(h.dir, "subdir", "links.txt")
			if err := os.WriteFile(source, []byte("https://example.com\nnested.txt:1:2"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := g.OpenFile(source); err != nil {
				t.Fatal(err)
			}
		}
		h.app.FocusEditor()
		opened := 0
		g.OnOpenURL = func(url string) {
			opened++
			if url != "https://example.com" {
				t.Fatalf("opened URL %q", url)
			}
		}
		h.ctrlClickText("https://example.com")
		if opened != 1 {
			t.Fatalf("opened URL %d times", opened)
		}
		h.app.Screen.SetPointerShapesEnabled(false)
		h.ctrlClickText("nested.txt:1:2")
		if g.ActiveFilePath() != target || g.Editor.Cursor.Line != 0 || g.Editor.Cursor.Col != 1 || h.app.Root.Focused != g {
			t.Fatalf("file link navigation failed: %q at %+v", g.ActiveFilePath(), g.Editor.Cursor)
		}
	}
}

func TestEditorFileLinksInSecondaryRepositoryDiff(t *testing.T) {
	for _, kind := range []string{"staged", "deleted", "commit"} {
		t.Run(kind, func(t *testing.T) {
			h := newTestHarness(t, 120, 25)
			defer h.stop()
			repo := t.TempDir()
			runHistoryGit(t, repo, "init", "-q")
			runHistoryGit(t, repo, "config", "user.name", "Test")
			runHistoryGit(t, repo, "config", "user.email", "test@example.com")
			if err := os.MkdirAll(filepath.Join(repo, "subdir"), 0700); err != nil {
				t.Fatal(err)
			}
			source, target := filepath.Join(repo, "subdir", "links.txt"), filepath.Join(repo, "subdir", "target.txt")
			for path, text := range map[string]string{source: "target.txt:2:3\nold\n", target: "first\nsecond target\n"} {
				if err := os.WriteFile(path, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			runHistoryGit(t, repo, "add", ".")
			runHistoryGit(t, repo, "commit", "-qm", "initial")
			if err := os.WriteFile(source, []byte("target.txt:2:3\nnew\n"), 0600); err != nil {
				t.Fatal(err)
			}
			status := git.FileStatus{Path: "subdir/links.txt", Status: "M"}
			switch kind {
			case "staged":
				runHistoryGit(t, repo, "add", ".")
				status.Staged = true
				h.app.OpenChangeDiff(repo, git.ToChange(status), true)
			case "deleted":
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
				status.Status = "D"
				h.app.OpenChangeDiff(repo, git.ToChange(status), true)
			case "commit":
				runHistoryGit(t, repo, "add", ".")
				runHistoryGit(t, repo, "commit", "-qm", "change")
				h.app.OpenCommitDiff(repo, "HEAD", "HEAD", git.ToChange(status), true)
			}
			h.ctrlClickText("target.txt:2:3")
			g := h.app.EditorGroup
			if g.ActiveFilePath() != target || g.Editor.Cursor.Line != 1 || g.Editor.Cursor.Col != 2 {
				t.Fatalf("diff link navigation failed: %q at %+v", g.ActiveFilePath(), g.Editor.Cursor)
			}
		})
	}
}

func TestEditorPRDiffLinksUseWorkspace(t *testing.T) {
	for _, search := range []bool{false, true} {
		h := newTestHarness(t, 120, 25)
		defer h.stop()
		target := filepath.Join(h.dir, "target.txt")
		if err := os.WriteFile(target, []byte("first\nsecond target\n"), 0600); err != nil {
			t.Fatal(err)
		}
		status := git.FileStatus{Path: "src/links.txt", Status: "M"}
		diffs := map[string]string{status.Path: "--- a/src/links.txt\n+++ b/src/links.txt\n@@ -1 +1 @@\n-old\n+target.txt:2:3\n"}
		h.app.Changes.AddPRGroup("PR #1: example", "", "", "", "", "", "", []git.FileStatus{status}, diffs)
		if search {
			h.app.NavigateToSearchMatch(status.Path+" (diff)", 1, 1)
		} else {
			groups := h.app.Changes.Groups()
			h.app.OpenPRDiff(&groups[len(groups)-1], status, false)
		}
		g := h.app.EditorGroup
		view := g.ActiveDiffWidget()
		if view == nil {
			t.Fatal("PR diff did not open")
		}
		if view.FilePath != status.Path {
			t.Fatalf("PR diff source path = %q, want %q", view.FilePath, status.Path)
		}
		h.ctrlClickText("target.txt:2:3")
		if g.ActiveFilePath() != target || g.Editor.Cursor.Line != 1 || g.Editor.Cursor.Col != 2 {
			t.Fatalf("PR link did not use the workspace: %q at %+v", g.ActiveFilePath(), g.Editor.Cursor)
		}
	}
}

func TestEditorLinksInReadOnlyDiffTabs(t *testing.T) {
	for _, mode := range []ui.DiffMode{ui.DiffModeUnified, ui.DiffModeSplit} {
		h := newTestHarness(t, 120, 25)
		defer h.stop()
		g := h.app.EditorGroup
		g.DiffMode = mode
		g.OpenDiff(filepath.Join(h.dir, "links.txt"), diff.FileDiff{}, []string{"https://old.test"}, []string{"https://new.test"}, true)
		opened := ""
		g.OnOpenURL = func(url string) { opened = url }
		for _, url := range []string{"https://old.test", "https://new.test"} {
			h.ctrlClickText(url)
			if opened != url {
				t.Fatalf("opened %q, want %q", opened, url)
			}
		}
	}
}

func TestEditorLinksInCommitDetails(t *testing.T) {
	for _, current := range []bool{false, true} {
		for _, mode := range []ui.DiffMode{ui.DiffModeUnified, ui.DiffModeSplit} {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			repo := t.TempDir()
			if err := os.MkdirAll(filepath.Join(repo, "src"), 0700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(repo, "src", "target.txt")
			if err := os.WriteFile(target, []byte("first\nsecond target\n"), 0600); err != nil {
				t.Fatal(err)
			}
			detail := ui.NewCommitDetailWidget(repo, "HEAD", "HEAD", false)
			if current {
				detail = ui.NewCurrentChangesWidget(repo, false)
			}
			detail.SetDetail("Subject", []ui.CommitDetailFile{{Path: "src/change.txt", Diff: diff.Parse(
				"--- a/src/change.txt\n+++ b/src/change.txt\n@@ -1,1 +1,2 @@\n-https://old.test\n+https://new.test\n+target.txt:2:3\n",
			)}}, "")
			g := h.app.EditorGroup
			g.DiffMode = mode
			g.ApplyDiffDefaults(detail)
			g.OpenPluginTab("detail", "Details", detail)
			h.app.FocusEditor()
			var opened []string
			g.OnOpenURL = func(url string) { opened = append(opened, url) }
			for _, url := range []string{"https://old.test", "https://new.test"} {
				h.ctrlClickText(url)
				if len(opened) != 1 || opened[0] != url {
					t.Fatalf("detail URL routing = %v, want %q", opened, url)
				}
				opened = nil
			}
			h.ctrlClickText("target.txt:2:3")
			if g.ActiveFilePath() != target || g.Editor.Cursor.Line != 1 || g.Editor.Cursor.Col != 2 {
				t.Fatalf("detail file link opened %q at %+v", g.ActiveFilePath(), g.Editor.Cursor)
			}
		}
	}
}
