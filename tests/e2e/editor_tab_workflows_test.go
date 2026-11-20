package e2e

import (
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
	"path/filepath"
	"testing"
)

func TestEditorOpenToSideIgnoresNonTextTabs(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	group := h.app.EditorGroup
	group.OpenPluginTab("form", "Form", widgets.NewInputWidget(widgets.InputConfig{}))
	h.exec("editor.openToSide")
	if len(h.app.EditorSplit.Groups()) != 1 || h.app.EditorGroup != group || group.ActiveFilePath() != "form" {
		t.Fatal("Open to the Side created an empty split for a non-text tab")
	}
}

func TestEditorOpenUntitledToSideSharesOnlyWhenRequested(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	left := h.app.EditorGroup
	h.exec("editor.openToSide")
	right := h.app.EditorGroup
	if len(h.app.EditorSplit.Groups()) != 2 || right.TabCount() != 1 || right.ActiveBuffer() != left.ActiveBuffer() || right.ActiveFilePath() != left.ActiveFilePath() {
		t.Fatal("Open to the Side did not share the untitled document")
	}
	h.typeText("shared")
	if left.ActiveBuffer().Lines[0] != "shared" {
		t.Fatal("explicit untitled views did not share edits")
	}
}

func TestEditorSplitDragAcrossGroups(t *testing.T) {
	for _, command := range []string{"editor.split", "editor.splitDown"} {
		t.Run(command, func(t *testing.T) {
			h := newTestHarness(t, 160, 44)
			defer h.stop()
			left := h.app.EditorGroup
			alpha, beta := filepath.Join(h.dir, "alpha.txt"), filepath.Join(h.dir, "beta.txt")
			left.OpenFile(alpha)
			left.CommitActiveTab()
			left.OpenFile(beta)
			h.exec(command)
			right := h.app.EditorGroup
			h.app.Root.SetFocus(left)
			left.SwitchToTabByPath(alpha)
			h.redraw()
			r := left.TabBar.GetRect()
			x, y := r.X+4, r.Y+1
			dest := right.Editor.GetRect()
			dx, dy := dest.X+dest.W/2, dest.Y+dest.H/2
			h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
			h.app.Root.HandleEvent(tcell.NewEventMouse(dx, dy, tcell.Button1, 0))
			h.redraw()
			if !right.DropTarget || right.DropZone != ui.EditorDropCenter {
				t.Fatal("drag did not mark the destination group")
			}
			h.app.Root.HandleEvent(tcell.NewEventMouse(dx, dy, tcell.ButtonNone, 0))
			h.flushOnChange()
			h.redraw()
			if h.app.EditorGroup != right || right.ActiveFilePath() != alpha || left.BufferForPath(alpha) != nil || right.DropTarget {
				t.Fatal("dropping the tab did not transfer it to the destination")
			}
			h.app.Root.SetFocus(left)
			h.exec("editor.moveToOtherGroup")
			if len(h.app.EditorSplit.Groups()) != 1 || right.TabCount() != 2 || h.app.Root.Focused != right {
				t.Fatal("moving the last tab did not remove the empty group and deduplicate views")
			}
		})
	}
}

func TestEditorSplitOpenToSideAndCloseLastView(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	left := h.app.EditorGroup
	alpha, beta := filepath.Join(h.dir, "alpha.txt"), filepath.Join(h.dir, "beta.txt")
	left.OpenFile(alpha)
	h.exec("editor.openToSide")
	right := h.app.EditorGroup
	if left.ActiveBuffer() != right.ActiveBuffer() || len(h.app.EditorSplit.Groups()) != 2 {
		t.Fatal("Open to the Side did not share the document")
	}
	h.pressRune('x')
	h.exec("tab.close")
	if len(h.app.EditorSplit.Groups()) != 1 || h.app.Root.HasOverlay() || !left.ActiveBuffer().Dirty {
		t.Fatal("closing one shared view lost the document or kept an empty group")
	}
	h.app.ExplorerContextNode = &widgets.TreeNode{ID: beta}
	h.exec("explorer.openToSide")
	if h.app.EditorGroup.ActiveFilePath() != beta || left.ActiveFilePath() != alpha {
		t.Fatal("Explorer Open to the Side replaced the original document")
	}
	h.exec("editor.closeSplit")
	left.SwitchToTabByPath(alpha)
	h.app.Root.SetFocus(left)
	h.exec("tab.close")
	if !h.app.Root.HasOverlay() {
		t.Fatal("closing the last dirty view did not ask to save")
	}
	h.pressKey(tcell.KeyEscape, 0)
}

func TestEditorSplitDropUsesDestinationTabOrder(t *testing.T) {
	for _, name := range []string{"start", "end", "existing-end"} {
		t.Run(name, func(t *testing.T) {
			h := newTestHarness(t, 160, 40)
			defer h.stop()
			left := h.app.EditorGroup
			alpha, beta, gamma := filepath.Join(h.dir, "alpha.txt"), filepath.Join(h.dir, "beta.txt"), filepath.Join(h.dir, "gamma.txt")
			if err := left.OpenFile(alpha); err != nil {
				t.Fatal(err)
			}
			left.CommitActiveTab()
			if err := left.OpenFile(beta); err != nil {
				t.Fatal(err)
			}
			h.exec("editor.split")
			right := h.app.EditorGroup
			first := beta
			if name == "existing-end" {
				first = alpha
			}
			if err := right.OpenFile(first); err != nil {
				t.Fatal(err)
			}
			right.CommitActiveTab()
			if err := right.OpenFile(gamma); err != nil {
				t.Fatal(err)
			}
			h.app.Root.SetFocus(left)
			left.SwitchToTabByPath(alpha)
			h.redraw()
			src, dest := left.TabBar.GetRect(), right.TabBar.GetRect()
			x := dest.X + dest.W - 5
			if name == "start" {
				x = dest.X + 1
			}
			for _, event := range []*tcell.EventMouse{
				tcell.NewEventMouse(src.X+4, src.Y+1, tcell.Button1, 0),
				tcell.NewEventMouse(x, dest.Y+1, tcell.Button1, 0),
				tcell.NewEventMouse(x, dest.Y+1, tcell.ButtonNone, 0),
			} {
				h.app.Root.HandleEvent(event)
				h.redraw()
			}
			index, count := 2, 3
			if name == "start" {
				index = 0
			}
			if name == "existing-end" {
				index, count = 1, 2
			}
			path, _ := right.TabInfo(index)
			if right.TabCount() != count || path != alpha || left.BufferForPath(alpha) != nil {
				t.Fatal("drop did not insert or deduplicate the tab at the requested position")
			}
		})
	}
}

func TestEditorSplitDragCancellationClearsDestination(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	left := h.app.EditorGroup
	h.exec("editor.split")
	right := h.app.EditorGroup
	h.app.Root.SetFocus(left)
	h.redraw()
	src, dest := left.TabBar.GetRect(), right.Editor.GetRect()
	h.app.Root.HandleEvent(tcell.NewEventMouse(src.X+4, src.Y+1, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(dest.X+4, dest.Y+1, tcell.Button1, 0))
	h.redraw()
	if !right.DropTarget {
		t.Fatal("destination was not highlighted during drag")
	}
	h.pressKey(tcell.KeyEscape, 0)
	if right.DropTarget || h.app.Root.PointerCaptureActive() || left.TabCount() != 1 || right.TabCount() != 1 {
		t.Fatal("cancellation left a highlight, capture or moved tab")
	}
}

type rawEscapeContent struct {
	ui.BaseWidget
	escapes int
}

func (w *rawEscapeContent) Render(ui.Surface)  {}
func (w *rawEscapeContent) WantsRawKeys() bool { return true }
func (w *rawEscapeContent) Focusable() bool    { return true }
func (w *rawEscapeContent) HandleEvent(ev tcell.Event) ui.EventResult {
	if key, ok := ev.(*tcell.EventKey); ok && key.Key() == tcell.KeyEscape {
		w.escapes++
		return ui.EventConsumed
	}
	return ui.EventIgnored
}

func TestRawKeyTabDragEscapeCancelsWithoutForwarding(t *testing.T) {
	for _, forward := range []bool{false, true} {
		t.Run(map[bool]string{false: "reserved shortcuts", true: "forward shortcuts"}[forward], func(t *testing.T) {
			h := newTestHarness(t, 140, 32)
			defer h.stop()
			left := h.app.EditorGroup
			raw := &rawEscapeContent{}
			left.OpenPluginTab("raw", "Terminal", raw)
			left.CloseOtherTabs()
			h.exec("editor.split")
			right := h.app.EditorGroup
			h.app.Root.SetFocus(left)
			h.app.Root.SetForwardShortcuts(forward)
			h.pressKey(tcell.KeyEscape, 0)
			if raw.escapes != 1 {
				t.Fatal("normal Escape did not reach raw content")
			}
			src, dest := left.TabBar.GetRect(), right.Editor.GetRect()
			h.app.Root.HandleEvent(tcell.NewEventMouse(src.X+4, src.Y+1, tcell.Button1, 0))
			h.app.Root.HandleEvent(tcell.NewEventMouse(dest.X+4, dest.Y+1, tcell.Button1, 0))
			h.redraw()
			if !right.DropTarget || !h.app.Root.PointerCaptureActive() {
				t.Fatal("raw tab drag did not start")
			}
			h.pressKey(tcell.KeyEscape, 0)
			if right.DropTarget || h.app.Root.PointerCaptureActive() || raw.escapes != 1 {
				t.Fatal("Escape was forwarded instead of cancelling the raw tab drag")
			}
			h.app.Root.HandleEvent(tcell.NewEventMouse(dest.X+4, dest.Y+1, tcell.ButtonNone, 0))
			if left.TabCount() != 1 || left.TabContent(0) != raw || right.ActiveBuffer() == nil {
				t.Fatal("mouse release transferred the cancelled tab")
			}
			h.app.Root.SetFocus(left)
			h.pressKey(tcell.KeyEscape, 0)
			if raw.escapes != 2 {
				t.Fatal("normal Escape was swallowed after cancellation")
			}
		})
	}
}

func TestEditorTabDragToSplit(t *testing.T) {
	tests := []struct {
		name  string
		zone  ui.EditorDropZone
		point func(r ui.Rect) (int, int)
		check func(orig, added ui.Rect) bool
	}{
		{"left", ui.EditorDropLeft, func(r ui.Rect) (int, int) { return r.X + 2, r.Y + r.H/2 }, func(orig, added ui.Rect) bool { return added.X < orig.X }},
		{"bottom", ui.EditorDropBottom, func(r ui.Rect) (int, int) { return r.X + r.W/2, r.Y + r.H - 2 }, func(orig, added ui.Rect) bool { return added.Y > orig.Y }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t, 160, 44)
			defer h.stop()
			orig := h.app.EditorGroup
			alpha, beta := filepath.Join(h.dir, "alpha.txt"), filepath.Join(h.dir, "beta.txt")
			orig.OpenFile(alpha)
			orig.CommitActiveTab()
			orig.OpenFile(beta)
			orig.SwitchToTabByPath(alpha)
			h.redraw()

			tabs := orig.TabBar.GetRect()
			dx, dy := tc.point(orig.ContentRect())
			h.app.Root.HandleEvent(tcell.NewEventMouse(tabs.X+4, tabs.Y+1, tcell.Button1, 0))
			h.app.Root.HandleEvent(tcell.NewEventMouse(dx, dy, tcell.Button1, 0))
			h.redraw()
			if orig.DropZone != tc.zone {
				t.Fatalf("drag zone = %v, want %v", orig.DropZone, tc.zone)
			}
			h.app.Root.HandleEvent(tcell.NewEventMouse(dx, dy, tcell.ButtonNone, 0))
			h.flushOnChange()
			h.redraw()

			added := h.app.EditorGroup
			if len(h.app.EditorSplit.Groups()) != 2 || added == orig || added.ActiveFilePath() != alpha || orig.ActiveFilePath() != beta || orig.DropTarget {
				t.Fatalf("drop did not split and move tab cleanly")
			}
			if !tc.check(orig.GetRect(), added.GetRect()) {
				t.Fatalf("unexpected split placement: added=%+v orig=%+v", added.GetRect(), orig.GetRect())
			}
		})
	}

	t.Run("single tab ignores self split", func(t *testing.T) {
		h := newTestHarness(t, 140, 32)
		defer h.stop()
		group := h.app.EditorGroup
		h.redraw()
		tabs, content := group.TabBar.GetRect(), group.ContentRect()
		dx, dy := content.X+content.W-3, content.Y+content.H/2
		h.app.Root.HandleEvent(tcell.NewEventMouse(tabs.X+4, tabs.Y+1, tcell.Button1, 0))
		h.app.Root.HandleEvent(tcell.NewEventMouse(dx, dy, tcell.Button1, 0))
		h.redraw()
		if group.DropZone != ui.EditorDropNone {
			t.Fatalf("single-tab group advertised split zone %v", group.DropZone)
		}
		h.app.Root.HandleEvent(tcell.NewEventMouse(dx, dy, tcell.ButtonNone, 0))
		if len(h.app.EditorSplit.Groups()) != 1 {
			t.Fatalf("group count = %d, want 1", len(h.app.EditorSplit.Groups()))
		}
	})
}

func TestEditorTabDragAcrossGroupsToSplit(t *testing.T) {
	h := newTestHarness(t, 160, 44)
	defer h.stop()
	left := h.app.EditorGroup
	alpha, beta := filepath.Join(h.dir, "alpha.txt"), filepath.Join(h.dir, "beta.txt")
	left.OpenFile(alpha)
	h.exec("editor.split")
	right := h.app.EditorGroup
	right.OpenFile(beta)
	h.app.Root.SetFocus(left)
	h.redraw()

	tabs, dest := left.TabBar.GetRect(), right.ContentRect()
	dx, dy := dest.X+dest.W/2, dest.Y+dest.H-2
	h.app.Root.HandleEvent(tcell.NewEventMouse(tabs.X+4, tabs.Y+1, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(dx, dy, tcell.Button1, 0))
	h.redraw()
	if right.DropZone != ui.EditorDropBottom {
		t.Fatalf("right drop zone = %v, want EditorDropBottom", right.DropZone)
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(dx, dy, tcell.ButtonNone, 0))
	h.flushOnChange()
	h.redraw()

	bottom := h.app.EditorGroup
	if len(h.app.EditorSplit.Groups()) != 2 || bottom == right || bottom.ActiveFilePath() != alpha || right.ActiveFilePath() != beta || bottom.GetRect().Y <= right.GetRect().Y {
		t.Fatalf("cross-group edge drop did not split and remove empty source group")
	}
}
