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
	Root     *ui.Root
	RootBox  *widgets.VStackWidget
	Screen   *term.TcellScreen
	Renderer *render.Renderer
	Borders  *term.BorderSet

	MenuBar              *ui.MenuBarWidget
	menuReturnFocus      ui.Widget
	StatusBar            *ui.StatusBarWidget
	Status               *view.StatusBar
	Output               *ui.OutputWidget
	Reg                  *command.Registry
	Keybindings          []config.KeyBinding
	commandLine          *ui.CommandLineWidget
	commandLinePrevFocus ui.Widget

	Settings       *config.Settings
	settingsFile   string
	State          config.State
	settingsView   *settingsView
	language       languageState
	loadedGrammars []config.GrammarSetting
	// Last applied snapshot. Callers may mutate Settings before ApplySettings.
	appliedSettings config.Settings

	Sidebar               *ui.SidebarWidget
	SplitPanel            *ui.SplitPanelWidget
	ContentSplit          *ui.ContentSplitWidget
	BottomPanel           *ui.BottomPanelWidget
	panelResizing         bool
	panelResizeFromClosed bool

	Workspace           *workspace.Workspace
	Watcher             *watcher.Watcher
	Explorer            *NavigationPanel
	ExplorerContextNode *widgets.TreeNode

	EditorGroup     *ui.EditorGroupWidget // Tracks the active group for editor commands.
	EditorSplit     *ui.EditorLayoutWidget
	largeFiles      largeFileState
	fileLinks       *asynctask.Task[map[string]string]
	resolveFileLink func(string, string) string

	// Apply `path:line[:col]` targets after the first render, when viewport height is known.
	PendingFileTargets      []FileTarget
	sessionHasExplicitFiles bool
	sessionWorkspaceKey     string
	sessionSavingPaused     bool

	welcomeView         *welcomeView
	welcomeIsEmptyState bool // Welcome replaces the empty editor.
	// Return to Welcome when the last tab closes in a workspace with no folders.
	// Opening a folder clears this. Loose-file launches end on an untitled tab.
	welcomeWhenEmpty bool

	ImageLayer *image.Layer
	imageCellW int
	imageCellH int
	graphics   *cachedGraphics

	Search        *ui.SearchWidget
	searchReplace searchReplaceState
	findRefresh   *asynctask.Task[struct{}]
	findStale     bool
	quickOpen     *quickOpenState

	Changes                   *ChangesPanel
	Repository                *RepositoryState
	runningRepoOp             string // Progress label, empty when idle.
	pendingCurrentChangesOpen bool
	diffOpenTask              *asynctask.Task[*diffOpenResult]
	inlineDiffRepos           map[string]inlineDiffRepo
	blame                     statusBlameState
	gitGutterDebounce         *asynctask.Task[struct{}]
	gitGutterTask             *asynctask.Task[*gitGutterResult]
	commitDetailMu            sync.Mutex
	commitDetailNext          uint64
	commitDetailRequests      map[string]commitDetailRequest

	Palette         *ui.TerminalColorPalette
	TerminalPalette interface {
		Snapshot() (string, map[string]string)
		Request()
	}
	TerminalPanel   *ui.TerminalPanelWidget
	Terminals       []TerminalTab // Matches TerminalPanel tab order.
	EditorTerminals []TerminalTab
	TerminalSeq     int    // Monotonic to keep plugin terminal handles unique.
	forwardSocket   string // UI goroutine only, including listener startup and shutdown.

	Apps          *AppsPanel
	apps          []config.Application
	appCommandIDs []string
	appsSearch    *asynctask.Task[[]config.Application]
	appTerminals  map[appLaunchKey]string // Last terminal launched for each app and path.

	PluginManager          *plugin.Manager
	PluginsPanel           *PluginsPanel
	PendingPluginApprovals []*plugin.Plugin
	approvalDialog         ui.Widget
	pluginDetailWidgets    map[string]*pluginDetailState
	pluginDrawer           ui.Widget
	pluginDrawerPanel      *plugin.PluginPanelWidget
	pluginDialog           *pluginDialogState
	fileHandlerTabs        map[string]*plugin.Plugin
	// Set on the first registry request, usually when the Plugins panel opens.
	registryRequested bool
	FetchRegistry     func() ([]plugin.RemoteRegistryEntry, error) // Test hook for registry downloads.

	Symbols            *SymbolsPanel
	symbolsTask        *asynctask.Task[symbolsReply]
	CompletionItems    []ui.CompletionItem
	LspCompletionItems []lsp.CompletionItem
	completion         completionState
	hover              hoverState

	LspManager             *lsp.Manager
	LspNotified            map[string]bool
	lookupCodeActionClient func(string, string) (codeActionCommandClient, error)
	References             *ui.ReferencesWidget
	Problems               *ui.ProblemsWidget
	diagPull               *asynctask.Task[pulledDiagnostics]
	diagBackground         map[string]*asynctask.Task[[]backgroundPull] // One task per server.
	// State of the last applied pull per path. UI goroutine only.
	lastPulled map[string]pullKey
	// Publishing roots for idle diagnostic cleanup. UI goroutine only.
	lspDiagRoots map[lspDiagKey]string

	DocVersionsMu sync.Mutex // Protects DocVersions, lspSent, and lspEpochs.
	DocVersions   map[string]int
	// Per-document channel closes after the latest queued send attempt.
	lspSent map[string]<-chan struct{}
	// Per-server restart counts. UI goroutine writes only.
	lspEpochs map[string]int

	lspDocRootsMu sync.Mutex // Protects document pins and idle generations.
	// Pins open documents to their LSP roots across marker changes.
	// Released on document close or explicit server restart.
	lspDocRoots map[string]lspPin
	// Bumped on pin or idle scheduling. Only the current generation can stop the server.
	lspIdleGens map[lspPin]uint64

	Running            *bool
	quitPending        bool
	quitAction         func()
	eventLoopDoneOnce  sync.Once
	eventLoopCloseOnce sync.Once
	eventLoopDone      chan struct{}
	tasks              taskOwner

	Version           string
	RestartRequested  bool
	RestartMethod     update.Kind
	Updates           UpdateService
	UpdateMethod      *update.Method
	updateDialog      *updateDialogState
	updateStartupTask *asynctask.Task[update.Release]
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
