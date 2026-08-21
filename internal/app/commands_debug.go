package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func registerDebugCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID:       "debug.simulateClick",
		Title:    "Debug: Simulate Click",
		Keywords: []string{"debug", "click", "mouse", "test"},
		Handler:  func() { app.debugSimulateClick() },
	})

	reg.Register(command.Command{
		ID:       "debug.runPlugin",
		Title:    "Debug: Run Current File as Plugin",
		Keywords: []string{"debug", "plugin", "lua", "test", "run"},
		Handler:  func() { app.debugRunPlugin() },
	})

	reg.Register(command.Command{
		ID:       "debug.screenshot",
		Title:    "Debug: Screenshot",
		Keywords: []string{"debug", "screenshot", "capture"},
		Handler:  func() { app.debugScreenshot() },
	})

	reg.Register(command.Command{
		ID:       "debug.dumpState",
		Title:    "Debug: Dump State",
		Keywords: []string{"debug", "dump", "state", "json"},
		Handler:  func() { app.debugDumpState() },
	})

	reg.Register(command.Command{
		ID:       "debug.keyboardTester",
		Title:    "Debug: Keyboard Tester",
		Keywords: []string{"debug", "keyboard", "key", "tester", "input"},
		Handler:  func() { app.debugKeyboardTester() },
	})
}

func (a *App) debugSimulateClick() {
	submit := func(text string) {
		a.DismissDialog()
		parts := strings.Fields(text)
		if len(parts) < 2 {
			a.Status.SetNotification("Usage: x y", view.NotifyWarning, 3*time.Second)
			return
		}
		x, err1 := strconv.Atoi(parts[0])
		y, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			a.Status.SetNotification("Invalid coordinates", view.NotifyWarning, 3*time.Second)
			return
		}
		go func() {
			a.Screen.PostEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
			time.Sleep(50 * time.Millisecond)
			a.Screen.PostEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
		}()
	}

	input := widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: "x y",
		OnSubmit:    submit,
	})

	dialog := widgets.NewDialogWidget(40)
	dialog.Title = "Simulate Click"
	dialog.Borders = *a.Borders
	dialog.Buttons = []widgets.DialogButton{
		{Label: "&Click", Handler: func() { submit(input.Text()) }},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.SetContent(input)
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}

func (a *App) debugRunPlugin() {
	if a.PluginManager == nil || a.EditorGroup.Editor == nil {
		a.Status.SetNotification("No active editor", view.NotifyWarning, 3*time.Second)
		return
	}

	source := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n")
	if strings.TrimSpace(source) == "" {
		a.Status.SetNotification("Buffer is empty", view.NotifyWarning, 3*time.Second)
		return
	}

	path := a.EditorGroup.ActiveFilePath()
	name := "debug-plugin"
	dir := "."
	if path != "" {
		parts := strings.Split(path, "/")
		name = strings.TrimSuffix(parts[len(parts)-1], ".lua")
		dir = filepath.Dir(path)
	}

	p := &plugin.Plugin{
		Name:    name,
		Dir:     dir,
		Enabled: true,
		Manifest: plugin.Manifest{
			Name:  name,
			Entry: "inline",
		},
		Granted: plugin.PermissionSet{
			PanelSidebar:      true,
			PanelBottom:       true,
			PanelDrawer:       true,
			PanelEditor:       true,
			Commands:          true,
			Keybindings:       true,
			EditorRead:        true,
			EditorWrite:       true,
			EditorDiagnostics: true,
			FsRead:            true,
			FsWrite:           true,
			SystemEnv:         true,
			NetworkHTTP:       plugin.NetworkHTTP{All: true},
			EventsFile:        true,
			EventsEditor:      true,
		},
	}

	if err := p.InitFromSource(source); err != nil {
		a.Status.SetNotification("Plugin error: "+err.Error(), view.NotifyError, 5*time.Second)
		return
	}

	a.PluginManager.RegisterDebugPlugin(p)
	a.WirePlugin(p)

	hasSidebar := p.SidebarTitle != ""
	hasBottom := p.BottomTitle != ""

	if hasSidebar {
		a.Sidebar.SetActivePanel("plugin." + p.Name)
	}
	if hasBottom {
		a.BottomPanel.SetActivePanel("plugin." + p.Name)
	}

	a.Status.SetNotification("Plugin loaded: "+name, view.NotifyInfo, 3*time.Second)
}

func (a *App) debugScreenshot() {
	a.DismissDialog()
	time.AfterFunc(50*time.Millisecond, func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(&DebugWriteRequest{Kind: "screenshot", Path: "screenshot.txt"}))
	})
}

func (a *App) debugDumpState() {
	a.DismissDialog()
	time.AfterFunc(50*time.Millisecond, func() {
		a.Screen.PostEvent(tcell.NewEventInterrupt(&DebugWriteRequest{Kind: "state", Path: "debug-state.json"}))
	})
}

// DebugWriteRequest returns delayed debug capture work to the main event loop.
// Both Screenshot and BuildDebugState read widget state and must not race the
// renderer from a timer goroutine.
type DebugWriteRequest struct {
	Kind string
	Path string
}

func (a *App) HandleDebugWriteRequest(request *DebugWriteRequest) {
	var err error
	switch request.Kind {
	case "screenshot":
		err = a.DumpScreenshot(request.Path)
		if err != nil {
			a.Status.SetNotification("Screenshot error: "+err.Error(), view.NotifyError, 3*time.Second)
		} else {
			a.Status.SetNotification("Screenshot saved: "+request.Path, view.NotifyInfo, 3*time.Second)
		}
	case "state":
		err = a.DumpDebugState(request.Path)
		if err != nil {
			a.Status.SetNotification("Dump error: "+err.Error(), view.NotifyError, 3*time.Second)
		} else {
			a.Status.SetNotification("State saved: "+request.Path, view.NotifyInfo, 3*time.Second)
		}
	}
}

func (a *App) debugKeyboardTester() {
	if a.Root.HasOverlay() {
		return
	}
	kt := ui.NewKeyTesterWidget()
	kt.Borders = a.Borders
	kt.LookupBinding = func(combo string) string {
		for _, kb := range a.Keybindings {
			if kb.Key == combo {
				return kb.Command
			}
		}
		return ""
	}
	kt.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(kt)
}

func LoadPluginFromFile(a *App, path string) {
	source, err := os.ReadFile(path)
	if err != nil {
		slog.Error("load plugin file", "path", path, "error", err)
		return
	}

	name := strings.TrimSuffix(filepath.Base(path), ".lua")
	p := &plugin.Plugin{
		Name:    name,
		Dir:     filepath.Dir(path),
		Enabled: true,
		Manifest: plugin.Manifest{
			Name:  name,
			Entry: "inline",
		},
		Granted: plugin.PermissionSet{
			PanelSidebar:      true,
			PanelBottom:       true,
			PanelDrawer:       true,
			PanelEditor:       true,
			Commands:          true,
			Keybindings:       true,
			EditorRead:        true,
			EditorWrite:       true,
			EditorDiagnostics: true,
			FsRead:            true,
			FsWrite:           true,
			SystemEnv:         true,
			NetworkHTTP:       plugin.NetworkHTTP{All: true},
			EventsFile:        true,
			EventsEditor:      true,
			Settings:          true,
			SettingsKeys:      []string{"*"},
		},
	}

	// Wire the settings API before init so plugins loaded via --plugin can read
	// settings at startup, matching the production ApproveAndLoad path where
	// wireAPIs runs before Init.
	p.Settings = NewPluginSettingsAPI(a)

	if err := p.InitFromSource(string(source)); err != nil {
		slog.Error("init plugin from file", "path", path, "error", err)
		return
	}

	a.PluginManager.RegisterDebugPlugin(p)
	a.WirePlugin(p)
	a.ensureKeyInterceptor()

	slog.Info("plugin loaded from file", "path", path, "name", name)
}
