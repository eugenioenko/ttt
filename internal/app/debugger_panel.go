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
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
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
	threads                  []debugThread
	threadNodes              map[*widgets.TreeNode]int
	variables, stack, points *widgets.TreeWidget
	primary, secondary       *widgets.HStackWidget
	adapter                  *ui.WidgetAdapter
	controls                 debugControlState
	frames                   []debugFrame
	scopes                   []*widgets.TreeNode
	references               map[*widgets.TreeNode]int
	frameNodes               map[*widgets.TreeNode]debugFrame
	breakNodes               map[*widgets.TreeNode]debugLocation
	epoch                    uint64
}
type debugLocation struct {
	path string
	line int
}

func (a *App) ensureDebugPanel() {
	p := &a.debugger.panel
	if p.adapter != nil {
		return
	}
	config := widgets.TreeConfig{Indent: 2, OnExpand: a.expandDebugNode, OnCommand: func(cmd string, n *widgets.TreeNode) {
		if cmd != "activate" {
			return
		}
		if f, ok := p.frameNodes[n]; ok {
			a.inspectDebugFrame(f)
		}
		if b, ok := p.breakNodes[n]; ok {
			a.revealDebugLocation(b.path, b.line)
		}
	}}
	section := func(id, label string) *widgets.TreeWidget {
		tree := widgets.NewTreeWidget(config)
		tree.SetItems([]*widgets.TreeNode{{ID: id, Label: label, Expanded: true, LabelStyle: term.StyleHoverBold}})
		return tree
	}
	p.variables = section("vars", i18n.Text("debug.variables"))
	p.stack = section("stack", i18n.Text("debug.stack"))
	p.points = section("points", i18n.Text("debug.breakpoints"))
	p.primary = widgets.NewHStackWidget()
	p.secondary = widgets.NewHStackWidget()
	for _, row := range []*widgets.HStackWidget{p.primary, p.secondary} {
		row.FixedHeight = 1
		row.Gap = 1
	}
	inspection := debugSectionSplit(p.variables, p.stack, 0.5, 2)
	sections := debugSectionSplit(inspection, p.points, 0.25, 5)
	a.refreshDebugControls()
	p.adapter = ui.NewWidgetAdapter(widgets.NewVStackWidget(p.primary, p.secondary, widgets.NewDividerWidget(widgets.DividerConfig{}), sections))
	a.Sidebar.AddPanel("debug", i18n.Text("debug.panel"), p.adapter)
	a.refreshDebugPanel()
}

func debugSectionSplit(top, bottom ui.Widget, ratio float64, minTop int) *ui.ContentSplitWidget {
	split := ui.NewContentSplitWidget()
	split.Top, split.Bottom = top, bottom
	split.ShowBottom = true
	split.BottomH, split.BottomRatio = 0, ratio
	split.MinTopH, split.MinBottomH = minTop, 2
	split.OnResize = func(h int) { split.BottomH = h }
	return split
}

type debugControlState struct{ active, paused, ready, attached bool }

func (a *App) refreshDebugControls() {
	p := &a.debugger.panel
	state := debugControlState{}
	if r := a.debugger.run; r != nil {
		state = debugControlState{active: true, paused: r.thread != nil, ready: r.ready, attached: r.attached}
	}
	if p.controls == state && len(p.primary.Children) > 0 {
		return
	}
	p.controls = state
	p.primary.Children = nil
	p.secondary.Children = nil
	button := func(row *widgets.HStackWidget, id, label string, enabled bool) {
		b := widgets.NewButtonWidget(widgets.ButtonConfig{Label: label, Command: id, Variant: widgets.ButtonBracket, Box: &widgets.BoxModel{}, OnCommand: func(cmd string) { a.Reg.Execute(cmd) }})
		b.Disabled = !enabled
		row.Children = append(row.Children, b)
	}
	if !state.active {
		button(p.primary, "debug.start", i18n.Text("debug.start"), true)
	} else {
		if state.paused {
			button(p.primary, "debug.continue", i18n.Text("debug.continueButton"), state.ready)
		} else {
			button(p.primary, "debug.pause", i18n.Text("debug.pauseButton"), state.ready)
		}
		if state.attached {
			button(p.primary, "debug.detach", i18n.Text("debug.detachButton"), true)
		} else {
			button(p.primary, "debug.stop", i18n.Text("debug.stopButton"), true)
		}
		button(p.primary, "debug.stepOver", i18n.Text("debug.stepOverButton"), state.ready && state.paused)
		button(p.secondary, "debug.stepIn", i18n.Text("debug.stepInButton"), state.ready && state.paused)
		button(p.secondary, "debug.stepOut", i18n.Text("debug.stepOutButton"), state.ready && state.paused)
	}
	if p.adapter != nil {
		p.adapter.RebuildFocus()
	}
}

func (a *App) refreshDebugPanel() {
	p := &a.debugger.panel
	if p.adapter == nil {
		return
	}
	a.refreshDebugControls()
	stack := p.stack.Config.Items[0]
	stack.Children = nil
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
	points := p.points.Config.Items[0]
	points.Children = nil
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
	p.variables.Config.Items[0].Children = p.scopes
	for _, tree := range []*widgets.TreeWidget{p.variables, p.stack, p.points} {
		tree.SetItems(tree.Config.Items)
	}
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
		a.revealDebugLocation(f.Source.Path, f.Line)
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
		p.variables.SetItems(p.variables.Config.Items)
	})
}

func (a *App) revealDebugLocation(path string, line int) {
	if a.EditorGroup.ActiveFilePath() != path {
		for _, g := range a.editorGroups() {
			if g.BufferForPath(path) != nil {
				a.activateEditorGroup(g)
				g.SwitchToTabByPath(path)
				break
			}
		}
	}
	if a.EditorGroup.ActiveFilePath() != path {
		a.navigateToLocation(path, line-1, 0)
		return
	}
	a.EditorGroup.GoToLine(line)
	a.Root.SetFocus(a.EditorGroup)
}
