package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/gdamore/tcell/v3"
)

func awaitPluginAction(ctx context.Context, lifecycle *execRequestLifecycle) error {
	ctx, cancel := context.WithTimeout(ctx, execRequestTimeout)
	defer cancel()
	select {
	case err := <-lifecycle.done:
		return err
	case <-ctx.Done():
		if lifecycle.cancel() {
			return ctx.Err()
		}
		return <-lifecycle.done
	}
}

func (a *App) runPluginAutomation(ctx context.Context, p *plugin.Plugin, action, args string) (json.RawMessage, error) {
	allowed := func() bool { return p.Enabled && p.State != nil }
	post := func(event tcell.Event) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		lifecycle := newExecRequestLifecycle()
		if err := a.Screen.PostEvent(tcell.NewEventInterrupt(&pluginInput{Events: []tcell.Event{event}, Lifecycle: lifecycle, Allowed: allowed})); err != nil {
			return err
		}
		return awaitPluginAction(ctx, lifecycle)
	}
	var result any = map[string]any{"ok": true}
	var encoded json.RawMessage
	main := func(fn func() error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		lifecycle := newExecRequestLifecycle()
		err := a.Screen.PostEvent(tcell.NewEventInterrupt(&execMainRequest{Lifecycle: lifecycle, Run: func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !allowed() || !a.acceptsPluginInput() {
				return fmt.Errorf("automation unavailable while permissions are pending or plugin is unloaded")
			}
			if err := fn(); err != nil {
				return err
			}
			var err error
			encoded, err = json.Marshal(result)
			return err
		}}))
		if err != nil {
			return err
		}
		return awaitPluginAction(ctx, lifecycle)
	}
	var err error
	switch action {
	case "click", "rclick", "hover", "drag":
		count := 2
		if action == "drag" {
			count = 4
		}
		x, y, parseErr := parseCoordinates(action, args, count)
		if parseErr != nil {
			return nil, parseErr
		}
		button := tcell.Button1
		if action == "rclick" {
			button = tcell.Button2
		}
		if action == "hover" {
			button = tcell.ButtonNone
		}
		err = post(tcell.NewEventMouse(x[0], y[0], button, tcell.ModNone))
		if action == "drag" {
			for i := 1; i <= 10 && err == nil; i++ {
				err = post(tcell.NewEventMouse(x[0]+(x[1]-x[0])*i/10, y[0]+(y[1]-y[0])*i/10, button, tcell.ModNone))
			}
		}
		if err == nil && action != "hover" {
			end := 0
			if action == "drag" {
				end = 1
			}
			err = post(tcell.NewEventMouse(x[end], y[end], tcell.ButtonNone, tcell.ModNone))
		}
	case "key":
		steps, parseErr := config.ParseKeyString(args)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, step := range steps {
			key, mod, ch := comboToTcell(step)
			err = post(tcell.NewEventKey(key, keyEventStr(ch), mod))
			if err != nil {
				break
			}
		}
	case "type":
		for _, event := range pluginTextEvents(args) {
			err = post(event)
			if err != nil {
				break
			}
		}
	case "scroll":
		parts := strings.Fields(args)
		if len(parts) != 3 {
			return nil, fmt.Errorf("scroll requires x y signed-count")
		}
		x, e1 := strconv.Atoi(parts[0])
		y, e2 := strconv.Atoi(parts[1])
		n, e3 := strconv.Atoi(parts[2])
		if e1 != nil || e2 != nil || e3 != nil || n < -1000 || n > 1000 {
			return nil, fmt.Errorf("invalid scroll coordinates or count (maximum 1000)")
		}
		button := tcell.WheelDown
		if n < 0 {
			button = tcell.WheelUp
			n = -n
		}
		for i := 0; i < n; i++ {
			err = post(tcell.NewEventMouse(x, y, button, tcell.ModNone))
			if err != nil {
				break
			}
		}
	case "wait":
		var duration time.Duration
		duration, err = parseExecMilliseconds(args, true)
		if err == nil {
			if duration > 30*time.Second {
				return nil, fmt.Errorf("wait cannot exceed 30000 ms")
			}
			timer := time.NewTimer(duration)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				err = ctx.Err()
			}
		}
	case "wait-for":
		text, timeout, parseErr := parseWaitForArgs(args)
		if parseErr != nil {
			return nil, parseErr
		}
		if timeout > 30*time.Second {
			return nil, fmt.Errorf("wait-for cannot exceed 30000 ms")
		}
		deadline := time.Now().Add(timeout)
		for {
			found := false
			err = main(func() error { found = strings.Contains(a.Screenshot(), text); return nil })
			if err != nil || found {
				break
			}
			if time.Now().After(deadline) {
				err = fmt.Errorf("timed out waiting for %q", args)
				break
			}
			timer := time.NewTimer(25 * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			}
		}
	default:
		err = main(func() error {
			switch action {
			case "session":
				result = map[string]any{"pid": os.Getpid(), "workspaces": a.Workspace.Paths(), "active_file": a.EditorGroup.ActiveFilePath(), "version": a.Version}
			case "screenshot":
				result = a.Screenshot()
			case "debug":
				result = a.BuildDebugState()
			case "buffer":
				result = map[string]any{"path": p.Editor.FilePath(), "text": p.Editor.BufferText(), "cursor": func() any { l, c := p.Editor.CursorPos(); return map[string]int{"line": l + 1, "col": c + 1} }()}
			case "commands":
				commands := make([]map[string]string, 0)
				for _, cmd := range p.ListCommands() {
					commands = append(commands, map[string]string{"id": cmd.ID, "title": cmd.Title})
				}
				result = commands
			case "command", "exec":
				id := args
				if action == "exec" {
					cmd, ok := a.Reg.FindByTitle(args)
					if !ok {
						return fmt.Errorf("unknown command title %q", args)
					}
					id = cmd.ID
				}
				if !a.Reg.Execute(id) {
					return fmt.Errorf("unknown command %q", id)
				}
				a.FlushEditorOnChange()
			case "paste":
				a.PasteText(args)
				a.FlushEditorOnChange()
			case "copy":
				a.Copy()
			case "clipboard":
				result = p.ClipboardRead()
			case "open":
				var spec struct {
					Path     string `json:"path"`
					Line     int    `json:"line"`
					Readonly bool   `json:"readonly"`
				}
				if err := json.Unmarshal([]byte(args), &spec); err != nil {
					return err
				}
				if spec.Path == "" {
					return fmt.Errorf("open requires path")
				}
				p.OpenFile(spec.Path, spec.Line, spec.Readonly)
			case "panel":
				if !a.focusBottomPanel(args) {
					return fmt.Errorf("unknown panel %q", args)
				}
			default:
				return fmt.Errorf("unknown automation action %q", action)
			}
			return nil
		})
	}
	if err != nil {
		return nil, err
	}
	if encoded != nil {
		return encoded, nil
	}
	return json.Marshal(result)
}
