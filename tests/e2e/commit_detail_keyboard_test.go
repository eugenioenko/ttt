package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestCommitDetailKeyboardOnlyDisclosureFlow(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	detail := ui.NewCommitDetailWidget(h.dir, "ref", "abcdef0", false)
	detail.SetDetail("Keyboard detail", []ui.CommitDetailFile{{
		Path: "detail.txt", Diff: diff.Parse("--- a/detail.txt\n+++ b/detail.txt\n@@ -1 +1 @@\n-old content\n+new content\n"),
	}}, "")
	h.app.EditorGroup.OpenPluginTab("commit-detail", "Commit abcdef0", detail)
	h.app.Root.SetFocus(h.app.EditorGroup)
	h.redraw()
	h.assertContains("new content")
	h.pressKey(tcell.KeyTab, 0)
	h.pressKey(tcell.KeyEnter, 0)
	h.assertNotContains("new content")
	h.assertContains("Expand all")
	h.pressRune(' ')
	h.assertContains("new content")
	h.pressKey(tcell.KeyTab, 0)
	h.pressKey(tcell.KeyEnter, 0)
	h.assertNotContains("new content")
	h.pressKey(tcell.KeyEnter, 0)
	h.assertContains("new content")
}
