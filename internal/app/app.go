package app

import (
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/watcher"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"

	"github.com/gdamore/tcell/v3"
)

const terminalStripWidth = ui.VerticalTabBarWidth
const hoverGraceMargin = 3

type TerminalTab struct {
	ID     string
	Term   *terminal.Terminal
	Widget *ui.TerminalWidget
}

type App struct {
	Root                   *ui.Root
	EditorGroup            *ui.EditorGroupWidget
	Sidebar                *ui.SidebarWidget
	SplitPanel             *ui.SplitPanelWidget
	ContentSplit           *ui.ContentSplitWidget
	BottomPanel            *ui.BottomPanelWidget
	Search                 *ui.SearchWidget
	MenuBar                *ui.MenuBarWidget
	RootBox                *widgets.VStackWidget
	StatusBar              *ui.StatusBarWidget
	Status                 *view.StatusBar
	Borders                *term.BorderSet
	Screen                 *term.TcellScreen
	Renderer               *render.Renderer
	Settings               *config.Settings
	Workspace              *workspace.Workspace
	Palette                *ui.TerminalColorPalette
	TerminalPanel          *ui.TerminalPanelWidget
	Terminals              []TerminalTab
	LspManager             *lsp.Manager
	DocVersionsMu          sync.Mutex
	DocVersions            map[string]int
	CompletionItems        []ui.CompletionItem
	LspCompletionItems     []lsp.CompletionItem
	CompletionTriggers     []string
	AutocompleteTimer      *time.Timer
	HoverTimer             *time.Timer
	HoverGen               uint64
	LastHoverLine          int
	LastHoverCol           int
	Problems               *ui.ProblemsWidget
	References             *ui.ReferencesWidget
	Keybindings            []config.KeyBinding
	LspNotified            map[string]bool
	Explorer               *NavigationPanel
	ExplorerContextNode    *widgets.TreeNode
	Changes                *ChangesPanel
	Symbols                *SymbolsPanel
	Reg                    *command.Registry
	Running                *bool
	quitPending            bool
	menuReturnFocus        ui.Widget
	Watcher                *watcher.Watcher
	GitGutterGen           int
	GitGutterTimer         *time.Timer
	Version                string
	PluginManager          *plugin.Manager
	PendingPluginApprovals []*plugin.Plugin
	// PendingFileTargets holds cursor positions parsed from `path:line[:col]`
	// arguments. They are applied after the first render, once the viewport
	// has a real height to scroll against.
	PendingFileTargets []FileTarget
	PluginsPanel       *PluginsPanel
	Output             *ui.OutputWidget
	// runningRepoOp is the progress label of the in-flight task, empty when idle.
	runningRepoOp        string
	pluginDetailWidgets  map[string]*pluginDetailState
	pluginDrawer         ui.Widget
	commandLine          *ui.CommandLineWidget
	commandLinePrevFocus ui.Widget
	settingsView         *settingsView
	// appliedSettings is the last value ApplySettings acted on. Callers routinely
	// mutate a.Settings before calling it, so a.Settings cannot serve as "before".
	appliedSettings    config.Settings
	eventLoopDoneOnce  sync.Once
	eventLoopCloseOnce sync.Once
	eventLoopDone      chan struct{}
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
			return formatKeyDisplay(kb.Key)
		}
	}
	return ""
}

func formatKeyDisplay(key string) string {
	parts := strings.Fields(key)
	for i, part := range parts {
		tokens := strings.Split(part, "+")
		for j, tok := range tokens {
			if len(tok) > 0 {
				tokens[j] = strings.ToUpper(tok[:1]) + tok[1:]
			}
		}
		parts[i] = strings.Join(tokens, "+")
	}
	return strings.Join(parts, " ")
}

func (a *App) ShowSidebar() {
	a.Sidebar.Visible = true
	a.SplitPanel.ShowLeft = true
	if a.SplitPanel.DividerPos < ui.MinSidebarWidth {
		a.SplitPanel.DividerPos = ui.DefaultSidebarWidth
	}
	a.applySearchHighlights()
}

func (a *App) HideSidebar() {
	a.Sidebar.Visible = false
	a.SplitPanel.ShowLeft = false
	a.EditorGroup.ClearSearch()
}

func (a *App) applySearchHighlights() {
	if a.Sidebar.ActivePanel == "search" && a.Search.Input.Text != "" {
		matches, _ := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, a.Search.Input.Text, a.Search.Options)
		a.EditorGroup.SetSearch(a.Search.Input.Text, matches)
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
	if w <= 0 {
		a.HideSidebar()
		return
	}
	if !a.Sidebar.Visible {
		a.ShowSidebar()
	}
	a.SplitPanel.DividerPos = w
}

func (a *App) FocusEditor() {
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) FocusEditorIfEnabled() {
	if a.Settings.Editor.FocusOnOpen {
		a.Root.SetFocus(a.EditorGroup)
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

func (a *App) SpawnTerminal() {
	r := a.ContentSplit.GetRect()
	cols := r.W - terminalStripWidth
	bottomH := a.ContentSplit.BottomH
	if bottomH <= 1 {
		bottomH = min(r.H/2, r.H-4)
	}
	rows := bottomH - 2
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	t, err := terminal.New(a.Settings.Terminal.Shell, cols, rows, a.Settings.Terminal.Scrollback, nil, a.Workspace.Primary())
	if err != nil {
		slog.Error("terminal.New", "err", err)
		a.StatusError("Failed to open terminal: " + err.Error())
		return
	}

	tw := ui.NewTerminalWidget(t, a.Palette)
	tw.WorkDir = a.Workspace.Primary()
	tw.OnOpenURL = func(url string) {
		OpenURL(url)
	}
	tw.OnOpenFile = func(path string, line, col int) {
		a.EditorGroup.OpenFile(path)
		if line > 0 {
			a.EditorGroup.GoToLineCol(line, col)
		}
		a.FocusEditor()
	}

	panelID := fmt.Sprintf("terminal-%d", len(a.Terminals))
	a.Terminals = append(a.Terminals, TerminalTab{ID: panelID, Term: t, Widget: tw})
	a.TerminalPanel.AddTerminal(tw)
	a.BottomPanel.SetActivePanel("terminal")

	if !a.ContentSplit.ShowBottom {
		a.ContentSplit.ShowBottom = true
	}
	a.Root.SetFocus(a.TerminalPanel)

	t.OnUpdate = func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	t.OnExit = func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(panelID))
	}
	t.Run()
}

func (a *App) CloseTerminal(panelID string) {
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
	a.Terminals = append(a.Terminals[:idx], a.Terminals[idx+1:]...)
	a.TerminalPanel.RemoveTerminal(idx)

	if a.TerminalPanel.Count() == 0 {
		a.FocusEditor()
	} else {
		a.Root.SetFocus(a.TerminalPanel)
	}
}

func (a *App) CloseAllTerminals() {
	terms := a.Terminals
	a.Terminals = nil
	for i := len(terms) - 1; i >= 0; i-- {
		a.TerminalPanel.RemoveTerminal(i)
	}
	for _, tt := range terms {
		tt.Term.Close()
	}
	a.FocusEditor()
}

func (a *App) refreshWorkspaceWidgets() {
	paths := a.Workspace.Paths()

	a.Explorer.SetRoots(paths)

	a.Search.SetWorkDirs(paths)
	a.Changes.SetDirs(paths)
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
		return
	}
	r := a.EditorGroup.Editor.GetRect()
	if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
		a.DismissHover()
		a.cancelHoverTimer()
		return
	}
	gw := a.EditorGroup.Editor.GutterWidth()
	line := my - r.Y + a.EditorGroup.Editor.Viewport.TopLine
	col := mx - r.X - gw + a.EditorGroup.Editor.Viewport.LeftCol
	if col < 0 {
		a.DismissHover()
		a.cancelHoverTimer()
		return
	}
	if line == a.LastHoverLine && col == a.LastHoverCol {
		return
	}
	a.LastHoverLine = line
	a.LastHoverCol = col
	a.DismissHover()
	a.cancelHoverTimer()
	delay := a.Settings.LSP.HoverDelay
	if delay <= 0 {
		delay = 400
	}
	path := a.EditorGroup.ActiveFilePath()
	lang := ""
	if a.EditorGroup.Editor.Highlighter != nil {
		lang = a.EditorGroup.Editor.Highlighter.Language()
	}
	diagText := ""
	if a.EditorGroup.Editor != nil {
		if d := a.EditorGroup.Editor.DiagnosticAt(line, col); d != nil {
			diagText = d.Message
		}
	}
	gen := a.HoverGen
	a.HoverTimer = time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
		a.RequestHover(path, lang, line, col, mx, my, diagText, gen)
	})
}

func (a *App) cancelHoverTimer() {
	if a.HoverTimer != nil {
		a.HoverTimer.Stop()
		a.HoverTimer = nil
	}
}

func (a *App) ShowHover(text string, anchorX, anchorY int) {
	if text == "" {
		return
	}
	a.EditorGroup.Hover = ui.NewHoverWidget(text, anchorX, anchorY)
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

func (a *App) DismissHover() {
	a.EditorGroup.Hover = nil
	a.cancelHoverTimer()
	a.HoverGen++
}

func (a *App) Init(screen *term.TcellScreen, renderer *render.Renderer, lspManager *lsp.Manager) {
	a.Screen = screen
	a.Renderer = renderer
	a.LspManager = lspManager
	a.EditorGroup.TabBar.PostDragAutoScrollTick = func(generation uint64) {
		screen.PostEvent(tcell.NewEventInterrupt(&ui.TabDragAutoScrollTick{Generation: generation}))
	}
	a.StartWatcher()

	a.EditorGroup.OnError = func(msg string) {
		a.StatusError(msg)
	}
	a.EditorGroup.OnNotify = func(msg string) {
		a.StatusNotify(msg)
	}
	a.EditorGroup.FlushNotifications()
	a.EditorGroup.OnFileOpen = func(path, lang, text string) {
		a.NotifyLSPOpen(path, lang, text)
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
	}
	if path := a.EditorGroup.ActiveFilePath(); path != "" {
		if a.EditorGroup.Editor != nil && a.EditorGroup.Editor.Highlighter != nil {
			lang := a.EditorGroup.Editor.Highlighter.Language()
			text := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n")
			a.NotifyLSPOpen(path, lang, text)
		}
	}
	a.Problems.OnNavigate = func(file string, line, col int) {
		a.EditorGroup.OpenFile(file)
		a.EditorGroup.GoToLine(line + 1)
		a.Root.SetFocus(a.EditorGroup)
	}
	a.References.OnNavigate = func(file string, line, col int) {
		a.EditorGroup.OpenFile(file)
		a.EditorGroup.GoToLine(line + 1)
		a.Root.SetFocus(a.EditorGroup)
	}
	a.EditorGroup.Editor.OnChange = func() {
		path := a.EditorGroup.ActiveFilePath()
		lang := ""
		if a.EditorGroup.Editor.Highlighter != nil {
			lang = a.EditorGroup.Editor.Highlighter.Language()
		}
		text := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n")
		a.NotifyLSPChange(path, lang, text)
		a.ScheduleAutocomplete()
		a.CheckSignatureHelpTrigger()
		a.ScheduleGitGutter()
		if a.PluginManager != nil {
			a.PluginManager.DispatchEvent("editor.change", path)
		}
	}

	lspManager.OnLog = func(server, level, message string) {
		a.LogOutputAsync(level, "lsp:"+server, message)
	}

	// A server can die at any time; waking the loop redraws the status bar so
	// the indicator does not keep claiming the server is up.
	lspManager.OnStateChange = func() {
		screen.PostEvent(tcell.NewEventInterrupt(&LSPStateChanged{}))
	}

	lspManager.OnDiagnostics = func(params lsp.PublishDiagnosticsParams) {
		path := URIToPath(params.URI)
		diags := LspToUIDiagnostics(params.Diagnostics)
		slog.Debug("lsp diagnostics", "path", path, "count", len(diags))
		screen.PostEvent(tcell.NewEventInterrupt(&DiagnosticsResult{
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

func OpenURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func (a *App) FlushEditorOnChange() {
	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.FlushOnChange()
	}
}

func (a *App) Copy() {
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.CopySelection()
			return
		}
	}
	a.EditorGroup.Copy()
}

func (a *App) Cut() {
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.CutSelection()
			return
		}
	}
	a.EditorGroup.Cut()
}

func (a *App) Paste() {
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteClipboard()
			return
		}
	}
	a.EditorGroup.Paste()
}

func (a *App) PasteText(text string) {
	if tp, ok := a.Root.Focused.(*ui.TerminalPanelWidget); ok && tp.WantsRawKeys() {
		if tw, ok := tp.ActiveWidget().(*ui.TerminalWidget); ok {
			tw.PasteText(text)
		}
		return
	}
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteText(text)
			return
		}
	}
	if adapter, ok := a.Root.Focused.(*ui.WidgetAdapter); ok {
		if inp, ok := adapter.FocusedWidget().(*widgets.InputWidget); ok {
			inp.PasteText(text)
			return
		}
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
	a.FocusEditor()
}

func (a *App) DismissDialog() {
	a.Root.PopOverlay()
	a.FocusEditor()
}

func (a *App) ShowInputDialog(title, placeholder, initial string, onSubmit func(string)) {
	a.ShowInputDialogEx(title, placeholder, initial, "Save", onSubmit)
}

func (a *App) ShowInputDialogEx(title, placeholder, initial, confirmLabel string, onSubmit func(string)) {
	submit := func(text string) {
		if text != "" {
			a.DismissDialog()
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
		{Label: "&Cancel", Handler: func() { a.DismissDialog() }},
		{Label: "&" + confirmLabel, Handler: func() {
			if input.Text() != "" {
				a.DismissDialog()
				onSubmit(input.Text())
			}
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
		{Label: "&Close", Handler: func() { a.DismissDialog() }},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}

func (a *App) ShowConfirmDialog(message string, buttons []string, callbacks []func()) {
	a.ShowConfirmDialogEx("", message, buttons, callbacks)
}

func (a *App) ShowConfirmDialogEx(title, message string, buttons []string, callbacks []func()) {
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
}

func (a *App) showDiffFindBar(dv *ui.DiffViewWidget) {
	findBar := ui.NewFindBarWidget()
	findBar.Borders = a.Borders
	findBar.OnSearch = func(query string, opts ui.SearchOptions) []ui.FindMatch {
		leftMatches, err := ui.FindInLines(dv.LeftLines(), query, opts)
		if err != nil {
			a.StatusWarn("Invalid regex: " + err.Error())
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
		a.DismissDialog()
		dv.ClearSearch()
	}
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
