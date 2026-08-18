package app

import (
	"log/slog"
	"os/exec"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/gdamore/tcell/v3"
)

func registerOutputCommands(app *App) {
	reg := app.Reg
	reg.Register(command.Command{
		ID:       "output.clear",
		Title:    "Output: Clear",
		Keywords: []string{"output", "clear", "log", "plugin", "lsp"},
		Handler:  func() { app.Output.Clear() },
	})
	reg.Register(command.Command{
		ID:       "output.show",
		Title:    "Output: Show Panel",
		Keywords: []string{"output", "log", "panel", "show", "lsp"},
		Handler:  func() { app.ShowOutputPanel() },
	})
	reg.Register(command.Command{
		ID:       "output.copyLine",
		Title:    "Output: Copy Selected Line",
		Keywords: []string{"output", "copy", "log", "line"},
		Handler:  func() { app.OutputCopyLine() },
	})
}

// SyncLanguageSegment refreshes the language status bar segment. It is separate
// from syncStatus so a language server changing state asynchronously can update
// the indicator without the full status sync, which the event loop runs only on
// user input.
func (a *App) SyncLanguageSegment() {
	if a.EditorGroup.Editor == nil || a.EditorGroup.Editor.Highlighter == nil {
		a.Status.SetSegment(view.StatusSegment{ID: "language", Side: "right", Priority: 500, Text: ""})
		return
	}

	lang := a.EditorGroup.Editor.Highlighter.Language()
	icon, iconStyle := a.lspStatusIcon(a.EditorGroup.ActiveFilePath(), lang)
	text := lang
	if icon != "" {
		text += " " + icon
	}
	a.Status.SetSegment(view.StatusSegment{
		ID: "language", Side: "right", Priority: 500,
		Text:    text,
		Style:   iconStyle,
		OnClick: a.ShowOutputPanel,
	})
}

// lspStatusIcon reports the language server indicator for the status bar.
//
// The icons are all single-width under both normal and East Asian width rules,
// so the segment does not shift as the state changes. ⊕, ● and ○ are ambiguous
// width and would.
func (a *App) lspStatusIcon(filePath, lang string) (string, term.Style) {
	serverKey, _, configured := a.lspResolve(filePath, lang)
	if !configured {
		return "", 0
	}

	binaryOK := true
	if cfg := a.LspManager.ServerConfig(serverKey); len(cfg.Command) > 0 {
		_, err := exec.LookPath(cfg.Command[0])
		binaryOK = err == nil
	}
	return lspIcon(a.LspManager.State(serverKey), binaryOK)
}

func lspIcon(state lsp.ServerState, binaryOK bool) (string, term.Style) {
	switch state {
	case lsp.ServerReady:
		return "◉", 0
	case lsp.ServerStarting:
		return "◌", 0
	case lsp.ServerFailed:
		return "⚠", term.StyleDanger
	}

	// Not started — servers launch lazily on the first request. A missing
	// binary is the one failure knowable before then, and ttt never even tries.
	if !binaryOK {
		return "⚠", term.StyleDanger
	}
	return "◌", 0
}

// ShowOutputPanel reveals the bottom panel on the Output tab and focuses it.
func (a *App) ShowOutputPanel() {
	a.BottomPanel.SetActivePanel("output")
	a.FocusPanel()
}

// OutputCopyLine copies the selected Output line, mirroring what Ctrl+C does
// while the panel has focus.
func (a *App) OutputCopyLine() {
	if a.Output == nil || !a.Output.CopySelected() {
		return
	}
	a.StatusNotify("Output line copied to clipboard")
}

// LSPStateChanged tells the event loop a language server changed state.
type LSPStateChanged struct{}

// OutputLineResult carries a log line from a background goroutine to the event
// loop. Widget state must only be mutated on the main thread.
type OutputLineResult struct {
	Source  string
	Level   string
	Message string
}

// LogOutput appends a line to the Output panel and mirrors it to slog, so the
// debug log file stays a superset of what the panel shows. Main thread only —
// background goroutines must use LogOutputAsync.
func (a *App) LogOutput(level, source, message string) {
	if a.Output == nil {
		return
	}
	a.Output.AddLine(ui.OutputLine{
		Time:       time.Now().Format("15:04:05"),
		PluginName: source,
		Level:      level,
		Message:    message,
	})
	switch level {
	case "error":
		slog.Error(message, "source", source)
	case "warn":
		slog.Warn(message, "source", source)
	default:
		slog.Info(message, "source", source)
	}
}

// LogOutputAsync routes a log line through the event loop so it lands on the
// main thread.
func (a *App) LogOutputAsync(level, source, message string) {
	if a.Screen == nil {
		return
	}
	a.Screen.PostEvent(tcell.NewEventInterrupt(&OutputLineResult{
		Source:  source,
		Level:   level,
		Message: message,
	}))
}

func outputLevelForNotify(level view.NotifyLevel) string {
	switch level {
	case view.NotifyWarning:
		return "warn"
	case view.NotifyError:
		return "error"
	default:
		return "info"
	}
}
