package app

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type debugRun struct {
	ctx     context.Context
	cancel  context.CancelFunc
	session *dap.Session
	thread  int
}

func registerDebuggerCommands(a *App) {
	for _, cmd := range []command.Command{
		{ID: "debug.start", Title: i18n.Text("debug.start"), Handler: a.StartDebugging},
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
		items := make([]widgets.SelectItem, 0, len(targets))
		for i, t := range targets {
			items = append(items, widgets.SelectItem{ID: strconv.Itoa(i), Label: t.Name + " (" + t.Folder + ")"})
		}
		a.ShowSelectDialog(i18n.Text("debug.start"), items, func(id string) {
			i, err := strconv.Atoi(id)
			if err == nil && i >= 0 && i < len(targets) {
				a.prepareDebugTarget(targets[i])
			}
		}, nil)
	}
}
func (a *App) prepareDebugTarget(t dap.Target) {
	args, err := t.LaunchArguments()
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
	a.startDebugRun(t, args)
}
func (a *App) startDebugRun(t dap.Target, args map[string]any) {
	ctx, cancel := context.WithCancel(context.Background())
	run := &debugRun{ctx: ctx, cancel: cancel}
	a.debugger.run = run
	for _, g := range a.editorGroups() {
		a.wireDebuggerGutter(g)
	}
	points := make(map[string][]int)
	for path, lines := range a.debugger.points {
		points[path] = slices.Sorted(maps.Keys(lines))
	}
	post := a.tasks.post
	deliver := func(f func()) {
		_ = post(func() {
			if a.debugger.run == run {
				f()
			}
		})
	}
	go func() {
		startup, endStartup := context.WithTimeout(ctx, 90*time.Second)
		defer endStartup()
		s, err := dap.StartDelve(startup, "dlv", t.Folder, func(event dap.Event) { deliver(func() { a.onDebugEvent(run, t.Name, event) }) })
		if err == nil {
			deliver(func() { run.session = s })
			err = s.Launch(startup, args, func() error {
				for path, lines := range points {
					bps, err := s.SetBreakpoints(startup, path, lines)
					if err != nil {
						return err
					}
					deliver(func() {
						for i, bp := range bps {
							if i < len(lines) {
								if _, ok := a.debugger.points[path][lines[i]]; ok {
									a.debugger.points[path][lines[i]] = bp.Verified
								}
							}
						}
					})
				}
				return nil
			})
			if err == nil {
				select {
				case <-ctx.Done():
				case <-s.Done():
				}
			}
			s.Close()
		}
		deliver(func() {
			if err != nil && ctx.Err() == nil {
				a.StatusError(t.Name + ": " + err.Error())
			}
			cancel()
			a.debugger.run = nil
			a.debugger.stoppedFile = ""
			for _, lines := range a.debugger.points {
				for line := range lines {
					lines[line] = false
				}
			}
		})
	}()
}
func (a *App) onDebugEvent(run *debugRun, name string, event dap.Event) {
	switch event.Name {
	case "stopped":
		var b struct{ ThreadID int }
		if json.Unmarshal(event.Body, &b) == nil {
			run.thread = b.ThreadID
		}
	case "continued":
		run.thread = 0
		a.debugger.stoppedFile = ""
	case "output":
		var b struct{ Output string }
		if json.Unmarshal(event.Body, &b) == nil {
			for _, line := range strings.Split(strings.TrimSuffix(b.Output, "\n"), "\n") {
				a.LogOutput("info", "debug:"+name, line)
			}
		}
	case "terminated":
		run.cancel()
	}
}
func (a *App) StopDebugging() {
	if r := a.debugger.run; r != nil {
		r.cancel()
	}
}
func (a *App) DebugContinue() { a.debugControl("continue") }
func (a *App) DebugStepOver() { a.debugControl("next") }
func (a *App) DebugStepIn()   { a.debugControl("stepIn") }
func (a *App) DebugStepOut()  { a.debugControl("stepOut") }
func (a *App) debugControl(cmd string) {
	r := a.debugger.run
	if r == nil || r.session == nil || r.thread == 0 {
		return
	}
	tid := r.thread
	post := a.tasks.post
	go func() {
		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
		defer cancel()
		if err := r.session.Call(ctx, cmd, map[string]int{"threadId": tid}, nil); err != nil {
			_ = post(func() {
				if a.debugger.run == r {
					a.StatusError(err.Error())
				}
			})
		}
	}()
}
