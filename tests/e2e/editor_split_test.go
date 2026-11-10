package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
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
			chooseEditorMore(t, h, "Split Right")
			right := h.app.EditorGroup
			if left == right || len(h.app.EditorSplit.Groups()) < 2 || right.ActiveBuffer() != buf || right.Editor.Undo != stack {
				t.Fatal("split did not share the active document and undo history")
			}
			if left.ActiveFilePath() != beta || right.ActiveFilePath() != beta {
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
			left.SwitchToTabByPath(alpha)
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
			if len(h.app.EditorSplit.Groups()) > 1 || h.app.EditorGroup.TabCount() != 2 {
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
	layout := h.app.EditorSplit
	split := layout.Split
	x, y := split.DividerScreenX(), split.GetRect().Y+4
	assertEditorDividerSkipsTabs(t, h, x)
	tabs := layout.Groups()[0].TabBar.GetRect()
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, tabs.Y+1, tcell.Button1, 0))
	if split.OwnsPointerCapture() {
		t.Fatal("tab-bar gap captured a resize drag")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, 0))
	h.redraw()
	if split.DividerScreenX() != x {
		t.Fatal("dragging the tab-bar gap resized the split")
	}
	if shape := h.app.Root.PointerShapeAt(x, y); shape != term.PointerEWResize {
		t.Fatalf("divider did not show the resize pointer: %q", shape)
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
	assertEditorDividerSkipsTabs(t, h, split.DividerScreenX())
	h.exec("editor.closeSplit")
	h.redraw()
	if len(layout.Groups()) != 1 || layout.Group.GetRect().W != layout.GetRect().W {
		t.Fatal("closing split did not restore the full editor width")
	}
}

func assertEditorDividerSkipsTabs(t *testing.T, h *testHarness, x int) {
	t.Helper()
	cells, width, _ := h.screen.GetContents()
	for _, group := range h.app.EditorSplit.Groups() {
		tabs := group.TabBar.GetRect()
		for y := tabs.Y; y < tabs.Y+tabs.H; y++ {
			if cells[y*width+x].Str != " " {
				t.Fatalf("divider crossed tab bar at (%d, %d)", x, y)
			}
			if h.app.Root.PointerShapeAt(x, y) == term.PointerEWResize {
				t.Fatalf("tab-bar gap showed a resize pointer at (%d, %d)", x, y)
			}
		}
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
	if h.app.EditorGroup != left || left.BufferForPath(path) != right.ActiveBuffer() {
		t.Fatal("opening an existing file did not share its buffer in the focused group")
	}
	h.app.Root.SetFocus(left)
	if err := left.OpenFile(filepath.Join(h.dir, "beta.txt")); err != nil {
		t.Fatal(err)
	}
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
	if !ok {
		t.Fatalf("More menu = %T", h.app.Root.Focused)
	}
	index := -1
	for i, item := range menu.Items {
		if item.Label == label {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("More menu = %T, want first item %q", h.app.Root.Focused, label)
	}
	for menu.Selected != index {
		h.pressKey(tcell.KeyDown, 0)
	}
	h.pressKey(tcell.KeyEnter, 0)
}

func TestEditorSplitFourGroupsAndDirectionalFocus(t *testing.T) {
	h := newTestHarness(t, 180, 50)
	defer h.stop()
	leftTop := h.app.EditorGroup
	h.exec("editor.split")
	rightTop := h.app.EditorGroup
	h.exec("editor.splitDown")
	rightBottom := h.app.EditorGroup
	h.exec("focus.editorLeft")
	if h.app.EditorGroup != leftTop {
		t.Fatal("left focus did not reach the adjacent group")
	}
	h.exec("editor.splitDown")
	leftBottom := h.app.EditorGroup
	if len(h.app.EditorSplit.Groups()) != 4 {
		t.Fatal("split did not create four groups")
	}
	h.exec("editor.split")
	if len(h.app.EditorSplit.Groups()) != 4 {
		t.Fatal("group limit was not enforced")
	}
	h.exec("focus.editorUp")
	if h.app.EditorGroup != leftTop {
		t.Fatal("up focus did not reach the adjacent group")
	}
	h.exec("focus.editorRight")
	if h.app.EditorGroup != rightTop {
		t.Fatal("right focus did not reach the adjacent group")
	}
	h.exec("focus.editorDown")
	if h.app.EditorGroup != rightBottom {
		t.Fatal("down focus did not reach the adjacent group")
	}
	h.pressCtrl(tcell.KeyCtrlK)
	h.pressKey(tcell.KeyLeft, 0)
	if h.app.EditorGroup != leftBottom {
		t.Fatal("directional focus chord did not reach the left group")
	}
	if !leftBottom.TabBar.GroupActive || rightBottom.TabBar.GroupActive || rightTop.TabBar.ShowGroupFocus {
		t.Fatal("active group indication did not follow focus")
	}
	h.exec("editor.equalizeGroups")
	if delta := leftTop.GetRect().H - leftBottom.GetRect().H; delta < -1 || delta > 1 {
		t.Fatal("equalizing did not balance row heights")
	}
	cells, width, _ := h.screen.GetContents()
	crossX, crossY := leftTop.GetRect().X+leftTop.GetRect().W, leftTop.GetRect().Y+leftTop.GetRect().H
	if cells[crossY*width+crossX].Str != string(h.app.Borders.Cross) {
		t.Fatal("grid dividers did not meet at a themed intersection")
	}
	assertEditorDividerSkipsTabs(t, h, crossX)
	groupCount, activeCount := 0, 0
	var inspect func(*app.DebugWidgetNode)
	inspect = func(node *app.DebugWidgetNode) {
		if node.Type == "EditorGroup" {
			groupCount++
			if node.Props["active_group"] == true {
				activeCount++
			}
		}
		for _, child := range node.Children {
			inspect(child)
		}
	}
	inspect(h.app.BuildDebugState().WidgetTree)
	if groupCount != 4 || activeCount != 1 {
		t.Fatalf("debug reported %d groups, %d active", groupCount, activeCount)
	}

	for len(h.app.EditorSplit.Groups()) > 1 {
		h.exec("editor.closeSplit")
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
			dx, dy := dest.X+4, dest.Y+2
			if command == "editor.splitDown" {
				dx = x
			}
			h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
			h.app.Root.HandleEvent(tcell.NewEventMouse(dx, dy, tcell.Button1, 0))
			h.redraw()
			if !right.DropTarget {
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

func TestEditorSplitReloadSettingsAndSharedUndo(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	path := filepath.Join(h.dir, "alpha.txt")
	left := h.app.EditorGroup
	left.OpenFile(path)
	h.exec("editor.split")
	right := h.app.EditorGroup
	if err := os.WriteFile(path, []byte("external\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.HandleFileChanged(path)
	h.redraw()
	if right.ActiveBuffer().Lines[0] != "external" || left.Editor.Undo != right.Editor.Undo {
		t.Fatal("external reload split the shared document or undo history")
	}
	h.pressRune('x')
	h.app.Root.SetFocus(left)
	h.exec("editor.undo")
	if right.ActiveBuffer().Lines[0] != "external" || left.ActiveBuffer().Dirty {
		t.Fatal("undo in the other group did not restore the reloaded document")
	}
	s := *h.app.Settings
	s.Editor.LineNumbers = false
	s.Editor.IndentGuides = true
	h.app.ApplySettings(s)
	if left.Editor.LineNumbers || right.Editor.LineNumbers || !left.Editor.IndentGuides || !right.Editor.IndentGuides {
		t.Fatal("live settings were applied to only one group")
	}
}

func TestEditorSplitInheritsKeyboardModifierPreference(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	h.app.Root.SetControlModifiers(tcell.ModMeta)
	h.exec("editor.split")
	h.typeText("one two")
	h.pressKey(tcell.KeyLeft, tcell.ModMeta)
	if _, col := h.app.EditorGroup.ActiveCursor(); col != 4 {
		t.Fatalf("word movement in the new group ignored Super preference: col %d", col)
	}
	h.app.Root.SetControlModifiers(tcell.ModCtrl)
	h.pressKey(tcell.KeyEnd, 0)
	h.pressKey(tcell.KeyLeft, tcell.ModCtrl)
	if _, col := h.app.EditorGroup.ActiveCursor(); col != 4 {
		t.Fatalf("new group did not follow changed modifier preference: col %d", col)
	}
}

func TestEditorSplitBorderHighlightIsOptionalAndLeavesTabNamesAlone(t *testing.T) {
	h := newTestHarness(t, 160, 42)
	defer h.stop()
	left := h.app.EditorGroup
	h.exec("editor.split")
	right := h.app.EditorGroup
	check := func(group *ui.EditorGroupWidget, border term.Style) {
		t.Helper()
		cells := h.renderer.NextFrame(h.app.Root.Width, h.app.Root.Height)
		h.app.Root.Render(cells)
		r := group.TabBar.GetRect()
		if cells[r.Y][r.X].Style != border {
			t.Fatalf("tab border style = %v, want %v", cells[r.Y][r.X].Style, border)
		}
		if cells[r.Y+1][r.X+2].Style != term.StyleActiveTab {
			t.Fatal("split focus changed the tab name background")
		}
	}
	check(left, term.StyleBorder)
	check(right, term.StyleBorder)
	if !left.Editor.Passive || right.Editor.Passive {
		t.Fatal("turning border highlighting off changed active-line ownership")
	}
	s := *h.app.Settings
	s.Appearance.HighlightActiveGroupBorder = true
	h.app.ApplySettings(s)
	check(left, term.StyleBorder)
	check(right, term.StyleBorderActive)
	h.app.Root.SetFocus(left)
	check(left, term.StyleBorderActive)
	check(right, term.StyleBorder)
	h.exec("editor.splitDown")
	check(h.app.EditorGroup, term.StyleBorderActive)
	s.Appearance.HighlightActiveGroupBorder = false
	h.app.ApplySettings(s)
	for _, group := range h.app.EditorSplit.Groups() {
		check(group, term.StyleBorder)
	}
}
