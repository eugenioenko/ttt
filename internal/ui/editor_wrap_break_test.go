package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestWordWrapBreakRendersWholeWords(t *testing.T) {
	e := newRowsEditor([]string{"hello world foo", "next"}, 40, 6)
	e.WordWrap, e.WordWrapColumn, e.WrapWords = true, 8, true
	rows := renderRows(e)
	if want := []string{"hello", "world", "foo", "next"}; !slices.Equal(rows[:4], want) {
		t.Fatalf("rows = %q, want %q", rows[:4], want)
	}
	clickAt(e, e.GutterWidth()+7, 0)
	if e.Cursor.Line != 0 || e.Cursor.Col != 5 {
		t.Fatalf("click past a short row = %d/%d, want 0/5", e.Cursor.Line, e.Cursor.Col)
	}
	e.Cursor.Col = 0
	e.moveCursorRows(1)
	if e.Cursor.Col != 6 {
		t.Fatalf("Down moved to %d, want 6", e.Cursor.Col)
	}
	e.Cursor.Col = 6
	renderRows(e)
	if e.CursorX != e.GutterWidth() || e.CursorY != 1 {
		t.Fatalf("cursor = %d/%d", e.CursorX, e.CursorY)
	}

	e.WrapWords = false
	rows = renderRows(e)
	if want := []string{"hello wo", "rld foo", "next"}; !slices.Equal(rows[:3], want) {
		t.Fatalf("character rows = %q, want %q", rows[:3], want)
	}
	if total := e.layout().total(); total != 3 {
		t.Fatalf("character layout reused word-mode row counts: total = %d, want 3", total)
	}
}

func TestWordWrapBreakLanguageOverride(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "notes.md")
	goFile := filepath.Join(dir, "main.go")
	for _, p := range []string{md, goFile} {
		if err := os.WriteFile(p, []byte("hello world\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	g := NewEditorGroupWidget(nil, 4, true, "compact")
	g.WordWrap, g.WordWrapBreak = true, config.WordWrapBreakWord
	g.LanguageOverrides = map[string]config.EditorLanguageOverride{"go": {WordWrapBreak: config.WordWrapBreakCharacter}}
	g.OpenFile(md)
	if !g.Editor.WrapWords {
		t.Fatal("markdown should inherit word breaks")
	}
	g.OpenFile(goFile)
	if g.Editor.WrapWords {
		t.Fatal("go override should break at characters")
	}
	g.WordWrapBreak = config.WordWrapBreakCharacter
	g.LanguageOverrides = nil
	g.OpenFile(md)
	if g.Editor.WrapWords {
		t.Fatal("global character setting ignored")
	}
}
