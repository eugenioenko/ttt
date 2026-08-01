package plugin

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

type MarkdownSpan struct {
	Text  string
	Style term.Style
}

type MarkdownLine struct {
	Spans []MarkdownSpan
}

type PluginCommand struct {
	ID      string
	Title   string
	Handler func() error
}

type PluginKeybinding struct {
	Key     string
	Command string
}

type Plugin struct {
	mu sync.Mutex

	Name     string
	Dir      string
	Repo     string
	RepoPath string
	Manifest Manifest
	Granted  PermissionSet
	Enabled  bool
	State    *lua.LState

	SidebarTitle       string
	SidebarMenuEntries []widgets.MenuEntry
	sidebarMenuFunc    *lua.LFunction
	RenderFunc         *lua.LFunction
	EventFunc          *lua.LFunction

	BottomTitle      string
	BottomRenderFunc *lua.LFunction
	BottomEventFunc  *lua.LFunction

	InstallFunc   *lua.LFunction
	UninstallFunc *lua.LFunction

	Commands          []PluginCommand
	PluginKeybindings []PluginKeybinding

	RequestRedraw      func()
	PostAsync          func(*PluginAsyncResult)
	Log                func(level, message string)
	ShowContextMenu    func(entries []widgets.MenuEntry, x, y int, onCommand func(cmd string))
	ShowInfoDialog     func(title string, entries []widgets.KeyValueEntry)
	ShowConfirmDialog  func(message string, confirmLabel string, cancelLabel string, onConfirm func())
	OpenDrawer         func(panel *PluginPanelWidget, width, minWidth int, side string)
	CloseDrawer        func()
	OpenTab            func(id string, panel *PluginPanelWidget)
	CloseTab           func(id string)
	RenderMarkdown     func(text string) []MarkdownLine
	Markdown           config.MarkdownSettings
	Borders            *term.BorderSet
	SimulateClick      func(x, y int)
	SimulateDrag       func(x1, y1, x2, y2 int)
	ScreenshotToFile   func(path string) error
	DebugDumpToFile    func(path string) error
	QuitApp            func()
	OpenFile           func(path string, line int)
	Notify             func(message, level string)
	SetStatusItem      func(side, id, text string, priority int, onClick func())
	RemoveStatusItem   func(id string)
	SetEcho            func(text string) // status bar shows only this text when non-empty
	ExecCommand        func(id string) bool
	ListCommands       func() []CommandInfo
	PublishDiagnostics func(path string, items []DiagnosticItem)
	ClearDiagnostics   func(path string)

	ShowCommandLine    func(CommandLineOptions)
	HideCommandLine    func()
	SetCommandLineText func(text string)
	CommandLineActive  func() bool

	EditorContextProvider *lua.LFunction

	Editor     EditorAPI
	Filesystem FilesystemAPI
	System     SystemAPI
	Network    NetworkAPI
	Settings   SettingsAPI

	EventListeners map[string][]*lua.LFunction

	pendingDrawer *pendingDrawerCall
	LastError     error
	errorCount    int

	timers      map[int]func()
	nextTimerID int
}

type CommandInfo struct {
	ID    string
	Title string
}

type pendingDrawerCall struct {
	panel    *PluginPanelWidget
	width    int
	minWidth int
	side     string
}

func (p *Plugin) FlushPendingDrawer() {
	if p.pendingDrawer != nil && p.OpenDrawer != nil {
		pd := p.pendingDrawer
		p.pendingDrawer = nil
		p.OpenDrawer(pd.panel, pd.width, pd.minWidth, pd.side)
	}
}

func (p *Plugin) Init() error {
	p.EventListeners = make(map[string][]*lua.LFunction)
	p.State = NewSandbox()
	setupTTTModule(p.State, p)
	setupEditorModule(p.State, p)
	setupDiagnosticsModule(p.State, p)
	setupFsModule(p.State, p)
	setupSystemModule(p.State, p)
	setupNetModule(p.State, p)
	setupEventsModule(p.State, p)
	setupJSONModule(p.State)
	setupSettingsModule(p.State, p)

	entry := filepath.Join(p.Dir, p.Manifest.Entry)
	absEntry, err := filepath.Abs(entry)
	if err != nil {
		p.State.Close()
		p.State = nil
		return fmt.Errorf("invalid entry path: %w", err)
	}
	absDir, _ := filepath.Abs(p.Dir)
	if !strings.HasPrefix(absEntry, absDir+string(filepath.Separator)) {
		p.State.Close()
		p.State = nil
		return fmt.Errorf("entry path %q escapes plugin directory", p.Manifest.Entry)
	}
	if err := p.State.DoFile(entry); err != nil {
		p.LastError = err
		p.State.Close()
		p.State = nil
		p.logError("init", err)
		return err
	}

	p.Enabled = true
	return nil
}

func (p *Plugin) InitFromSource(source string) error {
	p.EventListeners = make(map[string][]*lua.LFunction)
	p.State = NewSandbox()
	setupTTTModule(p.State, p)
	setupEditorModule(p.State, p)
	setupDiagnosticsModule(p.State, p)
	setupFsModule(p.State, p)
	setupSystemModule(p.State, p)
	setupNetModule(p.State, p)
	setupEventsModule(p.State, p)
	setupJSONModule(p.State)
	setupSettingsModule(p.State, p)

	if err := p.State.DoString(source); err != nil {
		p.LastError = err
		p.State.Close()
		p.State = nil
		p.logError("init", err)
		return err
	}

	p.Enabled = true
	return nil
}

const maxPluginErrors = 10

func (p *Plugin) logError(context string, err error) {
	p.errorCount++
	slog.Error("plugin error", "plugin", p.Name, "context", context, "error", err)
	if p.Log != nil {
		p.Log("error", context+": "+err.Error())
	}
	if p.errorCount >= maxPluginErrors {
		p.Enabled = false
		if p.Log != nil {
			p.Log("error", fmt.Sprintf("plugin %q disabled after %d errors", p.Name, p.errorCount))
		}
	}
}

func (p *Plugin) SafePostAsync(result *PluginAsyncResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.PostAsync != nil {
		p.PostAsync(result)
	}
}

func (p *Plugin) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopTimersLocked()
	if p.State != nil {
		p.State.Close()
		p.State = nil
	}
	p.Enabled = false
	p.RenderFunc = nil
	p.EventFunc = nil
	p.BottomRenderFunc = nil
	p.BottomEventFunc = nil
	p.sidebarMenuFunc = nil
	p.Commands = nil
	p.PluginKeybindings = nil
	p.RequestRedraw = nil
	p.PostAsync = nil
	p.Log = nil
	p.ShowContextMenu = nil
	p.ShowInfoDialog = nil
	p.ShowConfirmDialog = nil
	p.OpenDrawer = nil
	p.CloseDrawer = nil
	p.OpenTab = nil
	p.CloseTab = nil
	p.RenderMarkdown = nil
	p.Borders = nil
	p.SimulateClick = nil
	p.SimulateDrag = nil
	p.ScreenshotToFile = nil
	p.DebugDumpToFile = nil
	p.QuitApp = nil
	p.Notify = nil
	p.SetStatusItem = nil
	p.RemoveStatusItem = nil
	p.SetEcho = nil
	p.ExecCommand = nil
	p.ListCommands = nil
	p.PublishDiagnostics = nil
	p.ClearDiagnostics = nil
	p.ShowCommandLine = nil
	p.HideCommandLine = nil
	p.SetCommandLineText = nil
	p.CommandLineActive = nil
	p.EditorContextProvider = nil
	p.EventListeners = nil
}

func (p *Plugin) HasSidebarMenu() bool {
	return p.sidebarMenuFunc != nil
}

func (p *Plugin) CallSidebarAction(cmd string) {
	if p.sidebarMenuFunc != nil && p.State != nil {
		p.CallLuaFunc(p.sidebarMenuFunc, lua.LString(cmd))
	}
}

func (p *Plugin) DispatchKeyEvent(ev *lua.LTable) bool {
	listeners := p.EventListeners["key.press"]
	if len(listeners) == 0 || p.State == nil {
		return false
	}
	for _, fn := range listeners {
		err := p.State.CallByParam(lua.P{
			Fn:      fn,
			NRet:    1,
			Protect: true,
		}, ev)
		if err != nil {
			p.logError("event key.press", err)
			continue
		}
		ret := p.State.Get(-1)
		p.State.Pop(1)
		if lua.LVAsBool(ret) {
			return true
		}
	}
	return false
}

func (p *Plugin) DispatchEvent(name string, args ...lua.LValue) {
	listeners := p.EventListeners[name]
	if len(listeners) == 0 || p.State == nil {
		return
	}
	for _, fn := range listeners {
		if err := p.CallLuaFunc(fn, args...); err != nil {
			p.logError("event "+name, err)
		}
	}
}

func (p *Plugin) CallRender(proxy *PanelProxy) error {
	return p.CallRenderWith(p.RenderFunc, proxy)
}

func (p *Plugin) CallRenderWith(renderFunc *lua.LFunction, proxy *PanelProxy) error {
	if p.State == nil || renderFunc == nil {
		return nil
	}

	ud := PushPanelProxy(p.State, proxy)
	err := p.State.CallByParam(lua.P{
		Fn:      renderFunc,
		NRet:    0,
		Protect: true,
	}, ud)
	if err != nil {
		p.LastError = err
		p.logError("render", err)
	}
	return err
}
