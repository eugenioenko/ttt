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
type debugThread struct {
	ID   int
	Name string
}
type debugSidebar struct {
	threads                  []debugThread
	threadNodes              map[*widgets.TreeNode]int
	variables, stack, points *widgets.TreeWidget
	toolbar                  *widgets.HStackWidget
	adapter                  *ui.WidgetAdapter
	idle                     debugIdleAction
	controls                 debugControlState
	frames                   []debugFrame
	scopes                   []*widgets.TreeNode
	values                   map[*widgets.TreeNode]debugVariable
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
	inspection := debugSectionSplit(p.variables, p.stack, 0.5, 2)
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
	active, paused, ready, attached bool
	idle                            debugIdleAction
}

func (a *App) refreshDebugControls() {
	p := &a.debugger.panel
	state := debugControlState{idle: p.idle}
	if r := a.debugger.run; r != nil {
		state = debugControlState{active: true, paused: r.thread != nil, ready: r.ready, attached: r.attached}
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
		p.toolbar.Children = append(p.toolbar.Children, widgets.NewLabelWidget(widgets.LabelConfig{Text: p.idle.label, Style: term.StyleMuted}))
		button(p.idle.command, p.idle.button, "", true)
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
		if state.attached {
			button("debug.detach", i18n.Text("debug.detachButton"), icons.DebugDetach, true)
		} else {
			button("debug.stop", i18n.Text("debug.stopButton"), icons.DebugStop, true)
		}

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
	var points []*widgets.TreeNode
	p.breakNodes = make(map[*widgets.TreeNode]debugLocation)
	for _, path := range slices.Sorted(maps.Keys(a.debugger.points)) {
		for _, line := range slices.Sorted(maps.Keys(a.debugger.points[path])) {
			point := a.debugger.points[path][line]
			b := point.binding
			label := fmt.Sprintf("%s:%d", filepath.Base(path), line)
			if b.Line > 0 && (b.Line != line || b.Source.Path != "" && b.Source.Path != path) {
				dest := path
				if b.Source.Path != "" {
					dest = b.Source.Path
				}
				label += fmt.Sprintf(" %c %s:%d", icons.Rune(icons.ArrowRight), filepath.Base(dest), b.Line)
			}
			if a.debugger.points[path][line].confirm {
				label += "  " + i18n.Text("debug.confirmLine")
			} else if r := a.debugger.run; r != nil && r.edited[path] && point.runtimeLine == 0 {
				label += "  " + i18n.Text("debug.editedSource")
			} else if b.Message != "" {
				label += "  " + b.Message
			}
			n := &widgets.TreeNode{ID: fmt.Sprintf("%s:%d", path, line), IconName: a.breakpointIcon(point), IconStyle: term.StyleDebugBreakpoint, SelectedIconStyle: term.StyleDebugBreakpoint, Label: label}
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

func (a *App) clearDebugInspection() {
	p := &a.debugger.panel
	p.epoch++
	p.threads = nil
	p.frames = nil
	p.scopes = nil
	p.values = nil
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
	p.values = nil
	a.refreshDebugPanel()
	debugQuery(a, r, "stackTrace", map[string]int{"threadId": *r.thread, "levels": 50}, func(result struct{ StackFrames []debugFrame }) {
		p := &a.debugger.panel
		p.frames = result.StackFrames
		for i := range p.frames {
			p.frames[i].Source.Path = a.debugSourcePath(p.frames[i].Source.Path)
		}
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
	p.values = make(map[*widgets.TreeNode]debugVariable)
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
			p.values[n] = debugVariable{VariablesReference: scope.VariablesReference}
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
	ref := p.values[n].VariablesReference
	if ref == 0 || len(n.Children) > 0 || a.debugger.run == nil {
		return
	}
	debugQuery(a, a.debugger.run, "variables", map[string]int{"variablesReference": ref}, func(result struct{ Variables []debugVariable }) {
		n.Children = nil
		for i, v := range result.Variables {
			child := &widgets.TreeNode{ID: fmt.Sprintf("%s/%d", n.ID, i), Label: v.Name + " = " + v.Value, Expandable: v.VariablesReference != 0}
			n.Children = append(n.Children, child)
			p.values[child] = v
		}
		p.variables.SetItems(p.variables.Config.Items)
	})
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
