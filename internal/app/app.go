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
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/watcher"
	"github.com/eugenioenko/ttt/internal/workspace"

	"github.com/gdamore/tcell/v2"
)

const terminalStripWidth = ui.VerticalTabBarWidth

type TerminalTab struct {
	ID     string
	Term   *terminal.Terminal
	Widget *ui.TerminalWidget
}

type App struct {
	Root               *ui.Root
	EditorGroup        *ui.EditorGroupWidget
	Sidebar            *ui.SidebarWidget
	SplitPanel         *ui.SplitPanelWidget
	ContentSplit       *ui.ContentSplitWidget
	BottomPanel        *ui.BottomPanelWidget
	Explorer           *ui.ExplorerWidget
	Search             *ui.SearchWidget
	Changes            *ui.ChangesWidget
	MenuBar            *ui.MenuBarWidget
	StatusBar          *ui.StatusBarWidget
	Status             *view.StatusBar
	Borders            *term.BorderSet
	Screen             *term.TcellScreen
	Renderer           *render.Renderer
	Settings           *config.Settings
	Workspace          *workspace.Workspace
	Palette            *ui.TerminalColorPalette
	TerminalPanel      *ui.TerminalPanelWidget
	Terminals          []TerminalTab
	LspManager         *lsp.Manager
	DocVersionsMu      sync.Mutex
	DocVersions        map[string]int
	CompletionItems    []ui.CompletionItem
	LspCompletionItems []lsp.CompletionItem
	CompletionTriggers []string
	AutocompleteTimer  *time.Timer
	HoverTimer         *time.Timer
	HoverGen           uint64
	LastHoverLine      int
	LastHoverCol       int
	Problems           *ui.ProblemsWidget
	References         *ui.ReferencesWidget
	AllDiagnostics     map[string][]ui.Diagnostic
	Keybindings        []config.KeyBinding
	LspNotified        map[string]bool
	CommentPanel       *ui.CommentPanelWidget
	Reg                *command.Registry
	Running            *bool
	quitPending        bool
	Watcher            *watcher.Watcher
	GitGutterGen       int
	GitGutterTimer     *time.Timer
	Version            string
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
	rows := r.H - 3
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
}

func (a *App) CloseTerminal(panelID string) {
	for i, tt := range a.Terminals {
		if tt.ID == panelID {
			tt.Term.Close()
			a.Terminals = append(a.Terminals[:i], a.Terminals[i+1:]...)
			a.TerminalPanel.RemoveTerminal(i)
			break
		}
	}
	if a.TerminalPanel.Count() == 0 {
		a.FocusEditor()
	} else {
		a.Root.SetFocus(a.TerminalPanel)
	}
}

func (a *App) CloseAllTerminals() {
	for i := len(a.Terminals) - 1; i >= 0; i-- {
		a.CloseTerminal(a.Terminals[i].ID)
	}
}

func (a *App) refreshWorkspaceWidgets() {
	paths := a.Workspace.Paths()

	existing := make(map[string]bool)
	for _, r := range a.Explorer.Roots {
		existing[r.Path] = true
	}
	wanted := make(map[string]bool)
	for _, p := range paths {
		wanted[p] = true
		if !existing[p] {
			a.Explorer.AddRoot(p)
		}
	}
	for _, r := range a.Explorer.Roots {
		if !wanted[r.Path] {
			a.Explorer.RemoveRoot(r.Path)
		}
	}

	a.Search.SetWorkDirs(paths)
	a.Changes.SetDirs(paths)
	a.Changes.Refresh()
}

func (a *App) refreshProblems() {
	var items []ui.ProblemItem
	for path, diags := range a.AllDiagnostics {
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
	a.HoverTimer = time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
		a.RequestHover(path, lang, line, col, mx, my)
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
	return mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H
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
	a.StartWatcher()

	a.EditorGroup.OnError = func(msg string) {
		a.StatusError(msg)
	}
	a.EditorGroup.OnFileOpen = func(path, lang, text string) {
		a.NotifyLSPOpen(path, lang, text)
		a.RequestGitGutterForActiveFile()
	}
	a.EditorGroup.OnFileClose = func(path, lang string) {
		a.NotifyLSPClose(path, lang)
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

func (a *App) ShowDialog(w ui.Widget) {
	a.Root.PushOverlay(ui.Overlay{Widget: w, Modal: true})
	a.Root.SetFocus(w)
}

func (a *App) ShowFindBar(w ui.Widget) {
	a.Root.PushOverlay(ui.Overlay{Widget: w, Modal: false})
}

func (a *App) DismissDialog() {
	a.Root.PopOverlay()
	a.FocusEditor()
}

func (a *App) ShowInputDialog(title, placeholder, initial string, onSubmit func(string)) {
	dialog := ui.NewInputDialogWidget(title, placeholder, initial)
	dialog.Borders = a.Borders
	dialog.OnSubmit = func(value string) {
		a.DismissDialog()
		onSubmit(value)
	}
	dialog.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(dialog)
}

func (a *App) ShowConfirmDialog(message string, buttons []string, callbacks []func()) {
	var dialog *ui.ConfirmDialogWidget
	if len(buttons) == 3 {
		dialog = ui.NewConfirmDialogWidget3(message, buttons[0], buttons[1], buttons[2])
	} else if len(buttons) == 2 {
		dialog = ui.NewConfirmDialogWidget2(message, buttons[0], buttons[1])
	} else {
		dialog = ui.NewConfirmDialogWidget(message)
	}
	dialog.Borders = a.Borders
	for i, cb := range callbacks {
		if i < len(dialog.OnButton) {
			dialog.OnButton[i] = cb
		}
	}
	dialog.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(dialog)
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

func (a *App) ShowPicker(items []command.Command, onSelect func(id string)) {
	picker := ui.NewSelectDialogWidget(items)
	picker.Borders = a.Borders
	picker.OnExecute = func(id string) {
		a.DismissDialog()
		onSelect(id)
	}
	picker.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(picker)
}
