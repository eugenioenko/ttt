package app

import (
	"fmt"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/update"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/watcher"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"

	"github.com/gdamore/tcell/v3"
)

const hoverGraceMargin = 3

type TerminalTab struct {
	ID     string
	Label  string
	Term   *terminal.Terminal
	Widget *ui.TerminalWidget
	// Tab is set when the terminal lives in an editor tab instead of the panel.
	Tab *ui.TerminalTabWidget
}

// TerminalSpec describes the program one terminal tab runs. The zero value
// inherits the global terminal settings and the workspace directory.
type TerminalSpec struct {
	Shell string
	Args  []string
	// Env are extra KEY=VALUE entries for the child (the process environment is
	// inherited; these are appended).
	Env []string
	Cwd string
	// Label is a short badge for the strip ("pi", "lg") and the tab record, or
	// the editor tab's title.
	Label    string
	InEditor bool
	// KeepOpen leaves the tab, with the program's last output, after it exits.
	KeepOpen bool
	// Nil uses the global padding; a pointer to zero disables padding.
	PaddingX *int
}

type App struct {
	language        languageState
	Root            *ui.Root
	EditorGroup     *ui.EditorGroupWidget
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
	Output             *ui.OutputWidget
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
	appliedSettings config.Settings
	loadedGrammars  []config.GrammarSetting
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

	// lspSent holds, per document, a channel closed once its latest didOpen or
	// didChange was sent. Guarded by DocVersionsMu.
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

func (a *App) ShowSidebar() {
	a.showSidebar()
	a.persistSidebarLayout()
}

func (a *App) showSidebar() {
	a.Sidebar.Visible = true
	a.SplitPanel.ShowLeft = true
	if a.SplitPanel.DividerPos < ui.MinSidebarWidth {
		a.SplitPanel.DividerPos = ui.DefaultSidebarWidth
	}
	a.applySearchHighlights()
	a.syncRepositoryObservation()
}

func (a *App) HideSidebar() {
	a.hideSidebar()
	a.persistSidebarLayout()
}

func (a *App) hideSidebar() {
	// Else keys keep going to the hidden panel. Checked before Visible flips.
	if a.focusedRegion() == "sidebar" {
		a.FocusEditor()
	}
	a.Sidebar.InvalidatePointerInteraction()
	a.Sidebar.Visible = false
	a.SplitPanel.ShowLeft = false
	a.EditorGroup.ClearSearch()
	a.syncRepositoryObservation()
}

func (a *App) applySearchHighlights() {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	if a.Sidebar.ActivePanel == "search" && a.Search.Input.Text() != "" {
		matches, _ := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, a.Search.Input.Text(), a.Search.Options)
		a.EditorGroup.SetSearch(a.Search.Input.Text(), a.Search.Options, matches)
	} else {
		a.EditorGroup.Editor.RefreshSearch()
	}
}

func (a *App) ShowPanel(id string, widget ui.Widget) {
	a.Sidebar.SetActivePanel(id)
	if !a.Sidebar.Visible {
		a.ShowSidebar()
	}
	a.Root.SetFocus(widget)
}

func (a *App) ToggleSidebar() {
	if a.Sidebar.Visible {
		a.HideSidebar()
	} else {
		a.ShowSidebar()
	}
}

func (a *App) SetSidebarWidth(w int) {
	a.resizeSidebar(w)
	a.persistSidebarLayout()
}

func (a *App) resizeSidebar(w int) {
	if w <= 0 {
		a.hideSidebar()
		return
	}
	if !a.Sidebar.Visible {
		a.showSidebar()
	}
	a.SplitPanel.DividerPos = w
}

// Widths below MinSidebarWidth are not saved: dragging the sidebar closed
// passes through them, and restoring one would reopen it as a sliver.
func (a *App) persistSidebarLayout() {
	next := a.State
	next.SidebarHidden = !a.Sidebar.Visible
	if w := a.SplitPanel.DividerPos; w >= ui.MinSidebarWidth {
		next.SidebarWidth = w
	}
	if next.SidebarHidden == a.State.SidebarHidden && next.SidebarWidth == a.State.SidebarWidth {
		return
	}
	a.State = next
	if err := config.SaveState(a.State); err != nil {
		a.StatusError(i18n.Format("status.failedToSaveSidebarLayout", i18n.Args{"error": err.Error()}))
	}
}

// The panel's new rect only exists after the next render, so the terminal
// resize rides along with the caller's redraw.
func (a *App) SetPanelPosition(pos ui.SplitPosition) {
	if a.ContentSplit.Position == pos {
		return
	}
	a.ContentSplit.Position = pos

	name := "bottom"
	if pos == ui.SplitRight {
		name = "right"
	}
	a.State.PanelPosition = name
	if err := config.SaveState(a.State); err != nil {
		a.StatusError(i18n.Format("status.failedToSavePanelPosition", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) FocusEditor() {
	if split := a.ContentSplit; split != nil && split.ShowBottom && split.Position == ui.SplitBottom {
		height := split.GetRect().H
		if height > 0 && height-split.BottomRect().H-1 < ui.MinEditorGroupHeight {
			previousHeight := split.BottomH
			split.BottomH = height / 2
			if height-split.BottomRect().H-1 < ui.MinEditorGroupHeight {
				split.ShowBottom = false
				split.BottomH = previousHeight
			}
			resizeTerminals(a)
		}
	}
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) FocusEditorIfEnabled() {
	if a.Settings.Editor.FocusOnOpen {
		a.FocusEditor()
	}
}

func (a *App) FocusSidebar() {
	if !a.Sidebar.Visible {
		a.ShowSidebar()
	}
	if w := a.Sidebar.ActiveWidget(); w != nil {
		a.Root.SetFocus(w)
	}
}

func (a *App) ensureUsablePanelSize() {
	r := a.ContentSplit.GetRect()
	maxH := r.H - 4
	if a.ContentSplit.BottomH < ui.MinPanelHeight || a.ContentSplit.BottomH > maxH {
		a.ContentSplit.BottomH = min(r.H/2, maxH)
	}
	if a.ContentSplit.RightW < ui.MinPanelWidth {
		a.ContentSplit.RightW = ui.DefaultPanelWidth
	}
}

func (a *App) resizePanel(size int) {
	if !a.panelResizing {
		a.panelResizing = true
		a.panelResizeFromClosed = !a.ContentSplit.ShowBottom
	}
	if size <= 0 {
		a.ContentSplit.ShowBottom = false
		return
	}
	a.ContentSplit.ShowBottom = true
	if a.ContentSplit.Position == ui.SplitRight {
		a.ContentSplit.RightW = size
	} else {
		a.ContentSplit.BottomH = size
	}
	resizeTerminals(a)
}

// Spawning a terminal moves focus, and a focus change cancels the pointer
// capture, so it must wait until the drag is over.
func (a *App) finishPanelResize() {
	fromClosed := a.panelResizeFromClosed
	a.panelResizing = false
	a.panelResizeFromClosed = false
	if fromClosed && a.ContentSplit.ShowBottom && len(a.Terminals) == 0 {
		a.SpawnTerminal()
	}
	a.persistPanelSize()
}

func (a *App) persistPanelSize() {
	next := a.State
	if h := a.ContentSplit.BottomH; h >= ui.MinPanelHeight {
		next.PanelHeight = h
	}
	if w := a.ContentSplit.RightW; w >= ui.MinPanelWidth {
		next.PanelWidth = w
	}
	if next.PanelHeight == a.State.PanelHeight && next.PanelWidth == a.State.PanelWidth {
		return
	}
	a.State = next
	if err := config.SaveState(a.State); err != nil {
		a.StatusError(i18n.Format("status.failedToSavePanelSize", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) ShowBottomPanel() {
	a.ContentSplit.ShowBottom = true
}

func (a *App) showTerminalPanel() {
	a.ContentSplit.ShowBottom = true
	if len(a.Terminals) == 0 {
		a.SpawnTerminal()
	} else {
		a.BottomPanel.SetActivePanel("terminal")
		a.Root.SetFocus(a.TerminalPanel)
	}
}

func (a *App) HideBottomPanel() {
	a.ContentSplit.ShowBottom = false
	a.FocusEditor()
}

func (a *App) ToggleBottomPanel() {
	if a.ContentSplit.ShowBottom {
		a.HideBottomPanel()
	} else {
		a.ShowBottomPanel()
	}
}

// SpawnTerminal opens a terminal tab running the configured shell.
func (a *App) SpawnTerminal() {
	a.SpawnTerminalSpec(TerminalSpec{})
}

// SpawnTerminalSpec opens a terminal tab for a specific program. A caller that
// names a Shell supplies its own Args (empty = run the program bare); an empty
// Shell inherits both from the global terminal settings.
func (a *App) SpawnTerminalSpec(spec TerminalSpec) (string, error) {
	tw := ui.NewTerminalWidget(nil, a.Palette)
	paddingX := a.Settings.Terminal.PaddingX
	if spec.PaddingX != nil {
		paddingX = max(*spec.PaddingX, 0)
		tw.PaddingX = &paddingX
	}
	var tab *ui.TerminalTabWidget
	var content ui.Rect
	if spec.InEditor {
		tab = ui.NewTerminalTabWidget(tw)
		tab.PaddingX = paddingX
		tab.SetRect(a.EditorGroup.ContentRect())
		content = tab.ContentRect()
	} else {
		r := a.ContentSplit.BottomRect()
		content = a.TerminalPanel.ContentRectFor(tw, ui.Rect{W: r.W, H: r.H - 2}, false)
	}
	cols, rows := content.W, content.H
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	if spec.Shell == "" {
		spec.Shell = a.Settings.Terminal.Shell
		spec.Args = a.Settings.Terminal.Args
		if a.Settings.Terminal.ShellIntegration {
			spec.Shell, spec.Args = terminal.IntegrateShell(spec.Shell, spec.Args)
		}
	}
	if spec.Cwd == "" {
		spec.Cwd = a.Workspace.Primary()
	} else if err := checkTerminalDir(spec.Cwd); err != nil {
		a.StatusError(i18n.Format("status.failedToOpenTerminal", i18n.Args{"error": err.Error()}))
		return "", err
	}
	env := slices.Concat(spec.Env, []string{forwardSocketEnv + "=" + a.forwardSocket})
	t, err := terminal.New(spec.Shell, cols, rows, a.Settings.Terminal.Scrollback, env, spec.Cwd, spec.Args...)
	if err != nil {
		slog.Error("terminal.New", "err", err)
		a.StatusError(i18n.Format("status.failedToOpenTerminal", i18n.Args{"error": err.Error()}))
		return "", err
	}

	tw.Term = t
	tw.WorkDir = spec.Cwd
	tw.CtrlVPastes = a.Settings.Terminal.CtrlVPastes
	tw.SuperShortcuts = a.Root.ControlModifiers&tcell.ModMeta != 0
	tw.OnOpenURL = OpenURL
	tw.OnOpenFile = a.openFileLink

	panelID := fmt.Sprintf("terminal-%d", a.TerminalSeq)
	a.TerminalSeq++
	entry := TerminalTab{ID: panelID, Label: spec.Label, Term: t, Widget: tw, Tab: tab}
	if tab != nil {
		a.openEditorTerminal(entry, spec)
	} else {
		a.Terminals = append(a.Terminals, entry)
		a.TerminalPanel.AddTerminalLabeled(tw, spec.Label)
		a.BottomPanel.SetActivePanel("terminal")

		if !a.ContentSplit.ShowBottom {
			a.ContentSplit.ShowBottom = true
		}
		a.Root.SetFocus(a.TerminalPanel)
	}

	t.OnUpdate = func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	t.OnExit = func() {
		if spec.KeepOpen {
			a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
			return
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(panelID))
	}
	t.Run()
	return panelID, nil
}

// checkTerminalDir rejects an explicit cwd that is missing or not a directory;
// terminal.New would otherwise fall back to the process directory.
func checkTerminalDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("terminal cwd: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("terminal cwd %s: not a directory", dir)
	}
	return nil
}

// ---- terminal handles for plugins (the ttt.terminal Lua module) ----

// SendToTerminal writes text into a tab's PTY. Actions run on the main thread,
// so no extra synchronisation is needed.
func (a *App) SendToTerminal(id, text string) error {
	if tt := a.terminalByID(id); tt != nil {
		tt.Term.WriteString(text)
		return nil
	}
	return fmt.Errorf("no terminal %q", id)
}

// TerminalTail returns the last maxBytes of a tab's raw PTY output.
func (a *App) TerminalTail(id string, maxBytes int) (string, error) {
	if tt := a.terminalByID(id); tt != nil {
		raw := tt.Term.RawTail()
		if maxBytes > 0 && len(raw) > maxBytes {
			raw = raw[len(raw)-maxBytes:]
		}
		return string(raw), nil
	}
	return "", fmt.Errorf("no terminal %q", id)
}

// FocusTerminal reveals the terminal's tab and focuses it.
func (a *App) FocusTerminal(id string) error {
	if a.focusEditorTerminal(id) {
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
		return nil
	}
	idx := -1
	for i, tt := range a.Terminals {
		if tt.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("no terminal %q", id)
	}
	if !a.ContentSplit.ShowBottom {
		a.ContentSplit.ShowBottom = true
	}
	a.TerminalPanel.SetActive(idx)
	a.BottomPanel.SetActivePanel("terminal")
	a.Root.SetFocus(a.TerminalPanel)
	a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	return nil
}

// TerminalScreen returns the visible screen of one tab as plain text.
func (a *App) TerminalScreen(id string) (string, error) {
	if tt := a.terminalByID(id); tt != nil {
		return tt.Term.ScreenText(), nil
	}
	return "", fmt.Errorf("terminal %q not found", id)
}

// ListTerminals reports the open tabs to a plugin.
func (a *App) ListTerminals() []plugin.TerminalInfo {
	terms := a.allTerminals()
	out := make([]plugin.TerminalInfo, 0, len(terms))
	for _, tt := range terms {
		cols, rows := tt.Term.Size()
		out = append(out, plugin.TerminalInfo{ID: tt.ID, Label: tt.Label, Cols: cols, Rows: rows, Location: tt.location()})
	}
	return out
}

func (a *App) CloseTerminal(panelID string) {
	if a.closeEditorTerminal(panelID) {
		return
	}
	idx := -1
	for i, tt := range a.Terminals {
		if tt.ID == panelID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	a.Terminals[idx].Term.Close()
	a.forgetAppTerminal(panelID)
	a.Terminals = append(a.Terminals[:idx], a.Terminals[idx+1:]...)
	a.TerminalPanel.RemoveTerminal(idx)

	if a.TerminalPanel.Count() == 0 {
		a.hideBottomPanelIfTerminalActive()
	} else {
		a.Root.SetFocus(a.TerminalPanel)
	}
}

func (a *App) CloseActiveTerminal() {
	if focused := a.focusedTerminalWidget(); focused != nil {
		for _, terminal := range a.EditorTerminals {
			if terminal.Widget == focused {
				a.CloseTerminal(terminal.ID)
				return
			}
		}
	}
	a.ClosePanelTerminal()
}

func (a *App) ClosePanelTerminal() {
	idx := a.TerminalPanel.ActiveIndex()
	if idx < 0 || idx >= len(a.Terminals) {
		return
	}
	a.CloseTerminal(a.Terminals[idx].ID)
}

// hideBottomPanelIfTerminalActive collapses the bottom panel once the last
// terminal is gone, so `exit` in the shell closes the panel instead of leaving
// a dead one on screen. The panel also hosts Problems, References and Output,
// so it only collapses when the terminal is the tab actually being shown.
func (a *App) hideBottomPanelIfTerminalActive() {
	if a.BottomPanel != nil && a.BottomPanel.ActivePanel != "terminal" {
		a.FocusEditor()
		return
	}
	a.HideBottomPanel()
}

func (a *App) CloseAllTerminals() {
	terms := a.Terminals
	a.Terminals = nil
	for i := len(terms) - 1; i >= 0; i-- {
		a.TerminalPanel.RemoveTerminal(i)
	}
	for _, tt := range terms {
		tt.Term.Close()
		a.forgetAppTerminal(tt.ID)
	}
	a.hideBottomPanelIfTerminalActive()
}

func (a *App) refreshWorkspaceWidgets() {
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

func (a *App) checkMouseHover(mx, my int) {
	if a.EditorGroup.Editor == nil || !a.Settings.LSP.IsHoverEnabled() {
		a.DismissHover()
		return
	}
	line, col, ok := a.EditorGroup.Editor.TextPosAt(mx, my)
	if !ok {
		a.DismissHover()
		return
	}
	path, lang := a.editorPathLang()
	a.scheduleHover(path, lang, line, col, mx, my, a.hoverDiagnostic(line, col))
}

func (a *App) ShowHover(text string, anchorX, anchorY int) {
	if text != "" {
		a.EditorGroup.Hover = ui.NewHoverWidget(text, anchorX, anchorY, &a.Root.ControlModifiers)
	}
}

func (a *App) isMouseOverHover(mx, my int) bool {
	h := a.EditorGroup.Hover
	if h == nil {
		return false
	}
	r := h.GetRect()
	m := hoverGraceMargin
	return mx >= r.X-m && mx < r.X+r.W+m && my >= r.Y-m && my < r.Y+r.H+m
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
		screen.PostEvent(tcell.NewEventInterrupt(nil))
	})
	a.RefreshImageCellSize()
	a.EditorGroup.TabBar.PostDragAutoScrollTick = func(generation uint64) {
		screen.PostEvent(tcell.NewEventInterrupt(&ui.TabDragAutoScrollTick{Generation: generation}))
	}
	a.EditorGroup.Editor.PostDragAutoScrollTick = func(generation uint64) {
		screen.PostEvent(tcell.NewEventInterrupt(&ui.EditorDragAutoScrollTick{Generation: generation}))
	}
	highlight.SetStepScheduler(func() {
		screen.PostEvent(tcell.NewEventInterrupt(&highlightStep{}))
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
	a.Problems.OnNavigate = func(file string, line, col int) {
		a.EditorGroup.OpenFile(file)
		a.EditorGroup.GoToLine(line + 1)
		a.FocusEditor()
	}
	a.References.OnNavigate = func(file string, line, col int) {
		a.EditorGroup.OpenFile(file)
		a.EditorGroup.GoToLine(line + 1)
		a.FocusEditor()
	}
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
		screen.PostEvent(tcell.NewEventInterrupt(&LSPStateChanged{}))
	}

	lspManager.OnDiagnosticRefresh = func(server string) {
		screen.PostEvent(tcell.NewEventInterrupt(&LSPDiagnosticRefresh{Server: server}))
	}

	lspManager.OnDiagnostics = func(server string, params lsp.PublishDiagnosticsParams) {
		path := URIToPath(params.URI)
		diags := params.Diagnostics
		slog.Debug("lsp diagnostics", "path", path, "count", len(diags))
		screen.PostEvent(tcell.NewEventInterrupt(&DiagnosticsResult{
			Server:      server,
			Path:        path,
			Diagnostics: diags,
		}))
	}
}

func (a *App) statusMessage(msg string, level view.NotifyLevel) {
	a.Status.SetNotification(msg, level, 5*time.Second)
	a.LogOutput(outputLevelForNotify(level), "notice", msg)
	time.AfterFunc(5*time.Second, func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	})
}

func (a *App) StatusNotify(msg string) { a.statusMessage(msg, view.NotifyInfo) }
func (a *App) StatusWarn(msg string)   { a.statusMessage(msg, view.NotifyWarning) }
func (a *App) StatusError(msg string)  { a.statusMessage(msg, view.NotifyError) }

func OpenURL(rawURL string) {
	cmd := openURLCommand(rawURL, runtime.GOOS)
	if cmd != nil {
		cmd.Start()
	}
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
	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.FlushOnChange()
	}
}

func (a *App) focusedTerminalWidget() *ui.TerminalWidget {
	if a.Root == nil {
		return nil
	}
	if tp, ok := a.Root.Focused.(*ui.TerminalPanelWidget); ok {
		if tw, ok := tp.ActiveWidget().(*ui.TerminalWidget); ok {
			return tw
		}
	} else if tw, ok := a.Root.Focused.(*ui.TerminalWidget); ok {
		return tw
	} else if g, ok := a.Root.Focused.(*ui.EditorGroupWidget); ok {
		if tab := g.ActiveTerminalTab(); tab != nil {
			return tab.Terminal
		}
	}
	return nil
}

func (a *App) Copy() {
	if tw := a.focusedTerminalWidget(); tw != nil {
		tw.CopySelection()
		return
	}
	if sel := a.EditorGroup.ActiveTextSelection(); sel != nil && sel.CopyTextSelection() {
		return
	}
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.CopySelection()
			return
		}
	}
	if inp := a.focusedWidgetInput(); inp != nil {
		inp.CopySelection()
		return
	}
	a.EditorGroup.Copy()
}

func (a *App) Cut() {
	if tw := a.focusedTerminalWidget(); tw != nil {
		tw.CopySelection()
		return
	}
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.CutSelection()
			return
		}
	}
	if inp := a.focusedWidgetInput(); inp != nil {
		inp.CutSelection()
		return
	}
	a.EditorGroup.Cut()
}

func (a *App) Paste() {
	if holder, ok := a.Root.TopOverlayWidget().(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteClipboard()
			return
		}
	}
	if tw := a.focusedTerminalWidget(); tw != nil {
		text := clipboard.Get()
		if text != "" {
			tw.PasteText(text)
		} else {
			tw.ClearSelection()
		}
		return
	}
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteClipboard()
			return
		}
	}
	if inp := a.focusedWidgetInput(); inp != nil {
		inp.PasteText(clipboard.Get())
		return
	}
	a.EditorGroup.Paste()
}

// focusedWidgetInput returns the widgets.InputWidget with keyboard focus
// inside a focused widget adapter or plugin editor tab, or nil. Copy, Cut
// and Paste use it so clipboard commands act on that input, not the editor.
func (a *App) focusedWidgetInput() *widgets.InputWidget {
	var w widgets.Widget
	switch f := a.Root.Focused.(type) {
	case *ui.WidgetAdapter:
		w = f.FocusedWidget()
	case *ui.EditorGroupWidget:
		w = f.TabContent(f.ActiveTabIndex())
	}
	switch w := w.(type) {
	case *widgets.InputWidget:
		return w
	case ui.InputHolder:
		return w.FocusedInput()
	}
	return nil
}

func (a *App) PasteText(text string) {
	if holder, ok := a.Root.TopOverlayWidget().(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteText(text)
			return
		}
	}
	if tw := a.focusedTerminalWidget(); tw != nil && tw.WantsRawKeys() {
		tw.PasteText(text)
		return
	}
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteText(text)
			return
		}
	}
	if inp := a.focusedWidgetInput(); inp != nil {
		inp.PasteText(text)
		return
	}
	a.EditorGroup.PasteText(text)
}

func (a *App) ShowDialog(w ui.Widget) {
	a.Root.PushOverlay(ui.Overlay{Widget: w, Modal: true})
	a.Root.SetFocus(w)
}

func (a *App) ShowFindBar(w ui.Widget) {
	a.Root.PushOverlay(ui.Overlay{Widget: w, Modal: false})
}

func (a *App) ShowDrawer(drawer *widgets.DrawerWidget) *ui.WidgetAdapter {
	adapter := ui.NewWidgetAdapter(drawer)
	a.Root.PushOverlay(ui.Overlay{Widget: adapter, Modal: true})
	a.Root.SetFocus(adapter)
	return adapter
}

// ClosePluginDrawer closes the plugin drawer without disturbing overlays stacked above it.
func (a *App) ClosePluginDrawer() {
	if a.pluginDrawer == nil {
		return
	}
	a.Root.RemoveOverlay(a.pluginDrawer)
	a.pluginDrawer = nil
	if a.pluginDrawerPanel != nil {
		a.pluginDrawerPanel.Close()
		a.pluginDrawerPanel = nil
	}
	a.FocusEditor()
}

func (a *App) DismissDialog() {
	restore := a.Root.Focused == a.Root.TopOverlayWidget()
	previous := a.Root.OverlayReturnFocus()
	a.dismissDialog()
	// A pane closing asynchronously may already have moved focus elsewhere.
	if !restore {
		return
	}
	if previous == nil || previous == a.EditorGroup {
		a.FocusEditor()
	} else {
		a.Root.SetFocus(previous)
	}
}

func (a *App) dismissDialog() {
	if a.quickOpen != nil && a.Root.TopOverlayWidget() == a.quickOpen.palette {
		a.releaseTask(a.quickOpen.task)
		a.quickOpen = nil
	}
	a.approvalDialog = nil
	a.Root.PopOverlay()
}

func (a *App) ShowInputDialog(title, placeholder, initial string, onSubmit func(string)) {
	a.ShowInputDialogEx(title, placeholder, initial, i18n.Text("general.save"), onSubmit)
}

func (a *App) ShowInputDialogEx(title, placeholder, initial, confirmLabel string, onSubmit func(string)) {
	submit := func(text string) {
		if text != "" {
			a.DismissDialog()
			a.FocusEditor()
			onSubmit(text)
		}
	}
	input := widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: placeholder,
		OnSubmit:    submit,
	})
	input.SetText(initial)

	dialog := widgets.NewDialogWidget(50)
	dialog.Title = title
	dialog.Borders = *a.Borders
	dialog.SetContent(input)
	dialog.Buttons = []widgets.DialogButton{
		{Label: i18n.Text("general.cancelButton"), Handler: func() { a.DismissDialog() }},
		{Label: "&" + confirmLabel, Handler: func() {
			submit(input.Text())
		}},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}

func (a *App) ShowInfoDialog(title string, entries []widgets.KeyValueEntry) {
	a.ShowInfoDialogEx(title, entries, false)
}

func (a *App) ShowInfoDialogEx(title string, entries []widgets.KeyValueEntry, invertStyles bool) {
	content := widgets.NewKeyValueListWidget(entries)
	content.InvertStyles = invertStyles

	dialog := widgets.NewDialogWidget(60)
	dialog.Title = title
	dialog.Borders = *a.Borders
	dialog.SetContent(content)
	dialog.Buttons = []widgets.DialogButton{
		{Label: i18n.Text("general.closeButton"), Handler: func() { a.DismissDialog() }},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}

func (a *App) ShowConfirmDialog(message string, buttons []string, callbacks []func()) {
	a.ShowConfirmDialogEx("", message, buttons, callbacks)
}

func (a *App) ShowConfirmDialogEx(title, message string, buttons []string, callbacks []func()) *widgets.DialogWidget {
	content := widgets.NewParagraphWidget(message)

	dialog := widgets.NewDialogWidget(50)
	dialog.Borders = *a.Borders
	dialog.SetContent(content)

	dialogButtons := make([]widgets.DialogButton, len(buttons))
	for i, label := range buttons {
		handler := callbacks[i]
		dialogButtons[i] = widgets.DialogButton{
			Label:   "&" + label,
			Handler: handler,
		}
	}
	dialog.Buttons = dialogButtons
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Title = title
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
	return dialog
}

func (a *App) showDiffFindBar(dv *ui.DiffEditorWidget) {
	findBar := ui.NewFindBarWidget()
	findBar.Input.ControlModifiers = &a.Root.ControlModifiers
	findBar.Borders = a.Borders
	findBar.OnSearch = func(query string, opts ui.SearchOptions) []ui.FindMatch {
		leftMatches, err := ui.FindInLines(dv.LeftLines(), query, opts)
		if err != nil {
			a.StatusWarn(i18n.Format("status.invalidRegex", i18n.Args{"error": err.Error()}))
			return nil
		}
		rightMatches, _ := ui.FindInLines(dv.RightLines(), query, opts)
		return dv.SetSearchMatches(leftMatches, rightMatches)
	}
	findBar.OnNavigate = func(match ui.FindMatch) {
		dv.SetActiveMatch(findBar.Current)
		dv.ScrollToLine(match.Line)
	}
	findBar.OnDismiss = func() {
		dv.OnRecompute = nil
		a.DismissDialog()
		dv.ClearSearch()
	}
	dv.OnRecompute = func() {
		findBar.Refresh()
		if len(findBar.Matches) > 0 {
			dv.SetActiveMatch(findBar.Current)
		}
	}
	a.prefillFindBar(findBar)
	a.ShowFindBar(findBar)
}

func (a *App) ShowSelectDialog(title string, items []widgets.SelectItem, onSelect func(id string), onChange func(id string)) {
	sel := widgets.NewSelectWidget(widgets.SelectConfig{
		Items:       items,
		ShowDivider: true,
		OnSelect: func(id string) {
			a.DismissDialog()
			onSelect(id)
		},
		OnChange:  onChange,
		OnDismiss: func() { a.DismissDialog() },
	})

	dialog := widgets.NewDialogWidget(50)
	dialog.Title = title
	dialog.Borders = *a.Borders
	dialog.SetContent(sel)
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}
