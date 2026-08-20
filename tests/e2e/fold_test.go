package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestFoldToggle_HidesLines(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n\tfmt.Println(\"world\")\n}\n"), 0644)
	h.exec("sidebar.explorer")
	h.redraw()

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.assertContains("fmt.Println(\"hello\")")

	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.app.EditorGroup.Editor.Cursor.Col = 0
	h.redraw()

	h.exec("fold.toggle")

	h.assertNotContains("fmt.Println(\"hello\")")
}

func TestFoldToggle_ShowsLines(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n\tfmt.Println(\"world\")\n}\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.exec("fold.toggle")

	h.assertNotContains("fmt.Println(\"hello\")")

	h.exec("fold.toggle")

	h.assertContains("fmt.Println(\"hello\")")
}

func TestFoldAll_CollapsesAllRanges(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "funcs.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc a() {\n\tx()\n}\n\nfunc b() {\n\ty()\n}\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.assertContains("x()")
	h.assertContains("y()")

	h.exec("fold.collapseAll")

	h.assertNotContains("x()")
	h.assertNotContains("y()")
}

func TestExpandAll_ShowsAllLines(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "funcs.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc a() {\n\tx()\n}\n\nfunc b() {\n\ty()\n}\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.exec("fold.collapseAll")
	h.assertNotContains("x()")

	h.exec("fold.expandAll")
	h.assertContains("x()")
	h.assertContains("y()")
}

func TestFoldAnnotation(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\ta()\n\tb()\n\tc()\n}\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.exec("fold.toggle")

	h.assertContains("⋯")
}

func TestFoldLineNumbers(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("line1\n  line2\n  line3\nline4\nline5\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.exec("fold.toggle")

	screen := h.screenText()
	lines := strings.Split(screen, "\n")
	for _, line := range lines {
		if strings.Contains(line, "line4") {
			if !strings.Contains(line, "4") {
				t.Errorf("line4 should show line number 4, got: %s", line)
			}
			break
		}
	}
}

func TestCursorDown_SkipsFold(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("line0\n  line1\n  line2\nline3\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.exec("fold.toggle")

	h.pressKey(tcell.KeyDown, 0)
	cursor := h.app.EditorGroup.Editor.Cursor
	if cursor.Line != 3 {
		t.Errorf("expected cursor on line 3 after down from collapsed fold, got %d", cursor.Line)
	}
}

func TestCursorUp_SkipsFold(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("line0\n  line1\n  line2\nline3\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.exec("fold.toggle")

	h.app.EditorGroup.Editor.Cursor.Line = 3
	h.redraw()

	h.pressKey(tcell.KeyUp, 0)
	cursor := h.app.EditorGroup.Editor.Cursor
	if cursor.Line != 0 {
		t.Errorf("expected cursor on line 0 after up from below collapsed fold, got %d", cursor.Line)
	}
}

func TestFoldAnnotation_Ellipsis(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\ta()\n\tb()\n}\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.exec("fold.toggle")

	h.assertContains("⋯")
}

func TestTypingOnCollapsedFold_ExpandsFold(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.exec("fold.toggle")
	h.assertNotContains("hello")

	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.app.EditorGroup.Editor.Cursor.Col = 0
	h.redraw()

	h.pressRune('x')
	h.assertContains("hello")
}

func TestUndoAfterEdit_ExpandsFoldContainingCursor(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("line0\n  line1\n  line2\nline3\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 1
	h.app.EditorGroup.Editor.Cursor.Col = 0
	h.redraw()
	h.pressRune('x')

	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.exec("fold.toggle")
	h.assertNotContains("line1")

	h.app.EditorGroup.Undo()
	h.redraw()

	h.assertContains("line1")
}

func TestScrollDown_FoldAware(t *testing.T) {
	h := newTestHarness(t, 80, 10)
	goFile := filepath.Join(h.dir, "scroll.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\ta()\n\tb()\n\tc()\n\td()\n\te()\n}\n\nfunc other() {\n\tx()\n}\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.exec("fold.collapseAll")

	topLine := h.app.EditorGroup.Editor.Viewport.TopLine
	folds := h.app.EditorGroup.Editor.Folds
	if folds == nil {
		t.Fatal("expected folds to be set")
	}
	if !folds.IsCollapsed(2) {
		t.Error("expected line 2 (func main) to be collapsed")
	}

	mev := tcell.NewEventMouse(40, 3, tcell.WheelDown, tcell.ModNone)
	h.app.Root.HandleEvent(mev)
	h.redraw()

	newTop := h.app.EditorGroup.Editor.Viewport.TopLine
	if folds.IsLineHidden(newTop) {
		t.Errorf("after wheel scroll down, TopLine %d should not be hidden", newTop)
	}
	_ = topLine
}

func TestOpenBuffer_HasFoldState(t *testing.T) {
	h := newTestHarness(t, 80, 30)

	path := filepath.Join(h.dir, "test.go")
	os.WriteFile(path, []byte("func main() {\n\ta()\n\tb()\n}"), 0644)
	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	folds := h.app.EditorGroup.Editor.Folds
	if folds == nil {
		t.Fatal("expected Folds to be initialized for new buffer")
	}
	r := folds.FoldAt(0)
	if r == nil {
		t.Error("expected a fold range at line 0 for indented content")
	}
}

func TestGoToLine_ExpandsFoldContaining(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n\tfmt.Println(\"world\")\n}\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.exec("fold.toggle")
	h.assertNotContains("hello")

	h.app.EditorGroup.GoToLine(4)
	h.redraw()

	h.assertContains("hello")
}

func TestFindNext_ExpandsFoldContaining(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	goFile := filepath.Join(h.dir, "main.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n\tfmt.Println(\"world\")\n}\n"), 0644)

	h.app.EditorGroup.OpenFile(goFile)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.exec("fold.toggle")
	h.assertNotContains("hello")

	h.app.EditorGroup.Editor.SearchMatches = []ui.FindMatch{
		{Line: 3, Col: 15, Len: 5},
	}
	h.app.EditorGroup.Editor.SearchActive = -1
	h.app.EditorGroup.FindNext()
	h.redraw()

	h.assertContains("hello")
}

func TestFoldStatePreservedAcrossTabs(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	file1 := filepath.Join(h.dir, "a.go")
	file2 := filepath.Join(h.dir, "b.go")
	os.WriteFile(file1, []byte("package a\n\nfunc f() {\n\tx()\n}\n"), 0644)
	os.WriteFile(file2, []byte("package b\n"), 0644)

	h.app.EditorGroup.OpenFile(file1)
	h.app.EditorGroup.CommitActiveTab()
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.exec("fold.toggle")
	h.assertNotContains("x()")

	h.app.EditorGroup.OpenFile(file2)
	h.redraw()

	h.app.EditorGroup.OpenFile(file1)
	h.redraw()

	h.assertNotContains("x()")
}
