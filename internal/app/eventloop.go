package app

import (
	"fmt"
	"log/slog"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

type BlameResult struct {
	Gen  int
	Info *git.BlameInfo
}

func RunEventLoop(
	screen *term.TcellScreen,
	renderer *render.Renderer,
	app *App,
	running *bool,
	closeTerminal func(panelID string),
) {
	if app.Watcher != nil {
		defer app.Watcher.Close()
	}

	lastBlameLine := -1
	lastBlameFile := ""
	lastGutterFile := ""
	lastTabFile := ""
	lastOutlineFile := ""
	lastOutlineLine := -1
	lastCursorLine := -1
	lastCursorCol := -1
	lastCursorFile := ""
	lastBranchDir := app.Workspace.Primary()
	blameGen := 0
	app.Status.SetSegment(view.StatusSegment{ID: "branch", Side: "left", Priority: 100, Text: git.BranchName(lastBranchDir)})

	syncStatus := func() {
		line, col := app.EditorGroup.ActiveCursor()
		filePath := app.EditorGroup.ActiveFilePath()
		cursorCount := app.EditorGroup.MultiCursorCount()

		posText := fmt.Sprintf("Ln %d, Col %d", line+1, col+1)
		if cursorCount > 1 {
			posText += fmt.Sprintf(" (%d cursors)", cursorCount)
		}
		app.Status.SetSegment(view.StatusSegment{ID: "position", Side: "right", Priority: 100, Text: posText})

		app.Status.SetSegment(view.StatusSegment{ID: "encoding", Side: "right", Priority: 300, Text: "UTF-8"})

		lineEnding := "\n"
		if buf := app.EditorGroup.ActiveBuffer(); buf != nil {
			lineEnding = buf.LineEnding
		}
		eolLabel := "LF"
		if lineEnding == "\r\n" {
			eolLabel = "CRLF"
		}
		app.Status.SetSegment(view.StatusSegment{ID: "eol", Side: "right", Priority: 400, Text: eolLabel})

		app.Explorer.SetActiveFile(filePath)
		app.SyncWatched()

		var tabSize int
		var useTabs bool
		if app.EditorGroup.Editor != nil && app.EditorGroup.Editor.TabSize > 0 {
			tabSize = app.EditorGroup.Editor.TabSize
			useTabs = app.EditorGroup.Editor.UseTabs
		} else {
			tabSize = app.Settings.Editor.TabSize
			useTabs = !app.Settings.Editor.InsertSpaces
		}
		indentLabel := "Spaces"
		if useTabs {
			indentLabel = "Tab Size"
		}
		app.Status.SetSegment(view.StatusSegment{ID: "indent", Side: "right", Priority: 200, Text: fmt.Sprintf("%s: %d", indentLabel, tabSize)})

		app.SyncLanguageSegment()

		repoDir := ""
		if filePath != "" && !app.EditorGroup.IsActiveVirtual() {
			if folder := app.Workspace.FolderForFile(filePath); folder != nil && folder.IsRepo {
				repoDir = folder.Path
			}
		}

		if repoDir != lastBranchDir {
			lastBranchDir = repoDir
			if repoDir != "" {
				app.Status.SetSegment(view.StatusSegment{ID: "branch", Side: "left", Priority: 100, Text: git.BranchName(repoDir)})
			} else {
				app.Status.SetSegment(view.StatusSegment{ID: "branch", Side: "left", Priority: 100, Text: ""})
			}
		}

		// Trigger git gutter computation when switching to a new file
		if filePath != lastGutterFile {
			lastGutterFile = filePath
			app.RequestGitGutterForActiveFile()
		}

		// tab.change fires whenever the active file changes — including files
		// opened from the CLI (which never go through file.open) and tab
		// switches — giving plugins a reliable hook to (re)scan the active file.
		if filePath != lastTabFile {
			lastTabFile = filePath
			if app.PluginManager != nil && filePath != "" {
				app.PluginManager.DispatchEvent("tab.change", filePath)
			}
		}

		if filePath != lastOutlineFile {
			lastOutlineFile = filePath
			lastOutlineLine = line
			app.EnsureLSPOpen(filePath)
			app.RefreshSymbols()
		} else if line != lastOutlineLine {
			lastOutlineLine = line
			if app.Sidebar.ActivePanel == "outline" {
				app.Symbols.SelectNearest(line)
			}
		}

		if filePath != lastCursorFile || line != lastCursorLine || col != lastCursorCol {
			lastCursorFile = filePath
			lastCursorLine = line
			lastCursorCol = col
			if app.PluginManager != nil {
				app.PluginManager.DispatchEvent("cursor.change", filePath)
			}
		}

		if filePath != lastBlameFile || line != lastBlameLine {
			lastBlameFile = filePath
			lastBlameLine = line
			app.Status.SetSegment(view.StatusSegment{ID: "blame", Side: "left", Priority: 200, Text: ""})
			if repoDir != "" {
				blameGen++
				gen := blameGen
				blameLine := line + 1
				go func() {
					info := git.BlameLine(repoDir, filePath, blameLine)
					screen.PostEvent(tcell.NewEventInterrupt(&BlameResult{Gen: gen, Info: info}))
				}()
			}
		}
	}

	redraw := func() {
		cells := make([][]term.Cell, app.Root.Height)
		for y := range cells {
			cells[y] = make([]term.Cell, app.Root.Width)
		}
		app.Root.Render(cells)
		resizeTerminals(app)
		renderer.SetCurrent(cells)
		if cx, cy, visible := app.Root.CursorPosition(); visible {
			screen.ShowCursor(cx, cy)
		} else {
			screen.HideCursor()
		}
		renderer.Render(screen)
	}

	// Populate the status bar and register file watches for any files opened at
	// launch, before the first user interaction.
	syncStatus()
	redraw()

	// Cursor positions from `path:line[:col]` arguments are applied here rather
	// than at build time: GoToLineCol centres the target against the viewport
	// height, which is only real once the first render has laid the widgets out.
	if len(app.PendingFileTargets) > 0 {
		app.ApplyFileTargets(app.PendingFileTargets)
		app.PendingFileTargets = nil
		syncStatus()
		redraw()
	}

	app.ShowPendingPluginApprovals()

	for *running {
		ev := screen.PollEvent()
		switch tev := ev.(type) {
		case *tcell.EventPaste:
			if tev.Start() {
				var pasteKeys []*tcell.EventKey
				for {
					pev := screen.PollEvent()
					if pev == nil || !*running {
						break
					}
					if pe, ok := pev.(*tcell.EventPaste); ok && !pe.Start() {
						break
					}
					if ke, ok := pev.(*tcell.EventKey); ok {
						pasteKeys = append(pasteKeys, ke)
					}
				}
				text := term.CollectPasteText(pasteKeys)
				if text != "" {
					app.PasteText(text)
					app.FlushEditorOnChange()
					syncStatus()
					redraw()
				}
			}

		case *tcell.EventKey:
			app.cancelHoverTimer()
			if app.EditorGroup.SignatureHelp != nil {
				switch tev.Key() {
				case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight,
					tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
					app.DismissSignatureHelp()
				}
			}
			slog.Debug("key", "key", tev.Key(), "rune", string(term.KeyRune(tev)), "mod", tev.Modifiers())
			app.Root.HandleEvent(tev)
			app.FlushEditorOnChange()
			app.RefreshAutocomplete()
			syncStatus()
			redraw()

		case *tcell.EventMouse:
			mx, my := tev.Position()
			btn := tev.Buttons()
			slog.Debug("mouse", "x", mx, "y", my, "btn", btn)
			app.DismissSignatureHelp()
			if app.EditorGroup.Hover == nil {
				if btn == 0 {
					app.checkMouseHover(mx, my)
				}
			} else if !app.isMouseOverHover(mx, my) && !app.EditorGroup.Hover.IsDragging() {
				app.DismissHover()
			}
			app.Root.HandleEvent(tev)
			app.FlushEditorOnChange()
			syncStatus()
			redraw()

		case *tcell.EventResize:
			w, h := screen.Size()
			app.Root.SetSize(w, h)
			resizeTerminals(app)
			renderer.Clear()
			redraw()

		case *tcell.EventInterrupt:
			switch v := tev.Data().(type) {
			case string:
				if v != "" {
					closeTerminal(v)
				}
			case *BlameResult:
				if v.Gen == blameGen && v.Info != nil {
					app.Status.SetSegment(view.StatusSegment{ID: "blame", Side: "left", Priority: 200, Text: fmt.Sprintf("%s, %s",
						v.Info.Author, git.FormatRelativeTime(v.Info.Time))})
				}
			case *GitGutterResult:
				if v.Gen == app.GitGutterGen {
					app.EditorGroup.SetLineChanges(v.Path, v.Changes)
				}
			case *GitGutterTrigger:
				app.RequestGitGutterForActiveFile()
			case *AutocompleteTrigger:
				triggerChar := app.charBeforeCursor()
				isTrigger := triggerChar != "" && (len(app.CompletionTriggers) == 0 || app.isCompletionTrigger(triggerChar))
				if isTrigger && app.IsAutocompleteActive() {
					app.DismissAutocomplete()
				}
				if !app.IsAutocompleteActive() {
					prefix := app.currentPrefix()
					if len(prefix) >= 1 || isTrigger {
						path := app.EditorGroup.ActiveFilePath()
						lang := ""
						if app.EditorGroup.Editor != nil && app.EditorGroup.Editor.Highlighter != nil {
							lang = app.EditorGroup.Editor.Highlighter.Language()
						}
						line, col := app.EditorGroup.ActiveCursor()
						app.RequestCompletions(path, lang, line, col, triggerChar)
					}
				}
			case *SignatureHelpResult:
				if v.Label != "" {
					app.ShowSignatureHelp(v)
				}
			case *CompletionResult:
				if len(v.Items) > 0 {
					app.CompletionTriggers = v.TriggerChars
					app.ShowAutocomplete(v.Items, v.LspItems)
				}
			case *HoverResult:
				if v.Text != "" && v.Gen == app.HoverGen {
					app.ShowHover(v.Text, v.AnchorX, v.AnchorY)
				}
			case *LocationResult:
				if len(v.Locations) > 0 {
					loc := v.Locations[0]
					path := URIToPath(loc.URI)
					app.EditorGroup.OpenFile(path)
					app.EditorGroup.GoToLine(loc.Range.Start.Line + 1)
					app.Root.SetFocus(app.EditorGroup)
				}
			case *RenameResult:
				if v.Edit != nil && len(v.Edit.Changes) > 0 {
					app.ApplyWorkspaceEdit(v.Edit)
				}
			case *ReferencesResult:
				if len(v.Locations) > 0 {
					app.ShowReferences(v.Locations)
				} else {
					app.StatusNotify("No references found")
				}
			case *SymbolsResult:
				if v.Path == app.EditorGroup.ActiveFilePath() {
					if len(v.Symbols) == 0 && v.Status != "" {
						app.Symbols.SetStatus(v.Status)
					} else {
						app.ApplySymbols(v.Symbols)
					}
				}
			case *FormattingResult:
				if len(v.Edits) > 0 {
					app.ApplyTextEdits(v.Edits)
				}
			case *DiagnosticsResult:
				// SetDiagnostics routes through the "lsp" source and fires
				// OnDiagnosticsChanged, which rebuilds the Diagnostics panel.
				app.EditorGroup.SetDiagnostics(v.Path, v.Diagnostics)
			case *OutputLineResult:
				app.LogOutput(v.Level, v.Source, v.Message)
			case *LSPStateChanged:
				app.SyncLanguageSegment()
			case *RepoOpResult:
				app.HandleRepoOpResult(v)
			case *FileChangedResult:
				app.HandleFileChanged(v.Path)
			case *ui.SearchBatch:
				app.Search.ApplyBatch(v)
			case *DiffContentResult:
				if v.Err != nil {
					app.StatusError("Failed to fetch file content: " + v.Err.Error())
					if dv := app.EditorGroup.DiffWidgetByTab(v.TabName); dv != nil {
						dv.Loading = false
						dv.SetExtended(false)
					}
				} else {
					if dv := app.EditorGroup.DiffWidgetByTab(v.TabName); dv != nil {
						dv.SetOldLines(v.OldLines)
						dv.SetNewLines(v.NewLines)
						dv.FinishLoading()
					}
				}
			case *plugin.PluginAsyncResult:
				if v.Callback != nil {
					v.Callback()
				}
			case *pluginInstallResult:
				app.handlePluginInstallResult(v)
			case *pluginUpdateResult:
				app.handlePluginUpdateResult(v)
			case *RemoteRegistryResult:
				app.handleRemoteRegistryResult(v)
			case *pluginReadmeResult:
				app.handlePluginReadmeResult(v)
			case *PrFetchResult:
				if v.Err != nil {
					app.StatusError("PR fetch failed: " + v.Err.Error())
				} else {
					var files []git.FileStatus
					for _, f := range v.Info.Files {
						files = append(files, git.FileStatus{
							Status: f.Status,
							Path:   f.Path,
						})
					}
					groupName := fmt.Sprintf("PR #%d: %s", v.Info.Number, v.Info.Title)
					app.Changes.AddPRGroup(groupName, v.URL, v.Info.Owner, v.Info.Repo, v.Info.BaseSHA, v.Info.HeadSHA, files, v.Diffs)
					app.Sidebar.SetActivePanel("changes")
					if !app.Sidebar.Visible {
						app.ShowSidebar()
					}
					app.Root.SetFocus(app.Changes.Adapter)
					app.Sidebar.SetPanelDirty("changes", app.Changes.TotalChanges() > 0)
					app.StatusNotify(fmt.Sprintf("Opened PR #%d: %s (%d files)", v.Info.Number, v.Info.Title, len(v.Info.Files)))
				}
			}
			redraw()
		}
	}
}

func resizeTerminals(app *App) {
	if !app.ContentSplit.ShowBottom {
		return
	}
	r := app.BottomPanel.GetRect()
	cols := r.W - terminalStripWidth
	rows := r.H - 2
	if cols <= 0 || rows <= 0 {
		return
	}
	for _, tt := range app.Terminals {
		tt.Term.Resize(cols, rows)
	}
}
