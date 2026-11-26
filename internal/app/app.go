package app

import (
	"github.com/eugenioenko/ttt/internal/highlight"
	"log/slog"
	"net/url"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/update"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/watcher"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"

	"github.com/gdamore/tcell/v3"
)

type App struct {
	language        languageState
	Root            *ui.Root
	EditorGroup     *ui.EditorGroupWidget // Follows activation so commands target the group receiving keys.
	EditorSplit     *ui.EditorLayoutWidget
	Sidebar         *ui.SidebarWidget
	SplitPanel      *ui.SplitPanelWidget
	ContentSplit    *ui.ContentSplitWidget
	BottomPanel     *ui.BottomPanelWidget
	Search          *ui.SearchWidget
	MenuBar         *ui.MenuBarWidget
	RootBox         *widgets.VStackWidget
	StatusBar       *ui.StatusBarWidget
	Status          *view.StatusBar
	Borders         *term.BorderSet
	Screen          *term.TcellScreen
	Renderer        *render.Renderer
	ImageLayer      *image.Layer
	imageCellW      int
	imageCellH      int
	Settings        *config.Settings
	settingsFile    string
	State           config.State
	Workspace       *workspace.Workspace
	Palette         *ui.TerminalColorPalette
	TerminalPalette interface {
		Snapshot() (string, map[string]string)
		Request()
	}
	TerminalPanel *ui.TerminalPanelWidget
	Terminals     []TerminalTab
	Apps          *AppsPanel
	apps          []config.Application
	appCommandIDs []string
	appsSearch    *asynctask.Task[[]config.Application]
	// appTerminals maps an app name to the terminal it last launched.
	appTerminals map[appLaunchKey]string
	// EditorTerminals live in editor tabs. They are kept apart from Terminals,
	// whose order matches the terminal panel's tabs.
	EditorTerminals []TerminalTab
	// TerminalSeq is monotonic so a closed-and-reopened tab never reuses an id
	// (plugin handles key off it).
	TerminalSeq            int
	forwardSocket          string // UI goroutine only, including listener startup and shutdown.
	LspManager             *lsp.Manager
	lookupCodeActionClient func(string, string) (codeActionCommandClient, error)
	DocVersionsMu          sync.Mutex
	DocVersions            map[string]int
	CompletionItems        []ui.CompletionItem
	LspCompletionItems     []lsp.CompletionItem
	completion             completionState
	hover                  hoverState
	diffOpenTask           *asynctask.Task[*diffOpenResult]
	Problems               *ui.ProblemsWidget
	References             *ui.ReferencesWidget
	Keybindings            []config.KeyBinding
	LspNotified            map[string]bool
	Explorer               *NavigationPanel
	ExplorerContextNode    *widgets.TreeNode
	Changes                *ChangesPanel
	Repository             *RepositoryState
	Symbols                *SymbolsPanel
	symbolsTask            *asynctask.Task[symbolsReply]
	Reg                    *command.Registry
	Running                *bool
	quitPending            bool
	quitAction             func()
	RestartRequested       bool
	RestartMethod          update.Kind
	Updates                UpdateService
	UpdateMethod           *update.Method
	updateDialog           *updateDialogState
	updateStartupTask      *asynctask.Task[update.Release]
	menuReturnFocus        ui.Widget
	Watcher                *watcher.Watcher
	largeFiles             largeFileState
	blame                  statusBlameState
	gitGutterDebounce      *asynctask.Task[struct{}]
	findRefresh            *asynctask.Task[struct{}]
	findStale              bool
	gitGutterTask          *asynctask.Task[*gitGutterResult]
	inlineDiffRepos        map[string]inlineDiffRepo
	commitDetailMu         sync.Mutex
	commitDetailNext       uint64
	commitDetailRequests   map[string]commitDetailRequest
	Version                string
	PluginManager          *plugin.Manager
	PendingPluginApprovals []*plugin.Plugin
	approvalDialog         ui.Widget
	// PendingFileTargets holds cursor positions parsed from `path:line[:col]`
	// arguments. They are applied after the first render, once the viewport
	// has a real height to scroll against.
	PendingFileTargets []FileTarget
	PluginsPanel       *PluginsPanel
	// registryRequested is set once the community registry has been fetched,
	// which waits until the plugins panel is first shown.
	registryRequested bool
	// FetchRegistry replaces the community registry download in tests.
	FetchRegistry func() ([]plugin.RemoteRegistryEntry, error)
	Output        *ui.OutputWidget
	// runningRepoOp is the progress label of the in-flight task, empty when idle.
	runningRepoOp             string
	pluginDetailWidgets       map[string]*pluginDetailState
	pluginDrawer              ui.Widget
	pluginDrawerPanel         *plugin.PluginPanelWidget
	pluginDialog              *pluginDialogState
	graphics                  *cachedGraphics
	fileHandlerTabs           map[string]*plugin.Plugin
	commandLine               *ui.CommandLineWidget
	commandLinePrevFocus      ui.Widget
	settingsView              *settingsView
	pendingCurrentChangesOpen bool
	// appliedSettings is the last value ApplySettings acted on. Callers routinely
	// mutate a.Settings before calling it, so a.Settings cannot serve as "before".
	appliedSettings         config.Settings
	loadedGrammars          []config.GrammarSetting
	restoreSessionOnStartup bool
	sessionWorkspaceKey     string
	sessionSavingPaused     bool
	// welcomeIsEmptyState is set while the welcome page stands in for an
	// empty editor, as opposed to being opened from Help.
	welcomeIsEmptyState bool
	// welcomeWhenEmpty is set once the session reaches the welcome page with no
	// folder, and cleared when a folder opens: only then does closing every tab
	// bring the page back. Loose files opened from the command line still end
	// on an untitled tab.
	welcomeWhenEmpty      bool
	welcomeView           *welcomeView
	panelResizing         bool
	panelResizeFromClosed bool
	eventLoopDoneOnce     sync.Once
	eventLoopCloseOnce    sync.Once
	eventLoopDone         chan struct{}
	tasks                 taskOwner
	quickOpen             *quickOpenState
	fileLinks             *asynctask.Task[map[string]string]
	resolveFileLink       func(string, string) string
	searchReplace         searchReplaceState

	// lspSent holds, per document, a channel closed once its latest queued
	// notification send attempt finishes. Guarded by DocVersionsMu.
	lspSent map[string]<-chan struct{}
	// lspEpochs counts restarts per server. Written on the UI goroutine under
	// DocVersionsMu, so other goroutines must hold the lock to read it.
	lspEpochs map[string]int
	diagPull  *asynctask.Task[pulledDiagnostics]
	// diagBackground holds one task per server.
	diagBackground map[string]*asynctask.Task[[]backgroundPull]
	// lastPulled holds the document state behind each path's applied pull.
	// UI goroutine only.
	lastPulled map[string]pullKey
	// lspDocRoots pins the root each open document was opened on, so its
	// requests and synchronization keep reaching the client that has it open
	// even when a marker change resolves another root for its directory. A
	// close or an explicit server restart drops the pin.
	lspDocRootsMu sync.Mutex
	lspDocRoots   map[string]lspPin
	// lspIdleGens counts each root's pins and idle periods; only the timer of
	// the current generation may stop the root's server. Guarded by
	// lspDocRootsMu.
	lspIdleGens map[lspPin]uint64
	// lspDiagRoots records which root published each path's diagnostics, so
	// an idle root's stop can clear them. UI goroutine only.
	lspDiagRoots map[lspDiagKey]string
}

func (a *App) eventLoopDoneSignal() chan struct{} {
	a.eventLoopDoneOnce.Do(func() {
		a.eventLoopDone = make(chan struct{})
	})
	return a.eventLoopDone
}

func (a *App) closeEventLoopDone() {
	done := a.eventLoopDoneSignal()
	a.eventLoopCloseOnce.Do(func() {
		close(done)
	})
}

func (a *App) KeyFor(cmd string) string {
	for _, kb := range a.Keybindings {
		if kb.Command == cmd {
			return FormatKeyBinding(kb.Key, controlModifiers(a.Settings.Keyboard.ModifierKey))
		}
	}
	return ""
}

func (a *App) FocusEditor() {
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) FocusEditorIfEnabled() {
	if a.Settings.Editor.FocusOnOpen {
		a.Root.SetFocus(a.EditorGroup)
	}
}

func (a *App) refreshWorkspaceWidgets() {
	a.updateEditorSessionWorkspace()
	paths := a.Workspace.Paths()
	a.RememberRecentFolders()
	wasEmpty := len(a.Explorer.Roots) == 0

	a.Explorer.SetRoots(paths)
	switch {
	case len(paths) == 0 && (a.editorIsBlank() || a.welcomeIsEmptyState):
		a.ShowEmptyState()
	case len(paths) == 0:
		// Files are still open: the welcome page waits for them to close.
		a.welcomeWhenEmpty = true
	case wasEmpty:
		a.welcomeWhenEmpty = false
		a.closeWelcome()
		a.ShowSidebar()
	}

	a.Search.SetWorkDirs(paths)
	if a.Repository != nil {
		a.Repository.SetDirs(paths)
	} else {
		a.Changes.SetDirs(paths)
	}
}

func (a *App) refreshProblems() {
	var items []ui.ProblemItem
	for path, diags := range a.EditorGroup.DiagnosticsByPath() {
		for _, d := range diags {
			items = append(items, ui.ProblemItem{
				File:     path,
				Line:     d.StartLine,
				Col:      d.StartCol,
				Severity: d.Severity,
				Message:  d.Message,
				Source:   d.Source,
			})
		}
	}
	a.Problems.SetItems(items)
}

func (a *App) Init(screen *term.TcellScreen, renderer *render.Renderer, lspManager *lsp.Manager, imageLayer *image.Layer) {
	a.Screen = screen
	if a.Screen != nil && a.Settings != nil {
		a.Screen.SetPointerShapesEnabled(a.Settings.Appearance.PointerShapesEnabled())
	}
	a.initTaskDelivery(screen)
	a.Renderer = renderer
	a.LspManager = lspManager
	a.ImageLayer = imageLayer
	if a.Root != nil {
		a.Root.ImageLayer = imageLayer
	}
	if imageLayer != nil {
		imageLayer.SetLockFunc(func(r image.Rect, locked bool) {
			if a.Screen != nil {
				a.Screen.LockRegion(r.X, r.Y, r.W, r.H, locked)
			}
		})
	}
	a.EditorGroup.SetImageRedraw(func() {
		_ = screen.PostEvent(tcell.NewEventInterrupt(nil))
	})
	a.RefreshImageCellSize()
	a.wireEditorAutoScroll(a.EditorGroup)
	highlight.SetStepScheduler(func() {
		_ = screen.PostEvent(tcell.NewEventInterrupt(&highlightStep{}))
	})
	a.StartWatcher()

	if a.Changes != nil {
		a.Changes.Screen = screen
		a.Changes.OnRefreshed = a.onChangesRefreshed
	}
	if a.Repository != nil {
		a.Repository.SetPoster(screen)
		a.Repository.Start()
		a.syncRepositoryObservation()
	}

	a.EditorGroup.OnError = func(msg string) {
		a.StatusError(msg)
	}
	a.EditorGroup.OnNotify = func(msg string) {
		a.StatusNotify(msg)
	}
	a.EditorGroup.FlushNotifications()
	a.EditorGroup.OnFileOpen = func(path, lang string, lines []string) {
		a.NotifyLSPOpen(path, lang, lines)
		a.RequestGitGutterForActiveFile()
		if a.PluginManager != nil {
			a.PluginManager.DispatchEvent("file.open", path)
		}
	}
	a.EditorGroup.OnFileClose = func(path, lang string) {
		a.NotifyLSPClose(path, lang)
		if a.PluginManager != nil {
			a.PluginManager.DispatchEvent("file.close", path)
		}
	}
	a.EditorGroup.OnContentTabClose = func(id string) {
		a.cleanupPluginDetailTab(id)
		a.cleanupSettingsTab(id)
		a.closeFileHandlerTab(id)
	}
	if path := a.EditorGroup.ActiveFilePath(); path != "" {
		if lang := a.EditorGroup.ActiveLanguage(); lang != "" {
			a.NotifyLSPOpen(path, lang, a.EditorGroup.Editor.Buf.Lines)
		}
	}
	a.Problems.OnNavigate = a.navigateToLocation
	a.References.OnNavigate = a.navigateToLocation
	a.EditorGroup.Editor.OnChange = func() {
		path, lang := a.editorPathLang()
		changeDone := a.NotifyLSPChange(path, lang, a.EditorGroup.Editor.Buf.Lines)
		a.cancelBackgroundPull(path, lang)
		a.ScheduleDiagnosticPull(false)
		a.ScheduleAutocomplete()
		a.CheckSignatureHelpTrigger(changeDone)
		a.ScheduleGitGutter()
		a.scheduleFindRefresh()
		if a.PluginManager != nil {
			a.PluginManager.DispatchEvent("editor.change", path)
		}
	}
	a.EditorGroup.OnBookmarkChanged = func(path string, line int, action string, icon rune, style term.Style) {
		if a.PluginManager != nil {
			a.PluginManager.DispatchEvent("bookmark.change", path, line+1, action, string(icon), plugin.NameByStyle(style))
		}
	}
	a.EditorGroup.OnGutterClick = func(path string, line int) bool {
		if a.PluginManager == nil || !a.PluginManager.HasEventListeners("gutter.click") {
			return false
		}
		a.PluginManager.DispatchEvent("gutter.click", path, line+1)
		return true
	}

	lspManager.OnLog = func(server, level, message string) {
		a.LogOutputAsync(level, "lsp:"+server, message)
	}

	lspManager.OnApplyEdit = func(_ string, edit lsp.WorkspaceEdit) lsp.ApplyWorkspaceEditResult {
		result := make(chan lsp.ApplyWorkspaceEditResult, 1)
		lifecycle := newExecRequestLifecycle()
		request := &LSPApplyEditRequest{Edit: edit, Result: result, Lifecycle: lifecycle}
		if err := screen.PostEvent(tcell.NewEventInterrupt(request)); err != nil {
			return lsp.ApplyWorkspaceEditResult{FailureReason: err.Error()}
		}
		select {
		case applied := <-result:
			return applied
		case <-time.After(10 * time.Second):
			if lifecycle.cancel() {
				return lsp.ApplyWorkspaceEditResult{FailureReason: "timed out waiting to apply workspace edit"}
			}
			// The UI claimed the request, so let its edit finish and report the result.
			return <-result
		}
	}

	// A server can die at any time; waking the loop redraws the status bar so
	// the indicator does not keep claiming the server is up.
	lspManager.OnStateChange = func() {
		_ = screen.PostEvent(tcell.NewEventInterrupt(&LSPStateChanged{}))
	}

	lspManager.OnDiagnosticRefresh = func(server string) {
		_ = screen.PostEvent(tcell.NewEventInterrupt(&LSPDiagnosticRefresh{Server: server}))
	}

	lspManager.OnDiagnostics = func(server, root string, params lsp.PublishDiagnosticsParams) {
		path := URIToPath(params.URI)
		diags := params.Diagnostics
		slog.Debug("lsp diagnostics", "path", path, "count", len(diags))
		_ = screen.PostEvent(tcell.NewEventInterrupt(&DiagnosticsResult{
			Server:      server,
			Path:        path,
			Root:        root,
			Diagnostics: diags,
		}))
	}
}

func OpenURL(rawURL string) {
	cmd := openURLCommand(rawURL, runtime.GOOS)
	if cmd == nil {
		return
	}
	if err := cmd.Start(); err != nil {
		slog.Error("open URL", "url", rawURL, "error", err)
		return
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			slog.Error("open URL", "url", rawURL, "error", err)
		}
	}()
}

func openURLCommand(rawURL, platform string) *exec.Cmd {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil
	}
	var cmd *exec.Cmd
	switch platform {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd
}

func (a *App) FlushEditorOnChange() {
	a.EditorGroup.SyncDocumentViews()
	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.FlushOnChange()
	}
}
