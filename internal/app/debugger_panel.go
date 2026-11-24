package app

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/eugenioenko/ttt/internal/i18n"
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
type debugSidebar struct {
	tree       *widgets.TreeWidget
	frames     []debugFrame
	scopes     []*widgets.TreeNode
	references map[*widgets.TreeNode]int
	frameNodes map[*widgets.TreeNode]debugFrame
	breakNodes map[*widgets.TreeNode]debugLocation
	epoch      uint64
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
	for _, f := range p.frames {
		node := &widgets.TreeNode{ID: fmt.Sprintf("frame:%d", f.ID), Label: f.Name + "  " + filepath.Base(f.Source.Path) + ":" + strconv.Itoa(f.Line)}
		stack.Children = append(stack.Children, node)
		p.frameNodes[node] = f
	}
	points := &widgets.TreeNode{ID: "points", Label: i18n.Text("debug.breakpoints"), Expanded: true}
	p.breakNodes = make(map[*widgets.TreeNode]debugLocation)
	for _, path := range slices.Sorted(maps.Keys(a.debugger.points)) {
		for _, line := range slices.Sorted(maps.Keys(a.debugger.points[path])) {
			n := &widgets.TreeNode{ID: fmt.Sprintf("%s:%d", path, line), Label: fmt.Sprintf("%s:%d", filepath.Base(path), line)}
			points.Children = append(points.Children, n)
			p.breakNodes[n] = debugLocation{path, line}
		}
	}
	vars := &widgets.TreeNode{ID: "vars", Label: i18n.Text("debug.variables"), Expanded: true, Children: p.scopes}
	p.tree.SetItems([]*widgets.TreeNode{vars, stack, points})
}
func (a *App) clearDebugInspection() {
	p := &a.debugger.panel
	p.epoch++
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
	if r.thread == nil {
		debugQuery(a, r, "threads", struct{}{}, func(result struct{ Threads []struct{ ID int } }) {
			if len(result.Threads) > 0 {
				r.thread = &result.Threads[0].ID
				a.debugStopped(r)
			}
		})
		return
	}
	debugQuery(a, r, "stackTrace", map[string]int{"threadId": *r.thread, "levels": 50}, func(result struct{ StackFrames []debugFrame }) {
		p := &a.debugger.panel
		p.frames = result.StackFrames
		if len(p.frames) > 0 {
			f := p.frames[0]
			a.debugger.stoppedFile = f.Source.Path
			a.debugger.stoppedLine = f.Line
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
