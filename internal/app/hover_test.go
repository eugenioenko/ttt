package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

func hoverTestHarness(t *testing.T, delay, wantDelay int) *gitGutterTestHarness {
	t.Helper()
	h := newGitGutterTestHarness(t)
	h.a.Settings.LSP.HoverDelay = delay
	h.a.EditorGroup.Editor.SetRect(ui.Rect{W: 80, H: 20})
	h.a.EditorGroup.Editor.Diagnostics = []ui.Diagnostic{{StartLine: 0, EndLine: 0, EndCol: 10, Message: "diagnostic"}}
	h.a.hover.delay = time.Duration(wantDelay) * time.Millisecond
	h.a.hover.task = NewTask[string](h.a, asynctask.TaskOptions{
		Delay: h.a.hover.delay, Clock: &h.clock,
		Run: func(fn func()) { h.work = append(h.work, fn) },
	})
	return h
}

func TestHoverDelayDeduplicationAndSamePositionRetry(t *testing.T) {
	for _, delay := range []int{123, 0} {
		want := delay
		if want == 0 {
			want = 400
		}
		h := hoverTestHarness(t, delay, want)
		a := h.a
		x := a.EditorGroup.Editor.GutterWidth() + 1
		a.checkMouseHover(x, 0)
		h.clock.advance(a.hover.delay - time.Millisecond)
		a.checkMouseHover(x, 0)
		if len(h.work) != 0 {
			t.Fatal("hover started before its configured delay")
		}
		h.clock.advance(time.Millisecond)
		deliver := h.run(t)
		if a.EditorGroup.Hover != nil {
			t.Fatal("hover appeared before UI delivery")
		}
		deliver()
		popup := a.EditorGroup.Hover
		if popup == nil || popup.AnchorX != x || !popup.HasContent() {
			t.Fatal("diagnostic hover did not appear at its anchor")
		}
		a.checkMouseHover(x, 0)
		h.clock.advance(a.hover.delay)
		if len(h.work) != 0 || a.EditorGroup.Hover != popup {
			t.Fatal("same key replaced completed hover")
		}
		a.DismissHover()
		a.checkMouseHover(x, 0)
		h.clock.advance(a.hover.delay)
		h.run(t)()
		if a.EditorGroup.Hover == nil {
			t.Fatal("dismissal prevented retry at the same position")
		}
	}
}

func TestExplicitHoverFlushesAndRefreshesCompletedKey(t *testing.T) {
	h := hoverTestHarness(t, 500, 500)
	a := h.a
	for range 2 {
		a.RequestHover(a.EditorGroup.ActiveFilePath(), "plaintext", 0, 1, 8, 4, "diagnostic")
		h.run(t)()
		if a.EditorGroup.Hover == nil || a.EditorGroup.Hover.AnchorX != 8 || a.EditorGroup.Hover.AnchorY != 4 {
			t.Fatal("explicit hover did not flush and show the popup")
		}
	}
	h.clock.advance(a.hover.delay)
	if len(h.work) != 0 {
		t.Fatal("explicit hover left delayed work behind")
	}
}

func TestHoverQueuedReplyCannotReopenAfterInvalidation(t *testing.T) {
	for _, action := range []string{"dismiss", "move", "edit", "file", "disable", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			h := hoverTestHarness(t, 123, 123)
			a := h.a
			x := a.EditorGroup.Editor.GutterWidth() + 1
			a.checkMouseHover(x, 0)
			h.clock.advance(a.hover.delay)
			stale := h.run(t)
			switch action {
			case "dismiss":
				a.DismissHover()
			case "move":
				a.checkMouseHover(x+1, 0)
			case "edit":
				a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
			case "file":
				a.EditorGroup.OpenFile(filepath.Join(filepath.Dir(a.EditorGroup.ActiveFilePath()), "other.txt"))
			case "disable":
				enabled := false
				a.Settings.LSP.Hover = &enabled
				a.ApplySettings(*a.Settings)
			case "shutdown":
				a.CloseTasks()
			}
			stale()
			if a.EditorGroup.Hover != nil {
				t.Fatal("invalidated reply reopened hover")
			}
			if action == "move" {
				h.clock.advance(a.hover.delay)
				h.run(t)()
				if a.EditorGroup.Hover == nil || a.EditorGroup.Hover.AnchorX != x+1 {
					t.Fatal("latest pointer position did not receive hover")
				}
			}
		})
	}
}

func TestHoverDiagnosticFallbackWithoutServerOrOnStartupFailure(t *testing.T) {
	manager := lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"missing": {Command: []string{filepath.Join(t.TempDir(), "missing-server")}},
	}})
	for _, server := range []string{"", "missing"} {
		text, err := readHover(context.Background(), hoverKey{Server: server, Manager: manager, WorkDir: t.TempDir(), Diagnostic: "diagnostic"})
		if err != nil || text != "diagnostic" {
			t.Fatalf("fallback with server %q = %q, error = %v", server, text, err)
		}
	}
}

func TestHoverHeadlessEmptyReplyClearsPreviousPopup(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a
	a.tasks.post = nil
	a.RequestHover(a.EditorGroup.ActiveFilePath(), "plaintext", 0, 1, 8, 4, "diagnostic")
	if a.EditorGroup.Hover == nil {
		t.Fatal("headless hover did not apply synchronously")
	}
	a.RequestHover(a.EditorGroup.ActiveFilePath(), "plaintext", 0, 1, 8, 4, "")
	if a.EditorGroup.Hover != nil {
		t.Fatal("empty headless reply retained old hover")
	}
}
