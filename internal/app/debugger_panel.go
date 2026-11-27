package app

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
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
type debugInspection struct {
	frames []debugFrame
	scopes []*widgets.TreeNode
	values map[*widgets.TreeNode]debugVariable
}

type debugSidebar struct {
	debugInspection
	stale bool

	variables, stack, points *widgets.TreeWidget
	toolbar                  *widgets.HStackWidget
	adapter                  *ui.WidgetAdapter
	idle                     debugIdleAction
	controls                 debugControlState
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
	a.wireDebuggerGutter(a.EditorGroup)
	config := widgets.TreeConfig{Indent: 2, OnExpand: a.expandDebugNode, OnCommand: func(cmd string, n *widgets.TreeNode) {
		if cmd != "activate" {
			return
		}
		if f, ok := p.frameNodes[n]; ok && !p.stale && a.debugger.run != nil && !a.debugger.run.stepping {
			next := p.debugInspection
			a.beginDebugInspection()
			a.loadDebugFrame(a.debugger.run, &next, f)
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
	p.variables.Config.OnMenu = a.showDebugVariableMenu
	p.stack = section("stack", i18n.Text("debug.stack"))
	p.points = widgets.NewTreeWidget(config)
	title := widgets.NewLabelWidget(widgets.LabelConfig{Text: i18n.Text("debug.breakpoints"), Style: term.StyleHoverBold})
	more := widgets.NewDropdownWidget(widgets.DropdownConfig{
		Entries: []widgets.MenuEntry{{Label: i18n.Text("debug.clearBreakpoints"), Command: "debug.clearBreakpoints"}},
		OnMenu: func(entries []widgets.MenuEntry, x, y int) {
			a.showWidgetMenu(entries, x, y, func(cmd string) { a.Reg.Execute(cmd) })
		},
	})
	header := widgets.NewHStackWidget(title, more)
	header.FixedHeight = 1
	p.toolbar = widgets.NewHStackWidget()
	p.toolbar.FixedHeight = 1
	p.toolbar.Gap = 1
	inspection := debugSectionSplit(p.variables, p.stack, 1.0/3, 2)
	sections := debugSectionSplit(inspection, widgets.NewVStackWidget(header, p.points), 0.25, 5)
	a.refreshDebugConfiguration()
	p.adapter = ui.NewWidgetAdapter(widgets.NewVStackWidget(p.toolbar, widgets.NewDividerWidget(widgets.DividerConfig{}), sections))
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

type debugControlState struct {
	active, paused, ready, fileReady bool
	idle                             debugIdleAction
}

func (a *App) refreshDebugControls() {
	p := &a.debugger.panel
	if p.toolbar == nil {
		return
	}
	state := debugControlState{idle: p.idle}
	if r := a.debugger.run; r != nil {
		state = debugControlState{active: true, paused: r.thread != nil, ready: r.session != nil && !r.busy}
	} else {
		_, state.fileReady = a.currentFileDebugTarget()
	}
	if p.controls == state && len(p.toolbar.Children) > 0 {
		return
	}
	p.controls = state
	p.toolbar.Children = nil
	button := func(id, label string, icon icons.Name, enabled bool) {
		b := widgets.NewButtonWidget(widgets.ButtonConfig{Label: label, Icon: icon, Command: id, Variant: widgets.ButtonFilled, OnCommand: func(cmd string) { a.Reg.Execute(cmd) }})
		b.Disabled = !enabled
		p.toolbar.Children = append(p.toolbar.Children, b)
	}
	if !state.active {
		button(p.idle.command, p.idle.button, "", true)
		p.toolbar.Children = append(p.toolbar.Children, widgets.NewBoxWidget(widgets.BoxModel{}))
		button("debug.file", i18n.Text("debug.file"), "", state.fileReady)
	} else {
		button("debug.stepOver", i18n.Text("debug.stepOverButton"), icons.DebugStepOver, state.ready && state.paused)
		button("debug.stepIn", i18n.Text("debug.stepInButton"), icons.DebugStepIn, state.ready && state.paused)
		button("debug.stepOut", i18n.Text("debug.stepOutButton"), icons.DebugStepOut, state.ready && state.paused)
		p.toolbar.Children = append(p.toolbar.Children, widgets.NewBoxWidget(widgets.BoxModel{}))
		if state.paused {
			button("debug.continue", i18n.Text("debug.continueButton"), icons.DebugContinue, state.ready)
		} else {
			button("debug.pause", i18n.Text("debug.pauseButton"), icons.DebugPause, state.ready)
		}
		button("debug.stop", i18n.Text("debug.stopButton"), icons.DebugStop, true)
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
	for _, f := range p.frames {
		node := &widgets.TreeNode{ID: fmt.Sprintf("frame:%d", f.ID), Label: f.Name + "  " + filepath.Base(f.Source.Path) + ":" + strconv.Itoa(f.Line)}
		stack.Children = append(stack.Children, node)
		p.frameNodes[node] = f
	}
	var points []*widgets.TreeNode
	p.breakNodes = make(map[*widgets.TreeNode]debugLocation)
	for _, path := range slices.Sorted(maps.Keys(a.debugger.points)) {
		for _, line := range a.debugger.points[path] {
			label := fmt.Sprintf("%s:%d", filepath.Base(path), line)
			n := &widgets.TreeNode{ID: fmt.Sprintf("%s:%d", path, line), Label: label, IconName: icons.Breakpoint, IconStyle: term.StyleDebugBreakpoint, SelectedIconStyle: term.StyleDebugBreakpoint}
			points = append(points, n)
			p.breakNodes[n] = debugLocation{path, line}
		}
	}
	p.points.SetItems(points)
	p.variables.Config.Items[0].Children = p.scopes
	for _, tree := range []*widgets.TreeWidget{p.variables, p.stack} {
		tree.SetItems(tree.Config.Items)
	}
}

func (a *App) beginDebugInspection() {
	p := &a.debugger.panel
	p.epoch++
	p.stale = true
}

func (a *App) publishDebugInspection(next *debugInspection) {
	p := &a.debugger.panel
	p.debugInspection = *next
	p.stale = false
	a.refreshDebugPanel()
}

func (a *App) clearDebugInspection() {
	a.beginDebugInspection()
	a.debugger.file = ""
	a.publishDebugInspection(&debugInspection{})
}

func (a *App) debugResumed(r *debugRun) {
	r.thread = nil
	if r.stepping {
		a.beginDebugInspection()
		a.debugger.file = ""
		a.refreshDebugControls()
		return
	}
	a.clearDebugInspection()
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
			}
			apply(result)
		})
	}()
}
func (a *App) loadDebugFrame(r *debugRun, next *debugInspection, f debugFrame) {
	if f.Source.Path != "" && f.Line > 0 {
		a.revealDebugLocation(f.Source.Path, f.Line)
	}
	expanded := make(map[string]bool)
	for n := range a.debugger.panel.values {
		if n.Expandable {
			expanded[n.ID] = n.Expanded
		}
	}
	next.scopes = nil
	next.values = make(map[*widgets.TreeNode]debugVariable)
	debugQuery(a, r, "scopes", map[string]int{"frameId": f.ID}, func(result struct {
		Scopes []struct {
			Name               string
			VariablesReference int
			Expensive          bool
		}
	}) {
		// Count the scopes response as pending until every visible child request is scheduled.
		pending := 1
		done := func() {
			pending--
			if pending == 0 {
				a.publishDebugInspection(next)
			}
		}
		var loadExpanded func(*widgets.TreeNode)
		loadExpanded = func(n *widgets.TreeNode) {
			if wasOpen, ok := expanded[n.ID]; ok {
				n.Expanded = wasOpen
			}
			if !n.Expandable || !n.Expanded {
				return
			}
			pending++
			a.loadDebugVariables(r, next, n, func() {
				for _, child := range n.Children {
					loadExpanded(child)
				}
				done()
			})
		}
		for _, scope := range result.Scopes {
			n := &widgets.TreeNode{ID: "scope:" + strconv.Quote(scope.Name), Label: scope.Name, Expandable: scope.VariablesReference != 0, Expanded: !scope.Expensive}
			next.scopes = append(next.scopes, n)
			next.values[n] = debugVariable{VariablesReference: scope.VariablesReference}
			loadExpanded(n)
		}
		done()
	})
}

func (a *App) loadDebugVariables(r *debugRun, inspection *debugInspection, n *widgets.TreeNode, done func()) {
	ref := inspection.values[n].VariablesReference
	debugQuery(a, r, "variables", map[string]int{"variablesReference": ref}, func(result struct{ Variables []debugVariable }) {
		n.Children = nil
		for _, v := range result.Variables {
			child := &widgets.TreeNode{ID: n.ID + "/" + strconv.Quote(v.Name), Label: v.Name + " = " + v.Value, Expandable: v.VariablesReference != 0}
			n.Children = append(n.Children, child)
			inspection.values[child] = v
		}
		done()
	})
}

func (a *App) expandDebugNode(n *widgets.TreeNode) {
	p := &a.debugger.panel
	r := a.debugger.run
	if p.stale || r == nil || r.thread == nil || r.stepping {
		return
	}
	if p.values[n].VariablesReference != 0 && len(n.Children) == 0 {
		a.loadDebugVariables(r, &p.debugInspection, n, func() { p.variables.SetItems(p.variables.Config.Items) })
	}
}

func (a *App) revealDebugLocation(path string, line int) {
	if a.Root.HasModalOverlay() {
		return
	}
	focus := a.Root.Focused
	follow := focus == a.EditorGroup || focus == a.debugger.panel.adapter
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
		if err := a.EditorGroup.OpenFile(path); err != nil {
			return
		}
	}
	a.EditorGroup.GoToLine(line)
	if follow {
		a.Root.SetFocus(a.EditorGroup)
	} else if focus != nil && a.Root.Focused != focus {
		a.Root.SetFocus(focus)
	}
}

func (a *App) showDebugVariableMenu(_ []widgets.MenuEntry, n *widgets.TreeNode, x, y int) {
	p := &a.debugger.panel
	v, ok := p.values[n]
	if !ok || slices.Contains(p.scopes, n) {
		return
	}
	a.showWidgetMenu([]widgets.MenuEntry{
		{Label: i18n.Text("debug.copyKey"), Command: "copyKey"},
		{Label: i18n.Text("debug.copyValue"), Command: "copyValue"},
	}, x, y, func(command string) {
		switch command {
		case "copyKey":
			clipboard.Set(v.Name)
		case "copyValue":
			clipboard.Set(v.Value)
		}
	})
}

func (a *App) showDebugStop(r *debugRun, tid int, frames []debugFrame) {
	r.generation++
	r.busy = false
	r.stepping = false
	r.thread = &tid
	r.lastThread = &tid
	a.beginDebugInspection()
	a.refreshDebugControls()
	next := &debugInspection{frames: frames}
	if len(frames) == 0 {
		a.debugger.file = ""
		a.publishDebugInspection(next)
		return
	}
	f := frames[0]
	a.debugger.file, a.debugger.line = f.Source.Path, f.Line
	if f.Source.Path != "" && f.Line > 0 {
		a.StatusNotify(i18n.Format("debug.stoppedAt", i18n.Args{"file": filepath.Base(f.Source.Path), "line": f.Line}))
	}
	a.loadDebugFrame(r, next, f)
}
