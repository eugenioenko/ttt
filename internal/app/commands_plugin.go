package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/markdown"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func registerPluginCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID:       "plugin.list",
		Title:    "Plugins: List Installed",
		Keywords: []string{"plugin", "extension"},
		Handler:  func() { app.showPluginList() },
	})

	reg.Register(command.Command{
		ID:       "plugin.install",
		Title:    "Plugins: Install from URL",
		Keywords: []string{"plugin", "extension", "install"},
		Handler:  func() { app.pluginInstall() },
	})

	reg.Register(command.Command{
		ID:       "plugin.uninstall",
		Title:    "Plugins: Uninstall",
		Keywords: []string{"plugin", "extension", "remove"},
		Handler:  func() { app.pluginUninstall() },
	})

	reg.Register(command.Command{
		ID:       "plugin.update",
		Title:    "Plugins: Update",
		Keywords: []string{"plugin", "extension", "upgrade"},
		Handler:  func() { app.pluginUpdate() },
	})

	reg.Register(command.Command{
		ID:       "plugin.showPanel",
		Title:    "Plugins: Show Panel",
		Keywords: []string{"plugin", "extension", "panel"},
		Handler: func() {
			if app.PluginsPanel != nil {
				app.ShowPanel("plugins", app.PluginsPanel.Adapter)
			} else {
				app.Sidebar.SetActivePanel("plugins")
				app.SplitPanel.ShowLeft = true
			}
		},
	})

	reg.Register(command.Command{
		ID:       "plugin.refresh",
		Title:    "Plugins: Refresh",
		Keywords: []string{"plugin", "refresh", "registry", "available"},
		Handler:  func() { app.PluginRefreshRegistry() },
	})

	reg.Register(command.Command{
		ID:       "plugin.reload",
		Title:    "Plugins: Reload",
		Keywords: []string{"plugin", "reload", "refresh"},
		Handler:  func() { app.pluginReload() },
	})

	reg.Register(command.Command{
		ID:       "plugin.reloadAll",
		Title:    "Plugins: Reload All",
		Keywords: []string{"plugin", "reload", "refresh", "all"},
		Handler:  func() { app.pluginReloadAll() },
	})

}

func (a *App) showPluginList() {
	plugins := a.PluginManager.Plugins()
	if len(plugins) == 0 {
		a.ShowConfirmDialogEx("Plugins", "No plugins installed.", []string{"Close"}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}

	var entries []widgets.KeyValueEntry
	for _, p := range plugins {
		status := "disabled"
		if p.Enabled {
			status = "enabled"
		}
		if p.LastError != nil {
			status = "error"
		}
		entries = append(entries, widgets.KeyValueEntry{
			Key:   p.Manifest.Title(),
			Value: status + " v" + p.Manifest.Version,
		})
	}

	a.ShowInfoDialog("Installed Plugins", entries)
}

func (a *App) pluginInstall() {
	a.ShowInputDialogEx("Install Plugin", "Repository URL", "", "Install", func(repoURL string) {
		go func() {
			p, err := a.PluginManager.Install(repoURL, "")
			a.Screen.PostEvent(tcell.NewEventInterrupt(&pluginInstallResult{
				plugin: p,
				err:    err,
			}))
		}()
	})
}

type pluginInstallResult struct {
	plugin *plugin.Plugin
	name   string
	err    error
}

func (a *App) handlePluginInstallResult(result *pluginInstallResult) {
	if result.err != nil {
		a.resetPluginDetailButton(result.name)
		a.ShowConfirmDialogEx("Install Failed", result.err.Error(), []string{"Close"}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}
	a.updatePluginDetailButtons()
	a.PendingPluginApprovals = append(a.PendingPluginApprovals, result.plugin)
	if len(a.PendingPluginApprovals) == 1 {
		a.ShowPluginApprovalDialog(result.plugin)
	}
}

func (a *App) resetPluginDetailButton(name string) {
	if state, ok := a.pluginDetailWidgets[name]; ok {
		state.installBtn.SetLabel("Install")
		state.installBtn.Disabled = false
	}
}

func (a *App) pluginUninstall() {
	names := a.PluginManager.InstalledPluginNames()
	if len(names) == 0 {
		a.ShowConfirmDialogEx("Uninstall", "No plugins installed.", []string{"Close"}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}

	var items []widgets.SelectItem
	for _, name := range names {
		items = append(items, widgets.SelectItem{ID: name, Label: name})
	}

	a.ShowSelectDialog("Uninstall Plugin", items, func(name string) {
		a.ShowConfirmDialogEx("Confirm Uninstall", "Remove plugin \""+name+"\"?", []string{"Cancel", "Uninstall"}, []func(){
			func() { a.DismissDialog() },
			func() {
				a.DismissDialog()
				a.doPluginUninstall(name)
			},
		})
	}, nil)
}

// ClearPluginSources removes the diagnostics, decorations and status items plugin name set.
func (a *App) ClearPluginSources(name string) {
	a.EditorGroup.ClearDiagnosticsSource("plugin:" + name)
	a.EditorGroup.ClearDecorationsSource("plugin:" + name)
	a.Status.RemoveSegmentsByOwner("plugin:" + name)
}

func (a *App) doPluginUninstall(name string) {
	if err := a.PluginManager.RunUninstallHook(name); err != nil {
		a.ShowConfirmDialogEx("Uninstall Failed", "\""+name+"\" could not clean up: "+err.Error()+". Files it created outside its directory may remain.", []string{"Cancel", "Remove Anyway"}, []func(){
			func() { a.DismissDialog() },
			func() {
				a.DismissDialog()
				a.removePlugin(name)
			},
		})
		return
	}
	a.removePlugin(name)
}

func (a *App) removePlugin(name string) {
	a.RemovePluginPanels(name)

	if err := a.PluginManager.Uninstall(name); err != nil {
		slog.Error("plugin uninstall", "error", err)
	}
	// After the uninstall hook: its on_uninstall callback can still publish and open tabs.
	a.ClearPluginSources(name)
	a.closePluginDrawerAndTabs(name)
	a.uninstallPluginGrammars(name)
	a.uninstallPluginThemes(name)

	a.resetPluginDetailButton(name)
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
}

func (a *App) pluginUpdate() {
	names := a.PluginManager.InstalledPluginNames()
	if len(names) == 0 {
		a.ShowConfirmDialogEx("Update", "No plugins installed.", []string{"Close"}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}

	var items []widgets.SelectItem
	for _, name := range names {
		items = append(items, widgets.SelectItem{ID: name, Label: name})
	}

	a.ShowSelectDialog("Update Plugin", items, a.PluginUpdateByName, nil)
}

type PluginUpdateResult struct {
	update *plugin.PluginUpdate
	err    error
}

// HandlePluginUpdateResult swaps the plugin's Lua state, so it runs on the UI goroutine.
func (a *App) HandlePluginUpdateResult(result *PluginUpdateResult) {
	var p *plugin.Plugin
	var needsApproval bool
	err := result.err
	if err == nil {
		old := a.PluginManager.FindPlugin(result.update.Name())
		p, needsApproval, err = a.PluginManager.ApplyUpdate(result.update)
		if old != nil && !old.Loaded() {
			a.ClearPluginSources(old.Name)
			if p != nil && !needsApproval {
				a.closePluginDrawerAndTabs(old.Name)
			} else {
				a.RemovePluginPanels(old.Name)
			}
		}
	}
	if err != nil {
		a.ShowConfirmDialogEx("Update Failed", err.Error(), []string{"Close"}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}
	if needsApproval && p != nil {
		a.PendingPluginApprovals = append(a.PendingPluginApprovals, p)
		if len(a.PendingPluginApprovals) == 1 {
			a.ShowPluginApprovalDialog(p)
		}
		return
	}
	if !needsApproval && p != nil {
		if p.Loaded() {
			a.WirePlugin(p)
			a.installPluginGrammars(p)
		} else {
			a.installPluginThemes(p)
		}
	}
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
}

func (a *App) ShowPluginApprovalDialog(p *plugin.Plugin) {
	permEntries := p.Manifest.Permissions.DisplayEntries()

	var kvEntries []widgets.KeyValueEntry
	for _, e := range permEntries {
		kvEntries = append(kvEntries, widgets.KeyValueEntry{
			Key:   e.Name,
			Value: e.Value,
		})
	}
	for _, g := range p.Manifest.Grammars {
		kvEntries = append(kvEntries, widgets.KeyValueEntry{
			Key:   "Grammar",
			Value: grammarLabel(g),
		})
	}
	for _, theme := range p.Manifest.Themes {
		kvEntries = append(kvEntries, widgets.KeyValueEntry{Key: "Theme", Value: theme.ID()})
	}

	content := widgets.NewWrappingKeyValueListWidget(kvEntries)
	content.InvertStyles = true

	dialog := widgets.NewDialogWidget(max(50, content.NaturalWidth()+4))
	dialog.Title = p.Manifest.Title() + " requests"
	dialog.Borders = *a.Borders
	dialog.SetContent(content)
	dialog.Buttons = []widgets.DialogButton{
		{Label: "&Deny", Handler: func() {
			a.DismissDialog()
			a.denyPluginApproval(p)
		}},
		{Label: "&Allow", Handler: func() {
			a.DismissDialog()
			if err := a.PluginManager.ApproveAndLoad(p); err == nil {
				a.WirePlugin(p)
				a.installPluginGrammars(p)
			}
			a.updatePluginDetailButtons()
			if a.PluginsPanel != nil {
				a.PluginsPanel.Refresh()
			}
			a.showNextPluginApproval()
		}},
	}
	dialog.OnDismiss = func() {
		a.DismissDialog()
		a.denyPluginApproval(p)
	}
	dialog.Build()

	if a.approvalDialog != nil {
		a.Root.RemoveOverlay(a.approvalDialog)
	}
	adapter := ui.NewWidgetAdapter(dialog)
	a.approvalDialog = adapter
	a.ShowDialog(adapter)
}

// ShowExecPathApproval asks the user whether plugin p may run the binary at
// path. The full path is shown, wrapped rather than clipped, since it is what
// the user approves.
func (a *App) ShowExecPathApproval(p *plugin.Plugin, path string, done func(granted bool)) error {
	if a.Root.HasModalOverlay() {
		return errors.New("another dialog is open")
	}
	content := widgets.NewParagraphWidget(fmt.Sprintf(
		"Allow it to run %s? Its manifest allows programs named %s; this approves only this file.",
		path, filepath.Base(path)))
	dialog := widgets.NewDialogWidget(max(50, textwidth.String(path)+6))
	dialog.Title = p.Manifest.Title() + " requests"
	dialog.Borders = *a.Borders
	dialog.SetContent(content)
	deny := func() {
		a.DismissDialog()
		done(false)
	}
	dialog.Buttons = []widgets.DialogButton{
		{Label: "&Deny", Handler: deny},
		{Label: "&Allow", Handler: func() {
			a.DismissDialog()
			if err := a.PluginManager.GrantExecPath(p, path); err != nil {
				a.StatusError("Plugin permission not saved: " + err.Error())
				done(false)
				return
			}
			done(true)
		}},
	}
	dialog.OnDismiss = deny
	dialog.Build()
	a.ShowDialog(ui.NewWidgetAdapter(dialog))
	return nil
}

func (a *App) WirePlugin(p *plugin.Plugin) {
	a.installPluginThemes(p)
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
	a.syncPluginPanel(&a.Sidebar.TabbedPanel, id, sidebarTitle, sidebar)
	a.syncPluginPanel(&a.BottomPanel.TabbedPanel, id, bottomTitle, bottom)
	p.RequestRedraw = func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
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
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.OpenFile = func(path string, line int, readonly bool) {
		absPath := path
		if !filepath.IsAbs(path) {
			if cwd, err := os.Getwd(); err == nil {
				absPath = filepath.Join(cwd, path)
			}
		}
		if readonly {
			a.EditorGroup.OpenFileReadOnly(absPath, "")
		} else {
			a.EditorGroup.OpenFile(absPath)
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
		a.Screen.PostEvent(tcell.NewEventInterrupt(result))
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
		a.EditorGroup.SetDiagnosticsSource(pluginSource, path, diags)
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.ClearDiagnostics = func(path string) {
		if path == "" {
			a.EditorGroup.ClearDiagnosticsSource(pluginSource)
		} else {
			a.EditorGroup.SetDiagnosticsSource(pluginSource, path, nil)
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.SetDecorations = func(path string, items []plugin.DecorationItem) {
		decos := make([]ui.Decoration, len(items))
		for i, it := range items {
			decos[i] = ui.Decoration{
				StartLine: it.StartLine,
				StartCol:  it.StartCol,
				EndLine:   it.EndLine,
				EndCol:    it.EndCol,
				Text:      it.Text,
				Note:      it.Note,
				Style:     it.Style,
			}
		}
		a.EditorGroup.SetDecorations(pluginSource, path, decos)
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.ClearDecorations = func(path string) {
		if path == "" {
			a.EditorGroup.ClearDecorationsSource(pluginSource)
		} else {
			a.EditorGroup.SetDecorations(pluginSource, path, nil)
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.SetBookmark = func(line int, icon rune, style term.Style) {
		a.EditorGroup.Editor.SetBookmark(line, icon, style)
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.GetBookmark = func(line int) (rune, term.Style, bool) {
		b, ok := a.EditorGroup.Editor.GetBookmark(line)
		return b.Icon, b.Style, ok
	}
	p.RemoveBookmark = func(line int) {
		a.EditorGroup.Editor.RemoveBookmark(line)
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	p.SetAllBookmarks = func(items []plugin.BookmarkItem) {
		m := make(map[int]ui.Bookmark, len(items))
		for _, it := range items {
			m[it.Line] = ui.Bookmark{Icon: it.Icon, Style: it.Style}
		}
		a.EditorGroup.Editor.SetAllBookmarks(m)
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
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
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
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
		a.ShowConfirmDialogEx("Confirm", message, []string{cancelLabel, confirmLabel}, []func(){
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
	p.ShowContextMenu = func(entries []widgets.MenuEntry, x, y int, onCommand func(string)) {
		items := contextMenuItemsFromWidgetEntries(entries)
		a.captureMenuFocus()
		menu := ui.NewContextMenuWidget(items, x, y)
		menu.Borders = a.Borders
		menu.Icons = a.Settings.Appearance.Icons
		menu.OnExec = func(cmd string) {
			a.Root.PopOverlay()
			a.restoreMenuFocus()
			onCommand(cmd)
		}
		menu.OnDismiss = func() {
			a.Root.PopOverlay()
			a.restoreMenuFocus()
		}
		a.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
		a.Root.SetFocus(menu)
	}

	p.OpenDrawer = func(panel *plugin.PluginPanelWidget, width, minWidth int, side string) {
		a.ClosePluginDrawer()
		var adapter *ui.WidgetAdapter
		drawer := widgets.NewDrawerWidget(widgets.DrawerConfig{
			Width:    width,
			MinWidth: minWidth,
			Side:     side,
			Borders:  *a.Borders,
			OnDismiss: func() {
				a.Root.RemoveOverlay(adapter)
				if a.pluginDrawer == adapter {
					a.pluginDrawer = nil
					a.pluginDrawerPanel = nil
				}
				panel.Close()
				a.FocusEditor()
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
				ID:    cmd,
				Title: entry.Label,
				Handler: func() {
					p.CallSidebarAction(cmd)
				},
			})
		}
	}

	a.registerPluginCommandsAndKeys(p)
	p.FlushPendingDrawer()
	if len(p.FileHandlers) > 0 {
		a.EditorGroup.RetryFileHandlers()
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
		a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
}

func (a *App) registerPluginCommandsAndKeys(p *plugin.Plugin) {
	for _, cmd := range p.Commands {
		handler := cmd.Handler
		cmdID := cmd.ID
		plugName := p.Name
		a.Reg.Register(command.Command{
			ID:    cmd.ID,
			Title: cmd.Title,
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
			a.Root.AddChordKey(tcellSteps, func() {
				a.Reg.Execute(cmdID)
			})
		} else {
			key, mod, rn := comboToTcell(steps[0])
			a.Root.AddGlobalKey(key, mod, rn, func() {
				a.Reg.Execute(cmdID)
			})
		}
		a.Reg.SetShortcut(cmdID, FormatKeyBinding(kb.Key))
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

func (a *App) pluginReload() {
	plugins := a.PluginManager.Plugins()
	if len(plugins) == 0 {
		a.ShowConfirmDialogEx("Reload", "No plugins loaded.", []string{"Close"}, []func(){
			func() { a.DismissDialog() },
		})
		return
	}

	if len(plugins) == 1 {
		a.doPluginReload(plugins[0].Name)
		return
	}

	var items []widgets.SelectItem
	for _, p := range plugins {
		items = append(items, widgets.SelectItem{ID: p.Name, Label: p.Name})
	}
	a.ShowSelectDialog("Reload Plugin", items, func(name string) {
		a.doPluginReload(name)
	}, nil)
}

func (a *App) doPluginReload(name string) {
	a.RemovePluginPanels(name)
	a.ClearPluginSources(name)

	p, err := a.PluginManager.Reload(name)
	if err != nil {
		a.Output.AddLine(ui.OutputLine{
			Time:       time.Now().Format("15:04:05"),
			PluginName: name,
			Level:      "error",
			Message:    "reload failed: " + err.Error(),
		})
		if a.PluginsPanel != nil {
			a.PluginsPanel.Refresh()
		}
		return
	}

	a.WirePlugin(p)
	a.installPluginGrammars(p)

	a.Output.AddLine(ui.OutputLine{
		Time:       time.Now().Format("15:04:05"),
		PluginName: name,
		Level:      "info",
		Message:    "reloaded successfully",
	})
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
}

func (a *App) pluginReloadAll() {
	plugins := a.PluginManager.Plugins()
	if len(plugins) == 0 {
		return
	}

	var names []string
	for _, p := range plugins {
		names = append(names, p.Name)
	}
	for _, name := range names {
		a.doPluginReload(name)
	}
}

type RemoteRegistryResult struct {
	Entries []plugin.RemoteRegistryEntry
	Err     error
}

func (a *App) PluginInstallFromURL(repoURL, repoPath, name string) {
	go func() {
		p, err := a.PluginManager.Install(repoURL, repoPath)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&pluginInstallResult{
			plugin: p,
			name:   name,
			err:    err,
		}))
	}()
}

func (a *App) PluginUninstallByName(name string) {
	a.ShowConfirmDialogEx("Confirm Uninstall", "Remove plugin \""+name+"\"?", []string{"Cancel", "Uninstall"}, []func(){
		func() { a.DismissDialog() },
		func() {
			a.DismissDialog()
			a.doPluginUninstall(name)
		},
	})
}

func (a *App) PluginToggleByName(name string, enabled bool) {
	if !enabled {
		a.RemovePluginPanels(name)
		a.ClearPluginSources(name)
	}
	p, err := a.PluginManager.SetEnabled(name, enabled)
	if err != nil {
		slog.Error("toggle plugin", "error", err)
	}
	if p != nil && p.Loaded() {
		a.WirePlugin(p)
		a.installPluginGrammars(p)
	}
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
	if p != nil && !p.Loaded() {
		a.RemovePluginPanels(name)
		a.ClearPluginSources(name)
		a.requestPluginApproval(p)
	}
}

func (a *App) requestPluginApproval(p *plugin.Plugin) {
	a.PendingPluginApprovals = slices.DeleteFunc(a.PendingPluginApprovals, func(existing *plugin.Plugin) bool {
		return existing.Name == p.Name
	})
	a.PendingPluginApprovals = slices.Insert(a.PendingPluginApprovals, 0, p)
	a.ShowPluginApprovalDialog(p)
}

func (a *App) PluginUpdateByName(name string) {
	update := a.PluginManager.PrepareUpdate(name)
	go func() {
		err := update.Fetch()
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PluginUpdateResult{update: update, err: err}))
	}()
}

func (a *App) handleRemoteRegistryResult(result *RemoteRegistryResult) {
	if a.PluginsPanel == nil {
		return
	}
	if result.Err != nil {
		a.PluginsPanel.SearchTree.Config.EmptyText = "Could not load registry"
		a.PluginsPanel.SearchTree.SetItems(nil)
		return
	}
	a.PluginsPanel.SetAvailable(result.Entries)
}

func (a *App) ShowPluginDropdownMenu(entries []widgets.MenuEntry, x, y int) {
	items := contextMenuItemsFromWidgetEntries(entries)
	a.captureMenuFocus()
	menu := ui.NewContextMenuWidget(items, x, y)
	menu.Borders = a.Borders
	menu.Icons = a.Settings.Appearance.Icons
	menu.OnExec = func(cmd string) {
		a.Root.PopOverlay()
		a.restoreMenuFocus()
		a.handlePluginDropdownCommand(cmd)
	}
	menu.OnDismiss = func() {
		a.Root.PopOverlay()
		a.restoreMenuFocus()
	}
	a.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	a.Root.SetFocus(menu)
}

func contextMenuItemFromWidgetEntry(entry widgets.MenuEntry) ui.ContextMenuItem {
	item := ui.ContextMenuItem{
		Label:   entry.Label,
		Command: entry.Command,
		IsSep:   entry.Separator,
	}
	if !entry.Separator && entry.Checked != nil {
		item.Checked = ui.MenuUnchecked
		if *entry.Checked {
			item.Checked = ui.MenuChecked
		}
	}
	return item
}

func contextMenuItemsFromWidgetEntries(entries []widgets.MenuEntry) []ui.ContextMenuItem {
	items := make([]ui.ContextMenuItem, len(entries))
	for i, entry := range entries {
		items[i] = contextMenuItemFromWidgetEntry(entry)
	}
	return items
}

func (a *App) handlePluginDropdownCommand(cmd string) {
	switch cmd {
	case "updateAll":
		names := a.PluginManager.InstalledPluginNames()
		for _, name := range names {
			a.PluginUpdateByName(name)
		}
	case "refresh":
		a.PluginRefreshRegistry()
	case "help":
		a.Reg.Execute("plugin.help")
	}
}

// ShowPluginRowMenu opens the per-plugin actions menu (enable/disable, update,
// uninstall) for one installed plugin. Selecting a row no longer toggles it;
// these actions live here and on the e/u/x shortcuts.
func (a *App) ShowPluginRowMenu(name string, enabled bool, x, y int) {
	toggleLabel := "Disable"
	if !enabled {
		toggleLabel = "Enable"
	}
	items := []ui.ContextMenuItem{
		{Label: toggleLabel, Command: "toggle"},
		{Label: "Update", Command: "update"},
		{Label: "Uninstall", Command: "uninstall"},
	}
	menu := ui.NewContextMenuWidget(items, x, y)
	menu.Borders = a.Borders
	menu.Icons = a.Settings.Appearance.Icons
	restoreFocus := func() {
		a.Root.PopOverlay()
		if a.PluginsPanel != nil {
			a.Root.SetFocus(a.PluginsPanel.InstalledTree)
		}
	}
	menu.OnExec = func(cmd string) {
		restoreFocus()
		if a.PluginsPanel == nil {
			return
		}
		switch cmd {
		case "toggle":
			if a.PluginsPanel.OnToggle != nil {
				a.PluginsPanel.OnToggle(name, !enabled)
			} else {
				a.PluginToggleByName(name, !enabled)
			}
		case "update":
			if a.PluginsPanel.OnUpdate != nil {
				a.PluginsPanel.OnUpdate(name)
			} else {
				a.PluginUpdateByName(name)
			}
		case "uninstall":
			if a.PluginsPanel.OnUninstall != nil {
				a.PluginsPanel.OnUninstall(name)
			} else {
				a.PluginUninstallByName(name)
			}
		}
	}
	menu.OnDismiss = func() {
		restoreFocus()
	}
	a.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	a.Root.SetFocus(menu)
}

func (a *App) PluginRefreshRegistry() {
	if a.PluginsPanel != nil {
		a.PluginsPanel.SearchTree.Config.EmptyText = "Loading plugins..."
		a.PluginsPanel.SearchTree.SetItems(nil)
	}
	go func() {
		entries, err := plugin.FetchRemoteRegistry(plugin.DefaultRegistryURL)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&RemoteRegistryResult{Entries: entries, Err: err}))
	}()
}

// denyPluginApproval permanently declines a pending plugin: it deletes a fresh
// ttt-installed clone (or disables an on-disk one) so it stops prompting,
// resets the install button, and advances to the next pending approval.
func (a *App) denyPluginApproval(p *plugin.Plugin) {
	if err := a.PluginManager.DenyPlugin(p); err != nil {
		slog.Error("deny plugin", "error", err)
	}
	a.resetPluginDetailButton(p.Manifest.Name)
	a.updatePluginDetailButtons()
	if a.PluginsPanel != nil {
		a.PluginsPanel.Refresh()
	}
	a.showNextPluginApproval()
}

func (a *App) showNextPluginApproval() {
	if len(a.PendingPluginApprovals) > 0 {
		a.PendingPluginApprovals = a.PendingPluginApprovals[1:]
	}
	if len(a.PendingPluginApprovals) > 0 {
		a.ShowPluginApprovalDialog(a.PendingPluginApprovals[0])
	}
}

func (a *App) ShowPendingPluginApprovals() {
	if len(a.PendingPluginApprovals) > 0 {
		a.ShowPluginApprovalDialog(a.PendingPluginApprovals[0])
	}
}

// ShowPluginPanel makes plugin name's panel the active tab of area ("bottom"
// or "sidebar") and shows the area. Unless focus is set, it leaves alone an
// area whose active tab has keyboard focus and flags the panel's tab instead,
// reporting false. With focus it also focuses the panel and its autofocus
// input, unless a dialog is open; then it reports false.
func (a *App) ShowPluginPanel(name, area string, focus bool) (bool, error) {
	id := "plugin." + name
	tp, visible := &a.BottomPanel.TabbedPanel, a.ContentSplit.ShowBottom
	if area == "sidebar" {
		tp, visible = &a.Sidebar.TabbedPanel, a.Sidebar.Visible
	}
	if !tp.HasPanel(id) {
		return false, fmt.Errorf("plugin %s has no %s panel", name, area)
	}
	if !focus && visible && tp.ActivePanel != id && a.Root.Focused != nil && a.Root.Focused == tp.ActiveWidget() {
		tp.FlagPanel(id)
		return false, nil
	}
	tp.SetActivePanel(id)
	if area == "sidebar" {
		if !visible {
			a.ShowSidebar()
		}
	} else {
		a.ContentSplit.ShowBottom = true
	}
	if !focus {
		return true, nil
	}
	if a.Root.HasModalOverlay() {
		return false, nil
	}
	w := tp.ActiveWidget()
	a.Root.ClaimFocus(w)
	if adapter, ok := w.(*ui.WidgetAdapter); ok {
		if panel, ok := adapter.W.(*plugin.PluginPanelWidget); ok {
			panel.FocusAutofocusInput()
		}
	}
	return true, nil
}

func (a *App) pluginTab(name, key string) (int, *plugin.PluginPanelWidget) {
	for i := range a.EditorGroup.TabCount() {
		panel, ok := a.EditorGroup.TabContent(i).(*plugin.PluginPanelWidget)
		if ok && panel.PluginName() == name && a.EditorGroup.PluginTabKey(i) == key {
			return i, panel
		}
	}
	return -1, nil
}

func (a *App) switchToPluginTab(name, key string, focus bool) bool {
	i, panel := a.pluginTab(name, key)
	if i < 0 {
		return false
	}
	a.EditorGroup.SwitchTab(i)
	if focus && !a.Root.HasModalOverlay() {
		a.Root.ClaimFocus(a.EditorGroup)
		panel.FocusAutofocusInput()
	}
	return true
}

func (a *App) closePluginTab(name, key string) bool {
	if i, _ := a.pluginTab(name, key); i < 0 {
		return false
	}
	return a.EditorGroup.ClosePluginTab(key)
}

// Panel images live in the terminal until released, so a plugin panel is closed whenever it leaves the UI.
func (a *App) RemovePluginPanels(name string) {
	id := "plugin." + name
	for _, tp := range []*ui.TabbedPanel{&a.Sidebar.TabbedPanel, &a.BottomPanel.TabbedPanel} {
		closePluginPanel(tp.PanelWidget(id))
		tp.RemovePanel(id)
	}
	a.closePluginDrawerAndTabs(name)
}

func (a *App) closePluginDrawerAndTabs(name string) {
	a.closePluginDialog(name, false)
	if a.pluginDrawerPanel != nil && a.pluginDrawerPanel.PluginName() == name {
		a.ClosePluginDrawer()
	}
	// File-handler tabs are left to SyncFileHandlerTabs, which falls back to the binary card.
	var keys []string
	for i := range a.EditorGroup.TabCount() {
		path, _ := a.EditorGroup.TabInfo(i)
		if panel, ok := a.EditorGroup.TabContent(i).(*plugin.PluginPanelWidget); ok && panel.PluginName() == name {
			if _, handled := a.fileHandlerTabs[path]; !handled {
				keys = append(keys, a.EditorGroup.PluginTabKey(i))
			}
		}
	}
	for _, key := range keys {
		a.EditorGroup.ClosePluginTab(key)
	}
}

func (a *App) syncPluginPanel(tp *ui.TabbedPanel, id, title string, current *plugin.PluginPanelWidget) {
	switch {
	case current == nil:
		if tp.HasPanel(id) {
			closePluginPanel(tp.PanelWidget(id))
			tp.RemovePanel(id)
		}
	case !tp.HasPanel(id):
		tp.AddPanel(id, title, ui.NewWidgetAdapter(current))
	default:
		a.replaceStalePluginPanel(tp, id, current)
	}
}

func (a *App) replaceStalePluginPanel(tp *ui.TabbedPanel, id string, current *plugin.PluginPanelWidget) {
	old, ok := tp.PanelWidget(id).(*ui.WidgetAdapter)
	if !ok || old.W == current {
		return
	}
	closePluginPanel(old)
	replacement := ui.NewWidgetAdapter(current)
	tp.ReplacePanelWidget(id, replacement)
	if a.Root.Focused == old {
		a.Root.SetFocus(replacement)
	}
}

func closePluginPanel(w ui.Widget) {
	if adapter, ok := w.(*ui.WidgetAdapter); ok {
		if panel, ok := adapter.W.(*plugin.PluginPanelWidget); ok {
			panel.Close()
		}
	}
}

// pluginInput is input a plugin synthesized. The event loop drops it while a
// plugin approval dialog is open, so no plugin can grant permissions.
type pluginInput struct {
	Events    []tcell.Event
	Lifecycle *execRequestLifecycle
	Allowed   func() bool
}

// postPluginInput sends events as one interrupt: PostEvent falls back to a
// goroutine when the queue is full, which would reorder separate posts.
func (a *App) postPluginInput(allowed func() bool, events ...tcell.Event) {
	a.Screen.PostEvent(tcell.NewEventInterrupt(&pluginInput{Events: events, Allowed: allowed}))
}

func (a *App) simulatePluginKeys(combo string, allowed func() bool) error {
	steps, err := config.ParseKeyString(combo)
	if err != nil {
		return err
	}
	events := make([]tcell.Event, 0, len(steps))
	for _, step := range steps {
		key, mod, ch := comboToTcell(step)
		events = append(events, tcell.NewEventKey(key, keyEventStr(ch), mod))
	}
	a.postPluginInput(allowed, events...)
	return nil
}

const maxPluginTypeBytes = 64 << 10

func pluginTextEvents(text string) ([]tcell.Event, error) {
	if len(text) > maxPluginTypeBytes {
		return nil, fmt.Errorf("type input exceeds %d bytes; use automation paste for bulk text", maxPluginTypeBytes)
	}
	var events []tcell.Event
	for _, r := range strings.ReplaceAll(text, "\r\n", "\n") {
		if r == '\n' {
			events = append(events, tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
		} else {
			events = append(events, tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
		}
	}
	return events, nil
}

func (a *App) simulatePluginText(text string, allowed func() bool) error {
	events, err := pluginTextEvents(text)
	if err != nil {
		return err
	}
	if len(events) > 0 {
		a.postPluginInput(allowed, events...)
	}
	return nil
}

func (a *App) acceptsPluginInput() bool {
	return len(a.PendingPluginApprovals) == 0
}
