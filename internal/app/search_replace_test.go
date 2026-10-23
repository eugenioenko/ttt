package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/search"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type replaceTestHarness struct {
	a                *App
	path             string
	work, deliveries []func()
}

func newReplaceTestHarness(t *testing.T) *replaceTestHarness {
	t.Helper()
	h := &replaceTestHarness{a: prepareListenTestApp(t), path: filepath.Join(t.TempDir(), "a.txt")}
	if err := os.WriteFile(h.path, []byte("foo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	h.a.Search.Input.Text, h.a.Search.ReplaceInput.Text = "foo", "bar"
	h.a.Search.Groups = []ui.SearchFileGroup{{FilePath: h.path, Matches: []ui.SearchMatch{{FilePath: h.path, LineNum: 1, ColEnd: 3, LineText: "foo"}}}}
	h.a.tasks.post = func(fn func()) error { h.deliveries = append(h.deliveries, fn); return nil }
	h.a.searchReplace.task = NewTask[map[string][]search.Match](h.a, asynctask.TaskOptions{Run: func(fn func()) { h.work = append(h.work, fn) }})
	h.a.searchReplace.scan = func(context.Context, search.Options, []string) (map[string][]search.Match, error) {
		return map[string][]search.Match{h.path: {{FilePath: h.path, LineNum: 1, ColEnd: 3, LineText: "foo"}}}, nil
	}
	t.Cleanup(h.a.CloseTasks)
	return h
}

func (h *replaceTestHarness) finish(t *testing.T) func() {
	t.Helper()
	if len(h.work) == 0 {
		t.Fatal("no queued scan")
	}
	work := h.work[0]
	h.work = h.work[1:]
	work()
	if len(h.deliveries) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(h.deliveries))
	}
	deliver := h.deliveries[0]
	h.deliveries = nil
	return deliver
}

func (h *replaceTestHarness) unchanged(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(h.path)
	if err != nil || string(data) != "foo\n" {
		t.Fatalf("unexpected write: %q, %v", data, err)
	}
}

func (h *replaceTestHarness) dialog(t *testing.T) *widgets.DialogWidget {
	t.Helper()
	adapter, ok := h.a.Root.TopOverlayWidget().(*ui.WidgetAdapter)
	if !ok {
		t.Fatal("missing confirmation")
	}
	return adapter.Inner().(*widgets.DialogWidget)
}

func TestSearchReplacePreflightDeliveryAndCompleteCount(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "all"}[all], func(t *testing.T) {
			h := newReplaceTestHarness(t)
			text := strings.Repeat("foo\n", 150) + strings.Repeat("x", 1200) + "foo\n"
			if err := os.WriteFile(h.path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			if all {
				gone := filepath.Join(filepath.Dir(h.path), "gone.txt")
				if err := os.WriteFile(gone, []byte("nothing here\n"), 0600); err != nil {
					t.Fatal(err)
				}
				h.a.Search.Groups = append(h.a.Search.Groups, ui.SearchFileGroup{FilePath: gone, Matches: []ui.SearchMatch{{LineNum: 1, ColStart: 1, ColEnd: 4}}})
			}
			h.a.searchReplace.scan = nil
			h.a.Search.OnReplaceRequest = func(req ui.SearchReplaceRequest) { req.AllFiles = all; h.a.startSearchReplace(req) }
			h.a.Search.ReplaceAllFiles()
			deliver := h.finish(t)
			if h.a.Root.HasOverlay() {
				t.Fatal("confirmation before UI delivery")
			}
			data, _ := os.ReadFile(h.path)
			if string(data) != text {
				t.Fatal("write before UI delivery")
			}
			deliver()
			if all {
				dialog := h.dialog(t)
				if !strings.Contains(dialog.Content.(*widgets.ParagraphWidget).Text, "151 matches across 1 files") {
					t.Fatal("confirmation lost uncapped matches")
				}
				dialog.Buttons[1].Handler()
			}
			data, _ = os.ReadFile(h.path)
			if string(data) != strings.ReplaceAll(text, "foo", "bar") {
				t.Fatal("uncapped replacement incomplete")
			}
		})
	}
}

func TestSearchReplacePreflightRejectsStaleDeliveries(t *testing.T) {
	for _, action := range []string{"cancel", "shutdown", "new request", "query", "include", "exclude", "case", "regex", "replacement", "workspace", "clear", "round trip"} {
		t.Run(action, func(t *testing.T) {
			h := newReplaceTestHarness(t)
			h.a.Search.ReplaceAllFiles()
			stale := h.finish(t)
			s := h.a.Search
			switch action {
			case "cancel":
				s.ReplaceInput.Actions[0].OnClick()
			case "shutdown":
				h.a.CloseTasks()
			case "new request":
				s.ReplaceAllFiles()
			case "query":
				s.Input.Text = "different"
			case "include":
				s.Include.Text = "*.go"
			case "exclude":
				s.Exclude.Text = "*.txt"
			case "case":
				s.Options.CaseSensitive = true
			case "regex":
				s.Options.UseRegex = true
			case "replacement":
				s.ReplaceInput.Text = "other"
			case "workspace":
				s.SetWorkDirs([]string{t.TempDir()})
			case "clear":
				h.a.ClearGlobalSearch()
			case "round trip":
				s.ReplaceInput.Text = "other"
				s.ReplaceInput.OnChange("other")
				s.ReplaceInput.Text = "bar"
				s.ReplaceInput.OnChange("bar")
			}
			stale()
			if h.a.Root.HasOverlay() {
				t.Fatal("stale scan presented confirmation")
			}
			h.unchanged(t)
			if action == "new request" {
				h.finish(t)()
				h.dialog(t)
			}
		})
	}
}

func TestSearchReplaceRevalidatesConfirmation(t *testing.T) {
	for _, action := range []string{"cancel", "shutdown", "query", "replacement", "new request"} {
		t.Run(action, func(t *testing.T) {
			h := newReplaceTestHarness(t)
			h.a.Search.ReplaceAllFiles()
			h.finish(t)()
			confirm := h.dialog(t).Buttons[1].Handler
			switch action {
			case "cancel":
				h.a.Search.CancelReplacement()
			case "shutdown":
				h.a.CloseTasks()
			case "query":
				h.a.Search.Input.Text = "different"
			case "replacement":
				h.a.Search.ReplaceInput.Text = "other"
			case "new request":
				h.a.Search.ReplaceAllFiles()
				h.finish(t)()
			}
			confirm()
			h.unchanged(t)
			if action == "new request" {
				h.dialog(t)
			}
		})
	}
}

func TestSearchReplacePreflightErrorsAndEmptyResults(t *testing.T) {
	for _, kind := range []string{"error", "cancel", "empty"} {
		t.Run(kind, func(t *testing.T) {
			h := newReplaceTestHarness(t)
			h.a.searchReplace.scan = func(context.Context, search.Options, []string) (map[string][]search.Match, error) {
				if kind == "cancel" {
					return nil, context.Canceled
				}
				if kind == "error" {
					return nil, errors.New("rg unavailable")
				}
				return nil, nil
			}
			h.a.Search.ReplaceAllFiles()
			h.finish(t)()
			if kind == "error" {
				h.dialog(t)
			} else if h.a.Root.HasOverlay() {
				t.Fatal("empty or cancelled scan used stale fallback")
			}
			h.unchanged(t)
		})
	}
}

func TestSearchReplacePreflightPreservesWriteChecks(t *testing.T) {
	for _, kind := range []string{"dirty", "changed", "gone"} {
		t.Run(kind, func(t *testing.T) {
			h := newReplaceTestHarness(t)
			h.a.Search.ReplaceAllFiles()
			h.finish(t)()
			switch kind {
			case "dirty":
				h.a.EditorGroup.OpenFile(h.path)
				h.a.EditorGroup.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
			case "changed":
				if err := os.WriteFile(h.path, []byte("xxfoo\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "gone":
				if err := os.Remove(h.path); err != nil {
					t.Fatal(err)
				}
			}
			h.dialog(t).Buttons[1].Handler()
			data, err := os.ReadFile(h.path)
			if kind == "dirty" && (err != nil || string(data) != "foo\n" || !h.a.EditorGroup.IsDirtyPath(h.path)) {
				t.Fatal("dirty file overwritten")
			}
			if kind == "changed" && (err != nil || string(data) != "xxfoo\n") {
				t.Fatal("stale offsets applied")
			}
			if kind == "gone" && !os.IsNotExist(err) {
				t.Fatal("deleted file recreated")
			}
		})
	}
}

func TestSearchReplaceCancellationReachesRunningScan(t *testing.T) {
	for _, action := range []string{"cancel", "shutdown", "new request", "query", "replacement", "workspace"} {
		t.Run(action, func(t *testing.T) {
			h := newReplaceTestHarness(t)
			started, done := make(chan struct{}), make(chan struct{})
			h.a.searchReplace.scan = func(ctx context.Context, _ search.Options, _ []string) (map[string][]search.Match, error) {
				close(started)
				<-ctx.Done()
				return nil, ctx.Err()
			}
			h.a.Search.ReplaceAllFiles()
			go func() { h.work[0](); close(done) }()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("scan did not start")
			}
			switch action {
			case "shutdown":
				h.a.CloseTasks()
			case "cancel":
				h.a.Search.CancelReplacement()
			case "new request":
				h.a.Search.ReplaceAllFiles()
			case "query":
				h.a.Search.Input.Text = ""
				h.a.Search.Input.OnChange("")
			case "replacement":
				h.a.Search.ReplaceInput.Text = "other"
				h.a.Search.ReplaceInput.OnChange("other")
			case "workspace":
				h.a.Search.SetWorkDirs([]string{t.TempDir()})
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("scan did not stop")
			}
			if len(h.deliveries) != 0 || h.a.Root.HasOverlay() {
				t.Fatal("cancelled scan delivered a result")
			}
			h.unchanged(t)
		})
	}
}

func TestSearchReplaceKeepsEventLoopResponsive(t *testing.T) {
	a := prepareListenTestApp(t)
	started, done := make(chan struct{}), make(chan struct{})
	a.searchReplace.scan = func(ctx context.Context, _ search.Options, _ []string) (map[string][]search.Match, error) {
		close(started)
		<-ctx.Done()
		close(done)
		return nil, ctx.Err()
	}
	startListenTestEventLoop(t, a)
	if err := runExecMain(a, func() error {
		a.Search.Input.Text, a.Search.ReplaceInput.Text = "foo", "bar"
		a.Search.ReplaceInput.CursorPos = 3
		a.Search.Groups = []ui.SearchFileGroup{{FilePath: "unused.txt"}}
		a.Search.SetReplaceMode(true)
		a.showSidebar()
		a.Sidebar.SetActivePanel("search")
		a.Root.SetFocus(a.Search)
		a.Search.ReplaceAllFiles()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("scan did not start")
	}
	if _, err := RunExecScript(a, `wait-for "Checking" timeout=2000; key Tab; type x`); err != nil {
		var screen string
		_ = runExecMain(a, func() error { screen = a.Screenshot(); return nil })
		t.Fatalf("%v\n%s", err, screen)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("typing did not cancel scan")
	}
	var text string
	var overlay bool
	if err := runExecMain(a, func() error {
		text, overlay = a.Search.ReplaceInput.Text, a.Root.HasOverlay()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if text != "barx" || overlay {
		t.Fatalf("input failed during scan: %q, overlay %v", text, overlay)
	}
}
