//go:build js && wasm

package main

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"syscall/js"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			js.Global().Call("tttDemoError", fmt.Sprint(err))
		}
	}()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	clipboard.DisableSystem()
	config.OverrideConfigDir = "/home/demo/.config/ttt"
	cfg := config.Load("")
	disabled := false
	cfg.Settings.LSP.Enabled = &disabled
	cfg.Settings.LSP.Servers = nil
	cfg.Settings.Plugins.Enabled = &disabled
	cfg.Settings.Editor.GitGutter = &disabled
	cfg.Settings.Editor.FormatOnSave = false
	cfg.Settings.Autocomplete.Enabled = false
	cfg.Settings.Appearance.Icons = config.IconsNone
	config.ParseKeyBindings(cfg.Keybindings)

	screen, err := term.NewTcellScreen()
	if err != nil {
		panic(err)
	}
	defer screen.Fini()
	app.ApplyThemeStyles(screen, cfg.Theme)
	screen.SetCursorStyle(term.ParseCursorStyle(cfg.Settings.Editor.CursorStyle))
	renderer := &render.Renderer{}
	borders := app.BuildBorderSet(cfg.Theme.Borders)
	editor := app.BuildAppFromConfig(&cfg, &borders, workspace.New([]string{"/ttt"}), []app.FileTarget{{Path: "/ttt/README.md"}})
	editor.Repository = nil
	editor.ApplyBorderStyle()
	languages := lsp.NewManager(&cfg.Settings.LSP)
	defer languages.Shutdown()
	editor.Init(screen, renderer, languages, nil)
	editor.Reg = command.NewRegistry()
	editor.Version = "browser demo"
	editor.Keybindings = cfg.Keybindings
	running := true
	editor.Running = &running
	app.RegisterCommands(editor)
	for _, cmd := range editor.Reg.List() {
		if !supportedCommand(cmd.ID) {
			cmd.Hidden = true
			cmd.Handler = func() { editor.StatusNotify("Available in the desktop editor; this demo supports editing only.") }
			editor.Reg.Register(cmd)
		}
	}
	app.BindKeys(editor.Root, editor.Reg, cfg.Keybindings)
	for _, id := range []string{"search", "changes", "outline"} {
		editor.Sidebar.RemovePanel(id)
	}
	for _, id := range []string{"terminal", "problems", "references"} {
		editor.BottomPanel.RemovePanel(id)
	}
	w, h := screen.Size()
	editor.Root.SetSize(w, h)
	time.AfterFunc(0, func() { js.Global().Call("tttDemoReady") })
	app.RunEventLoop(screen, renderer, editor, &running, editor.CloseTerminal)
}

func supportedCommand(id string) bool {
	switch id {
	case "editor.autocomplete", "editor.hover", "editor.goToDefinition", "editor.goToImplementation", "editor.goToTypeDefinition", "editor.findReferences", "editor.rename", "editor.organizeImports", "editor.fixAll", "editor.formatDocument", "editor.formatSelection", "editor.formatExternal", "editor.quit", "explorer.reveal", "options.toggleLSP", "options.toggleLSPHover", "options.toggleGitGutter", "options.toggleFormatOnSave":
		return false
	case "sidebar.search", "sidebar.searchReplace", "sidebar.changes", "sidebar.outline":
		return false
	case "command.palette", "theme.switch", "view.keybindings", "about", "search.find", "search.findNext", "search.findPrev", "search.clearFind", "search.replace":
		return true
	}
	for _, prefix := range []string{"editor.", "file.", "tab.", "fold.", "multicursor.", "selection.", "explorer.", "menu.", "options.", "sidebar.", "panel.", "focus."} {
		if strings.HasPrefix(id, prefix) {
			return id != "focus.terminal"
		}
	}
	return false
}
