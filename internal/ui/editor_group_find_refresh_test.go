package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/undo"
)

func openFooLines(t *testing.T) *EditorGroupWidget {
	t.Helper()
	path := filepath.Join(t.TempDir(), "foo.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nfoo\nfour\nfive\nfoo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.OpenFile(path)
	matches, err := FindInLines(g.Editor.Buf.Lines, "foo", SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g.SetFindSearch("foo", matches, SearchOptions{})
	g.SetSearchActive(0)
	g.ClearFindHighlights()
	return g
}

func insertTwoLinesAtStart(g *EditorGroupWidget) {
	g.Editor.Cursor.Line = 0
	g.Editor.Cursor.Col = 0
	g.Editor.exec(&undo.InsertLineCommand{Idx: 0, Text: "inserted"})
	g.Editor.exec(&undo.InsertLineCommand{Idx: 0, Text: "inserted"})
}

func TestFindNextAfterEditWithFindClosedUsesCurrentMatch(t *testing.T) {
	g := openFooLines(t)
	insertTwoLinesAtStart(g)
	g.FindNext()
	line := g.Editor.Buf.Lines[g.Editor.Cursor.Line]
	if !strings.Contains(line, "foo") {
		t.Fatalf("cursor line %d %q, matches %+v", g.Editor.Cursor.Line, line, g.Editor.SearchMatches)
	}
}

func TestFindNextAfterRestoreThenEditUsesCurrentMatch(t *testing.T) {
	g := openFooLines(t)
	g.FindNext()
	insertTwoLinesAtStart(g)
	g.FindNext()
	line := g.Editor.Buf.Lines[g.Editor.Cursor.Line]
	if !strings.Contains(line, "foo") {
		t.Fatalf("cursor line %d %q, matches %+v", g.Editor.Cursor.Line, line, g.Editor.SearchMatches)
	}
}

func TestFindPrevAfterRestoreThenEditUsesCurrentMatch(t *testing.T) {
	g := openFooLines(t)
	g.FindNext()
	insertTwoLinesAtStart(g)
	g.FindPrev()
	line := g.Editor.Buf.Lines[g.Editor.Cursor.Line]
	if !strings.Contains(line, "foo") {
		t.Fatalf("cursor line %d %q, matches %+v", g.Editor.Cursor.Line, line, g.Editor.SearchMatches)
	}
}
