package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/gdamore/tcell/v3"
)

func runHistoryGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func (h *testHarness) awaitCommitHistoryDetail() *app.CommitDetailResult {
	h.t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-h.screen.EventQ():
			interrupt, ok := event.(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			result, ok := interrupt.Data().(*app.CommitDetailResult)
			if !ok {
				continue
			}
			h.app.ApplyCommitDetail(result)
			h.redraw()
			return result
		case <-deadline:
			h.t.Fatal("timed out waiting for commit detail")
			return nil
		}
	}
}

func TestChangesHistoryUsesResponsiveHalfLayoutAndKeyboardNavigation(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	h.exec("sidebar.changes")
	cp := h.app.Changes
	want := cp.Split.GetRect().H / 2
	if got := cp.CommitLog.GetRect().H + 1; got != want {
		t.Fatalf("initial history section height = %d, want half-derived %d", got, want)
	}

	h.screen.SetSize(100, 40)
	h.app.Root.SetSize(100, 40)
	h.redraw()
	want = cp.Split.GetRect().H / 2
	if got := cp.CommitLog.GetRect().H + 1; got != want {
		t.Fatalf("resized history section height = %d, want half-derived %d", got, want)
	}

	cp.Adapter.SetFocused(true)
	cp.Adapter.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	cp.Adapter.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	if cp.Adapter.FocusedWidget() != cp.CommitLog {
		t.Fatalf("keyboard focus = %T, want commit history", cp.Adapter.FocusedWidget())
	}
}

func TestCommitHistoryOpensMetadataDetailAndWholeHeaderRowCollapses(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	runHistoryGit(t, h.dir, "init", "-q", "-b", "main")
	runHistoryGit(t, h.dir, "config", "user.email", "test@test.com")
	runHistoryGit(t, h.dir, "config", "user.name", "Test User")
	runHistoryGit(t, h.dir, "config", "commit.gpgsign", "false")
	runHistoryGit(t, h.dir, "add", "-A")
	runHistoryGit(t, h.dir, "commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(h.dir, "alpha.txt"), []byte("alpha changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "beta.txt"), []byte("beta changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runHistoryGit(t, h.dir, "add", "-A")
	runHistoryGit(t, h.dir, "commit", "-qm", "Detail subject", "-m", "Detail body.")

	h.app.Changes.Screen = nil
	h.app.Changes.Refresh()
	h.exec("sidebar.changes")
	log := h.app.Changes.CommitLog
	commitIndex := -1
	for index, node := range log.FlatList() {
		if strings.HasPrefix(node.ID, "commit:") {
			commitIndex = index
			break
		}
	}
	if commitIndex < 0 {
		t.Fatal("commit history has no commit row")
	}
	rowY := log.GetRect().Y + commitIndex - log.ScrollTop()
	h.click(log.GetRect().X+4, rowY)
	result := h.awaitCommitHistoryDetail()
	metadata := "Authored " + result.AuthoredAt.Format("Jan 2, 2006 at 3:04 PM -0700")
	for _, text := range []string{metadata, "Detail subject", "Detail body.", "alpha.txt", "alpha changed", "beta.txt", "beta changed"} {
		h.assertContains(text)
	}

	lines := strings.Split(h.screenText(), "\n")
	headingY := -1
	for y, line := range lines {
		if strings.Contains(line, "alpha.txt") {
			headingY = y
			break
		}
	}
	if headingY < 0 {
		t.Fatal("alpha heading is not visible")
	}
	detail := h.app.EditorGroup.ActiveCommitDetailWidget()
	r := detail.GetRect()
	h.click(r.X+r.W-2, headingY)
	h.assertNotContains("alpha changed")
	h.assertContains("beta changed")
}
