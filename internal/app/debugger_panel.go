package app

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type debugFrame struct {
	ID, Line int
	Name     string
	Source   struct{ Path string }
}
type debugVariable struct {
	Name, Value        string
	VariablesReference int
}
type debugThread struct {
	ID   int
	Name string
}
type debugSidebar struct {
	threads     []debugThread
	threadNodes map[*widgets.TreeNode]int
	tree        *widgets.TreeWidget
	frames      []debugFrame
	scopes      []*widgets.TreeNode
	references  map[*widgets.TreeNode]int
	frameNodes  map[*widgets.TreeNode]debugFrame
	breakNodes  map[*widgets.TreeNode]debugLocation
	epoch       uint64
}
type debugLocation struct {
	path string
	line int
}

func (a *App) ensureDebugPanel() {
	p := &a.debugger.panel
	if p.tree != nil {
		return
	}
	p.tree = widgets.NewTreeWidget(widgets.TreeConfig{Indent: 2, OnExpand: a.expandDebugNode, OnCommand: func(cmd string, n *widgets.TreeNode) {
		if cmd != "activate" {
			return
		}
		if n != nil && strings.HasPrefix(n.ID, "debug.") {
			a.Reg.Execute(n.ID)
			return
		}
		if f, ok := p.frameNodes[n]; ok {
			a.inspectDebugFrame(f)
		}
		if b, ok := p.breakNodes[n]; ok {
			a.navigateToLocation(b.path, b.line-1, 0)
		}
	}})
	a.Sidebar.AddPanel("debug", i18n.Text("debug.panel"), ui.NewWidgetAdapter(p.tree))
	a.refreshDebugPanel()
}
func (a *App) refreshDebugPanel() {
	p := &a.debugger.panel
	if p.tree == nil {
		return
	}
	stack := &widgets.TreeNode{ID: "stack", Label: i18n.Text("debug.stack"), Expanded: true}
	p.frameNodes = make(map[*widgets.TreeNode]debugFrame)
	p.threadNodes = make(map[*widgets.TreeNode]int)
	frameParent := stack
	if r := a.debugger.run; r != nil && r.thread != nil {
		for _, t := range p.threads {
			selected := t.ID == *r.thread
			n := &widgets.TreeNode{ID: fmt.Sprintf("thread:%d", t.ID), Label: t.Name, Expandable: true, Expanded: selected}
			p.threadNodes[n] = t.ID
			if selected {
				frameParent = n
				stack.Children = append([]*widgets.TreeNode{n}, stack.Children...)
			} else {
				stack.Children = append(stack.Children, n)
			}
		}
	}
	for _, f := range p.frames {
		node := &widgets.TreeNode{ID: fmt.Sprintf("frame:%d", f.ID), Label: f.Name + "  " + filepath.Base(f.Source.Path) + ":" + strconv.Itoa(f.Line)}
		frameParent.Children = append(frameParent.Children, node)
		p.frameNodes[node] = f
	}
	points := &widgets.TreeNode{ID: "points", Label: i18n.Text("debug.breakpoints"), Expanded: true}
	p.breakNodes = make(map[*widgets.TreeNode]debugLocation)
	for _, path := range slices.Sorted(maps.Keys(a.debugger.points)) {
		for _, line := range slices.Sorted(maps.Keys(a.debugger.points[path])) {
			b := a.debugger.points[path][line].binding
			marker := icons.PendingBreakpoint
			if b.Verified {
				marker = icons.Breakpoint
			}
			label := fmt.Sprintf("%c %s:%d", icons.Rune(marker), filepath.Base(path), line)
			if b.Line > 0 && (b.Line != line || b.Source.Path != "" && b.Source.Path != path) {
				dest := path
				if b.Source.Path != "" {
					dest = b.Source.Path
				}
				label += fmt.Sprintf(" %c %s:%d", icons.Rune(icons.ArrowRight), filepath.Base(dest), b.Line)
			}
			if a.debugger.points[path][line].confirm {
				label += "  " + i18n.Text("debug.confirmLine")
			} else if r := a.debugger.run; r != nil && r.edited[path] {
				label += "  " + i18n.Text("debug.editedSource")
			} else if b.Message != "" {
				label += "  " + b.Message
			}
			n := &widgets.TreeNode{ID: fmt.Sprintf("%s:%d", path, line), Label: label}
			points.Children = append(points.Children, n)
			p.breakNodes[n] = debugLocation{path, line}
		}
	}
	vars := &widgets.TreeNode{ID: "vars", Label: i18n.Text("debug.variables"), Expanded: true, Children: p.scopes}
	nodes := []*widgets.TreeNode{vars, stack, points}
	ids := []string{"debug.start"}
	if r := a.debugger.run; r != nil {
		ids = nil
		if r.ready {
			if r.thread == nil {
				ids = append(ids, "debug.pause")
			} else {
				ids = append(ids, "debug.continue", "debug.stepOver", "debug.stepIn", "debug.stepOut")
			}
		}
		if r.attached {
			ids = append(ids, "debug.detach")
		} else {
			ids = append(ids, "debug.stop")
		}
	}
	for _, id := range ids {
		if cmd, ok := a.Reg.Get(id); ok {
			nodes = append(nodes, &widgets.TreeNode{ID: id, Label: cmd.Title})
		}
	}
	p.tree.SetItems(nodes)
}
func (a *App) clearDebugInspection() {
	p := &a.debugger.panel
	p.epoch++
	p.threads = nil
	p.frames = nil
	p.scopes = nil
	p.references = nil
	a.debugger.stoppedFile = ""
	a.refreshDebugPanel()
}

// DAP frame and variable references expire as soon as execution resumes.
func debugQuery[T any](a *App, r *debugRun, method string, args any, apply func(T)) {
	epoch := a.debugger.panel.epoch
	post := a.tasks.post
	go func() {
		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
		defer cancel()
		var result T
		err := r.session.Call(ctx, method, args, &result)
		_ = post(func() {
			if a.debugger.run != r || a.debugger.panel.epoch != epoch || r.ctx.Err() != nil {
				return
			}
			if err != nil {
				a.StatusError(err.Error())
				return
			}
			apply(result)
		})
	}()
}
func (a *App) debugStopped(r *debugRun) {
	a.clearDebugInspection()
	debugQuery(a, r, "threads", struct{}{}, func(result struct{ Threads []debugThread }) {
		if r.thread == nil && len(result.Threads) > 0 {
			r.thread = &result.Threads[0].ID
		}
		if r.thread == nil {
			return
		}
		for _, t := range result.Threads {
			if t.ID == *r.thread || r.allThreadsStopped {
				a.debugger.panel.threads = append(a.debugger.panel.threads, t)
			}
		}
		a.loadDebugStack(r, *r.thread, true)
	})
}
func (a *App) loadDebugStack(r *debugRun, tid int, executing bool) {
	p := &a.debugger.panel
	p.epoch++
	r.thread = &tid
	r.lastThread = tid
	p.frames = nil
	p.scopes = nil
	p.references = nil
	a.refreshDebugPanel()
	debugQuery(a, r, "stackTrace", map[string]int{"threadId": *r.thread, "levels": 50}, func(result struct{ StackFrames []debugFrame }) {
		p := &a.debugger.panel
		p.frames = result.StackFrames
		if len(p.frames) > 0 {
			f := p.frames[0]
			if executing {
				a.debugger.stoppedFile = f.Source.Path
				a.debugger.stoppedLine = f.Line
			}
			a.inspectDebugFrame(f)
		}
		a.refreshDebugPanel()
	})
}
func (a *App) inspectDebugFrame(f debugFrame) {
	r := a.debugger.run
	if r == nil {
		return
	}
	p := &a.debugger.panel
	p.epoch++
	p.scopes = nil
	p.references = make(map[*widgets.TreeNode]int)
	if f.Source.Path != "" && f.Line > 0 {
		a.navigateToLocation(f.Source.Path, f.Line-1, 0)
	}
	a.refreshDebugPanel()
	debugQuery(a, r, "scopes", map[string]int{"frameId": f.ID}, func(result struct {
		Scopes []struct {
			Name               string
			VariablesReference int
			Expensive          bool
		}
	}) {
		for _, scope := range result.Scopes {
			n := &widgets.TreeNode{ID: fmt.Sprintf("scope:%d", scope.VariablesReference), Label: scope.Name, Expandable: scope.VariablesReference != 0, Expanded: !scope.Expensive}
			p.scopes = append(p.scopes, n)
			p.references[n] = scope.VariablesReference
			if n.Expanded {
				a.expandDebugNode(n)
			}
		}
		a.refreshDebugPanel()
	})
}
func (a *App) expandDebugNode(n *widgets.TreeNode) {
	p := &a.debugger.panel
	if tid, ok := p.threadNodes[n]; ok {
		if r := a.debugger.run; r != nil && r.thread != nil && *r.thread != tid {
			a.loadDebugStack(r, tid, false)
		}
		return
	}
	ref := p.references[n]
	if ref == 0 || len(n.Children) > 0 || a.debugger.run == nil {
		return
	}
	debugQuery(a, a.debugger.run, "variables", map[string]int{"variablesReference": ref}, func(result struct{ Variables []debugVariable }) {
		n.Children = nil
		for i, v := range result.Variables {
			child := &widgets.TreeNode{ID: fmt.Sprintf("%s/%d", n.ID, i), Label: v.Name + " = " + v.Value, Expandable: v.VariablesReference != 0}
			n.Children = append(n.Children, child)
			p.references[child] = v.VariablesReference
		}
		p.tree.SetItems(p.tree.Config.Items)
	})
}
