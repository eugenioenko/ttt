package app

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/config"

	"github.com/gdamore/tcell/v3"
)

// DefaultExecSeparator splits exec scripts when no override is given.
const DefaultExecSeparator = ";"

// RunExecScript parses a separator-delimited script string and executes each
// command sequentially. Intended to be run in a goroutine after the event loop
// starts.
func RunExecScript(a *App, script string) {
	RunExecScriptSep(a, script, DefaultExecSeparator)
}

// RunExecScriptSep is RunExecScript with a caller-chosen separator. Semicolons
// are unusable when the script must type or send one -- `;` is a Vim motion,
// for instance -- so callers can pick a delimiter that cannot collide with
// their payload.
func RunExecScriptSep(a *App, script, sep string) {
	if sep == "" {
		sep = DefaultExecSeparator
	}
	commands := strings.Split(script, sep)
	for _, raw := range commands {
		cmd := strings.TrimSpace(raw)
		if cmd == "" {
			continue
		}
		action, args := parseExecCommand(cmd)
		slog.Debug("exec_script", "action", action, "args", args)

		switch action {
		case "click":
			execClick(a, args)
		case "rclick":
			execRClick(a, args)
		case "hover":
			execHover(a, args)
		case "drag":
			execDrag(a, args)
		case "key":
			execKey(a, args)
		case "type":
			execType(a, args)
		case "exec":
			execCommand(a, args)
		case "screenshot":
			execScreenshot(a, args)
		case "debug":
			execDebug(a, args)
		case "wait":
			execWait(args)
		case "paste":
			execPaste(a, args)
		case "copy":
			a.Copy()
		case "panel":
			execPanel(a, args)
		case "quit", "shutdown":
			execQuit(a)
		default:
			slog.Error("exec_script: unknown command", "action", action)
		}

		// Small implicit delay between commands to let the event loop process
		time.Sleep(50 * time.Millisecond)
	}
}

// parseExecCommand splits a command string into action and arguments.
// Handles quoted strings for the exec command (e.g., exec "Command Name").
func parseExecCommand(cmd string) (string, string) {
	// Find the first space to split action from args
	idx := strings.IndexByte(cmd, ' ')
	if idx < 0 {
		return cmd, ""
	}
	return cmd[:idx], strings.TrimSpace(cmd[idx+1:])
}

func execClick(a *App, args string) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		slog.Error("exec_script: click requires X Y", "args", args)
		return
	}
	x, err := strconv.Atoi(parts[0])
	if err != nil {
		slog.Error("exec_script: invalid click X", "value", parts[0], "error", err)
		return
	}
	y, err := strconv.Atoi(parts[1])
	if err != nil {
		slog.Error("exec_script: invalid click Y", "value", parts[1], "error", err)
		return
	}

	// Press
	a.Screen.PostEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	time.Sleep(50 * time.Millisecond)
	// Release
	a.Screen.PostEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
}

// execRClick simulates a secondary (right) mouse click, which opens context
// menus. Right-click is tcell.Button2 (see menus.go).
func execRClick(a *App, args string) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		slog.Error("exec_script: rclick requires X Y", "args", args)
		return
	}
	x, err := strconv.Atoi(parts[0])
	if err != nil {
		slog.Error("exec_script: invalid rclick X", "value", parts[0], "error", err)
		return
	}
	y, err := strconv.Atoi(parts[1])
	if err != nil {
		slog.Error("exec_script: invalid rclick Y", "value", parts[1], "error", err)
		return
	}

	// Press
	a.Screen.PostEvent(tcell.NewEventMouse(x, y, tcell.Button2, tcell.ModNone))
	time.Sleep(50 * time.Millisecond)
	// Release
	a.Screen.PostEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
}

func execHover(a *App, args string) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		slog.Error("exec_script: hover requires X Y", "args", args)
		return
	}
	x, err := strconv.Atoi(parts[0])
	if err != nil {
		slog.Error("exec_script: invalid hover X", "value", parts[0], "error", err)
		return
	}
	y, err := strconv.Atoi(parts[1])
	if err != nil {
		slog.Error("exec_script: invalid hover Y", "value", parts[1], "error", err)
		return
	}
	a.Screen.PostEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
}

func execDrag(a *App, args string) {
	parts := strings.Fields(args)
	if len(parts) < 4 {
		slog.Error("exec_script: drag requires X1 Y1 X2 Y2", "args", args)
		return
	}
	x1, err1 := strconv.Atoi(parts[0])
	y1, err2 := strconv.Atoi(parts[1])
	x2, err3 := strconv.Atoi(parts[2])
	y2, err4 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		slog.Error("exec_script: invalid drag coordinates", "args", args)
		return
	}

	a.Screen.PostEvent(tcell.NewEventMouse(x1, y1, tcell.Button1, tcell.ModNone))
	time.Sleep(30 * time.Millisecond)

	steps := 10
	for i := 1; i <= steps; i++ {
		mx := x1 + (x2-x1)*i/steps
		my := y1 + (y2-y1)*i/steps
		a.Screen.PostEvent(tcell.NewEventMouse(mx, my, tcell.Button1, tcell.ModNone))
		time.Sleep(15 * time.Millisecond)
	}

	a.Screen.PostEvent(tcell.NewEventMouse(x2, y2, tcell.ButtonNone, tcell.ModNone))
}

func execKey(a *App, args string) {
	combo := strings.TrimSpace(args)
	if combo == "" {
		slog.Error("exec_script: key requires a key combo")
		return
	}

	steps, err := config.ParseKeyString(combo)
	if err != nil {
		slog.Error("exec_script: invalid key combo", "combo", combo, "error", err)
		return
	}

	for _, step := range steps {
		key, mod, ch := comboToTcell(step)
		a.Screen.PostEvent(tcell.NewEventKey(key, keyEventStr(ch), mod))
		time.Sleep(30 * time.Millisecond)
	}
}

func execType(a *App, args string) {
	text := stripQuotes(args)
	for _, r := range text {
		a.Screen.PostEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
		time.Sleep(10 * time.Millisecond)
	}
}

func execPaste(a *App, args string) {
	text := stripQuotes(args)
	if text == "" {
		slog.Error("exec_script: paste requires text")
		return
	}
	a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	time.Sleep(50 * time.Millisecond)
	a.PasteText(text)
	a.FlushEditorOnChange()
	a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
}

func execCommand(a *App, args string) {
	title := stripQuotes(strings.TrimSpace(args))
	if title == "" {
		slog.Error("exec_script: exec requires a command title")
		return
	}

	cmd, ok := a.Reg.FindByTitle(title)
	if !ok {
		slog.Error("exec_script: command not found", "title", title)
		return
	}
	// Hand the command to the event loop instead of running it here. `key` and
	// `click` already post real events, so they get the status bar sync that
	// follows real input; running a command straight from the script goroutine
	// skipped that sync and mutated widget state off the main thread.
	a.Screen.PostEvent(tcell.NewEventInterrupt(&ExecCommandRequest{ID: cmd.ID}))
}

// ExecCommandRequest is posted as an EventInterrupt so a command named by an
// --exec script runs on the main thread, on the same path as key and mouse
// input.
type ExecCommandRequest struct {
	ID string
}

func execScreenshot(a *App, args string) {
	path := stripQuotes(strings.TrimSpace(args))
	if path == "" {
		slog.Error("exec_script: screenshot requires a file path")
		return
	}
	// Trigger a redraw so the screen is up-to-date
	a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	time.Sleep(50 * time.Millisecond)

	if err := a.DumpScreenshot(path); err != nil {
		slog.Error("exec_script: screenshot failed", "path", path, "error", err)
	}
}

func execDebug(a *App, args string) {
	path := stripQuotes(strings.TrimSpace(args))
	if path == "" {
		slog.Error("exec_script: debug requires a file path")
		return
	}
	// Trigger a redraw so state is current
	a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	time.Sleep(50 * time.Millisecond)

	if err := a.DumpDebugState(path); err != nil {
		slog.Error("exec_script: debug dump failed", "path", path, "error", err)
	}
}

func execWait(args string) {
	ms := strings.TrimSpace(args)
	if ms == "" {
		slog.Error("exec_script: wait requires milliseconds")
		return
	}
	n, err := strconv.Atoi(ms)
	if err != nil {
		slog.Error("exec_script: invalid wait duration", "value", ms, "error", err)
		return
	}
	time.Sleep(time.Duration(n) * time.Millisecond)
}

func execPanel(a *App, args string) {
	id := stripQuotes(strings.TrimSpace(args))
	if id == "" {
		slog.Error("exec_script: panel requires a panel ID")
		return
	}
	if !a.BottomPanel.HasPanel(id) {
		slog.Error("exec_script: panel not found", "id", id)
		return
	}
	a.BottomPanel.SetActivePanel(id)
	if !a.ContentSplit.ShowBottom {
		r := a.ContentSplit.GetRect()
		maxH := r.H - 4
		if a.ContentSplit.BottomH <= 1 || a.ContentSplit.BottomH > maxH {
			a.ContentSplit.BottomH = min(r.H/2, maxH)
		}
		a.ContentSplit.ShowBottom = true
	}
	if w := a.BottomPanel.ActiveWidget(); w != nil {
		a.Root.SetFocus(w)
	}
	a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
}

func execQuit(a *App) {
	*a.Running = false
	a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
}

// stripQuotes removes surrounding double quotes from a string if present.
func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// ExecScriptUsage returns the usage text for the --exec flag.
func ExecScriptUsage() string {
	return fmt.Sprintf(`--exec "commands"  Execute semicolon-separated commands after startup

Supported commands:
  click X Y          Simulate left mouse click at coordinates
  rclick X Y         Simulate right mouse click at coordinates
  hover X Y          Simulate mouse hover (move) at coordinates
  drag X1 Y1 X2 Y2   Simulate a mouse drag between two points
  key COMBO          Simulate key press (e.g., key ctrl+p, key enter)
  type TEXT           Type a string of text
  paste TEXT          Simulate bracketed paste (terminal paste)
  copy                Copy selection to clipboard
  exec "Command"     Run a command palette command by title
  screenshot PATH    Save screen text to file
  debug PATH         Save debug state JSON to file
  wait MS            Wait milliseconds
  panel ID           Show and focus a bottom panel by ID
  quit               Exit the editor
  shutdown           Alias for quit, for use over --listen

--listen starts an HTTP command server on 127.0.0.1:4242 that accepts the
same script format over POST /exec, so a running editor can be driven
interactively instead of scripted in advance:
  curl -X POST --data "type hi; screenshot /tmp/s1.txt" http://127.0.0.1:4242/exec

Example:
  %s --exec "wait 200; screenshot /tmp/s1.txt; quit"`, os.Args[0])
}
