package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestEditorSplitMoreMenuAndEditing(t *testing.T) {
	for _, closeRight := range []bool{false, true} {
		t.Run(map[bool]string{false: "close from left", true: "close from right"}[closeRight], func(t *testing.T) {
			h := newTestHarness(t, 140, 32)
			defer h.stop()
			left := h.app.EditorGroup
			alpha, beta := filepath.Join(h.dir, "alpha.txt"), filepath.Join(h.dir, "beta.txt")
			if err := left.OpenFile(alpha); err != nil {
				t.Fatal(err)
			}
			left.CommitActiveTab()
			if err := left.OpenFile(beta); err != nil {
				t.Fatal(err)
			}
			h.app.FocusEditor()
			h.pressRune('R')
			buf, stack := left.ActiveBuffer(), left.Editor.Undo
			chooseEditorMore(t, h, "Split in Half")
			right := h.app.EditorGroup
			if left == right || !h.app.EditorSplit.ShowBottom || right.ActiveBuffer() != buf || right.Editor.Undo != stack {
				t.Fatal("split did not move the active tab with its buffer and undo history")
			}
			if left.ActiveFilePath() != alpha || right.ActiveFilePath() != beta {
				t.Fatal("split lost the open files")
			}
			if left.GetRect().Y != right.GetRect().Y || left.GetRect().H != right.GetRect().H {
				t.Fatal("split groups are not aligned")
			}
			if delta := left.GetRect().W - right.GetRect().W; delta < -1 || delta > 1 {
				t.Fatal("split did not start in equal halves")
			}
			h.exec("editor.undo")
			if buf.Lines[0] != "b" {
				t.Fatalf("undo after moving tab = %q", buf.Lines[0])
			}
			h.pressRune('R')
			r := left.Editor.GetRect()
			h.click(r.X+left.Editor.GutterWidth(), r.Y)
			h.pressRune('L')
			if h.app.EditorGroup != left || left.ActiveBuffer().Lines[0] != "La" || right.ActiveBuffer().Lines[0] != "Rb" {
				t.Fatal("clicking the left group did not route edits to it")
			}
			h.exec("focus.nextGroup")
			if h.app.EditorGroup != right || h.app.Root.Focused != right {
				t.Fatal("focus cycling did not reach the right group")
			}
			if !closeRight {
				h.exec("focus.prevGroup")
			}
			chooseEditorMore(t, h, "Close Split")
			if h.app.EditorSplit.ShowBottom || h.app.EditorGroup.TabCount() != 2 {
				t.Fatal("closing split did not merge both groups")
			}
			if h.app.EditorGroup.BufferForPath(alpha).Lines[0] != "La" || h.app.EditorGroup.BufferForPath(beta).Lines[0] != "Rb" {
				t.Fatal("closing split lost edits")
			}
			h.exec("editor.split")
			h.exec("editor.closeSplit")
			if h.app.EditorGroup.TabCount() != 2 {
				t.Fatal("repeated split changed the tab count")
			}
		})
	}
}

func TestEditorSplitDivider(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	h.exec("editor.split")
	h.redraw()
	split := h.app.EditorSplit
	x, y := split.DividerScreenX(), split.GetRect().Y+4
	if h.app.Root.PointerShapeAt(x, y) != term.PointerEWResize {
		t.Fatal("divider did not show the resize pointer")
	}
	for _, event := range []*tcell.EventMouse{
		tcell.NewEventMouse(x, y, tcell.Button1, 0),
		tcell.NewEventMouse(x+12, y, tcell.Button1, 0),
		tcell.NewEventMouse(x+12, y, tcell.ButtonNone, 0),
	} {
		h.app.Root.HandleEvent(event)
		h.redraw()
	}
	if split.DividerScreenX() != x+12 || split.OwnsPointerCapture() {
		t.Fatalf("divider ended at %d, want %d, captured %v", split.DividerScreenX(), x+12, split.OwnsPointerCapture())
	}
	h.exec("editor.closeSplit")
	h.redraw()
	if split.Bottom != nil || split.Top.GetRect().W != split.GetRect().W {
		t.Fatal("closing split did not restore the full editor width")
	}
}

func TestEditorSplitExistingFileAndQuit(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	path := filepath.Join(h.dir, "alpha.txt")
	left := h.app.EditorGroup
	if err := left.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	h.exec("editor.split")
	right := h.app.EditorGroup
	h.pressRune('x')
	h.app.Root.SetFocus(left)
	if err := left.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	if h.app.EditorGroup != right || left.BufferForPath(path) != nil {
		t.Fatal("opening an existing file created a second buffer")
	}
	h.app.Root.SetFocus(left)
	h.exec("editor.quit")
	if !h.app.Root.HasOverlay() || !*h.app.Running {
		t.Fatal("quit ignored unsaved edits in the inactive group")
	}
	h.pressKey(tcell.KeyEscape, 0)
	if err := left.OpenFile(filepath.Join(h.dir, "beta.txt")); err != nil {
		t.Fatal(err)
	}
	h.pressRune('y')
	h.app.Settings.Editor.SaveAllOnQuit = true
	h.app.Settings.Editor.SaveAllOnQuitExceptUnnamed = false
	h.exec("editor.quit")
	h.pressRune('s')
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "xa\n" || *h.app.Running {
		t.Fatalf("save all and quit = %q, %v, running %v", content, err, *h.app.Running)
	}
	if content, err := os.ReadFile(filepath.Join(h.dir, "beta.txt")); err != nil || string(content) != "yb\n" {
		t.Fatalf("save all missed the other group: %q, %v", content, err)
	}
	h.app.CloseEditorSplit()
}

func chooseEditorMore(t *testing.T, h *testHarness, label string) {
	t.Helper()
	h.redraw()
	r := h.app.EditorGroup.TabBar.MoreButton.GetRect()
	h.click(r.X+1, r.Y)
	menu, ok := h.app.Root.Focused.(*ui.ContextMenuWidget)
	if !ok || menu.Items[0].Label != label {
		t.Fatalf("More menu = %T, want first item %q", h.app.Root.Focused, label)
	}
	h.pressKey(tcell.KeyEnter, 0)
}
