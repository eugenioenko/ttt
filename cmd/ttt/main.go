package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	"path/filepath"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

var (
	version         = "dev"
	profilerEnabled bool
)

func initLogger(debug bool) *os.File {
	if !debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))
		return nil
	}
	logPath := config.ConfigFilePath("ttt.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
		return nil
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return f
}

func handlePanic(screen *term.TcellScreen) {
	r := recover()
	if r == nil {
		return
	}
	if screen != nil {
		screen.Fini()
	}
	stack := debug.Stack()
	crashMsg := fmt.Sprintf("ttt crashed: %v\n\n%s", r, stack)
	crashPath := config.ConfigFilePath("crash.log")
	header := fmt.Sprintf("ttt crash report — %s\nVersion: %s\n\n", time.Now().Format(time.RFC3339), version)
	os.WriteFile(crashPath, []byte(header+crashMsg), 0644)
	fmt.Fprintf(os.Stderr, "ttt crashed. Crash log saved to %s\n", crashPath)
	fmt.Fprintln(os.Stderr, crashMsg)
	os.Exit(1)
}

type cliFlags struct {
	configFile   string
	pluginFile   string
	exec         string
	execSplitOn  string
	sizeW, sizeH int
	debug        bool
}

func parseFlags() cliFlags {
	var f cliFlags
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 < len(args) {
				f.configFile = args[i+1]
				i++
			}
		case "--plugin":
			if i+1 < len(args) {
				f.pluginFile = args[i+1]
				i++
			}
		case "--exec":
			if i+1 < len(args) {
				f.exec = args[i+1]
				i++
			}
		case "--exec-split-on":
			if i+1 < len(args) {
				f.execSplitOn = args[i+1]
				i++
			}
		case "--size":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%dx%d", &f.sizeW, &f.sizeH)
				i++
			}
		case "--debug":
			f.debug = true
		}
	}
	return f
}

func initTerminalScreen() *term.TcellScreen {
	screen, err := term.NewTcellScreen()
	if err != nil {
		panic(err)
	}
	return screen
}

func initSimulationScreen(w, h int) *term.TcellScreen {
	if w <= 0 || h <= 0 {
		w, h = 80, 25
	}
	sim := term.NewSimScreen()
	_ = sim.Init()
	sim.SetSize(w, h)
	return term.NewTcellScreenFrom(sim)
}

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--help", "-h":
			fmt.Printf(`ttt %s - TTT Editor, Terminal Text Tool, an IDE for your terminal

Usage: ttt [options] [files/folders/URLs...]

Arguments:
  files               Open one or more files
  folders             Open directories as workspace roots
  .                   Open the current directory
  PR URL              Open a GitHub pull request for review

Options:
  --help, -h          Show this help message
  --version, -v       Show version
  --workspace <file>  Open a saved workspace (.ttt file)
  --config <file>     Use a custom config file
  --exec "commands"   Execute semicolon-separated commands after startup
  --exec-split-on <s> Split --exec on <s> instead of ";" (for scripts that
                      need to send a literal semicolon)

Examples:
  ttt                                           Open current directory
  ttt main.go utils.go                          Open specific files
  ttt ~/projectA ~/projectB                     Multi-root workspace
  ttt . https://github.com/o/r/pull/123         Review a PR with repo tree

Docs: https://tttedit.dev
`, version)
			os.Exit(0)
		case "--version", "-v":
			fmt.Println("ttt " + version)
			os.Exit(0)
		}
	}

	if profilerEnabled {
		defer startProfiler()()
	}

	if dir := os.Getenv("TTT_CONFIG_DIR"); dir != "" {
		config.OverrideConfigDir = dir
	}

	flags := parseFlags()
	cfg := config.Load(flags.configFile)
	config.ParseKeyBindings(cfg.Keybindings)

	if flags.debug {
		cfg.Settings.DebugMode = true
	}

	logFile := initLogger(cfg.Settings.DebugMode)
	if logFile != nil {
		defer logFile.Close()
	}
	slog.Info("starting", "debugMode", cfg.Settings.DebugMode)

	var screen *term.TcellScreen
	if flags.exec != "" {
		screen = initSimulationScreen(flags.sizeW, flags.sizeH)
	} else {
		screen = initTerminalScreen()
	}
	defer screen.Fini()
	defer handlePanic(screen)

	screen.SetStyleMap(app.BuildStyleMap(cfg.Theme))
	screen.SetCursorStyle(term.ParseCursorStyle(cfg.Settings.Editor.CursorStyle))

	// Route OSC 52 clipboard writes through the tty, not raw stderr
	if tty, ok := screen.Tty(); ok {
		clipboard.SetOSCWriter(tty)
	}

	lspManager := lsp.NewManager(&cfg.Settings.LSP)
	defer lspManager.Shutdown()

	renderer := &render.Renderer{}
	cmdRegistry := command.NewRegistry()
	borders := app.BuildBorderSet(cfg.Theme.Borders)

	editor, prURLs := app.BuildApp(&cfg, &borders)
	editor.ApplyBorderStyle()
	editor.Init(screen, renderer, lspManager)

	editor.Version = version
	editor.Keybindings = cfg.Keybindings
	editor.Reg = cmdRegistry
	running := true
	editor.Running = &running
	app.RegisterCommands(editor)
	app.BindKeys(editor.Root, cmdRegistry, cfg.Keybindings)

	registryPath := config.ConfigFilePath("plugins.ttt.json")
	pluginsDir := filepath.Join(filepath.Dir(registryPath), "plugins")
	localPluginsDir := filepath.Join(editor.Workspace.Primary(), "plugins")
	pluginManager := plugin.NewManager(pluginsDir, registryPath, localPluginsDir)
	editor.PluginManager = pluginManager
	defer pluginManager.Shutdown()

	if cfg.Settings.Plugins.IsEnabled() {
		// Before LoadAll, so a plugin that fails to load can say why.
		pluginManager.SetLogFactory(func(pluginName string) func(string, string) {
			return func(level, message string) {
				editor.LogOutput(level, pluginName, message)
				screen.PostEvent(tcell.NewEventInterrupt(nil))
			}
		})

		pendingApprovals := pluginManager.LoadAll()

		for _, reg := range pluginManager.SidebarPanels {
			editor.Sidebar.AddPanel(reg.ID, reg.Title, ui.NewWidgetAdapter(reg.Widget))
		}
		for _, reg := range pluginManager.BottomPanels {
			editor.BottomPanel.AddPanel(reg.ID, reg.Title, ui.NewWidgetAdapter(reg.Widget))
		}
		pluginManager.SetEditorAPI(app.NewPluginEditorAPI(editor))
		pluginManager.SetFilesystemAPI(func(pluginDir string) plugin.FilesystemAPI {
			roots := editor.Workspace.Paths()
			if pluginDir != "" {
				roots = append(roots, pluginDir)
			}
			return app.NewPluginFilesystemAPI(roots...)
		})
		pluginManager.SetSystemAPI(app.NewPluginSystemAPI())
		pluginManager.SetNetworkAPI(app.NewPluginNetworkAPI())
		pluginManager.SetSettingsAPI(app.NewPluginSettingsAPI(editor))

		for _, p := range pluginManager.Plugins() {
			p.RequestRedraw = func() {
				screen.PostEvent(tcell.NewEventInterrupt(nil))
			}
			p.PostAsync = func(result *plugin.PluginAsyncResult) {
				screen.PostEvent(tcell.NewEventInterrupt(result))
			}
		}
		editor.RegisterStartupPluginCommands()

		pluginsPanel := app.NewPluginsPanel(pluginManager)
		editor.Sidebar.AddPanel("plugins", "Plugins", pluginsPanel.Adapter)
		editor.PluginsPanel = pluginsPanel
		pluginsPanel.OnInstall = func(repoURL, repoPath, name string) {
			editor.PluginInstallFromURL(repoURL, repoPath, name)
		}
		pluginsPanel.OnUninstall = func(name string) {
			editor.PluginUninstallByName(name)
		}
		pluginsPanel.OnToggle = func(name string, enabled bool) {
			if !enabled {
				editor.Sidebar.RemovePanel("plugin." + name)
				editor.BottomPanel.RemovePanel("plugin." + name)
				editor.EditorGroup.ClearDiagnosticsSource("plugin:" + name)
			}
			p, err := pluginManager.SetEnabled(name, enabled)
			if err != nil {
				slog.Error("toggle plugin", "error", err)
			}
			if p != nil {
				editor.WirePlugin(p)
			}
			pluginsPanel.Refresh()
		}
		pluginsPanel.OnUpdate = func(name string) {
			editor.PluginUpdateByName(name)
		}
		pluginsPanel.OnOpenDetail = func(entry plugin.RemoteRegistryEntry) {
			editor.OpenPluginDetail(entry)
		}
		pluginsPanel.OnRowMenu = func(name string, enabled bool, screenX, screenY int) {
			editor.ShowPluginRowMenu(name, enabled, screenX, screenY)
		}
		pluginsPanel.OnDropdownMenu = func(entries []widgets.MenuEntry, screenX, screenY int) {
			editor.ShowPluginDropdownMenu(entries, screenX, screenY)
		}

		go func() {
			entries, err := plugin.FetchRemoteRegistry(plugin.DefaultRegistryURL)
			screen.PostEvent(tcell.NewEventInterrupt(&app.RemoteRegistryResult{Entries: entries, Err: err}))
		}()

		if len(pendingApprovals) > 0 {
			editor.PendingPluginApprovals = pendingApprovals
		}
	}

	if len(prURLs) > 0 {
		if !github.IsGHInstalled() {
			editor.StatusError("GitHub CLI (gh) is required. Install from https://cli.github.com/")
		} else {
			editor.ShowSidebar()
			editor.Sidebar.SetActivePanel("changes")
			for _, url := range prURLs {
				editor.FetchAndOpenPR(url)
			}
		}
	}

	w, h := screen.Size()
	if flags.sizeW > 0 && flags.sizeH > 0 {
		w, h = flags.sizeW, flags.sizeH
	}
	editor.Root.SetSize(w, h)

	if flags.pluginFile != "" {
		app.LoadPluginFromFile(editor, flags.pluginFile)
	}

	if flags.exec != "" {
		go app.RunExecScriptSep(editor, flags.exec, flags.execSplitOn)
	}

	app.RunEventLoop(screen, renderer, editor, &running, editor.CloseTerminal)
}
