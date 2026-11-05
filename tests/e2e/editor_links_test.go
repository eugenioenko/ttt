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
	for _, kind := range []string{"staged", "deleted", "commit", "pr"} {
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
				h.app.OpenChangeDiff(repo, status, true, true)
			case "deleted":
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
				status.Status = "D"
				h.app.OpenChangeDiff(repo, status, false, true)
			case "commit":
				runHistoryGit(t, repo, "add", ".")
				runHistoryGit(t, repo, "commit", "-qm", "change")
				h.app.OpenCommitDiff(repo, "HEAD", "HEAD", status, true)
			case "pr":
				group := &ui.ChangesGroup{Dir: repo, PRDiffs: map[string]string{
					status.Path: "--- a/subdir/links.txt\n+++ b/subdir/links.txt\n@@ -1,2 +1,2 @@\n target.txt:2:3\n-old\n+new\n",
				}}
				h.app.OpenPRDiff(group, status, false)
			}
			h.ctrlClickText("target.txt:2:3")
			g := h.app.EditorGroup
			if g.ActiveFilePath() != target || g.Editor.Cursor.Line != 1 || g.Editor.Cursor.Col != 2 {
				t.Fatalf("diff link navigation failed: %q at %+v", g.ActiveFilePath(), g.Editor.Cursor)
			}
		})
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
