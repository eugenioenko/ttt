package app

import (
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v2"
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
	lastBranchDir := app.Workspace.Primary()
	blameGen := 0
	app.Status.Branch = git.BranchName(lastBranchDir)
	app.Status.TabSize = app.Settings.Editor.TabSize

	syncStatus := func() {
		line, col := app.EditorGroup.ActiveCursor()
		filePath := app.EditorGroup.ActiveFilePath()
		app.Status.FileName = filePath
		app.Status.Line = line
		app.Status.Col = col
		app.Status.Dirty = app.EditorGroup.IsDirty()
		app.Status.CursorCount = app.EditorGroup.MultiCursorCount()
		if buf := app.EditorGroup.ActiveBuffer(); buf != nil {
			app.Status.LineEnding = buf.LineEnding
		} else {
			app.Status.LineEnding = "\n"
		}
		app.Explorer.ActiveFile = filePath
		app.SyncWatched()

		if app.EditorGroup.Editor != nil && app.EditorGroup.Editor.Highlighter != nil {
			lang := app.EditorGroup.Editor.Highlighter.Language()
			app.Status.Language = lang
			serverKey, _, lspOk := app.lspResolve(filePath, lang)
			if lspOk {
				serverCfg := app.LspManager.ServerConfig(serverKey)
				if len(serverCfg.Command) > 0 {
					_, err := exec.LookPath(serverCfg.Command[0])
					lspOk = err == nil
				}
			}
			app.Status.LSP = lspOk
		} else {
			app.Status.Language = ""
			app.Status.LSP = false
		}
		if app.EditorGroup.Editor != nil && app.EditorGroup.Editor.TabSize > 0 {
			app.Status.TabSize = app.EditorGroup.Editor.TabSize
			app.Status.UseTabs = app.EditorGroup.Editor.UseTabs
		} else {
			app.Status.TabSize = app.Settings.Editor.TabSize
			app.Status.UseTabs = !app.Settings.Editor.InsertSpaces
		}

		repoDir := ""
		if filePath != "" && filePath != "untitled" {
			if folder := app.Workspace.FolderForFile(filePath); folder != nil && folder.IsRepo {
				repoDir = folder.Path
			}
		}

		if repoDir != lastBranchDir {
			lastBranchDir = repoDir
			if repoDir != "" {
				app.Status.Branch = git.BranchName(repoDir)
			} else {
				app.Status.Branch = ""
			}
		}

		// Trigger git gutter computation when switching to a new file
		if filePath != lastGutterFile {
			lastGutterFile = filePath
			app.RequestGitGutterForActiveFile()
		}

		if filePath != lastBlameFile || line != lastBlameLine {
			lastBlameFile = filePath
			lastBlameLine = line
			app.Status.Blame = ""
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

	for *running {
		ev := screen.PollEvent()
		switch tev := ev.(type) {
		case *tcell.EventKey:
			app.DismissHover()
			if app.EditorGroup.SignatureHelp != nil {
				switch tev.Key() {
				case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight,
					tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
					app.DismissSignatureHelp()
				}
			}
			slog.Debug("key", "key", tev.Key(), "rune", string(tev.Rune()), "mod", tev.Modifiers())
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
					app.Status.Blame = fmt.Sprintf("%s, %s",
						v.Info.Author, git.FormatRelativeTime(v.Info.Time))
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
			case *FormattingResult:
				if len(v.Edits) > 0 {
					app.ApplyTextEdits(v.Edits)
				}
			case *DiagnosticsResult:
				app.EditorGroup.SetDiagnostics(v.Path, v.Diagnostics)
				if len(v.Diagnostics) == 0 {
					delete(app.AllDiagnostics, v.Path)
				} else {
					app.AllDiagnostics[v.Path] = v.Diagnostics
				}
				app.refreshProblems()
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
			case *PRCommentsResult:
				if v.Err != nil {
					app.StatusError("Failed to fetch PR comments: " + v.Err.Error())
					if app.CommentPanel != nil {
						app.CommentPanel.Loading = false
					}
				} else {
					if app.CommentPanel != nil {
						var items []ui.CommentItem
						for _, c := range v.Comments {
							items = append(items, ui.CommentItem{
								ID:        c.ID,
								Author:    c.User,
								Timestamp: c.CreatedAt,
								Body:      c.Body,
								FilePath:  c.Path,
								Line:      c.Line,
								IsInline:  c.IsInline,
								InReplyTo: c.InReplyTo,
							})
						}
						app.CommentPanel.SetComments(items)
						count := len(items)
						app.StatusNotify(fmt.Sprintf("Loaded %d comment(s)", count))
					}
				}
			case *PRCommentSubmitResult:
				if v.Err != nil {
					app.StatusError("Failed to submit comment: " + v.Err.Error())
				} else {
					app.StatusNotify("Comment submitted")
					// Refresh comments
					app.FetchPRComments(v.Owner, v.Repo, v.Number, v.Title)
				}
			case *PrFetchResult:
				app.Changes.Loading = false
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
					app.Root.SetFocus(app.Changes)
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
