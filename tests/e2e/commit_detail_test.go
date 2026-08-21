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

func gitDetailRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func (h *testHarness) awaitCommitDetail() *app.CommitDetailResult {
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

func TestCommitHistoryChevronExpandsWhileLabelOpensFullDetail(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	gitDetailRun(t, h.dir, "init", "-q", "-b", "main")
	gitDetailRun(t, h.dir, "config", "user.email", "test@test.com")
	gitDetailRun(t, h.dir, "config", "user.name", "Test User")
	gitDetailRun(t, h.dir, "config", "commit.gpgsign", "false")
	gitDetailRun(t, h.dir, "add", "-A")
	gitDetailRun(t, h.dir, "commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(h.dir, "alpha.txt"), []byte("alpha changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "beta.txt"), []byte("beta changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitDetailRun(t, h.dir, "add", "-A")
	gitDetailRun(t, h.dir, "commit", "-qm", "Detail subject", "-m", "Detail body paragraph.")

	// The harness has no event-loop goroutine, so populate the panel inline.
	// The detail open itself still uses the real screen and is applied from the
	// posted interrupt below, matching production's async boundary.
	h.app.Changes.Screen = nil
	h.app.Changes.Refresh()
	h.exec("sidebar.changes")

	var commitIndex int
	var commitID string
	for index, node := range h.app.Changes.CommitLog.FlatList() {
		if strings.HasPrefix(node.ID, "commit:") {
			commitIndex = index
			commitID = node.ID
			break
		}
	}
	if commitID == "" {
		t.Fatal("commit history has no commit row")
	}
	logRect := h.app.Changes.CommitLog.GetRect()
	rowY := logRect.Y + commitIndex - h.app.Changes.CommitLog.ScrollTop()

	// The first column is the rendered chevron. It changes disclosure state and
	// leaves the editor on its existing tab.
	h.click(logRect.X, rowY)
	commitNode := h.app.Changes.CommitLog.FlatList()[commitIndex]
	if !commitNode.Expanded {
		t.Fatal("chevron click did not expand the commit")
	}
	if strings.HasPrefix(h.app.EditorGroup.ActiveFilePath(), "commit:") {
		t.Fatal("chevron click opened commit detail")
	}
	h.click(logRect.X, rowY)
	if commitNode.Expanded {
		t.Fatal("second chevron click did not collapse the commit")
	}

	// The label is deliberately separate from the chevron. It opens a loading
	// editor tab immediately and does not alter expansion.
	h.click(logRect.X+4, rowY)
	if got := h.app.EditorGroup.ActiveFilePath(); got != commitID {
		t.Fatalf("active tab = %q, want full-hash key %q", got, commitID)
	}
	if commitNode.Expanded {
		t.Fatal("label activation changed commit expansion")
	}
	h.assertContains("Loading commit")

	result := h.awaitCommitDetail()
	if result.Ref != strings.TrimPrefix(commitID, "commit:") {
		t.Fatalf("detail result ref = %q, want %q", result.Ref, strings.TrimPrefix(commitID, "commit:"))
	}
	for _, text := range []string{"Detail subject", "Detail body paragraph.", "alpha.txt", "alpha changed", "beta.txt", "beta changed"} {
		h.assertContains(text)
	}
}
