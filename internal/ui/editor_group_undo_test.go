package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/gdamore/tcell/v3"
)

func TestEditorGroupUndoRedoRevealsCursor(t *testing.T) {
	for _, tc := range []struct {
		name                string
		text                string
		line, col           int
		awayLine, awayCol   int
		wrap, collapsedFold bool
	}{
		{name: "below viewport", text: strings.Repeat("line\n", 100), line: 89},
		{name: "above viewport", text: strings.Repeat("line\n", 100), line: 1, awayLine: 89},
		{name: "right of viewport", text: strings.Repeat("界", 80), col: 60},
		{name: "left of viewport", text: strings.Repeat("界", 80), col: 1, awayCol: 60},
		{name: "wrapped row", text: strings.Repeat("界", 240), col: 180, wrap: true},
		{name: "inside collapsed fold", text: "head\n" + strings.Repeat("\tline\n", 100), line: 89, col: 1, collapsedFold: true},
		{name: "already visible", text: strings.Repeat("line\n", 100), line: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, op := range []string{"undo", "redo"} {
				t.Run(op, func(t *testing.T) {
					g := NewEditorGroupWidget(nil, 4, false, "extended")
					g.NewFile()
					t.Cleanup(g.CloseAllTabs)
					e := g.Editor
					e.Buf.SetText(tc.text)
					e.WordWrap = tc.wrap
					if tc.collapsedFold {
						e.Folds = fold.NewState()
					}
					e.SetRect(Rect{W: 30, H: 6})
					renderGrid(e)
					e.ExecCommand(&undo.InsertRuneCommand{Line: tc.line, Col: tc.col, Rune: 'X'})
					if op == "redo" {
						g.Undo()
					}
					g.GoToLineCol(tc.awayLine+1, tc.awayCol+1)
					if tc.collapsedFold {
						e.Folds.SetRanges(fold.ComputeIndentRanges(e.Buf.Lines))
						e.Folds.Collapse(0)
					}
					renderGrid(e)
					top, left := e.Viewport.TopLine, e.Viewport.LeftCol
					wantCol := tc.col
					if op == "redo" {
						g.Redo()
						wantCol++
					} else {
						g.Undo()
					}
					renderGrid(e)
					assertCursor(t, e, tc.line, wantCol)
					if !editorCursorVisible(e) {
						t.Fatalf("restored cursor is off screen: cursor=(%d,%d), viewport=%+v", e.CursorX, e.CursorY, e.Viewport)
					}
					if tc.collapsedFold && e.Folds.IsCollapsed(0) {
						t.Fatal("restored cursor is still inside a collapsed fold")
					}
					if tc.name == "already visible" && (e.Viewport.TopLine != top || e.Viewport.LeftCol != left) {
						t.Fatal("viewport moved despite the restored cursor already being visible")
					}
				})
			}
		})
	}
}

type cursorlessEditCommand struct {
	undo.EditCommand
}

func TestEditorGroupUndoRedoWithoutCursorPreservesViewport(t *testing.T) {
	for _, command := range []string{"empty history", "cursorless edit"} {
		t.Run(command, func(t *testing.T) {
			for _, op := range []string{"undo", "redo"} {
				t.Run(op, func(t *testing.T) {
					g := NewEditorGroupWidget(nil, 4, false, "extended")
					g.NewFile()
					t.Cleanup(g.CloseAllTabs)
					e := g.Editor
					e.Buf.SetText(strings.Repeat("line\n", 100))
					e.SetRect(Rect{W: 30, H: 6})
					renderGrid(e)
					if command == "cursorless edit" {
						e.ExecCommand(&cursorlessEditCommand{&undo.InsertRuneCommand{Line: 89, Col: 0, Rune: 'X'}})
						if op == "redo" {
							g.Undo()
						}
					}
					for range 10 {
						e.HandleEvent(tcell.NewEventMouse(10, 2, tcell.WheelDown, 0))
					}
					renderGrid(e)
					if editorCursorVisible(e) {
						t.Fatal("wheel did not scroll away from the cursor")
					}
					before := *e.Viewport
					if op == "redo" {
						g.Redo()
					} else {
						g.Undo()
					}
					renderGrid(e)
					assertCursor(t, e, 0, 0)
					if *e.Viewport != before {
						t.Fatalf("viewport = %+v, want unchanged %+v", *e.Viewport, before)
					}
					want := "line"
					if command == "cursorless edit" && op == "redo" {
						want = "Xline"
					}
					if e.Buf.Lines[89] != want {
						t.Fatalf("line after %s = %q, want %q", op, e.Buf.Lines[89], want)
					}
				})
			}
		})
	}
}
