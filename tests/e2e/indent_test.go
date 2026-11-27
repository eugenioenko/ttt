package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestTabIndentDetection(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "tabs.go")
	requireNoError(t, os.WriteFile(f, []byte("func main() {\n\tfmt.Println()\n\tif true {\n\t\treturn\n\t}\n}\n"), 0644))

	h.openFile(f)
	h.redraw()

	if !h.app.EditorGroup.Editor.UseTabs {
		t.Error("expected UseTabs=true for tab-indented file")
	}
}

func TestSpaceIndentDetection(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "spaces.py")
	requireNoError(t, os.WriteFile(f, []byte("def main():\n  print('hello')\n  if True:\n    return\n"), 0644))

	h.openFile(f)
	h.redraw()

	if h.app.EditorGroup.Editor.UseTabs {
		t.Error("expected UseTabs=false for space-indented file")
	}
	if h.app.EditorGroup.Editor.TabSize != 2 {
		t.Errorf("expected TabSize=2, got %d", h.app.EditorGroup.Editor.TabSize)
	}
}

func TestTabKeyInsertsTabChar(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "tabkey.go")
	requireNoError(t, os.WriteFile(f, []byte("func main() {\n\tfmt.Println()\n}\n"), 0644))

	h.openFile(f)
	h.redraw()

	// Go to end of first line
	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.app.EditorGroup.Editor.Cursor.Col = len([]rune(h.app.EditorGroup.Editor.Buf.Lines[0]))
	h.redraw()

	// Press Enter to create new line, then Tab
	h.pressKey(tcell.KeyEnter, 0)
	h.redraw()
	h.pressKey(tcell.KeyTab, 0)
	h.redraw()

	line := h.app.EditorGroup.Editor.Buf.Lines[h.app.EditorGroup.Editor.Cursor.Line]
	if len(line) == 0 || line[0] != '\t' {
		t.Errorf("expected tab character inserted, got %q", line)
	}
}

func TestTabKeyInsertsSpaces(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "spacetab.py")
	requireNoError(t, os.WriteFile(f, []byte("def main():\n    pass\n"), 0644))

	h.openFile(f)
	h.redraw()

	// Go to end of first line
	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.app.EditorGroup.Editor.Cursor.Col = len([]rune(h.app.EditorGroup.Editor.Buf.Lines[0]))
	h.redraw()

	h.pressKey(tcell.KeyEnter, 0)
	h.redraw()
	h.pressKey(tcell.KeyTab, 0)
	h.redraw()

	line := h.app.EditorGroup.Editor.Buf.Lines[h.app.EditorGroup.Editor.Cursor.Line]
	for _, ch := range line {
		if ch == '\t' {
			t.Errorf("expected spaces only, got tab in %q", line)
			break
		}
	}
}

func TestEnterPreservesTabIndent(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "enter_tab.go")
	requireNoError(t, os.WriteFile(f, []byte("func main() {\n\tif true {\n\t}\n}\n"), 0644))

	h.openFile(f)
	h.redraw()

	// Move to end of line 1 ("\tif true {") - after the opening brace
	h.app.EditorGroup.Editor.Cursor.Line = 1
	h.app.EditorGroup.Editor.Cursor.Col = len([]rune(h.app.EditorGroup.Editor.Buf.Lines[1]))
	h.redraw()

	h.pressKey(tcell.KeyEnter, 0)
	h.redraw()

	// New line should have two tabs (existing \t indent + extra indent for {)
	newLine := h.app.EditorGroup.Editor.Buf.Lines[2]
	if newLine != "\t\t" {
		t.Errorf("expected extra tab indent '\\t\\t', got %q", newLine)
	}
}

func TestIndentPickerSetsTabs(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.app.EditorGroup.SetUseTabs(true)
	h.redraw()

	if !h.app.EditorGroup.Editor.UseTabs {
		t.Error("expected UseTabs=true after SetUseTabs(true)")
	}

	h.app.EditorGroup.SetUseTabs(false)
	h.redraw()

	if h.app.EditorGroup.Editor.UseTabs {
		t.Error("expected UseTabs=false after SetUseTabs(false)")
	}
}

func TestIndentPickerChangesTabSize(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "size.go")
	requireNoError(t, os.WriteFile(f, []byte("package main\n\nfunc main() {\n    fmt.Println()\n}\n"), 0644))
	h.openFile(f)
	h.redraw()

	if h.app.EditorGroup.Editor.TabSize != 4 {
		t.Fatalf("expected initial TabSize=4, got %d", h.app.EditorGroup.Editor.TabSize)
	}

	// Simulate picking "Spaces: 2" from the indent picker
	h.app.EditorGroup.SetTabSize(2)
	h.app.EditorGroup.SetUseTabs(false)
	h.redraw()

	if h.app.EditorGroup.Editor.TabSize != 2 {
		t.Errorf("expected TabSize=2 after SetTabSize(2), got %d", h.app.EditorGroup.Editor.TabSize)
	}

	// Verify it survives a render cycle (syncTabs doesn't overwrite)
	h.redraw()
	if h.app.EditorGroup.Editor.TabSize != 2 {
		t.Errorf("expected TabSize=2 after redraw, got %d", h.app.EditorGroup.Editor.TabSize)
	}
}
