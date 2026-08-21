package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/diff"
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
		a.RequestCompletions(path, lang, line, col, "")
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
	a.ShowConfirmDialog("Save changes to "+name+"?",
		[]string{"Discard", "Cancel", "Save"},
		[]func(){
			func() {
				a.DismissDialog()
				a.EditorGroup.CloseTab()
			},
			func() {
				a.DismissDialog()
			},
			func() {
				a.DismissDialog()
				a.Reg.Execute("file.save")
				if !a.EditorGroup.IsDirty() {
					a.EditorGroup.CloseTab()
				}
			},
		},
	)
}

func (a *App) CloseOtherTabs() {
	if !a.EditorGroup.HasDirtyOtherTabs() {
		a.EditorGroup.CloseOtherTabs()
		return
	}
	a.ShowConfirmDialog("Other tabs have unsaved changes.",
		[]string{"Abort", "Close Saved", "Discard All"},
		[]func(){
			func() { a.DismissDialog() },
			func() {
				a.DismissDialog()
				a.EditorGroup.CloseOtherSaved()
			},
			func() {
				a.DismissDialog()
				a.EditorGroup.CloseOtherTabs()
			},
		},
	)
}

func (a *App) CloseAllTabs() {
	if !a.EditorGroup.HasDirtyTabs() {
		a.EditorGroup.CloseAllTabs()
		return
	}
	a.ShowConfirmDialog("You have unsaved changes.",
		[]string{"Abort", "Close Saved", "Discard All"},
		[]func(){
			func() { a.DismissDialog() },
			func() {
				a.DismissDialog()
				a.EditorGroup.CloseAllSaved()
			},
			func() {
				a.DismissDialog()
				a.EditorGroup.CloseAllTabs()
			},
		},
	)
}

func (a *App) NewFile() {
	a.EditorGroup.NewFile()
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) SaveFileAs() {
	current := a.EditorGroup.ActiveFilePath()
	initial := ""
	if !a.EditorGroup.IsActiveVirtual() {
		initial = current
	}
	a.ShowInputDialog("Save As", "Filename", initial, func(path string) {
		if path != "" && a.EditorGroup.SaveAs(path) {
			path, lang := a.editorPathLang()
			a.afterFileSave(path, lang)
		}
	})
}

func (a *App) SaveFile() {
	path := a.EditorGroup.ActiveFilePath()
	buf := a.EditorGroup.ActiveBuffer()
	if buf != nil && !a.EditorGroup.IsActiveVirtual() && buf.DiskChanged(path) {
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
			if !a.FormatExternalOnSave(path) {
				a.FormatOnSave(path, lang)
			}
		}
	}
	if !a.EditorGroup.Save() {
		a.SaveFileAs()
		return
	}
	path, lang = a.editorPathLang()
	a.afterFileSave(path, lang)
}

func (a *App) afterFileSave(path, lang string) {
	if lang != "" {
		text := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n")
		a.NotifyLSPSave(path, lang, text)
	}
	a.RequestGitGutterForActiveFile()
	a.RefreshSymbols()
	if a.PluginManager != nil && path != "" {
		a.PluginManager.DispatchEvent("file.save", path)
	}
	a.invalidateRepositoryPath(path, RepositoryStatus)
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
	a.ShowConfirmDialogEx("Unsaved changes", "Press Ctrl+Q to force quit.", []string{"Cancel", "Quit"}, []func(){
		func() { a.quitPending = false; a.DismissDialog() },
		func() { a.forceQuit() },
	})
}

// hunkStarts returns the buffer-line indices (0-based) at which each diff hunk
// begins. A hunk is a maximal run of consecutive non-Unchanged lines; its start
// is the first line of that run.
func hunkStarts(changes []diff.LineChangeKind) []int {
	var starts []int
	for i := 0; i < len(changes); i++ {
		if changes[i] != diff.LineUnchanged && (i == 0 || changes[i-1] == diff.LineUnchanged) {
			starts = append(starts, i)
		}
	}
	return starts
}

// DiffNextHunk moves the cursor to the start of the next changed hunk after the
// current cursor line. No-op when git-gutter data is unavailable or there is no
// following hunk.
func (a *App) DiffNextHunk() {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	changes := a.EditorGroup.Editor.LineChanges
	if len(changes) == 0 {
		a.StatusNotify("No changes in this file")
		return
	}
	line, _ := a.EditorGroup.ActiveCursor() // 0-based
	target := -1
	for _, s := range hunkStarts(changes) {
		if s > line {
			target = s
			break
		}
	}
	if target < 0 {
		a.StatusNotify("No more changes below")
		return
	}
	a.EditorGroup.GoToLine(target + 1) // GoToLine is 1-based
}

// DiffPrevHunk moves the cursor to the start of the previous changed hunk before
// the current cursor line. No-op when git-gutter data is unavailable or there is
// no preceding hunk.
func (a *App) DiffPrevHunk() {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	changes := a.EditorGroup.Editor.LineChanges
	if len(changes) == 0 {
		a.StatusNotify("No changes in this file")
		return
	}
	line, _ := a.EditorGroup.ActiveCursor() // 0-based
	target := -1
	for _, s := range hunkStarts(changes) {
		if s < line {
			target = s
		} else {
			break
		}
	}
	if target < 0 {
		a.StatusNotify("No more changes above")
		return
	}
	a.EditorGroup.GoToLine(target + 1) // GoToLine is 1-based
}

func (a *App) DiffToggleWrap() {
	if surface := a.EditorGroup.ActiveDiffModeSurface(); surface != nil {
		surface.SetWrapMode(surface.WrapMode().Toggle())
	}
}

func (a *App) DiffToggleUnified() {
	if surface := a.EditorGroup.ActiveDiffModeSurface(); surface != nil {
		surface.SetMode(surface.Mode().Toggle())
	}
}

func (a *App) DiffUseSplitMode() {
	if surface := a.EditorGroup.ActiveDiffModeSurface(); surface != nil {
		surface.SetMode(ui.DiffModeSplit)
	}
}

func (a *App) DiffUseUnifiedMode() {
	if surface := a.EditorGroup.ActiveDiffModeSurface(); surface != nil {
		surface.SetMode(ui.DiffModeUnified)
	}
}

func (a *App) CommitDetailCollapseAll() {
	if detail := a.EditorGroup.ActiveCommitDetailWidget(); detail != nil {
		detail.CollapseAllFiles()
	}
}

func (a *App) CommitDetailExpandAll() {
	if detail := a.EditorGroup.ActiveCommitDetailWidget(); detail != nil {
		detail.ExpandAllFiles()
	}
}

func registerEditorCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "diff.nextHunk", Title: "Git: Next Changed Hunk",
		Keywords: []string{"git", "diff", "hunk", "change", "navigate"},
		Handler:  app.DiffNextHunk,
	})

	reg.Register(command.Command{
		ID: "diff.prevHunk", Title: "Git: Previous Changed Hunk",
		Keywords: []string{"git", "diff", "hunk", "change", "navigate"},
		Handler:  app.DiffPrevHunk,
	})

	reg.Register(command.Command{
		ID: "diff.toggleWrap", Title: "Git: Toggle Diff Wrap",
		Keywords: []string{"git", "diff", "wrap", "line"},
		Handler:  app.DiffToggleWrap,
	})

	reg.Register(command.Command{
		ID: "diff.toggleUnified", Title: "Git: Toggle Unified Diff",
		Keywords: []string{"git", "diff", "unified", "split", "stack"},
		Handler:  app.DiffToggleUnified,
	})

	reg.Register(command.Command{
		ID: "diff.splitView", Title: "Git: Split Diff",
		Keywords: []string{"git", "diff", "split", "side by side", "mode"},
		Handler:  app.DiffUseSplitMode,
	})

	reg.Register(command.Command{
		ID: "diff.unifiedView", Title: "Git: Unified Diff",
		Keywords: []string{"git", "diff", "unified", "stack", "mode"},
		Handler:  app.DiffUseUnifiedMode,
	})

	reg.Register(command.Command{
		ID: "commitDetail.collapseAll", Title: "Git: Collapse All Commit Files",
		Keywords: []string{"git", "commit", "detail", "collapse", "files"},
		Handler:  app.CommitDetailCollapseAll,
	})

	reg.Register(command.Command{
		ID: "commitDetail.expandAll", Title: "Git: Expand All Commit Files",
		Keywords: []string{"git", "commit", "detail", "expand", "files"},
		Handler:  app.CommitDetailExpandAll,
	})

	reg.Register(command.Command{
		ID: "editor.focus", Title: "View: Focus Editor",
		Keywords: []string{"editor"},
		Handler:  app.FocusEditor,
	})

	reg.Register(command.Command{
		ID: "editor.autocomplete", Title: "Trigger Autocomplete",
		Keywords: []string{"editor", "completion", "suggest", "intellisense"},
		Handler:  app.TriggerAutocomplete,
	})

	reg.Register(command.Command{
		ID: "editor.hover", Title: "Show Hover",
		Keywords: []string{"editor", "tooltip", "info"},
		Handler: func() {
			path, lang := app.editorPathLang()
			line, col := app.EditorGroup.ActiveCursor()
			ax, ay, _ := app.EditorGroup.CursorPosition()
			diagText := ""
			if app.EditorGroup.Editor != nil {
				if d := app.EditorGroup.Editor.DiagnosticAt(line, col); d != nil {
					diagText = d.Message
				}
			}
			gen := app.HoverGen
			app.RequestHover(path, lang, line, col, ax, ay, diagText, gen)
		},
	})

	reg.Register(command.Command{
		ID: "editor.goToDefinition", Title: "Go to Definition",
		Keywords: []string{"editor", "navigate", "jump"},
		Handler:  func() { app.withEditorLSP(app.RequestDefinition) },
	})

	reg.Register(command.Command{
		ID: "editor.goToImplementation", Title: "Go to Implementation",
		Keywords: []string{"editor", "navigate", "jump"},
		Handler:  func() { app.withEditorLSP(app.RequestImplementation) },
	})

	reg.Register(command.Command{
		ID: "editor.goToTypeDefinition", Title: "Go to Type Definition",
		Keywords: []string{"editor", "navigate", "jump"},
		Handler:  func() { app.withEditorLSP(app.RequestTypeDefinition) },
	})

	reg.Register(command.Command{
		ID: "editor.findReferences", Title: "Find All References",
		Keywords: []string{"editor", "navigate", "search", "usages"},
		Handler:  func() { app.withEditorLSP(app.RequestReferences) },
	})

	reg.Register(command.Command{
		ID: "editor.rename", Title: "Rename Symbol",
		Keywords: []string{"editor", "refactor"},
		Handler:  app.RenameSymbol,
	})

	reg.Register(command.Command{
		ID: "editor.organizeImports", Title: "Source Action: Organize Imports",
		Keywords: []string{"editor", "source", "cleanup"},
		Handler: func() {
			path, lang := app.editorPathLang()
			app.RequestCodeAction(path, lang, "source.organizeImports")
		},
	})

	reg.Register(command.Command{
		ID: "editor.fixAll", Title: "Source Action: Fix All",
		Keywords: []string{"editor", "source", "lint", "autofix"},
		Handler: func() {
			path, lang := app.editorPathLang()
			app.RequestCodeAction(path, lang, "source.fixAll")
		},
	})

	reg.Register(command.Command{
		ID: "editor.formatDocument", Title: "Source Action: Format Document",
		Keywords: []string{"editor", "source", "prettier", "beautify"},
		Handler: func() {
			path, lang := app.editorPathLang()
			app.RequestFormatting(path, lang)
		},
	})

	reg.Register(command.Command{
		ID: "editor.formatSelection", Title: "Source Action: Format Selection",
		Keywords: []string{"editor", "source", "prettier", "beautify"},
		Handler:  app.FormatSelection,
	})

	reg.Register(command.Command{
		ID: "editor.formatExternal", Title: "Format Document (External Formatter)",
		Keywords: []string{"editor", "format", "formatter", "external"},
		Handler:  app.RunExternalFormatter,
	})

	reg.Register(command.Command{
		ID: "tab.next", Title: "View: Next Tab",
		Keywords: []string{"tab", "switch"},
		Handler:  func() { app.contextNextTab() },
	})

	reg.Register(command.Command{
		ID: "tab.prev", Title: "View: Previous Tab",
		Keywords: []string{"tab", "switch"},
		Handler:  func() { app.contextPrevTab() },
	})

	reg.Register(command.Command{
		ID: "tab.close", Title: "View: Close Tab",
		Keywords: []string{"tab"},
		Handler:  app.CloseTab,
	})

	reg.Register(command.Command{
		ID: "tab.closeOthers", Title: "View: Close Other Tabs",
		Keywords: []string{"tab"},
		Handler:  app.CloseOtherTabs,
	})

	reg.Register(command.Command{
		ID: "tab.closeAll", Title: "View: Close All Tabs",
		Keywords: []string{"tab"},
		Handler:  app.CloseAllTabs,
	})

	reg.Register(command.Command{
		ID: "tab.closeAllSaved", Title: "View: Close All Saved Tabs",
		Keywords: []string{"tab", "close", "saved"},
		Handler:  func() { app.EditorGroup.CloseAllSaved() },
	})

	reg.Register(command.Command{
		ID: "tab.pin", Title: "View: Toggle Pin Tab",
		Keywords: []string{"tab", "pin", "unpin"},
		Handler:  func() { app.EditorGroup.TogglePinTab() },
	})

	reg.Register(command.Command{
		ID: "tab.moveLeft", Title: "View: Move Tab Left",
		Keywords: []string{"tab", "reorder", "move", "left"},
		Handler:  func() { app.EditorGroup.MoveActiveTab(-1) },
	})

	reg.Register(command.Command{
		ID: "tab.moveRight", Title: "View: Move Tab Right",
		Keywords: []string{"tab", "reorder", "move", "right"},
		Handler:  func() { app.EditorGroup.MoveActiveTab(1) },
	})

	reg.Register(command.Command{
		ID: "diff.extendedView", Title: "Git: Extended Diff",
		Keywords: []string{"git", "changes", "compare"},
		Handler: func() {
			if dv := app.EditorGroup.ActiveDiffWidget(); dv != nil {
				dv.SetExtended(true)
			}
		},
	})

	reg.Register(command.Command{
		ID: "diff.compactView", Title: "Git: Compact Diff",
		Keywords: []string{"git", "changes", "compare"},
		Handler: func() {
			if dv := app.EditorGroup.ActiveDiffWidget(); dv != nil {
				dv.SetExtended(false)
			}
		},
	})

	reg.Register(command.Command{
		ID: "fold.toggle", Title: "Toggle Fold",
		Keywords: []string{"editor", "collapse", "expand", "hide", "lines"},
		Handler: func() {
			if !app.EditorGroup.IsEditorActive() {
				return
			}
			e := app.EditorGroup.Editor
			if e.Folds != nil && !e.WordWrap {
				e.Folds.Toggle(e.Cursor.Line)
			}
		},
	})

	reg.Register(command.Command{
		ID: "fold.collapseAll", Title: "Fold All",
		Keywords: []string{"editor", "collapse", "expand", "hide", "lines"},
		Handler: func() {
			if !app.EditorGroup.IsEditorActive() {
				return
			}
			e := app.EditorGroup.Editor
			if e.Folds != nil && !e.WordWrap {
				e.Folds.CollapseAll()
				e.EnsureCursorVisible()
			}
		},
	})

	reg.Register(command.Command{
		ID: "fold.expandAll", Title: "Unfold All",
		Keywords: []string{"editor", "collapse", "expand", "hide", "lines"},
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
		Keywords: []string{"file", "create"},
		Handler:  app.NewFile,
	})

	reg.Register(command.Command{
		ID: "file.save", Title: "Save File",
		Keywords: []string{"file", "write"},
		Handler:  app.SaveFile,
	})

	reg.Register(command.Command{
		ID: "file.saveAs", Title: "Save As...",
		Keywords: []string{"file", "write", "export"},
		Handler:  app.SaveFileAs,
	})

	reg.Register(command.Command{
		ID: "editor.undo", Title: "Undo",
		Keywords: []string{"editor", "revert"},
		Handler:  func() { app.EditorGroup.Undo() },
	})

	reg.Register(command.Command{
		ID: "editor.redo", Title: "Redo",
		Keywords: []string{"editor"},
		Handler:  func() { app.EditorGroup.Redo() },
	})

	reg.Register(command.Command{
		ID: "editor.selectAll", Title: "Select All",
		Keywords: []string{"editor", "selection"},
		Handler:  func() { app.EditorGroup.SelectAll() },
	})

	reg.Register(command.Command{
		ID: "editor.copy", Title: "Copy",
		Keywords: []string{"editor", "clipboard"},
		Handler:  app.Copy,
	})

	reg.Register(command.Command{
		ID: "editor.cut", Title: "Cut",
		Keywords: []string{"editor", "clipboard"},
		Handler:  app.Cut,
	})

	reg.Register(command.Command{
		ID: "editor.paste", Title: "Paste",
		Keywords: []string{"editor", "clipboard"},
		Handler:  app.Paste,
	})

	reg.Register(command.Command{
		ID: "editor.moveLineUp", Title: "Move Line Up",
		Keywords: []string{"editor", "lines"},
		Handler:  func() { app.EditorGroup.MoveLineUp() },
	})
	reg.Register(command.Command{
		ID: "editor.moveLineDown", Title: "Move Line Down",
		Keywords: []string{"editor", "lines"},
		Handler:  func() { app.EditorGroup.MoveLineDown() },
	})
	reg.Register(command.Command{
		ID: "editor.duplicateLine", Title: "Duplicate Line",
		Keywords: []string{"editor", "lines", "clone"},
		Handler:  func() { app.EditorGroup.DuplicateLine() },
	})
	reg.Register(command.Command{
		ID: "editor.deleteLine", Title: "Delete Line",
		Keywords: []string{"editor", "lines", "remove"},
		Handler:  func() { app.EditorGroup.DeleteLine() },
	})
	reg.Register(command.Command{
		ID: "editor.joinLines", Title: "Join Lines",
		Keywords: []string{"editor", "lines", "merge", "combine"},
		Handler:  func() { app.EditorGroup.JoinLines() },
	})
	reg.Register(command.Command{
		ID: "editor.insertLineBelow", Title: "Insert Line Below",
		Keywords: []string{"editor", "lines"},
		Handler:  func() { app.EditorGroup.InsertLineBelow() },
	})
	reg.Register(command.Command{
		ID: "editor.insertLineAbove", Title: "Insert Line Above",
		Keywords: []string{"editor", "lines"},
		Handler:  func() { app.EditorGroup.InsertLineAbove() },
	})
	reg.Register(command.Command{
		ID: "editor.toggleComment", Title: "Toggle Line Comment",
		Keywords: []string{"editor", "comment", "uncomment"},
		Handler:  func() { app.EditorGroup.ToggleLineComment() },
	})
	reg.Register(command.Command{
		ID: "editor.moveWordLeft", Title: "Move Word Left",
		Keywords: []string{"editor", "navigate"},
		Handler:  func() { app.EditorGroup.MoveWordLeft(false) },
	})
	reg.Register(command.Command{
		ID: "editor.moveWordRight", Title: "Move Word Right",
		Keywords: []string{"editor", "navigate"},
		Handler:  func() { app.EditorGroup.MoveWordRight(false) },
	})
	reg.Register(command.Command{
		ID: "editor.selectWordLeft", Title: "Select Word Left",
		Keywords: []string{"editor", "selection"},
		Handler:  func() { app.EditorGroup.MoveWordLeft(true) },
	})
	reg.Register(command.Command{
		ID: "editor.selectWordRight", Title: "Select Word Right",
		Keywords: []string{"editor", "selection"},
		Handler:  func() { app.EditorGroup.MoveWordRight(true) },
	})
	reg.Register(command.Command{
		ID: "editor.deleteWordLeft", Title: "Delete Word Left",
		Keywords: []string{"editor"},
		Handler:  func() { app.EditorGroup.DeleteWordLeft() },
	})
	reg.Register(command.Command{
		ID: "editor.deleteWordRight", Title: "Delete Word Right",
		Keywords: []string{"editor"},
		Handler:  func() { app.EditorGroup.DeleteWordRight() },
	})

	reg.Register(command.Command{
		ID: "multicursor.selectNext", Title: "Add Next Occurrence",
		Keywords: []string{"editor", "multicursor", "selection"},
		Handler:  func() { app.EditorGroup.SelectNextOccurrence() },
	})
	reg.Register(command.Command{
		ID: "multicursor.selectAll", Title: "Select All Occurrences",
		Keywords: []string{"editor", "multicursor", "selection"},
		Handler:  func() { app.EditorGroup.SelectAllOccurrences() },
	})
	reg.Register(command.Command{
		ID: "multicursor.undoCursor", Title: "Undo Last Cursor",
		Keywords: []string{"editor", "multicursor"},
		Handler:  func() { app.EditorGroup.UndoLastCursor() },
	})
	reg.Register(command.Command{
		ID: "editor.splitSelectionToLines", Title: "Split Selection into Lines",
		Keywords: []string{"editor", "multicursor", "selection"},
		Handler:  func() { app.EditorGroup.SplitSelectionToLines() },
	})

	reg.Register(command.Command{
		ID: "editor.sortLinesAsc", Title: "Sort Lines Ascending",
		Keywords: []string{"editor", "lines", "order"},
		Handler:  func() { app.EditorGroup.SortLinesAsc() },
	})
	reg.Register(command.Command{
		ID: "editor.sortLinesDesc", Title: "Sort Lines Descending",
		Keywords: []string{"editor", "lines", "order"},
		Handler:  func() { app.EditorGroup.SortLinesDesc() },
	})
	reg.Register(command.Command{
		ID: "editor.reverseLines", Title: "Reverse Lines",
		Keywords: []string{"editor", "lines", "flip"},
		Handler:  func() { app.EditorGroup.ReverseLines() },
	})
	reg.Register(command.Command{
		ID: "editor.uniqueLines", Title: "Unique Lines",
		Keywords: []string{"editor", "lines", "deduplicate", "distinct"},
		Handler:  func() { app.EditorGroup.UniqueLines() },
	})

	reg.Register(command.Command{
		ID: "editor.upperCase", Title: "Transform to Uppercase",
		Keywords: []string{"editor", "case", "capitalize"},
		Handler:  func() { app.EditorGroup.UpperCase() },
	})
	reg.Register(command.Command{
		ID: "editor.lowerCase", Title: "Transform to Lowercase",
		Keywords: []string{"editor", "case"},
		Handler:  func() { app.EditorGroup.LowerCase() },
	})
	reg.Register(command.Command{
		ID: "editor.titleCase", Title: "Transform to Titlecase",
		Keywords: []string{"editor", "case", "capitalize"},
		Handler:  func() { app.EditorGroup.TitleCase() },
	})

	reg.Register(command.Command{
		ID: "editor.goToMatchingBracket", Title: "Go to Matching Bracket",
		Keywords: []string{"editor", "navigate", "parenthesis", "brace"},
		Handler:  func() { app.EditorGroup.GoToMatchingBracket() },
	})

	reg.Register(command.Command{
		ID: "editor.quit", Title: "Quit",
		Keywords: []string{"exit", "close"},
		Handler:  app.Quit,
	})
}
