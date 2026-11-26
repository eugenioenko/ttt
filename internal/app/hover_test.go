package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/markdown"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
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

func TestInactiveSplitTabHoverUsesItsOwnGroup(t *testing.T) {
	h := hoverTestHarness(t, 123, 123)
	a := h.a
	left := a.EditorGroup
	a.SplitEditor()
	right := a.EditorGroup
	// Only hover delivery belongs in this test's fake task queue.
	left.FoldTask, right.FoldTask = nil, nil
	right.NewFileFromText("right editor")
	path := filepath.Join(t.TempDir(), strings.Repeat("long-name-", 12)+"file.txt")
	requireNoError(t, right.OpenFile(path))
	a.activateEditorGroup(left)
	a.Root.SetFocus(left)
	a.Root.SetSize(120, 30)
	var renderer render.Renderer
	a.Root.Render(renderer.NextFrame(120, 30))

	r := right.TabBar.GetRect()
	for x := r.X; x < r.X+r.W; x++ {
		if _, _, _, _, ok := right.TabBar.CutTabAt(x, r.Y+1); !ok {
			continue
		}
		a.TrackMouseHover(tcell.NewEventMouse(x, r.Y+1, tcell.ButtonNone, tcell.ModNone))
		h.clock.advance(a.hover.delay)
		h.run(t)()
		if right.Hover == nil || !right.Hover.HasContent() || left.Hover != nil || a.Root.Focused != left {
			t.Fatal("hover did not appear in the inactive group without stealing focus")
		}

		editor := left.Editor.GetRect()
		a.TrackMouseHover(tcell.NewEventMouse(editor.X+left.Editor.GutterWidth()+1, editor.Y, tcell.ButtonNone, a.Root.ControlModifiers))
		h.clock.advance(a.hover.delay)
		if right.Hover != nil || len(h.work) != 0 {
			t.Fatal("moving into the active editor with Control held scheduled or retained a hover")
		}

		a.TrackMouseHover(tcell.NewEventMouse(x, r.Y+1, tcell.ButtonNone, tcell.ModNone))
		h.clock.advance(a.hover.delay)
		h.run(t)()
		a.TrackMouseHover(tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone))
		if right.Hover != nil {
			t.Fatal("leaving the tab bar kept the inactive group's tooltip")
		}
		return
	}
	t.Fatal("expected a visibly truncated tab in the inactive group")
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
				requireNoError(t, a.EditorGroup.OpenFile(filepath.Join(filepath.Dir(a.EditorGroup.ActiveFilePath()), "other.txt")))
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

func TestJoinHoverTextSeparatesDiagnosticWithDivider(t *testing.T) {
	rendered := markdown.Render(joinHoverText("declared and not used: x", "var x int"))
	nonBlank := make([]markdown.Line, 0, len(rendered))
	for _, line := range rendered {
		if line.Kind != markdown.KindBlank {
			nonBlank = append(nonBlank, line)
		}
	}
	if len(nonBlank) < 3 {
		t.Fatalf("expected diagnostic, divider, and hover lines, got %d", len(nonBlank))
	}
	if nonBlank[0].Kind != markdown.KindParagraph || nonBlank[0].Text() != "declared and not used: x" {
		t.Fatalf("diagnostic rendered as %#v %q", nonBlank[0].Kind, nonBlank[0].Text())
	}
	foundDivider := false
	for _, line := range nonBlank {
		if line.Kind == markdown.KindDivider {
			foundDivider = true
		}
		if line.Kind == markdown.KindHeading {
			t.Fatal("diagnostic was rendered as a heading")
		}
	}
	if !foundDivider {
		t.Fatal("diagnostic and hover text were not separated by a divider")
	}
	if !strings.Contains(nonBlank[len(nonBlank)-1].Text(), "var x int") {
		t.Fatalf("hover text missing from final line: %q", nonBlank[len(nonBlank)-1].Text())
	}
	for _, tc := range []struct {
		name, diagnostic, text, want string
	}{
		{name: "diagnostic only", diagnostic: "diagnostic", want: "diagnostic"},
		{name: "hover only", text: "hover", want: "hover"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinHoverText(tc.diagnostic, tc.text); got != tc.want {
				t.Fatalf("joinHoverText(%q, %q) = %q, want %q", tc.diagnostic, tc.text, got, tc.want)
			}
		})
	}
}
