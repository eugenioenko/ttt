package e2e

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/selection"
)

func TestUndoAndRedoRestoreSelectionForSurroundAndComment(t *testing.T) {
	for _, edit := range []string{"surround", "comment"} {
		for _, backward := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/backward=%t", edit, backward), func(t *testing.T) {
				h := newTestHarness(t, 110, 28)
				defer h.stop()

				const original = "first line\nsecond line"
				pasteText(h, pasteKeys(original))
				editor := h.app.EditorGroup.Editor
				anchor, caret := selection.Position{Line: 0, Col: 0}, selection.Position{Line: 1, Col: 8}
				if backward {
					anchor, caret = caret, anchor
				}
				editor.Selection.Start(anchor.Line, anchor.Col)
				editor.Cursor.Line, editor.Cursor.Col = caret.Line, caret.Col
				if edit == "surround" {
					h.pressRune('{')
				} else {
					h.exec("editor.toggleComment")
				}
				edited := editorText(h)
				if edited == original {
					t.Fatal("edit did not change the buffer")
				}
				editedSelection := *editor.Selection
				editedCaret := selection.Position{Line: editor.Cursor.Line, Col: editor.Cursor.Col}

				editor.Selection.Clear()
				h.exec("editor.undo")
				if got := editorText(h); got != original {
					t.Errorf("undo text = %q, want %q", got, original)
				}
				if got := *editor.Selection; got != (selection.Selection{Active: true, Anchor: anchor}) {
					t.Errorf("undo selection = %+v, want anchor %v", got, anchor)
				}
				if got := (selection.Position{Line: editor.Cursor.Line, Col: editor.Cursor.Col}); got != caret {
					t.Errorf("undo caret = %v, want %v", got, caret)
				}

				editor.Selection.Clear()
				h.exec("editor.redo")
				if got := editorText(h); got != edited {
					t.Errorf("redo text = %q, want %q", got, edited)
				}
				if got := *editor.Selection; got != editedSelection {
					t.Errorf("redo selection = %+v, want %+v", got, editedSelection)
				}
				if got := (selection.Position{Line: editor.Cursor.Line, Col: editor.Cursor.Col}); got != editedCaret {
					t.Errorf("redo caret = %v, want %v", got, editedCaret)
				}
			})
		}
	}
}

func TestUndoClearsRestoredSelectionForAnOlderEdit(t *testing.T) {
	h := newTestHarness(t, 110, 28)
	defer h.stop()
	pasteText(h, pasteKeys("first line\nsecond line"))
	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(1, 0)
	h.pressRune('{')

	h.exec("editor.undo")
	if !editor.Selection.Active {
		t.Fatal("undo did not restore the surround selection")
	}

	h.exec("editor.undo")
	if got := editorText(h); got != "" {
		t.Fatalf("undo of the original paste left %q", got)
	}
	if editor.Selection.Active {
		t.Error("undo of an edit without a saved selection left the previous selection active")
	}
}
