package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func TestWordWrapColumnLayout(t *testing.T) {
	for _, tc := range []struct {
		width, cap, want int
		wrap             bool
	}{
		{120, 0, 120, true}, {120, 65, 65, true}, {50, 65, 50, true},
		{120, 80, 80, true}, {120, 65, 120, false},
		{120, 1, 1, true},
	} {
		e := newRowsEditor([]string{strings.Repeat("x", 250)}, tc.width, 8)
		e.Embedded, e.NoWrapMargin, e.GutterStyle = true, true, "minimal"
		e.WordWrap, e.WordWrapColumn = tc.wrap, tc.cap
		renderRows(e)
		if e.Viewport.Width != tc.want {
			t.Errorf("%+v: width = %d", tc, e.Viewport.Width)
		}
		before := e.layout()
		e.SetRect(Rect{W: 40, H: 8})
		renderRows(e)
		wantWidth := 40
		if tc.wrap && tc.cap > 0 {
			wantWidth = min(wantWidth, tc.cap)
		}
		if e.Viewport.Width != wantWidth || (tc.wrap && tc.want != wantWidth && e.layout() == before) {
			t.Fatalf("resize reused old width/layout: %+v", tc)
		}
		if e.Buf.Dirty || e.Buf.Lines[0] != strings.Repeat("x", 250) {
			t.Fatal("soft wrap changed buffer")
		}
	}
	e := newRowsEditor([]string{strings.Repeat("x", 200)}, 80, 3)
	e.WordWrap, e.WordWrapColumn, e.LineNumbers = true, 5, true
	renderRows(e)
	if e.Viewport.Width != 5 || e.scrollbar.TotalItems != 42 {
		t.Fatalf("capped rows/scrollbar: width=%d items=%d", e.Viewport.Width, e.scrollbar.TotalItems)
	}
	e.WordWrapColumn = 0
	renderRows(e)
	if e.Viewport.Width != 80-e.GutterWidth()-2 {
		t.Fatalf("gutter/scrollbar/margin: %d", e.Viewport.Width)
	}
}

func TestWordWrapColumnPointerAndMovement(t *testing.T) {
	e := newRowsEditor([]string{"abcdefghij"}, 40, 6)
	e.WordWrap, e.WordWrapColumn = true, 4
	renderRows(e)
	e.moveCursorWrapped(1)
	if e.Cursor.Col != 4 {
		t.Fatalf("Down moved to %d", e.Cursor.Col)
	}
	clickAt(e, e.GutterWidth()+20, 0)
	if e.Cursor.Col != 3 {
		t.Fatalf("blank click escaped visual row: %d", e.Cursor.Col)
	}
	if _, _, ok := e.TextPosAt(e.GutterWidth()+20, 0); ok {
		t.Fatal("blank space hit text")
	}
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+1, 0, tcell.Button1, 0))
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+20, 1, tcell.Button1, 0))
	e.HandleEvent(tcell.NewEventMouse(e.GutterWidth()+20, 1, tcell.ButtonNone, 0))
	if got := e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col); got != "bcdefg" {
		t.Fatalf("drag selected %q", got)
	}
	clickAt(e, e.GutterWidth()+20, 2)
	if e.Cursor.Col != 10 {
		t.Fatalf("final row did not clamp to EOL: %d", e.Cursor.Col)
	}
	for _, text := range []string{"中中中中", "a\tbc\tdef", "🙂ab中"} {
		e := newRowsEditor([]string{text}, 40, 12)
		e.WordWrap, e.WordWrapColumn, e.TabSize = true, 4, 4
		renderRows(e)
		l := e.layout()
		for i, start := range l.segments(0) {
			if line, col := e.mouseToPos(e.GetRect(), e.GutterWidth(), i); line != 0 || col != start {
				t.Fatalf("%q row %d: %d/%d want 0/%d", text, i, line, col, start)
			}
			row, x := l.rowOf(0, start)
			if row != i || x != 0 {
				t.Fatalf("%q cursor maps to %d/%d", text, row, x)
			}
		}
	}
}

func TestWordWrapColumnFilePolicies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.md")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 100)), 0644); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, true, "compact")
	enabled := true
	column := 65
	g.LanguageOverrides = map[string]config.EditorLanguageOverride{"markdown": {WordWrap: &enabled, WordWrapColumn: &column}}
	g.OpenFile(path)
	if !g.Editor.WordWrap || g.Editor.WordWrapColumn != 65 {
		t.Fatal("language override requires global wrap or syntax highlighting")
	}
	g.WordWrapMaxBytes = 10
	g.ApplyFileWordWrap()
	if g.Editor.WordWrap {
		t.Fatal("language override bypassed large-file limit")
	}
	g.WordWrapMaxBytes = 0
	g.ApplyFileWordWrap()
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		g.DiffMode, g.DiffWordWrap = mode, true
		if !g.EnableInlineDiff(path, []string{"base"}) {
			t.Fatal("enable diff failed")
		}
		if !g.Editor.WordWrap || g.Editor.WordWrapColumn != 0 {
			t.Fatal("diff inherited file wrap cap")
		}
		g.DisableInlineDiff(path)
		if !g.Editor.WordWrap || g.Editor.WordWrapColumn != 65 {
			t.Fatal("file cap not restored")
		}
	}
	newPath := filepath.Join(filepath.Dir(path), "file.ts")
	if !g.SaveAs(newPath) {
		t.Fatal("Save As failed")
	}
	if g.Editor.WordWrap || g.Editor.WordWrapColumn != 0 {
		t.Fatal("Save As retained old language settings")
	}
}

func TestWordWrapColumnTabStops(t *testing.T) {
	e := newRowsEditor([]string{"abcde\tz"}, 40, 8)
	e.WordWrap, e.WordWrapColumn, e.TabSize = true, 5, 4
	rows := renderRows(e)
	if rows[0] != "abcde" || rows[1] != "    z" {
		t.Fatalf("tab at capped wrap boundary: %q", rows)
	}
	e.Cursor.Col = 6
	renderRows(e)
	if e.CursorX != e.GutterWidth()+4 || e.CursorY != 1 {
		t.Fatalf("tab cursor = %d/%d", e.CursorX, e.CursorY)
	}
	if _, col, ok := e.TextPosAt(e.GutterWidth()+4, 1); !ok || col != 6 {
		t.Fatalf("tab hit = %d/%v", col, ok)
	}
}

func TestWordWrapColumnReloadKeepsVisibleCursor(t *testing.T) {
	e := newRowsEditor([]string{strings.Repeat("x", 300)}, 120, 5)
	e.WordWrap, e.WordWrapColumn, e.Cursor.Col = true, 80, 200
	renderRows(e)
	if e.CursorY < 0 || e.CursorY >= 5 {
		t.Fatal("initial cursor is not visible")
	}
	e.WordWrapColumn = 20
	renderRows(e)
	if e.CursorY < 0 || e.CursorY >= 5 {
		t.Fatalf("narrower wrap hid cursor at row %d", e.CursorY)
	}
	e.SetRect(Rect{W: 15, H: 5})
	renderRows(e)
	if e.CursorY < 0 || e.CursorY >= 5 {
		t.Fatalf("resize hid cursor at row %d", e.CursorY)
	}
	e.Cursor.Col = 0
	e.scrollDown(3)
	oldTop := e.topRow(e.layout())
	e.WordWrapColumn = 3
	renderRows(e)
	if e.topRow(e.layout()) < oldTop {
		t.Fatal("reflow revealed an intentionally scrolled-away cursor")
	}
}
