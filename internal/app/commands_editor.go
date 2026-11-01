package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func (a *App) OpenFold() {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	e := a.EditorGroup.Editor
	if e.Folds == nil || e.WordWrap {
		return
	}
	e.Folds.Resolve()
	e.Folds.Expand(e.Cursor.Line)
}

func (a *App) CloseFold() {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	e := a.EditorGroup.Editor
	if e.Folds == nil || e.WordWrap {
		return
	}
	e.Folds.Resolve()
	e.Folds.Collapse(e.Cursor.Line)
	if r := e.Folds.ContainingFold(e.Cursor.Line); r != nil {
		e.Cursor.Line = r.StartLine
		e.Cursor.Col = 0
	}
	e.EnsureCursorVisible()
}

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

func (a *App) OpenFile() {
	a.ShowFilePicker("Open File", "Open", "", func(path string) {
		// Not validating the path here: ShowFilePicker already checks that it exists and is a regular file. OpenFile treats a missing path as a new file.
		a.EditorGroup.OpenFile(path)
		a.Root.SetFocus(a.EditorGroup)
	})
}

func (a *App) NewFile() {
	a.EditorGroup.NewFile()
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) NewFileFromText(text string) {
	a.EditorGroup.NewFileFromText(text)
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) SaveFileAs() {
	a.showSaveFileAs(nil)
}

func (a *App) showSaveFileAs(onSaved func()) {
	current := a.EditorGroup.ActiveFilePath()
	initial := ""
	if !a.EditorGroup.IsActiveVirtual() {
		initial = current
	}
	title := "Save As"
	if onSaved != nil {
		title += ": " + a.EditorGroup.ActiveFileName()
	}
	a.ShowInputDialog(title, "Filename", initial, func(path string) {
		if path == "" {
			return
		}
		path = a.saveAsPath(path)
		if saveAsReplaces(initial, path) {
			a.ShowConfirmDialog(
				fmt.Sprintf("%s already exists. Replace it?", filepath.Base(path)),
				[]string{"Cancel", "Replace"},
				[]func(){
					func() { a.DismissDialog() },
					func() { a.DismissDialog(); a.saveFileAs(path, onSaved) },
				},
			)
			return
		}
		a.saveFileAs(path, onSaved)
	})
}

func (a *App) saveFileAs(path string, onSaved func()) {
	if a.EditorGroup.SaveAs(path) {
		path, lang := a.editorPathLang()
		a.afterFileSave(path, lang)
		if onSaved != nil {
			onSaved()
		}
	}
}

// saveAsPath resolves a relative name against the primary workspace folder:
// Open Folder does not change the process working directory.
func (a *App) saveAsPath(input string) string {
	path := workspace.ExpandPath(input)
	if filepath.IsAbs(path) || a.Workspace == nil {
		return path
	}
	if base := a.Workspace.Primary(); base != "" {
		return filepath.Join(base, path)
	}
	return path
}

// saveAsReplaces reports whether saving to path would replace an existing
// entry other than the tab's own path. A dangling symlink counts, and so does
// another name for the current file: saving renames over a hard link and
// splits it from the original.
func saveAsReplaces(current, path string) bool {
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Stat(path); err == nil && target.IsDir() {
			return false
		}
	}
	return current == "" || !samePath(current, path)
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return absA == absB
}

func (a *App) SaveFile() {
	a.saveFile(nil)
}

func (a *App) saveFile(onSaved func()) {
	path := a.EditorGroup.ActiveFilePath()
	buf := a.EditorGroup.ActiveBuffer()
	if buf != nil && !a.EditorGroup.IsActiveVirtual() && buf.ReadOnlyOnDisk {
		a.ShowConfirmDialog(
			fmt.Sprintf("%s is read-only. Overwrite anyway?", filepath.Base(path)),
			[]string{"Cancel", "Overwrite"},
			[]func(){
				func() { a.DismissDialog() },
				func() { a.DismissDialog(); a.saveFileCheckingDisk(onSaved) },
			},
		)
		return
	}
	a.saveFileCheckingDisk(onSaved)
}

func (a *App) saveFileCheckingDisk(onSaved func()) {
	path := a.EditorGroup.ActiveFilePath()
	buf := a.EditorGroup.ActiveBuffer()
	if buf != nil && !a.EditorGroup.IsActiveVirtual() && buf.DiskChanged(path) {
		a.ShowConfirmDialog(
			fmt.Sprintf("%s was modified on disk. Overwrite with your version?", filepath.Base(path)),
			[]string{"Overwrite", "Cancel"},
			[]func(){
				func() { a.DismissDialog(); a.doSaveFile(onSaved) },
				func() { a.DismissDialog() },
			},
		)
		return
	}
	a.doSaveFile(onSaved)
}

func (a *App) doSaveFile(onSaved func()) {
	// Content tabs (image/binary viewers, diffs) hold no buffer: the format/code-action pipeline below must never run against their paths with a stale language.
	if a.EditorGroup.ActiveBuffer() == nil {
		return
	}
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
		if onSaved == nil || a.EditorGroup.IsActiveVirtual() {
			a.showSaveFileAs(onSaved)
		}
		return
	}
	path, lang = a.editorPathLang()
	a.afterFileSave(path, lang)
	if onSaved != nil {
		onSaved()
	}
}

func (a *App) afterFileSave(path, lang string) {
	if lang != "" {
		a.NotifyLSPSave(path, lang, a.EditorGroup.Editor.Buf.Lines)
		a.ScheduleDiagnosticPull(true)
	}
	a.RequestGitGutterForActiveFile()
	a.RefreshSymbols()
	if a.PluginManager != nil && path != "" {
		a.PluginManager.DispatchEvent("file.save", path)
	}
	a.invalidateRepositoryPath(path, RepositoryWorktree)
}

func (a *App) forceQuit() {
	for _, tt := range a.Terminals {
		tt.Term.Close()
	}
	*a.Running = false
}

func (a *App) Quit() {
	if a.quitPending {
		a.forceQuit()
		return
	}
	if !a.EditorGroup.AnyDirty() {
		a.forceQuit()
		return
	}
	if a.Root.HasOverlay() {
		return
	}
	a.quitPending = true
	cancel := func() { a.quitPending = false; a.DismissDialog() }
	buttons := []string{"Cancel", "Quit"}
	callbacks := []func(){cancel, a.forceQuit}
	showSaveAll := a.Settings.Editor.SaveAllOnQuit
	if a.Settings.Editor.SaveAllOnQuitExceptUnnamed {
		for i := 0; i < a.EditorGroup.TabCount(); i++ {
			if a.EditorGroup.IsTabUnnamed(i) {
				showSaveAll = false
				break
			}
		}
	}
	if showSaveAll {
		buttons = append(buttons, "Save All & Quit")
		callbacks = append(callbacks, func() {
			cancel()
			a.saveAllAndQuit()
		})
	}
	dialog := a.ShowConfirmDialogEx("Unsaved changes", "Press Ctrl+Q to force quit.", buttons, callbacks)
	dialog.OnDismiss = cancel
}

func (a *App) saveAllAndQuit() {
	if a.Root.HasOverlay() {
		return
	}
	next := -1
	for i := 0; i < a.EditorGroup.TabCount(); i++ {
		if _, dirty := a.EditorGroup.TabInfo(i); !dirty {
			continue
		}
		if !a.EditorGroup.IsTabUnnamed(i) {
			next = i
			break
		}
		if next < 0 {
			next = i
		}
	}
	if next < 0 {
		a.forceQuit()
		return
	}
	a.EditorGroup.SwitchTab(next)
	a.saveFile(a.saveAllAndQuit)
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
	if a.diffHunkStep(1) {
		return
	}
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
	if a.diffHunkStep(-1) {
		return
	}
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

func (a *App) DiffUseChangesOnlyContext() {
	if surface := a.EditorGroup.ActiveDiffContextSurface(); surface != nil {
		surface.SetContextMode(ui.DiffContextChangesOnly)
	}
}

func (a *App) DiffUseFullFileContext() {
	if surface := a.EditorGroup.ActiveDiffContextSurface(); surface != nil {
		surface.SetContextMode(ui.DiffContextFullFile)
	}
}

func registerEditorCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "editor.toggleDiff", Title: "Git: Toggle Inline Diff",
		Keywords: []string{"git", "diff", "inline", "changes", "head", "edit"},
		Handler:  app.ToggleInlineDiff,
	})

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
		ID: "diff.changesOnlyView", Title: "Git: Show Changes Only",
		Keywords: []string{"git", "diff", "compact", "collapsed", "context", "hunks"},
		Handler:  app.DiffUseChangesOnlyContext,
	})

	reg.Register(command.Command{
		ID: "diff.fullFileView", Title: "Git: Show Full File",
		Keywords: []string{"git", "diff", "extended", "context", "complete", "file"},
		Handler:  app.DiffUseFullFileContext,
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
		Handler:  app.ShowCursorHover,
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
		ID: "lsp.restartServer", Title: "LSP: Server Restart",
		Keywords: []string{"lsp", "language server", "restart", "reload"},
		Handler:  app.RestartLSPServer,
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
		ID: "diff.extendedView", Title: "Git: Extended Diff", Hidden: true,
		Handler: app.DiffUseFullFileContext,
	})

	reg.Register(command.Command{
		ID: "diff.compactView", Title: "Git: Compact Diff", Hidden: true,
		Handler: app.DiffUseChangesOnlyContext,
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
				e.Folds.Resolve()
				e.Folds.Toggle(e.Cursor.Line)
			}
		},
	})

	reg.Register(command.Command{
		ID: "fold.open", Title: "Open Fold",
		Keywords: []string{"editor", "expand", "show", "lines"},
		Handler:  app.OpenFold,
	})

	reg.Register(command.Command{
		ID: "fold.close", Title: "Close Fold",
		Keywords: []string{"editor", "collapse", "hide", "lines"},
		Handler:  app.CloseFold,
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
				e.Folds.Resolve()
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
		ID: "file.open", Title: "Open File",
		Keywords: []string{"file", "open"},
		Handler:  app.OpenFile,
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
		ID: "file.reloadFromDisk", Title: "Reload File from Disk",
		Keywords: []string{"file", "reload", "refresh"},
		Handler:  app.ReloadActiveFile,
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
		ID: "editor.goToFileStart", Title: "Go to Start of File",
		Keywords: []string{"editor", "navigate", "top", "beginning", "document"},
		Handler:  func() { app.EditorGroup.MoveToFileEdge(false, false) },
	})
	reg.Register(command.Command{
		ID: "editor.goToFileEnd", Title: "Go to End of File",
		Keywords: []string{"editor", "navigate", "bottom", "document"},
		Handler:  func() { app.EditorGroup.MoveToFileEdge(true, false) },
	})
	reg.Register(command.Command{
		ID: "editor.selectToFileStart", Title: "Select to Start of File",
		Keywords: []string{"editor", "selection", "top", "beginning", "document"},
		Handler:  func() { app.EditorGroup.MoveToFileEdge(false, true) },
	})
	reg.Register(command.Command{
		ID: "editor.selectToFileEnd", Title: "Select to End of File",
		Keywords: []string{"editor", "selection", "bottom", "document"},
		Handler:  func() { app.EditorGroup.MoveToFileEdge(true, true) },
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
		ID: "editor.trimTrailingWhitespace", Title: "Trim Trailing Whitespace",
		Keywords: []string{"editor", "lines", "whitespace", "trim", "spaces"},
		Handler:  func() { app.EditorGroup.TrimTrailingWhitespaceLines() },
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

// diffHunkStep navigates change blocks inside a diff view, where the buffer
// API and the git gutter do not apply. It reports whether it handled the key:
// false leaves normal file tabs on their existing gutter behaviour. Stops at
// the ends with a notification and never wraps, matching the palette command
// contract; plugins that want wrap-around build it from editor.diff().
func (a *App) diffHunkStep(delta int) bool {
	hunks, at, goTo := activeDiffHunks(a.EditorGroup)
	if goTo == nil {
		return false
	}
	if len(hunks) == 0 {
		a.StatusNotify("No changes in this diff")
		return true
	}
	target := at + delta
	if delta > 0 && at == 0 {
		target = 1
	}
	if target < 1 {
		a.StatusNotify("No more changes above")
		return true
	}
	if target > len(hunks) {
		a.StatusNotify("No more changes below")
		return true
	}
	if err := goTo(target); err != nil {
		a.StatusNotify(err.Error())
	}
	return true
}

// activeDiffHunks resolves the diff surface that navigation and the plugin
// API both work on. Commit Details spans files and answers as a whole, so it
// is checked before the single-file diff widget.
func activeDiffHunks(eg *ui.EditorGroupWidget) (hunks []ui.DiffHunk, at int, goTo func(int) error) {
	if detail := eg.ActiveCommitDetailWidget(); detail != nil {
		fileIndex, row := detail.ViewportFileAndRow()
		return detail.Hunks(), detail.HunkAt(fileIndex, row), detail.GoToHunk
	}
	if dv := eg.ActiveDiffWidget(); dv != nil {
		return dv.Hunks(), dv.HunkAt(dv.CursorRow()), dv.GoToHunk
	}
	// An editable inline diff is attached to the file tab, so it is not an
	// ActiveDiffWidget and has to be asked for separately.
	if dv := eg.ActiveInlineDiff(); dv != nil {
		return dv.Hunks(), dv.HunkAt(dv.CursorRow()), dv.GoToHunk
	}
	return nil, 0, nil
}
