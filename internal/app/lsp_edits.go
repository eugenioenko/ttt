package app

import (
	"errors"
	"slices"
	"sort"

	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/lsp"
)

func (a *App) workspaceTextEdits(edit *lsp.WorkspaceEdit) ([]lsp.FileEdits, bool) {
	batches, err := edit.TextEdits()
	if errors.Is(err, lsp.ErrFileOperations) {
		a.StatusWarn(i18n.Text("status.unsupportedWorkspaceEdit"))
		return nil, false
	}
	return batches, true
}

func (a *App) ApplyWorkspaceEdit(edit *lsp.WorkspaceEdit) {
	batches, ok := a.workspaceTextEdits(edit)
	if !ok || len(batches) == 0 {
		return
	}
	filesCount, _ := a.applyWorkspaceEditBatches(batches, true)
	a.StatusNotify(i18n.Format("status.renamedAcrossFileS", i18n.Args{"filesCount": filesCount}))
}

func (a *App) applyWorkspaceEditBatches(batches []lsp.FileEdits, saveOnRename bool) (int, bool) {
	currentPath := a.EditorGroup.ActiveFilePath()

	files := make(map[string]bool)
	allApplied := true
	for _, batch := range batches {
		path := URIToPath(batch.URI)
		if err := a.EditorGroup.OpenFile(path); err != nil {
			a.StatusWarn(err.Error())
			allApplied = false
			continue
		}
		if !a.EditorGroup.IsActiveEditableFile(path) {
			a.StatusWarn(i18n.Text("status.workspaceEditNotApplied"))
			allApplied = false
			continue
		}
		files[path] = true
		a.ApplyTextEdits(batch.Edits)

		if buf := a.EditorGroup.ActiveBuffer(); saveOnRename && a.Settings.LSP.SaveOnRename && buf != nil && !buf.ReadOnlyOnDisk {
			if a.EditorGroup.Save() {
				_, lang := a.editorPathLang()
				a.afterFileSave(path, lang)
			}
		}
	}

	_ = a.EditorGroup.OpenFile(currentPath)
	return len(files), allApplied
}

func (a *App) ApplyTextEdits(edits []lsp.TextEdit) {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	a.applyRuneTextEdits(runeTextEdits(a.EditorGroup.Editor.Buf.Lines, edits), nil)
}

func sortTextEdits(edits []lsp.TextEdit) {
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].Range.Start.Line != edits[j].Range.Start.Line {
			return edits[i].Range.Start.Line > edits[j].Range.Start.Line
		}
		return edits[i].Range.Start.Character > edits[j].Range.Start.Character
	})
}

// Reverse equal-position groups so descending application preserves LSP array order.
func reverseSamePositionTextEdits(edits []lsp.TextEdit) {
	for start := 0; start < len(edits); {
		end := start + 1
		for end < len(edits) && edits[end].Range.Start == edits[start].Range.Start {
			end++
		}
		slices.Reverse(edits[start:end])
		start = end
	}
}

func (a *App) applyRuneTextEdits(edits []lsp.TextEdit, cursor *lsp.Position) {
	editor := a.EditorGroup.Editor
	sortTextEdits(edits)
	reverseSamePositionTextEdits(edits)

	// One undo step for the whole batch: a rename, a format or a code action
	// is a single change, not one per edited range (#876).
	if editor.Undo != nil {
		editor.Undo.BeginTransaction()
		defer editor.Undo.EndTransaction()
	}
	for _, edit := range edits {
		sl, sc := edit.Range.Start.Line, edit.Range.Start.Character
		el, ec := edit.Range.End.Line, edit.Range.End.Character

		if sl != el || sc != ec {
			editor.ExecCommand(&undo.DeleteSelectionCommand{
				StartLine: sl, StartCol: sc,
				EndLine: el, EndCol: ec,
			})
		}
		if edit.NewText != "" {
			suffix := ""
			if sl < len(editor.Buf.Lines) {
				runes := []rune(editor.Buf.Lines[sl])
				if sc < len(runes) {
					suffix = string(runes[sc:])
				}
			}
			editor.ExecCommand(&undo.PasteCommand{
				Line: sl, Col: sc,
				Text: edit.NewText, Suffix: suffix,
			})
		}
	}
	if cursor != nil {
		editor.Cursor.Line, editor.Cursor.Col = cursor.Line, cursor.Character
		a.EditorGroup.ScrollToCursor()
	}
	editor.FlushOnChange()
}
