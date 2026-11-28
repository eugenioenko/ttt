package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/markdown"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func (a *App) WirePlugin(p *plugin.Plugin) {
	p.ControlModifiers = &a.Root.ControlModifiers
	a.installPluginThemes(p)
	a.installPluginLanguages(p)
	a.wirePluginDialogs(p)
	a.wirePluginChanges(p)
	id := "plugin." + p.Name
	var sidebar, bottom *plugin.PluginPanelWidget
	var sidebarTitle, bottomTitle string
	for _, reg := range a.PluginManager.SidebarPanels {
		if reg.ID == id {
			sidebar, sidebarTitle = reg.Widget, reg.Title
			break
		}
	}
	for _, reg := range a.PluginManager.BottomPanels {
		if reg.ID == id {
			bottom, bottomTitle = reg.Widget, reg.Title
			break
		}
	}
	a.ApplyPluginPanelIcons()
	a.syncPluginPanel(&a.Sidebar.TabbedPanel, id, sidebarTitle, sidebar)
	a.syncPluginPanel(&a.BottomPanel.TabbedPanel, id, bottomTitle, bottom)
	p.RequestRedraw = func() {
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.ShowPanel = func(area string, focus bool) (bool, error) {
		return a.ShowPluginPanel(p.Name, area, focus)
	}
	state := p.State
	allowed := func() bool { return p.Enabled && p.State == state && state != nil }
	p.SimulateClick = func(x, y int) {
		go func() {
			a.postPluginInput(allowed, tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
			time.Sleep(50 * time.Millisecond)
			a.postPluginInput(allowed, tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
		}()
	}
	p.SimulateDrag = func(x1, y1, x2, y2 int) {
		go func() {
			a.postPluginInput(allowed, tcell.NewEventMouse(x1, y1, tcell.Button1, tcell.ModNone))
			time.Sleep(30 * time.Millisecond)
			steps := 10
			for i := 1; i <= steps; i++ {
				mx := x1 + (x2-x1)*i/steps
				my := y1 + (y2-y1)*i/steps
				a.postPluginInput(allowed, tcell.NewEventMouse(mx, my, tcell.Button1, tcell.ModNone))
				time.Sleep(15 * time.Millisecond)
			}
			a.postPluginInput(allowed, tcell.NewEventMouse(x2, y2, tcell.ButtonNone, tcell.ModNone))
		}()
	}
	p.SimulateKeys = func(combo string) error { return a.simulatePluginKeys(combo, allowed) }
	p.SimulateText = func(text string) error { return a.simulatePluginText(text, allowed) }
	p.ScreenText = a.Screenshot
	p.DebugState = func() (string, error) {
		data, err := json.MarshalIndent(a.BuildDebugState(), "", "  ")
		return string(data), err
	}
	p.QuitApp = func() {
		if a.Running != nil {
			*a.Running = false
		}
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.OpenFile = func(path string, line int, readonly bool) {
		absPath := path
		if !filepath.IsAbs(path) {
			if cwd, err := os.Getwd(); err == nil {
				absPath = filepath.Join(cwd, path)
			}
		}
		if readonly {
			if err := a.EditorGroup.OpenFileReadOnly(absPath, ""); err != nil {
				return
			}
		} else {
			if err := a.EditorGroup.OpenFile(absPath); err != nil {
				return
			}
		}
		if line > 0 {
			a.EditorGroup.GoToLine(line)
		}
	}
	p.OpenDiff = func(title string, oldLines, newLines []string, filePath string, extended bool, diffText string) {
		path := filePath
		if path == "" {
			path = title
		}
		fd := diff.FileDiff{}
		if diffText != "" {
			fd = diff.Parse(diffText)
		}
		a.EditorGroup.OpenDiff(path, fd, oldLines, newLines, extended)
	}
	p.OpenReadOnly = func(title, filePath string, lines []string) {
		a.EditorGroup.OpenBufferReadOnly(title, filePath, lines)
	}
	p.Notify = func(message, level string) {
		switch level {
		case "warn":
			a.StatusWarn(message)
		case "error":
			a.StatusError(message)
		default:
			a.StatusNotify(message)
		}
	}
	p.SetStatusItem = func(side, id, text string, priority int, onClick func()) {
		a.Status.SetSegment(view.StatusSegment{
			ID:       id,
			Side:     side,
			Priority: priority,
			Text:     text,
			OnClick:  onClick,
			Owner:    "plugin:" + p.Name,
		})
	}
	p.RemoveStatusItem = func(id string) {
		a.Status.RemoveSegment(id)
	}
	p.ClipboardWrite = func(text string) {
		clipboard.Set(text)
	}
	p.ClipboardRead = func() string {
		return clipboard.Get()
	}
	p.SetEcho = func(text string) {
		a.Status.EchoText = text
	}
	p.ExecCommand = func(id string) bool {
		return a.Reg.Execute(id)
	}
	p.ShowCommandLine = func(opts plugin.CommandLineOptions) {
		w := a.ShowCommandLine(opts.Prefix, opts.OnChange, opts.OnSubmit, opts.OnCancel)
		if w == nil {
			return
		}
		w.OnKey = opts.OnKey
		if opts.Text != "" {
			w.SetText(opts.Text)
		}
	}
	p.HideCommandLine = func() {
		a.HideCommandLine()
	}
	p.SetCommandLineText = func(text string) {
		a.SetCommandLineText(text)
	}
	p.CommandLineActive = func() bool {
		return a.CommandLineActive()
	}
	p.ListCommands = func() []plugin.CommandInfo {
		cmds := a.Reg.List()
		result := make([]plugin.CommandInfo, len(cmds))
		for i, cmd := range cmds {
			result[i] = plugin.CommandInfo{ID: cmd.ID, Title: cmd.Title}
		}
		return result
	}
	p.SetPostAsync(func(result *plugin.PluginAsyncResult) {
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(result))
	})
	pluginSource := "plugin:" + p.Name
	p.PublishDiagnostics = func(path string, items []plugin.DiagnosticItem) {
		diags := make([]ui.Diagnostic, 0, len(items))
		for _, it := range items {
			diags = append(diags, ui.Diagnostic{
				StartLine: it.StartLine,
				StartCol:  it.StartCol,
				EndLine:   it.EndLine,
				EndCol:    it.EndCol,
				Severity:  ui.DiagnosticSeverity(it.Severity),
				Style:     it.Style,
				Message:   it.Message,
				Source:    it.Source,
			})
		}
		for _, group := range a.editorGroups() {
			group.SetDiagnosticsSource(pluginSource, path, diags)
		}
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.ClearDiagnostics = func(path string) {
		if path == "" {
			for _, group := range a.editorGroups() {
				group.ClearDiagnosticsSource(pluginSource)
			}
		} else {
			for _, group := range a.editorGroups() {
				group.SetDiagnosticsSource(pluginSource, path, nil)
			}
		}
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.SetDecorations = func(path string, items []plugin.DecorationItem) {
		decos := make([]ui.Decoration, len(items))
		for i, it := range items {
			decos[i] = ui.Decoration{
				StartLine:  it.StartLine,
				StartCol:   it.StartCol,
				EndLine:    it.EndLine,
				EndCol:     it.EndCol,
				Text:       it.Text,
				Note:       it.Note,
				GhostText:  it.GhostText,
				GhostAtCol: it.GhostAtCol,
				Style:      it.Style,
			}
		}
		for _, group := range a.editorGroups() {
			group.SetDecorations(pluginSource, path, decos)
		}
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.ClearDecorations = func(path string) {
		if path == "" {
			for _, group := range a.editorGroups() {
				group.ClearDecorationsSource(pluginSource)
			}
		} else {
			for _, group := range a.editorGroups() {
				group.SetDecorations(pluginSource, path, nil)
			}
		}
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.SetBookmark = func(line int, icon rune, style term.Style) {
		a.EditorGroup.Editor.SetBookmark(line, icon, style)
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.GetBookmark = func(line int) (rune, term.Style, bool) {
		b, ok := a.EditorGroup.Editor.GetBookmark(line)
		return b.Icon, b.Style, ok
	}
	p.RemoveBookmark = func(line int) {
		a.EditorGroup.Editor.RemoveBookmark(line)
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.SetAllBookmarks = func(items []plugin.BookmarkItem) {
		m := make(map[int]ui.Bookmark, len(items))
		for _, it := range items {
			m[it.Line] = ui.Bookmark{Icon: it.Icon, Style: it.Style}
		}
		a.EditorGroup.Editor.SetAllBookmarks(m)
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.GetAllBookmarks = func() []plugin.BookmarkItem {
		all := a.EditorGroup.Editor.GetAllBookmarks()
		items := make([]plugin.BookmarkItem, 0, len(all))
		for line, b := range all {
			items = append(items, plugin.BookmarkItem{Line: line, Icon: b.Icon, Style: b.Style})
		}
		slices.SortFunc(items, func(x, y plugin.BookmarkItem) int { return x.Line - y.Line })
		return items
	}
	p.ClearBookmarks = func() {
		a.EditorGroup.Editor.ClearBookmarks()
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.RenderMarkdown = func(text string) []plugin.MarkdownLine {
		rendered := markdown.Render(text)
		lines := make([]plugin.MarkdownLine, len(rendered))
		for i, line := range rendered {
			spans := make([]plugin.MarkdownSpan, len(line.Spans))
			for j, span := range line.Spans {
				spans[j] = plugin.MarkdownSpan{Text: span.Text, Style: span.Style}
			}
			lines[i] = plugin.MarkdownLine{Spans: spans}
		}
		return lines
	}
	p.Editor = NewPluginEditorAPI(a)
	fs := NewPluginFilesystemAPIFor(p, a.Workspace.Paths())
	fs.onWrite = func(path string) {
		a.postRepositoryInvalidation(path, RepositoryWorktree)
	}
	p.Filesystem = fs
	p.DialRPC = dialPluginRPC(fs.vars)
	p.RunAutomation = func(ctx context.Context, action, args string) (json.RawMessage, error) {
		return a.runPluginAutomation(ctx, p, action, args)
	}
	p.System = NewPluginSystemAPI()
	p.Terminal = NewPluginTerminalAPI(a)
	if system, ok := p.System.(*PluginSystemAPI); ok {
		system.onExec = func() {
			a.postRepositoryInvalidation("", RepositoryWorktree|RepositoryHistory)
		}
	}
	p.Network = NewPluginNetworkAPI().ForPlugin(func(u *url.URL) error { return p.Granted.CheckURL(u) })
	a.wirePluginLog(p)
	p.Markdown = a.Settings.Markdown
	p.Borders = a.Borders
	p.AppVersion = a.Version
	p.ShowInfoDialog = func(title string, entries []widgets.KeyValueEntry) {
		a.ShowInfoDialog(title, entries)
	}
	p.ShowConfirmDialog = func(message string, confirmLabel string, cancelLabel string, onConfirm func()) {
		a.ShowConfirmDialogEx(i18n.Text("general.confirm"), message, []string{cancelLabel, confirmLabel}, []func(){
			func() { a.DismissDialog() },
			func() {
				a.DismissDialog()
				onConfirm()
			},
		})
	}
	p.RequestExecPath = func(path string, done func(bool)) error {
		return a.ShowExecPathApproval(p, path, done)
	}
	p.ShowContextMenu = a.showWidgetMenu

	p.OpenDrawer = func(panel *plugin.PluginPanelWidget, width, minWidth int, side string) {
		a.ClosePluginDrawer()
		var adapter *ui.WidgetAdapter
		drawer := widgets.NewDrawerWidget(widgets.DrawerConfig{
			Width:    width,
			MinWidth: minWidth,
			Side:     side,
			Borders:  *a.Borders,
			OnDismiss: func() {
				prev := a.Root.RemoveOverlay(adapter)
				if a.pluginDrawer == adapter {
					a.pluginDrawer = nil
					a.pluginDrawerPanel = nil
				}
				panel.Close()
				a.restoreFocus(prev)
			},
		})
		drawer.SetContent(panel)
		adapter = a.ShowDrawer(drawer)
		a.pluginDrawer = adapter
		a.pluginDrawerPanel = panel
	}
	p.CloseDrawer = func() {
		a.ClosePluginDrawer()
	}
	p.Graphics = a.pluginGraphics
	p.OpenTab = func(key, title string, panel *plugin.PluginPanelWidget) {
		a.EditorGroup.OpenPluginTab(key, title, panel)
	}
	p.FocusTab = func(key string, focus bool) bool {
		return a.switchToPluginTab(p.Name, key, focus)
	}
	p.CloseTab = func(key string) bool {
		return a.closePluginTab(p.Name, key)
	}

	if p.HasSidebarMenu() {
		for _, entry := range p.SidebarMenuEntries {
			if entry.Separator || entry.Command == "" {
				continue
			}
			cmd := entry.Command
			a.Reg.Register(command.Command{
				ID:           cmd,
				Title:        entry.Label,
				DisplayTitle: pluginCommandTitle(p, entry.Label),
				Keywords:     []string{p.Manifest.DisplayName, p.Name, p.Manifest.Name},
				Handler: func() {
					p.CallSidebarAction(cmd)
				},
			})
		}
	}

	a.registerPluginCommandsAndKeys(p)
	p.FlushPendingDrawer()
	if len(p.FileHandlers) > 0 {
		for _, group := range a.editorGroups() {
			group.RetryFileHandlers()
		}
	}
}

func (a *App) wirePluginLog(p *plugin.Plugin) {
	p.Log = func(level, message string) {
		a.Output.AddLine(ui.OutputLine{
			Time:       time.Now().Format("15:04:05"),
			PluginName: p.Name,
			Level:      level,
			Message:    message,
		})
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
}

func pluginCommandTitle(p *plugin.Plugin, title string) string {
	name := strings.TrimSpace(p.Manifest.DisplayName)
	if name == "" {
		name = strings.TrimSpace(p.Name)
	}
	if name == "" {
		name = strings.TrimSpace(p.Manifest.Name)
	}
	if name == "" {
		return title
	}
	trimmed := strings.TrimSpace(title)
	titleRunes := []rune(trimmed)
	for _, alias := range []string{name, strings.TrimSpace(p.Name), strings.TrimSpace(p.Manifest.Name)} {
		prefixLen := utf8.RuneCountInString(alias)
		if prefixLen == 0 || len(titleRunes) < prefixLen || !strings.EqualFold(string(titleRunes[:prefixLen]), alias) {
			continue
		}
		rest := strings.TrimSpace(string(titleRunes[prefixLen:]))
		if strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, "-") || strings.HasPrefix(rest, "–") || strings.HasPrefix(rest, "—") || strings.HasPrefix(rest, "|") {
			return title
		}
	}
	return name + ": " + trimmed
}

func (a *App) registerPluginCommandsAndKeys(p *plugin.Plugin) {
	for _, cmd := range p.Commands {
		handler := cmd.Handler
		cmdID := cmd.ID
		plugName := p.Name
		a.Reg.Register(command.Command{
			ID:           cmd.ID,
			Title:        cmd.Title,
			DisplayTitle: pluginCommandTitle(p, cmd.Title),
			Keywords:     []string{p.Manifest.DisplayName, p.Name, p.Manifest.Name},
			Handler: func() {
				if err := handler(); err != nil {
					slog.Error("plugin command error", "plugin", plugName, "command", cmdID, "error", err)
				}
			},
		})
	}

	for _, kb := range p.PluginKeybindings {
		steps, err := config.ParseKeyString(kb.Key)
		if err != nil {
			slog.Warn("plugin keybinding parse error", "plugin", p.Name, "key", kb.Key, "error", err)
			continue
		}
		cmdID := kb.Command
		if len(steps) > 1 {
			tcellSteps := make([]ui.GlobalKeyBinding, len(steps))
			for i, step := range steps {
				key, mod, rn := comboToTcell(step)
				tcellSteps[i] = ui.GlobalKeyBinding{Key: key, Mod: mod, Rune: rn}
			}
			bindCommandChord(a.Root, a.Reg, tcellSteps, cmdID)
		} else {
			key, mod, rn := comboToTcell(steps[0])
			a.Root.AddGlobalKey(key, mod, rn, func() {
				a.Reg.Execute(cmdID)
			})
		}
		a.Reg.SetShortcut(cmdID, FormatKeyBinding(kb.Key, a.Root.ControlModifiers))
	}
}

func (a *App) RegisterStartupPluginCommands() {
	for _, p := range a.PluginManager.Plugins() {
		a.WirePlugin(p)
	}
	a.ensureKeyInterceptor()
}

func (a *App) ensureKeyInterceptor() {
	if a.Root.KeyInterceptor != nil {
		return
	}
	a.Root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		if a.Root.Focused != a.EditorGroup {
			return false
		}
		return a.PluginManager.DispatchKeyEvent(ev)
	}
}
