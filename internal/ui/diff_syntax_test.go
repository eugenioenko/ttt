package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/term"
)

func commentDiffSides() (old, cur []string) {
	old = []string{"package main", "", "/*"}
	for i := 0; i < 8; i++ {
		old = append(old, "inside the comment")
	}
	old = append(old, "removed inside comment", "*/", "", "func main() {}")
	for _, line := range old {
		if line != "removed inside comment" {
			cur = append(cur, line)
		}
	}
	return old, cur
}

func styleOfText(t *testing.T, grid [][]term.Cell, text string) term.Style {
	t.Helper()
	for y := range grid {
		row := cellRow(grid, y)
		if i := strings.Index(row, text); i >= 0 {
			return grid[y][len([]rune(row[:i]))].Style
		}
	}
	t.Fatalf("%q not on screen", text)
	return 0
}

func TestRemovedLineKeepsMultiLineCommentState(t *testing.T) {
	old, cur := commentDiffSides()

	t.Run("read-only unified", func(t *testing.T) {
		d := NewDiffEditorWidget("main.go", diff.FileDiff{}, old, cur, false)
		d.fileDiff = diff.Parse(diff.Generate(old, cur, "main.go"))
		d.SetMode(DiffModeUnified)
		d.rebuild()
		grid := renderDiffEditor(d, 80, 20)
		if s := styleOfText(t, grid, "removed inside"); s != term.StyleSyntaxComment {
			t.Fatalf("removed line style %v, want comment", s)
		}
	})

	t.Run("read-only split", func(t *testing.T) {
		d := NewDiffEditorWidget("main.go", diff.FileDiff{}, old, cur, false)
		d.fileDiff = diff.Parse(diff.Generate(old, cur, "main.go"))
		d.SetMode(DiffModeSplit)
		d.rebuild()
		grid := renderDiffEditor(d, 100, 20)
		if s := styleOfText(t, grid, "removed inside"); s != term.StyleSyntaxComment {
			t.Fatalf("removed line style %v, want comment", s)
		}
	})

	t.Run("editable unified", func(t *testing.T) {
		e := newRowsEditor(cur, 80, 20)
		e.LineNumbers = true
		e.Undo = &undo.UndoStack{}
		d := NewEditableDiffWidget("main.go", old, diff.FullDiffLines(old, cur))
		d.SetMode(DiffModeUnified)
		d.SetContextMode(DiffContextFullFile)
		d.bind(e)
		grid := renderDiffEditor(d, 80, 20)
		if s := styleOfText(t, grid, "removed inside"); s != term.StyleSyntaxComment {
			t.Fatalf("deleted phantom row style %v, want comment", s)
		}
	})

	t.Run("editable split base pane", func(t *testing.T) {
		e := newRowsEditor(cur, 50, 20)
		e.LineNumbers = true
		e.Undo = &undo.UndoStack{}
		d := NewEditableDiffWidget("main.go", old, diff.FullDiffLines(old, cur))
		d.SetMode(DiffModeSplit)
		d.bind(e)
		grid := renderDiffEditor(d, 100, 20)
		if s := styleOfText(t, grid, "removed inside"); s != term.StyleSyntaxComment {
			t.Fatalf("base pane style %v, want comment", s)
		}
	})
}
