package app

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/dap/providers"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type debugRun struct {
	terminalIDs       []string
	languages         []string
	failed            bool
	stopGeneration    uint64
	allThreadsStopped bool
	edited            map[string]bool
	folder            string
	output            map[string]string
	attached          bool
	ctx               context.Context
	cancel            context.CancelFunc
	session           *dap.Session
	thread            *int
	lastThread        int
	closed            chan struct{}
	ready             bool
	sending           map[string]bool
	revision          map[string]int
}

func registerDebuggerCommands(a *App) {
	for _, cmd := range []command.Command{
		{ID: "debug.pause", Title: i18n.Text("debug.pause"), Handler: a.DebugPause},
		{ID: "debug.clearBreakpoints", Title: i18n.Text("debug.clearBreakpoints"), Handler: a.ClearDebugBreakpoints},
		{ID: "debug.toggleBreakpoint", Title: i18n.Text("debug.toggleBreakpoint"), Handler: a.ToggleDebugBreakpoint},
		{ID: "debug.file", Title: i18n.Text("debug.file"), Handler: a.debugFile},
		{ID: "debug.configure", Title: i18n.Text("debug.createConfiguration"), Handler: a.configureDebugger},
		{ID: "debug.start", Title: i18n.Text("debug.start"), Handler: a.StartDebugging},
		{ID: "debug.detach", Title: i18n.Text("debug.detach"), Handler: a.DetachDebugger},
		{ID: "debug.stop", Title: i18n.Text("debug.stop"), Handler: a.StopDebugging},
		{ID: "debug.continue", Title: i18n.Text("debug.continue"), Handler: a.DebugContinue},
		{ID: "debug.stepOver", Title: i18n.Text("debug.stepOver"), Handler: a.DebugStepOver},
		{ID: "debug.stepIn", Title: i18n.Text("debug.stepIn"), Handler: a.DebugStepIn},
		{ID: "debug.stepOut", Title: i18n.Text("debug.stepOut"), Handler: a.DebugStepOut},
	} {
		a.Reg.Register(cmd)
	}
}
func (a *App) StartDebugging() {
	if a.debugger.run != nil || a.Root.HasModalOverlay() {
		return
	}
	for _, g := range a.editorGroups() {
		for _, folder := range a.Workspace.Folders {
			for _, dir := range []string{".ttt", ".vscode"} {
				path := filepath.Join(folder.Path, dir, "launch.json")
				if g.IsDirtyPath(path) {
					a.activateEditorGroup(g)
					g.SwitchToTabByPath(path)
					a.saveFile(a.StartDebugging)
					return
				}
			}
		}
	}
	if a.debugger.picker != nil {
		a.debugger.picker.Stop()
	}
	var targets []dap.Target
	for _, folder := range a.Workspace.Folders {
		list, err := dap.LoadTargets(folder.Path)
		if err != nil {
			a.StatusError(err.Error())
			return
		}
		targets = append(targets, list...)
	}
	switch len(targets) {
	case 0:
		a.StatusError(i18n.Text("debug.noTargets"))
	case 1:
		a.prepareDebugTarget(targets[0])
	default:
		selected := ""
		items := make([]widgets.SelectItem, 0, len(targets))
		for i, t := range targets {
			if t.Folder == a.debugger.lastFolder && t.Name == a.debugger.lastTarget {
				selected = strconv.Itoa(i)
			}
			items = append(items, widgets.SelectItem{ID: strconv.Itoa(i), Label: t.Name + " (" + t.Folder + ")"})
		}
		a.showSelectDialog(i18n.Text("debug.start"), items, selected, func(id string) {
			i, err := strconv.Atoi(id)
			if err == nil && i >= 0 && i < len(targets) {
				a.prepareDebugTarget(targets[i])
			}
		}, nil)
	}
}
func (a *App) pickDebugProcess(t dap.Target) {
	if a.debugger.picker == nil {
		a.debugger.picker = NewTask[[]dap.Process](a, asynctask.TaskOptions{})
	}
	a.debugger.picker.Schedule(func(ctx context.Context) ([]dap.Process, error) {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		return dap.Processes(ctx)
	}, func(processes []dap.Process, err error) {
		if a.debugger.run != nil || a.Root.HasModalOverlay() {
			return
		}
		if err != nil {
			a.StatusError(err.Error())
			return
		}
		items := make([]widgets.SelectItem, 0, len(processes))
		for _, p := range processes {
			id := strconv.Itoa(p.PID)
			items = append(items, widgets.SelectItem{ID: id, Label: id + "  " + p.Name})
		}
		a.ShowSelectDialog(i18n.Text("debug.pickProcess"), items, func(id string) {
			t.Arguments = maps.Clone(t.Arguments)
			t.Arguments["processId"] = id
			a.prepareDebugTarget(t)
		}, nil)
	})
}
func (a *App) prepareDebugTarget(t dap.Target) {
	provider, err := providers.For(t, a.Settings.Debug.Adapters)
	if err != nil {
		var missing providers.MissingAdapter
		if errors.As(err, &missing) {
			a.StatusError(i18n.Format("debug.missingAdapter", i18n.Args{"type": missing.Type}))
		} else {
			a.StatusError(err.Error())
		}
		return
	}
	if hook := a.Settings.Debug.Adapters[t.Type].Provider; hook != "" && hook != "generic" && t.Request == "attach" && t.Arguments["processId"] == dap.PickProcess {
		a.pickDebugProcess(t)
		return
	}
	args, err := provider.Arguments(t)
	if err != nil {
		a.StatusError(err.Error())
		return
	}
	for _, g := range a.editorGroups() {
		for _, path := range g.OpenFilePaths() {
			owner := a.Workspace.FolderForFile(path)
			if owner != nil && owner.Path == t.Folder && g.IsDirtyPath(path) {
				a.activateEditorGroup(g)
				g.SwitchToTabByPath(path)
				a.saveFile(func() { a.prepareDebugTarget(t) })
				return
			}
		}
	}
	if a.debugger.run != nil {
		return
	}
	a.startDebugRun(provider, t, args)
}
func (a *App) startDebugRun(provider dap.Provider, t dap.Target, args map[string]any) {
	ctx, cancel := context.WithCancel(context.Background())
	run := &debugRun{languages: a.Settings.Debug.Adapters[t.Type].Languages, edited: make(map[string]bool), folder: t.Folder, attached: t.Request == "attach", ctx: ctx, cancel: cancel, closed: make(chan struct{}), sending: make(map[string]bool), revision: make(map[string]int)}
	a.debugger.lastFolder, a.debugger.lastTarget = t.Folder, t.Name
	for path := range a.debugger.points {
		if a.debugOwnsSource(run, path) {
			a.checkDebugSource(path)
		}
	}
	a.debugger.run = run
	a.refreshDebugPanel()
	a.Sidebar.SetActivePanel("debug")
	a.ShowSidebar()
	points := make(map[string][]int)
	for path := range a.debugger.points {
		if a.debugOwnsSource(run, path) {
			points[path] = a.debugLines(run, path)
			for _, line := range points[path] {
				a.debugger.points[path][line].runtimeLine = line
			}
		}
	}
	post := a.tasks.post
	// PostEvent can deliver concurrent posts out of order when the terminal queue fills.
	var queueMu sync.Mutex
	var pending []func()
	deliver := func(f func()) {
		queueMu.Lock()
		schedule := len(pending) == 0
		pending = append(pending, f)
		queueMu.Unlock()
		if schedule {
			_ = post(func() {
				queueMu.Lock()
				batch := pending
				pending = nil
				queueMu.Unlock()
				for _, apply := range batch {
					if a.debugger.run == run {
						apply()
					}
				}
			})
		}
	}

	options := dap.SessionOptions{AdapterType: t.Type}
	options.OnChildFailure = func(err error) {
		deliver(func() {
			if run.ctx.Err() == nil {
				run.failed = true
				a.StatusError(t.Name + ": " + err.Error())
				run.cancel()
			}
		})
	}
	options.RunInTerminal = func(req dap.TerminalRequest) (int, error) {
		type result struct {
			pid int
			err error
		}
		done := make(chan result, 1)
		deliver(func() {
			if ctx.Err() != nil {
				return
			}
			var env []string
			for k, v := range req.Env {
				if v == nil {
					env = append(env, k)
				} else {
					env = append(env, k+"="+*v)
				}
			}
			cwd := req.Cwd
			if cwd == "" {
				cwd = t.Folder
			}
			id, err := a.SpawnTerminalSpec(TerminalSpec{Shell: req.Args[0], Args: req.Args[1:], Cwd: cwd, Env: env, Label: req.Title})
			var pid int
			if err == nil {
				pid = a.terminalByID(id).Term.PID()
				if !run.attached {
					run.terminalIDs = append(run.terminalIDs, id)
				}
			}
			done <- result{pid, err}
		})
		select {
		case r := <-done:
			return r.pid, r.err
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	go func() {
		defer close(run.closed)
		startup, endStartup := context.WithTimeout(ctx, 90*time.Second)
		defer endStartup()
		s, err := provider.Open(startup, t, args, func(event dap.Event) { deliver(func() { a.onDebugEvent(run, t.Name, event) }) }, options)
		if err == nil {
			deliver(func() { run.session = s })
			err = s.Start(startup, t.Type, t.Request, args, func() error {
				for path, lines := range points {
					bps, err := s.SetBreakpoints(startup, path, lines)
					if err != nil {
						if startup.Err() != nil {
							return startup.Err()
						}
						bps = make([]dap.Breakpoint, len(lines))
						for i := range bps {
							bps[i].Message = err.Error()
						}
					}
					deliver(func() {
						if run.revision[path] == 0 {
							a.applyDebugBindings(path, lines, bps)
						}
					})
				}
				return nil
			})
			if err == nil {
				deliver(func() {
					run.ready = true
					a.refreshDebugPanel()
					for path := range run.revision {
						a.syncDebugBreakpoints(path)
					}
				})
				select {
				case <-ctx.Done():
				case <-s.Done():
					deliver(func() {
						if run.ctx.Err() == nil {
							run.failed = true
							a.StatusError(t.Name + ": " + i18n.Text("debug.adapterClosed"))
							run.cancel()
						}
					})
				}
			}
			s.Close(t.Request == "launch")
		}
		deliver(func() {
			if err != nil && ctx.Err() == nil {
				run.failed = true
				a.StatusError(t.Name + ": " + err.Error())
			}
			a.flushDebugOutput(run, t.Name)
			for _, id := range run.terminalIDs {
				if tt := a.terminalByID(id); tt != nil && !tt.Term.Exited() {
					a.CloseTerminal(id)
				}
			}
			cancel()
			a.debugger.run = nil
			if !run.failed {
				a.StatusNotify(i18n.Text("debug.finished"))
			}
			for _, lines := range a.debugger.points {
				for _, p := range lines {
					p.runtimeLine = 0
					p.binding = dap.Breakpoint{}
				}
			}
			a.clearDebugInspection()
		})
	}()
}
func (a *App) onDebugEvent(run *debugRun, name string, event dap.Event) {
	switch event.Name {
	case "breakpoint":
		var b struct {
			Reason     string
			Breakpoint dap.Breakpoint
		}
		if json.Unmarshal(event.Body, &b) == nil {
			a.debugBindingEvent(run, b.Breakpoint, b.Reason)
		}
	case "stopped":
		var b struct {
			ThreadID          *int
			AllThreadsStopped bool
		}
		if json.Unmarshal(event.Body, &b) == nil {
			run.stopGeneration++
			run.allThreadsStopped = b.AllThreadsStopped
			run.thread = b.ThreadID
			if run.thread != nil {
				run.lastThread = *run.thread
			}
			a.debugStopped(run)
		}
	case "exited":
		var b struct{ ExitCode *int }
		if json.Unmarshal(event.Body, &b) == nil && b.ExitCode != nil {
			a.LogOutput("info", "debug:"+name, i18n.Format("debug.exitCode", i18n.Args{"code": strconv.Itoa(*b.ExitCode)}))
		}
	case "continued":
		run.stopGeneration++
		run.thread = nil
		a.clearDebugInspection()
	case "output":
		var b struct{ Category, Output string }
		if json.Unmarshal(event.Body, &b) == nil {
			a.appendDebugOutput(run, name, b.Category, b.Output)
		}
	case "terminated":
		run.cancel()
	}
}
func (a *App) appendDebugOutput(run *debugRun, name, category, chunk string) {
	if category == "telemetry" {
		return
	}
	if run.output == nil {
		run.output = make(map[string]string)
	}
	chunk = run.output[category] + chunk
	for {
		line, rest, ok := strings.Cut(chunk, "\n")
		if !ok {
			break
		}
		a.LogOutput("info", "debug:"+name, strings.TrimSuffix(line, "\r"))
		chunk = rest
	}
	if len(chunk) > 65536 || category != "stdout" && category != "stderr" {
		if chunk != "" {
			a.LogOutput("info", "debug:"+name, chunk)
		}
		chunk = ""
	}
	run.output[category] = chunk
}
func (a *App) flushDebugOutput(run *debugRun, name string) {
	for _, chunk := range run.output {
		if chunk != "" {
			a.LogOutput("info", "debug:"+name, chunk)
		}
	}
	clear(run.output)
}
func (a *App) StopDebugging() {
	if r := a.debugger.run; r != nil {
		r.cancel()
	}
}
func (a *App) DetachDebugger() {
	if r := a.debugger.run; r != nil && r.attached {
		r.cancel()
	}
}
func (a *App) DebugPause()    { a.debugControl("pause") }
func (a *App) DebugContinue() { a.debugControl("continue") }
func (a *App) DebugStepOver() { a.debugControl("next") }
func (a *App) DebugStepIn()   { a.debugControl("stepIn") }
func (a *App) DebugStepOut()  { a.debugControl("stepOut") }
func (a *App) debugControl(cmd string) {
	r := a.debugger.run
	if r == nil || r.session == nil || !r.ready || (cmd == "pause") != (r.thread == nil) {
		return
	}
	generation := r.stopGeneration
	tid := r.lastThread
	if r.thread != nil {
		tid = *r.thread
	}
	post := a.tasks.post
	go func() {
		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
		defer cancel()
		var err error
		if cmd == "pause" {
			var threads struct{ Threads []debugThread }
			if err = r.session.Call(ctx, "threads", struct{}{}, &threads); err == nil && len(threads.Threads) > 0 {
				tid = threads.Threads[0].ID
			}
		}
		if err == nil {
			err = r.session.Call(ctx, cmd, map[string]int{"threadId": tid}, nil)
		}
		_ = post(func() {
			if a.debugger.run != r || r.ctx.Err() != nil || r.stopGeneration != generation {
				return
			}
			if err != nil {
				a.StatusError(err.Error())
				return
			}
			if cmd != "pause" {
				r.thread = nil
				r.stopGeneration++
				a.clearDebugInspection()
			}
		})
	}()
}
