package ui

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

func newEditorWithLines(lines ...string) *EditorPaneWidget {
	buf := &buffer.Buffer{Lines: slices.Clone(lines)}
	cur := &cursor.Cursor{Line: 0, Col: 0}
	vp := &view.Viewport{TopLine: 0, LeftCol: 0, Width: 40, Height: 10}
	return NewEditorPaneWidget(buf, cur, vp)
}

func TestAutoIndentEnterAfterOpenBrace(t *testing.T) {
	e := newEditorWithLines("{")
	e.Cursor.Col = 1

	e.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))

	if e.Buf.Lines[1] != "    " {
		t.Fatalf("expected new line indented 4 spaces, got %q", e.Buf.Lines[1])
	}
	if e.Cursor.Line != 1 || e.Cursor.Col != 4 {
		t.Fatalf("expected cursor at (1,4), got (%d,%d)", e.Cursor.Line, e.Cursor.Col)
	}
}

func TestAutoIndentEnterBetweenBracesSplitsClosing(t *testing.T) {
	e := newEditorWithLines("{}")
	e.Cursor.Col = 1

	e.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))

	if e.Buf.Lines[0] != "{" || e.Buf.Lines[1] != "    " || e.Buf.Lines[2] != "}" {
		t.Fatalf("expected {, 4-space line, }, got %q", e.Buf.Lines)
	}
	if e.Cursor.Line != 1 || e.Cursor.Col != 4 {
		t.Fatalf("expected cursor at (1,4), got (%d,%d)", e.Cursor.Line, e.Cursor.Col)
	}
}

func TestAutoIndentDedentOnCloseBrace(t *testing.T) {
	e := newEditorWithLines("{", "    ")
	e.Cursor.Line = 1
	e.Cursor.Col = 4

	e.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "}", 0))

	if e.Buf.Lines[1] != "}" {
		t.Fatalf("expected closing brace dedented to column 0, got %q", e.Buf.Lines[1])
	}
	if e.Cursor.Col != 1 {
		t.Fatalf("expected cursor at col 1, got %d", e.Cursor.Col)
	}
}

func TestAutoIndentDedentNestedRemovesOneLevel(t *testing.T) {
	e := newEditorWithLines("        ") // two levels of indent
	e.Cursor.Col = 8

	e.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "}", 0))

	if e.Buf.Lines[0] != "    }" {
		t.Fatalf("expected one indent level removed leaving '    }', got %q", e.Buf.Lines[0])
	}
	if e.Cursor.Col != 5 {
		t.Fatalf("expected cursor at col 5, got %d", e.Cursor.Col)
	}
}

func TestAutoIndentDedentSkipsMidLine(t *testing.T) {
	e := newEditorWithLines("    foo")
	e.Cursor.Col = 7

	e.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "}", 0))

	if e.Buf.Lines[0] != "    foo}" {
		t.Fatalf("expected no dedent for mid-line brace, got %q", e.Buf.Lines[0])
	}
}

// Open-brace indentation applies on Enter regardless of AutoDedent; the flag
// only controls the closing-bracket dedent.
func TestOpenIndentAppliesRegardlessOfAutoDedent(t *testing.T) {
	e := newEditorWithLines("{")
	e.AutoDedent = false
	e.Cursor.Col = 1

	e.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))

	if e.Buf.Lines[1] != "    " {
		t.Fatalf("expected open-brace indent even with auto-dedent off, got %q", e.Buf.Lines[1])
	}
	if e.Cursor.Line != 1 || e.Cursor.Col != 4 {
		t.Fatalf("expected cursor at (1,4), got (%d,%d)", e.Cursor.Line, e.Cursor.Col)
	}
}

func TestAutoDedentDisabledNoDedent(t *testing.T) {
	e := newEditorWithLines("    ")
	e.AutoDedent = false
	e.Cursor.Col = 4

	e.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "}", 0))

	if e.Buf.Lines[0] != "    }" {
		t.Fatalf("expected brace appended without dedent, got %q", e.Buf.Lines[0])
	}
}

func TestAutoDedentEnabledByDefault(t *testing.T) {
	e := newEditorWithLines("")
	if !e.AutoDedent {
		t.Fatal("expected auto-dedent to be enabled by default")
	}
}

// Indentation inheritance always applies on Enter, independent of AutoDedent.
func TestEnterInheritsIndentation(t *testing.T) {
	e := newEditorWithLines("    foo")
	e.AutoDedent = false
	e.Cursor.Col = 7

	e.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))

	if e.Buf.Lines[1] != "    " {
		t.Fatalf("expected new line to inherit 4-space indent, got %q", e.Buf.Lines[1])
	}
	if e.Cursor.Col != 4 {
		t.Fatalf("expected cursor at col 4, got %d", e.Cursor.Col)
	}
}

// Single-cursor and multi-cursor Enter must indent identically.
func TestEnterIndentConsistentAcrossCursorModes(t *testing.T) {
	single := newEditorWithLines("    foo")
	single.Cursor.Line, single.Cursor.Col = 0, 7
	single.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	singleIndent := single.Buf.Lines[1]

	multi := newEditorWithLines("    foo", "    bar")
	multi.Cursor.Line, multi.Cursor.Col = 0, 7
	multi.ensureMulti()
	multi.Multi.Add(1, 7)
	multi.syncFromMulti()
	if !multi.isMultiActive() {
		t.Fatal("expected multi-cursor mode to be active")
	}
	multi.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	multiIndent := multi.Buf.Lines[1]

	if singleIndent != multiIndent {
		t.Fatalf("indent diverged between cursor modes: single=%q multi=%q", singleIndent, multiIndent)
	}
	if singleIndent != "    " {
		t.Fatalf("expected both to inherit 4-space indent, got %q", singleIndent)
	}
}

func TestAutoIndentDisabledPreservesNoIndent(t *testing.T) {
	e := newEditorWithLines("    foo")
	e.AutoIndent = false
	e.Cursor.Line, e.Cursor.Col = 0, 7 // end of "    foo"

	e.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))

	if e.Buf.Lines[1] != "" {
		t.Fatalf("expected column-1 new line with AutoIndent off, got %q", e.Buf.Lines[1])
	}
	if e.Cursor.Line != 1 || e.Cursor.Col != 0 {
		t.Fatalf("expected cursor at (1,0), got (%d,%d)", e.Cursor.Line, e.Cursor.Col)
	}
}

func TestAutoIndentDisabledNoSmartIndentAfterBrace(t *testing.T) {
	e := newEditorWithLines("{")
	e.AutoIndent = false
	e.Cursor.Col = 1

	e.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))

	if e.Buf.Lines[1] != "" {
		t.Fatalf("expected no shiftwidth after opener with AutoIndent off, got %q", e.Buf.Lines[1])
	}
}

func TestAutoIndentDisabledInMultiCursor(t *testing.T) {
	e := newEditorWithLines("    foo", "    bar")
	e.AutoIndent = false
	e.Cursor.Line, e.Cursor.Col = 0, 7
	e.ensureMulti()
	e.Multi.Add(1, 7)
	e.syncFromMulti()
	if !e.isMultiActive() {
		t.Fatal("expected multi-cursor mode to be active")
	}

	e.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))

	// Both split lines should open at column 1 with AutoIndent off.
	for _, ln := range []int{1, 3} {
		if e.Buf.Lines[ln] != "" {
			t.Fatalf("expected column-1 line at %d with AutoIndent off, got %q", ln, e.Buf.Lines[ln])
		}
	}
}
