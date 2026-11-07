package app

import (
	"fmt"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
	"log/slog"
	"time"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

type highlightStep struct{}

const highlightStepBudget = 4 * time.Millisecond

func RunEventLoop(
	screen *term.TcellScreen,
	renderer *render.Renderer,
	app *App,
	running *bool,
	closeTerminal func(panelID string),
	onStartupComplete func(),
) {
	app.eventLoopDoneSignal()
	defer app.closeEventLoopDone()
	defer app.Root.CancelPointerCapture()
	defer app.ShutdownGitReads()
	if app.Watcher != nil {
		defer app.Watcher.Close()
	}
	if app.Repository != nil {
		defer app.Repository.Close()
	}
	defer app.CloseTasks()

	lastGutterFile := ""
	lastGutterRepo := ""
	lastTabFile := ""
	lastOutlineFile := ""
	lastOutlineLine := -1
	lastCursorLine := -1
	lastCursorCol := -1
	lastCursorFile := ""

	syncStatus := func() {
		app.syncRepositoryObservation()
		line, col := app.EditorGroup.ActiveCursor()
		filePath := app.EditorGroup.ActiveFilePath()
		cursorCount := app.EditorGroup.MultiCursorCount()

		posText := i18n.Format("status.lnCol", i18n.Args{"line": line + 1, "column": col + 1})
		if cursorCount > 1 {
			posText += i18n.Format("status.cursors", i18n.Args{"cursorCount": cursorCount})
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
		indentLabel := i18n.Text("status.spaces")
		if useTabs {
			indentLabel = i18n.Text("status.tabSize")
		}
		app.Status.SetSegment(view.StatusSegment{ID: "indent", Side: "right", Priority: 200, Text: fmt.Sprintf("%s: %d", indentLabel, tabSize)})

		app.SyncLanguageSegment()

		repoDir := app.SyncRepositoryBranch()

		// Trigger git gutter computation when switching to a new file
		if filePath != lastGutterFile || repoDir != lastGutterRepo {
			lastGutterFile = filePath
			lastGutterRepo = repoDir
			app.RequestGitGutterForActiveFile()
		}

		// tab.change fires whenever the active file changes — including files
		// opened from the CLI (which never go through file.open) and tab
		// switches — giving plugins a reliable hook to (re)scan the active file.
		if filePath != lastTabFile {
			lastTabFile = filePath
			app.DismissSignatureHelp()
			if app.PluginManager != nil && filePath != "" {
				app.PluginManager.DispatchEvent("tab.change", filePath)
			}
			app.autoRevealActiveFile()
		}

		if filePath != lastOutlineFile {
			lastOutlineFile = filePath
			lastOutlineLine = line
			app.EnsureLSPOpen(filePath)
			app.ScheduleDiagnosticPull(true)
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
	}

	// Last mouse position, so redraw can refresh the pointer after a key or
	// async event opens an overlay or moves a divider. -1 until the first
	// mouse event: no pointer escape is written before the mouse is seen.
	mouseX, mouseY := -1, -1

	redraw := func() {
		// Every event path ends here, including scripted commands.
		app.SyncFileHandlerTabs()
		app.SyncEmptyState()
		app.SyncBlame()
		app.SyncReloadSegment()
		cells := renderer.NextFrame(app.Root.Width, app.Root.Height)
		if app.ImageLayer != nil {
			app.ImageLayer.Begin()
		}
		app.Root.Render(cells)
		if cx, cy, visible := app.Root.CursorPosition(); visible {
			screen.ShowCursor(cx, cy)
		} else {
			screen.HideCursor()
		}
		renderer.Render(screen)
		if app.commitImageLayer(screen) {
			screen.Show()
		}
		// Must run after Render/CursorPosition, else content and cursor read different Term states mid-frame.
		resizeTerminals(app)
		if app.Screen != nil && mouseX >= 0 && app.Screen.PointerShapesEnabled() {
			app.Screen.SetPointerShape(app.pointerShapeAt(mouseX, mouseY))
		}
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

	handleKey := func(tev *tcell.EventKey) {
		app.stopHoverTask()
		switch tev.Key() {
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight,
			tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
			// Also cancels a reply still on its way, which could otherwise
			// show after navigating past the end of the call.
			app.DismissSignatureHelp()
		}
		slog.Debug("key", "key", tev.Key(), "rune", string(term.KeyRune(tev)), "mod", tev.Modifiers())
		app.Root.HandleEvent(tev)
		app.FlushEditorOnChange()
		app.RefreshAutocomplete()
		syncStatus()
	}

	handleMouse := func(tev *tcell.EventMouse) {
		mx, my := tev.Position()
		mouseX, mouseY = mx, my
		if app.handlePathDrag(tev) {
			syncStatus()
			return
		}
		btn := tev.Buttons()
		if btn != 0 {
			app.stopHoverTask()
			app.DismissSignatureHelp()
		}
		slog.Debug("mouse", "x", mx, "y", my, "btn", btn)
		app.EditorGroup.SignatureHelp = nil
		// Dismiss before routing: an existing hover can consume the Ctrl mouse event.
		if tev.Modifiers()&tcell.ModCtrl != 0 && app.EditorGroup.GetRect().Contains(mx, my) {
			app.DismissHover()
		} else if app.EditorGroup.Hover == nil {
			if btn == 0 {
				app.checkMouseHover(mx, my)
			}
		} else if !app.isMouseOverHover(mx, my) && !app.EditorGroup.Hover.IsDragging() {
			app.DismissHover()
		}
		app.Root.HandleEvent(tev)
		app.FlushEditorOnChange()
		syncStatus()
	}

	if onStartupComplete != nil {
		onStartupComplete()
	}

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
			handleKey(tev)
			redraw()

		case *tcell.EventMouse:
			handleMouse(tev)
			redraw()
		case *tcell.EventFocus:
			app.Root.HandleEvent(tev)
			redraw()

		case *tcell.EventResize:
			w, h := screen.Size()
			app.Root.SetSize(w, h)
			resizeTerminals(app)
			renderer.Clear()
			app.invalidateImageLayer()
			app.RefreshImageCellSize()
			redraw()

		case *tcell.EventInterrupt:
			var execRequest *execRequestLifecycle
			var execErr error
			skipRedraw := false
			switch v := tev.Data().(type) {
			case *taskDelivery:
				v.run()
			case *highlightStep:
				// Short steps keep input responsive while a large file catches up;
				// the frame is redrawn once the deferred lines are ready.
				if highlight.Step(highlightStepBudget) {
					screen.PostEvent(tcell.NewEventInterrupt(v))
					skipRedraw = true
				}
			case string:
				if v != "" {
					closeTerminal(v)
				}
			case *ui.TabDragAutoScrollTick:
				app.EditorGroup.TabBar.HandleDragAutoScrollTick(v.Generation)
			case *ui.EditorDragAutoScrollTick:
				app.EditorGroup.Editor.HandleDragAutoScrollTick(v.Generation)
			case *execInputRequest:
				if v.Lifecycle.claim() {
					switch event := v.Event.(type) {
					case *tcell.EventKey:
						handleKey(event)
					case *tcell.EventMouse:
						handleMouse(event)
					default:
						execErr = failedExec(i18n.Text("status.unsupportedInputEventT"), v.Event)
					}
					execRequest = v.Lifecycle
				}
			case *pluginInput:
				if v.Lifecycle != nil && !v.Lifecycle.claim() {
					break
				}
				execRequest = v.Lifecycle
				for _, ev := range v.Events {
					if !app.acceptsPluginInput() || (v.Allowed != nil && !v.Allowed()) {
						execErr = failedExec("plugin input unavailable while permissions are pending or plugin is unloaded")
						break
					}
					switch event := ev.(type) {
					case *tcell.EventKey:
						handleKey(event)
					case *tcell.EventMouse:
						handleMouse(event)
					}
				}
			case *execMainRequest:
				if v.Lifecycle.claim() {
					execErr = v.Run()
					syncStatus()
					execRequest = v.Lifecycle
				}
			case *LocationResult:
				if len(v.Locations) > 0 {
					loc := v.Locations[0]
					path := URIToPath(loc.URI)
					if err := app.EditorGroup.OpenFile(path); err != nil {
						app.StatusWarn(err.Error())
						break
					}
					pos := lsp.PositionToRunes(app.lspLines(path), loc.Range.Start)
					app.EditorGroup.GoToLineCol(pos.Line+1, pos.Character+1)
					app.Root.SetFocus(app.EditorGroup)
					syncStatus()
				}
			case *RenameResult:
				app.ApplyWorkspaceEdit(v.Edit)
			case *CodeActionResult:
				app.applyCodeActionEdit(v.Path, v.Edit)
			case *ReferencesResult:
				if len(v.Locations) > 0 {
					app.ShowReferences(v.Locations)
				} else {
					app.StatusNotify(i18n.Text("status.noReferencesFound"))
				}
			case *FormattingResult:
				app.applyFormattingResult(v)
			case *DiagnosticsResult:
				var lines []string
				if len(v.Diagnostics) > 0 {
					lines = app.lspLines(v.Path)
				}
				app.EditorGroup.SetDiagnosticsSource(lspDiagnosticsSource(v.Server), v.Path, LspToUIDiagnostics(v.Diagnostics, lines))
			case *LSPDiagnosticRefresh:
				app.handleDiagnosticRefresh(v.Server)
			case *LSPServerStopped:
				app.handleLSPServerStopped(v)
			case *LSPServerRestarted:
				app.handleLSPServerRestarted(v)
			case *OutputLineResult:
				app.LogOutput(v.Level, v.Source, v.Message)
			case *LSPStateChanged:
				app.SyncLanguageSegment()
			case *RepoOpResult:
				app.HandleRepoOpResult(v)
			case *RepositoryStatusResult:
				app.Repository.HandleStatus(v)
				syncStatus()
			case *RepositoryIdentityResult:
				app.Repository.HandleIdentity(v)
				syncStatus()
			case *CurrentChangesResult:
				app.Repository.HandleCurrentChanges(v)
			case *RepositoryInvalidationRequest:
				app.handleRepositoryInvalidation(v)
			case *DebugWriteRequest:
				app.HandleDebugWriteRequest(v)
			case *repositoryDebounceTick:
				app.Repository.HandleDebounce(v)
			case *repositoryIdentityDebounceTick:
				app.Repository.HandleIdentityDebounce(v)
			case *repositoryPollTick:
				app.Repository.HandlePoll(v)
			case *CommitLogResult:
				app.Changes.ApplyCommitLog(v)
			case *CommitFilesResult:
				app.Changes.ApplyCommitFiles(v)
			case *CommitDetailResult:
				app.ApplyCommitDetail(v)
			case *CommitDetailContextResult:
				app.ApplyCommitDetailContext(v)
			case *FileChangedResult:
				app.HandleFileChanged(v.Path)
			case *ExplorerDirChangedResult:
				app.HandleExplorerDirChanged(v.Dir)
			case *ui.SearchBatch:
				app.Search.ApplyBatch(v)
			case *DiffContentResult:
				if v.Err != nil {
					app.StatusError(i18n.Format("status.failedToFetchFileContent", i18n.Args{"error": v.Err.Error()}))
					if dv := app.EditorGroup.DiffWidgetByTab(v.TabName); dv != nil {
						dv.FailLoading()
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
			case *PluginUpdateResult:
				app.HandlePluginUpdateResult(v)
			case *RemoteRegistryResult:
				app.handleRemoteRegistryResult(v)
			case *PrFetchResult:
				if v.Err != nil {
					app.StatusError(i18n.Format("status.prFetchFailed", i18n.Args{"error": v.Err.Error()}))
				} else {
					var files []git.FileStatus
					for _, f := range v.Info.Files {
						files = append(files, git.FileStatus{
							Status: f.Status,
							Path:   f.Path,
						})
					}
					groupName := i18n.Format("status.pr", i18n.Args{"number": v.Info.Number, "infoTitle": v.Info.Title})
					app.Changes.AddPRGroup(groupName, v.URL, v.Info.Owner, v.Info.Repo, v.Info.BaseSHA, v.Info.HeadSHA, v.Info.Body, files, v.Diffs)
					app.Sidebar.SetActivePanel("changes")
					if !app.Sidebar.Visible {
						app.ShowSidebar()
					}
					app.Root.SetFocus(app.Changes.Adapter)
					app.Sidebar.SetPanelDirty("changes", app.Changes.TotalChanges() > 0)
					app.StatusNotify(i18n.Format("status.openedPRFiles", i18n.Args{"number": v.Info.Number, "infoTitle": v.Info.Title, "infoFilesCount": len(v.Info.Files)}))
				}
			}
			if !skipRedraw {
				redraw()
			}
			if execRequest != nil {
				execRequest.complete(execErr)
			}
		}
	}
}

func resizeTerminals(app *App) {
	if !app.ContentSplit.ShowBottom {
		return
	}
	r := app.ContentSplit.BottomRect()
	for _, tt := range app.Terminals {
		tt.Term.AckUpdate()
		content := app.TerminalPanel.ContentRect(ui.Rect{W: r.W, H: r.H - 2}, tt.Widget.ScrollbarVisible())
		if content.W > 0 && content.H > 0 {
			tt.Term.Resize(content.W, content.H)
		}
	}
}
