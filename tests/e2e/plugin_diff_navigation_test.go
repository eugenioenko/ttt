package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/git"
)

// An editable inline diff is attached to the file tab rather than opened as a
// content tab, so the plugin API has to reach it through its own accessor.
// Testing the diff widget alone missed this: the widget worked while the tab
// lookup returned nil and editor.diff() reported no diff at all.
func TestPluginEditorAPISeesInlineDiffHunks(t *testing.T) {
	h := newTestHarness(t, 160, 40)
	defer h.stop()
	path := filepath.Join(h.dir, "code.txt")
	os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	os.WriteFile(path, []byte("one\nTWO\nthree\n"), 0644)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "code.txt", Status: "M"}, false, false)
	h.redraw()

	if !h.app.EditorGroup.IsInlineDiffActive() {
		t.Fatal("inline diff not active")
	}
	api := app.NewPluginEditorAPI(h.app)
	hunks := api.DiffHunks()
	if len(hunks) != 1 {
		t.Fatalf("editor.diff() hunks = %d, want 1", len(hunks))
	}
	if hunks[0].Kind != "modified" {
		t.Errorf("kind = %q, want modified", hunks[0].Kind)
	}
	if err := api.GoToDiffHunk(1); err != nil {
		t.Errorf("go_to_diff_hunk(1): %v", err)
	}
	if got := api.DiffHunkAt(); got != 1 {
		t.Errorf("current_hunk = %d, want 1", got)
	}
}

// A plain file tab is not a diff, so the API must say so rather than report an
// empty diff.
func TestPluginEditorAPIReturnsNilOnAPlainFile(t *testing.T) {
	h := newTestHarness(t, 160, 40)
	defer h.stop()
	path := filepath.Join(h.dir, "code.txt")
	os.WriteFile(path, []byte("one\ntwo\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	if hunks := app.NewPluginEditorAPI(h.app).DiffHunks(); hunks != nil {
		t.Errorf("editor.diff() on a plain file = %v, want nil", hunks)
	}
}
