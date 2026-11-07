package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestSelectNextOccurrenceUsesCurrentSelection(t *testing.T) {
	for _, method := range []string{"keyboard", "mouse", "another tab"} {
		t.Run(method, func(t *testing.T) {
			h := newTestHarness(t, 100, 30)
			defer h.stop()

			const matches = "const alpha = 1;\nconst beta = 2;\nconst gamma = 3;"
			path := filepath.Join(h.dir, "words.txt")
			if err := os.WriteFile(path, []byte("unique\n"+matches), 0644); err != nil {
				t.Fatal(err)
			}
			h.app.EditorGroup.OpenFile(path)
			h.app.FocusEditor()
			h.redraw()
			h.pressCtrl(tcell.KeyCtrlD)

			editor := h.app.EditorGroup.Editor
			if got := editor.Selection.Text(editor.Buf.Lines, editor.Cursor.Line, editor.Cursor.Col); got != "unique" {
				t.Fatalf("initial selection = %q, want unique", got)
			}

			firstLine := 1
			switch method {
			case "another tab":
				other := filepath.Join(h.dir, "other.txt")
				if err := os.WriteFile(other, []byte(matches), 0644); err != nil {
					t.Fatal(err)
				}
				h.app.EditorGroup.OpenFile(other)
				h.redraw()
				firstLine = 0
			case "keyboard":
				h.pressKey(tcell.KeyDown, tcell.ModNone)
				h.pressKey(tcell.KeyHome, tcell.ModNone)
			}

			if method == "mouse" {
				r := editor.GetRect()
				x, y := r.X+editor.GutterWidth(), r.Y+firstLine
				h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
				h.app.Root.HandleEvent(tcell.NewEventMouse(x+5, y, tcell.Button1, tcell.ModNone))
				h.app.Root.HandleEvent(tcell.NewEventMouse(x+5, y, tcell.ButtonNone, tcell.ModNone))
				h.redraw()
			} else {
				for range 5 {
					h.pressKey(tcell.KeyRight, tcell.ModShift)
				}
			}
			if got := editor.Selection.Text(editor.Buf.Lines, editor.Cursor.Line, editor.Cursor.Col); got != "const" {
				t.Fatalf("new selection = %q, want const", got)
			}

			for _, want := range []int{2, 3, 3} {
				h.pressCtrl(tcell.KeyCtrlD)
				if got := h.app.EditorGroup.MultiCursorCount(); got != want {
					t.Fatalf("cursor count = %d, want %d", got, want)
				}
				for i, c := range editor.Multi.Cursors {
					if !c.Sel.Active || c.Line != firstLine+i || c.Col != 5 || c.Sel.Anchor.Line != c.Line || c.Sel.Anchor.Col != 0 {
						t.Fatalf("cursor %d = %+v, want selected const on line %d", i, c, firstLine+i)
					}
				}
			}
		})
	}
}

func TestSelectNextOccurrenceScrollsToNewSelection(t *testing.T) {
	h := newTestHarness(t, 60, 20)
	defer h.stop()

	lines := make([]string, 70)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	lines[0] = "needle"
	lines[40] = "needle"
	lines[50] = strings.Repeat(" ", 80) + "needle"

	path := filepath.Join(h.dir, "scroll.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.FocusEditor()
	h.redraw()

	editor := h.app.EditorGroup.Editor
	if editor.Viewport.TopLine != 0 {
		t.Fatalf("initial TopLine = %d, want 0", editor.Viewport.TopLine)
	}
	if editor.Viewport.LeftCol != 0 {
		t.Fatalf("initial LeftCol = %d, want 0", editor.Viewport.LeftCol)
	}

	// 1. First Ctrl+D: selects word under cursor (line 0: "needle")
	h.pressCtrl(tcell.KeyCtrlD)
	if editor.Viewport.TopLine != 0 {
		t.Fatalf("after first Ctrl+D TopLine = %d, want 0", editor.Viewport.TopLine)
	}
	if got := h.app.EditorGroup.MultiCursorCount(); got != 1 {
		t.Fatalf("cursor count = %d, want 1", got)
	}

	// 2. Second Ctrl+D: adds occurrence on line 40 (below viewport)
	h.pressCtrl(tcell.KeyCtrlD)
	if got := h.app.EditorGroup.MultiCursorCount(); got != 2 {
		t.Fatalf("cursor count = %d, want 2", got)
	}
	// Viewport must have scrolled down to bring line 40 into view:
	if 40 < editor.Viewport.TopLine || 40 >= editor.Viewport.TopLine+editor.Viewport.Height {
		t.Fatalf("after second Ctrl+D line 40 not visible in viewport [TopLine: %d, Height: %d]",
			editor.Viewport.TopLine, editor.Viewport.Height)
	}

	// 3. Third Ctrl+D: adds occurrence on line 50 (outside horizontal viewport)
	h.pressCtrl(tcell.KeyCtrlD)
	if got := h.app.EditorGroup.MultiCursorCount(); got != 3 {
		t.Fatalf("cursor count = %d, want 3", got)
	}
	// Viewport must have scrolled vertically to line 50 and horizontally to col 80:
	if 50 < editor.Viewport.TopLine || 50 >= editor.Viewport.TopLine+editor.Viewport.Height {
		t.Fatalf("after third Ctrl+D line 50 not visible in viewport [TopLine: %d, Height: %d]",
			editor.Viewport.TopLine, editor.Viewport.Height)
	}
	if editor.Viewport.LeftCol == 0 {
		t.Fatalf("after third Ctrl+D LeftCol is 0, want scrolled horizontally to reveal col 80")
	}
	if 80 < editor.Viewport.LeftCol || 85 >= editor.Viewport.LeftCol+editor.Viewport.Width {
		t.Fatalf("needle range [80, 85] not visible in horizontal viewport [LeftCol: %d, Width: %d]",
			editor.Viewport.LeftCol, editor.Viewport.Width)
	}

	// 4. Fourth Ctrl+D: all occurrences already selected, no new occurrence exists
	savedTopLine := editor.Viewport.TopLine
	savedLeftCol := editor.Viewport.LeftCol
	h.pressCtrl(tcell.KeyCtrlD)
	if got := h.app.EditorGroup.MultiCursorCount(); got != 3 {
		t.Fatalf("cursor count = %d, want 3", got)
	}
	if editor.Viewport.TopLine != savedTopLine || editor.Viewport.LeftCol != savedLeftCol {
		t.Fatalf("viewport changed when no additional occurrence existed: TopLine=%d, LeftCol=%d (want %d, %d)",
			editor.Viewport.TopLine, editor.Viewport.LeftCol, savedTopLine, savedLeftCol)
	}

	// 5. Typing edits all selected occurrences simultaneously
	h.pressRune('X')
	if !strings.HasPrefix(editor.Buf.Lines[0], "X") {
		t.Fatalf("line 0 not edited: %q", editor.Buf.Lines[0])
	}
	if !strings.HasPrefix(editor.Buf.Lines[40], "X") {
		t.Fatalf("line 40 not edited: %q", editor.Buf.Lines[40])
	}
	if !strings.HasSuffix(editor.Buf.Lines[50], "X") {
		t.Fatalf("line 50 not edited: %q", editor.Buf.Lines[50])
	}
}

func TestSelectNextOccurrenceScrollsWhenWrapping(t *testing.T) {
	h := newTestHarness(t, 60, 20)
	defer h.stop()

	lines := make([]string, 60)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	lines[5] = "wrapneedle"
	lines[45] = "wrapneedle"

	path := filepath.Join(h.dir, "wrap.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.FocusEditor()
	h.app.EditorGroup.GoToLine(46) // 1-based line 46 = 0-based line 45
	h.redraw()

	editor := h.app.EditorGroup.Editor
	if editor.Cursor.Line != 45 {
		t.Fatalf("cursor line = %d, want 45", editor.Cursor.Line)
	}

	// Select word on line 45
	h.pressCtrl(tcell.KeyCtrlD)
	if got := h.app.EditorGroup.MultiCursorCount(); got != 1 {
		t.Fatalf("cursor count = %d, want 1", got)
	}

	// Next occurrence wraps around to line 5 (above the viewport)
	h.pressCtrl(tcell.KeyCtrlD)
	if got := h.app.EditorGroup.MultiCursorCount(); got != 2 {
		t.Fatalf("cursor count = %d, want 2", got)
	}
	// Viewport must scroll UP to bring line 5 into view
	if 5 < editor.Viewport.TopLine || 5 >= editor.Viewport.TopLine+editor.Viewport.Height {
		t.Fatalf("line 5 not visible after wrap in viewport [TopLine: %d, Height: %d]",
			editor.Viewport.TopLine, editor.Viewport.Height)
	}
}

func TestSelectNextOccurrenceScrollsToNewSelection_WordWrap(t *testing.T) {
	h := newTestHarness(t, 80, 20)
	defer h.stop()

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	lines[0] = "target"
	// Line 40 wraps across more than a viewport's worth of visual rows before target
	lines[40] = strings.Repeat("pad ", 300) + "target"

	path := filepath.Join(h.dir, "wrap_scroll.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.FocusEditor()
	h.exec("options.toggleWordWrap")
	h.redraw()

	editor := h.app.EditorGroup.Editor
	if !editor.WordWrap {
		t.Fatal("expected WordWrap to be true")
	}

	// 1. First Ctrl+D: selects word under cursor (line 0: "target")
	h.pressCtrl(tcell.KeyCtrlD)
	if got := h.app.EditorGroup.MultiCursorCount(); got != 1 {
		t.Fatalf("cursor count = %d, want 1", got)
	}

	// 2. Second Ctrl+D: selects wrapped occurrence on line 40
	h.pressCtrl(tcell.KeyCtrlD)
	if got := h.app.EditorGroup.MultiCursorCount(); got != 2 {
		t.Fatalf("cursor count = %d, want 2", got)
	}

	// In word wrap, LeftCol must remain 0
	if editor.Viewport.LeftCol != 0 {
		t.Fatalf("LeftCol = %d, want 0 in word wrap", editor.Viewport.LeftCol)
	}

	// Line 40 must be visible in viewport
	if 40 < editor.Viewport.TopLine || 40 >= editor.Viewport.TopLine+editor.Viewport.Height {
		t.Fatalf("line 40 not visible in viewport [TopLine: %d, Height: %d]",
			editor.Viewport.TopLine, editor.Viewport.Height)
	}

	// The wrapped occurrence must be rendered on screen
	h.redraw()
	if !strings.Contains(h.screenText(), "target") {
		t.Fatalf("wrapped occurrence not rendered on screen after scroll; screen:\n%s", h.screenText())
	}
}
