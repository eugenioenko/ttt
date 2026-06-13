package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) editorPathLang() (string, string) {
	path := a.EditorGroup.ActiveFilePath()
	lang := ""
	if a.EditorGroup.Editor != nil && a.EditorGroup.Editor.Highlighter != nil {
		lang = a.EditorGroup.Editor.Highlighter.Language()
	}
	return path, lang
}

func (a *App) withEditorLSP(action func(path, lang string, line, col int)) {
	path, lang := a.editorPathLang()
	line, col := a.EditorGroup.ActiveCursor()
	action(path, lang, line, col)
}

func (a *App) TriggerAutocomplete() {
	if !a.Settings.Autocomplete.Enabled {
		return
	}
	if a.IsAutocompleteActive() {
		a.DismissAutocomplete()
		return
	}
	path, lang := a.editorPathLang()
	if lang == "" {
		a.StatusWarn("No language detected for this file")
	} else if _, _, ok := a.lspResolve(path, lang); !ok {
		a.StatusWarn(lang + " language server is not configured. Add it to settings.json under lsp.servers")
	} else {
		line, col := a.EditorGroup.ActiveCursor()
		a.RequestCompletions(path, lang, line, col)
	}
}

func (a *App) RenameSymbol() {
	if a.EditorGroup.Editor == nil {
		return
	}
	path, lang := a.editorPathLang()
	line, col := a.EditorGroup.ActiveCursor()
	word := a.wordAtCursor()
	a.ShowInputDialog("Rename", "New name", word, func(newName string) {
		if newName != "" && newName != word {
			a.RequestRename(path, lang, line, col, newName)
		}
	})
}

func (a *App) FormatSelection() {
	if a.EditorGroup.Editor == nil {
		return
	}
	path, lang := a.editorPathLang()
	sel := a.EditorGroup.Editor.Selection
	if sel == nil || !sel.Active {
		a.RequestFormatting(path, lang)
		return
	}
	start, end := sel.Range(a.EditorGroup.Editor.Cursor.Line, a.EditorGroup.Editor.Cursor.Col)
	a.RequestRangeFormatting(path, lang, start.Line, start.Col, end.Line, end.Col)
}

func (a *App) CloseTab() {
	if !a.EditorGroup.IsDirty() {
		a.EditorGroup.CloseTab()
		return
	}
	name := a.EditorGroup.ActiveFileName()
	dialog := ui.NewConfirmDialogWidget3(
		"Save changes to "+name+"?",
		"Discard", "Cancel", "Save",
	)
	dialog.Borders = a.Borders
	dialog.OnButton[0] = func() {
		a.DismissDialog()
		a.EditorGroup.CloseTab()
	}
	dialog.OnButton[1] = func() {
		a.DismissDialog()
	}
	dialog.OnButton[2] = func() {
		a.DismissDialog()
		a.Reg.Execute("file.save")
		a.EditorGroup.CloseTab()
	}
	dialog.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(dialog)
}

func (a *App) NewFile() {
	a.EditorGroup.OpenBuffer("untitled", &buffer.Buffer{Lines: []string{""}})
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) SaveFileAs() {
	current := a.EditorGroup.ActiveFilePath()
	initial := ""
	if current != "untitled" {
		initial = current
	}
	a.ShowInputDialog("Save As", "Filename", initial, func(path string) {
		if path != "" {
			a.EditorGroup.SaveAs(path)
		}
	})
}

func (a *App) SaveFile() {
	path := a.EditorGroup.ActiveFilePath()
	buf := a.EditorGroup.ActiveBuffer()
	if buf != nil && path != "untitled" && buf.DiskChanged(path) {
		a.ShowConfirmDialog(
			fmt.Sprintf("%s was modified on disk. Overwrite with your version?", filepath.Base(path)),
			[]string{"Overwrite", "Cancel"},
			[]func(){
				func() { a.DismissDialog(); a.doSaveFile() },
				func() { a.DismissDialog() },
			},
		)
		return
	}
	a.doSaveFile()
}

func (a *App) doSaveFile() {
	path, lang := a.editorPathLang()
	if lang != "" {
		a.RunCodeActionsOnSave(path, lang)
		if a.Settings.Editor.FormatOnSave {
			a.FormatOnSave(path, lang)
		}
	}
	if !a.EditorGroup.Save() {
		a.SaveFileAs()
		return
	}
	path, lang = a.editorPathLang()
	if lang != "" {
		text := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n")
		a.NotifyLSPSave(path, lang, text)
	}
}

func (a *App) forceQuit() {
	for _, tt := range a.Terminals {
		tt.Term.Close()
	}
	*a.Running = false
}

func (a *App) Quit() {
	if a.quitPending || !a.EditorGroup.AnyDirty() {
		a.forceQuit()
		return
	}
	a.quitPending = true
	a.ShowConfirmDialog("Unsaved changes! Press Ctrl+Q to force quit", []string{"Cancel", "Quit"}, []func(){
		func() { a.quitPending = false; a.DismissDialog() },
		func() { a.forceQuit() },
	})
}

func registerEditorCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "editor.focus", Title: "Focus Editor",
		Handler: app.FocusEditor,
	})

	reg.Register(command.Command{
		ID: "editor.autocomplete", Title: "Trigger Autocomplete",
		Handler: app.TriggerAutocomplete,
	})

	reg.Register(command.Command{
		ID: "editor.hover", Title: "Show Hover",
		Handler: func() {
			path, lang := app.editorPathLang()
			line, col := app.EditorGroup.ActiveCursor()
			ax, ay, _ := app.EditorGroup.CursorPosition()
			app.RequestHover(path, lang, line, col, ax, ay)
		},
	})

	reg.Register(command.Command{
		ID: "editor.goToDefinition", Title: "Go to Definition",
		Handler: func() { app.withEditorLSP(app.RequestDefinition) },
	})

	reg.Register(command.Command{
		ID: "editor.goToImplementation", Title: "Go to Implementation",
		Handler: func() { app.withEditorLSP(app.RequestImplementation) },
	})

	reg.Register(command.Command{
		ID: "editor.goToTypeDefinition", Title: "Go to Type Definition",
		Handler: func() { app.withEditorLSP(app.RequestTypeDefinition) },
	})

	reg.Register(command.Command{
		ID: "editor.findReferences", Title: "Find All References",
		Handler: func() { app.withEditorLSP(app.RequestReferences) },
	})

	reg.Register(command.Command{
		ID: "editor.rename", Title: "Rename Symbol",
		Handler: app.RenameSymbol,
	})

	reg.Register(command.Command{
		ID: "editor.organizeImports", Title: "Source Action: Organize Imports",
		Handler: func() {
			path, lang := app.editorPathLang()
			app.RequestCodeAction(path, lang, "source.organizeImports")
		},
	})

	reg.Register(command.Command{
		ID: "editor.fixAll", Title: "Source Action: Fix All",
		Handler: func() {
			path, lang := app.editorPathLang()
			app.RequestCodeAction(path, lang, "source.fixAll")
		},
	})

	reg.Register(command.Command{
		ID: "editor.formatDocument", Title: "Source Action: Format Document",
		Handler: func() {
			path, lang := app.editorPathLang()
			app.RequestFormatting(path, lang)
		},
	})

	reg.Register(command.Command{
		ID: "editor.formatSelection", Title: "Source Action: Format Selection",
		Handler: app.FormatSelection,
	})

	reg.Register(command.Command{
		ID: "tab.next", Title: "Next Tab",
		Handler: func() { app.EditorGroup.NextTab() },
	})

	reg.Register(command.Command{
		ID: "tab.prev", Title: "Previous Tab",
		Handler: func() { app.EditorGroup.PrevTab() },
	})

	reg.Register(command.Command{
		ID: "tab.close", Title: "Close Tab",
		Handler: app.CloseTab,
	})

	reg.Register(command.Command{
		ID: "tab.closeOthers", Title: "Close Other Tabs",
		Handler: func() { app.EditorGroup.CloseOtherTabs() },
	})

	reg.Register(command.Command{
		ID: "tab.closeAll", Title: "Close All Tabs",
		Handler: func() { app.EditorGroup.CloseAllTabs() },
	})

	reg.Register(command.Command{
		ID: "diff.extendedView", Title: "Git: Extended Diff",
		Handler: func() {
			if dv := app.EditorGroup.ActiveDiffWidget(); dv != nil {
				dv.SetExtended(true)
			}
		},
	})

	reg.Register(command.Command{
		ID: "diff.compactView", Title: "Git: Compact Diff",
		Handler: func() {
			if dv := app.EditorGroup.ActiveDiffWidget(); dv != nil {
				dv.SetExtended(false)
			}
		},
	})

	reg.Register(command.Command{
		ID: "fold.toggle", Title: "Toggle Fold",
		Handler: func() {
			if !app.EditorGroup.IsEditorActive() {
				return
			}
			e := app.EditorGroup.Editor
			if e.Folds != nil {
				e.Folds.Toggle(e.Cursor.Line)
			}
		},
	})

	reg.Register(command.Command{
		ID: "fold.collapseAll", Title: "Fold All",
		Handler: func() {
			if !app.EditorGroup.IsEditorActive() {
				return
			}
			e := app.EditorGroup.Editor
			if e.Folds != nil {
				e.Folds.CollapseAll()
				e.EnsureCursorVisible()
			}
		},
	})

	reg.Register(command.Command{
		ID: "fold.expandAll", Title: "Unfold All",
		Handler: func() {
			if !app.EditorGroup.IsEditorActive() {
				return
			}
			e := app.EditorGroup.Editor
			if e.Folds != nil {
				e.Folds.ExpandAll()
			}
		},
	})

	reg.Register(command.Command{
		ID: "file.new", Title: "New File",
		Handler: app.NewFile,
	})

	reg.Register(command.Command{
		ID: "file.save", Title: "Save File",
		Handler: app.SaveFile,
	})

	reg.Register(command.Command{
		ID: "file.saveAs", Title: "Save As...",
		Handler: app.SaveFileAs,
	})

	reg.Register(command.Command{
		ID: "editor.undo", Title: "Undo",
		Handler: func() { app.EditorGroup.Undo() },
	})

	reg.Register(command.Command{
		ID: "editor.redo", Title: "Redo",
		Handler: func() { app.EditorGroup.Redo() },
	})

	reg.Register(command.Command{
		ID: "editor.selectAll", Title: "Select All",
		Handler: func() { app.EditorGroup.SelectAll() },
	})

	reg.Register(command.Command{
		ID: "editor.copy", Title: "Copy",
		Handler: app.Copy,
	})

	reg.Register(command.Command{
		ID: "editor.cut", Title: "Cut",
		Handler: app.Cut,
	})

	reg.Register(command.Command{
		ID: "editor.paste", Title: "Paste",
		Handler: app.Paste,
	})

	reg.Register(command.Command{
		ID: "editor.moveLineUp", Title: "Move Line Up",
		Handler: func() { app.EditorGroup.MoveLineUp() },
	})
	reg.Register(command.Command{
		ID: "editor.moveLineDown", Title: "Move Line Down",
		Handler: func() { app.EditorGroup.MoveLineDown() },
	})
	reg.Register(command.Command{
		ID: "editor.duplicateLine", Title: "Duplicate Line",
		Handler: func() { app.EditorGroup.DuplicateLine() },
	})
	reg.Register(command.Command{
		ID: "editor.deleteLine", Title: "Delete Line",
		Handler: func() { app.EditorGroup.DeleteLine() },
	})
	reg.Register(command.Command{
		ID: "editor.joinLines", Title: "Join Lines",
		Handler: func() { app.EditorGroup.JoinLines() },
	})
	reg.Register(command.Command{
		ID: "editor.insertLineBelow", Title: "Insert Line Below",
		Handler: func() { app.EditorGroup.InsertLineBelow() },
	})
	reg.Register(command.Command{
		ID: "editor.insertLineAbove", Title: "Insert Line Above",
		Handler: func() { app.EditorGroup.InsertLineAbove() },
	})
	reg.Register(command.Command{
		ID: "editor.toggleComment", Title: "Toggle Line Comment",
		Handler: func() { app.EditorGroup.ToggleLineComment() },
	})
	reg.Register(command.Command{
		ID: "editor.moveWordLeft", Title: "Move Word Left",
		Handler: func() { app.EditorGroup.MoveWordLeft(false) },
	})
	reg.Register(command.Command{
		ID: "editor.moveWordRight", Title: "Move Word Right",
		Handler: func() { app.EditorGroup.MoveWordRight(false) },
	})
	reg.Register(command.Command{
		ID: "editor.selectWordLeft", Title: "Select Word Left",
		Handler: func() { app.EditorGroup.MoveWordLeft(true) },
	})
	reg.Register(command.Command{
		ID: "editor.selectWordRight", Title: "Select Word Right",
		Handler: func() { app.EditorGroup.MoveWordRight(true) },
	})
	reg.Register(command.Command{
		ID: "editor.deleteWordLeft", Title: "Delete Word Left",
		Handler: func() { app.EditorGroup.DeleteWordLeft() },
	})
	reg.Register(command.Command{
		ID: "editor.deleteWordRight", Title: "Delete Word Right",
		Handler: func() { app.EditorGroup.DeleteWordRight() },
	})

	reg.Register(command.Command{
		ID: "multicursor.selectNext", Title: "Add Next Occurrence",
		Handler: func() { app.EditorGroup.SelectNextOccurrence() },
	})
	reg.Register(command.Command{
		ID: "multicursor.selectAll", Title: "Select All Occurrences",
		Handler: func() { app.EditorGroup.SelectAllOccurrences() },
	})
	reg.Register(command.Command{
		ID: "multicursor.undoCursor", Title: "Undo Last Cursor",
		Handler: func() { app.EditorGroup.UndoLastCursor() },
	})
	reg.Register(command.Command{
		ID: "editor.splitSelectionToLines", Title: "Split Selection into Lines",
		Handler: func() { app.EditorGroup.SplitSelectionToLines() },
	})

	reg.Register(command.Command{
		ID: "editor.sortLinesAsc", Title: "Sort Lines Ascending",
		Handler: func() { app.EditorGroup.SortLinesAsc() },
	})
	reg.Register(command.Command{
		ID: "editor.sortLinesDesc", Title: "Sort Lines Descending",
		Handler: func() { app.EditorGroup.SortLinesDesc() },
	})
	reg.Register(command.Command{
		ID: "editor.reverseLines", Title: "Reverse Lines",
		Handler: func() { app.EditorGroup.ReverseLines() },
	})
	reg.Register(command.Command{
		ID: "editor.uniqueLines", Title: "Unique Lines",
		Handler: func() { app.EditorGroup.UniqueLines() },
	})

	reg.Register(command.Command{
		ID: "editor.upperCase", Title: "Transform to Uppercase",
		Handler: func() { app.EditorGroup.UpperCase() },
	})
	reg.Register(command.Command{
		ID: "editor.lowerCase", Title: "Transform to Lowercase",
		Handler: func() { app.EditorGroup.LowerCase() },
	})
	reg.Register(command.Command{
		ID: "editor.titleCase", Title: "Transform to Titlecase",
		Handler: func() { app.EditorGroup.TitleCase() },
	})

	reg.Register(command.Command{
		ID: "bookmark.toggle", Title: "Toggle Bookmark",
		Handler: func() { app.EditorGroup.ToggleBookmark() },
	})
	reg.Register(command.Command{
		ID: "bookmark.next", Title: "Next Bookmark",
		Handler: func() { app.EditorGroup.NextBookmark() },
	})
	reg.Register(command.Command{
		ID: "bookmark.prev", Title: "Previous Bookmark",
		Handler: func() { app.EditorGroup.PrevBookmark() },
	})
	reg.Register(command.Command{
		ID: "bookmark.clearAll", Title: "Clear All Bookmarks",
		Handler: func() { app.EditorGroup.ClearBookmarks() },
	})

	reg.Register(command.Command{
		ID: "editor.quit", Title: "Quit",
		Handler: app.Quit,
	})
}
