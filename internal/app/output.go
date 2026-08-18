package app

import (
	"log/slog"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
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
