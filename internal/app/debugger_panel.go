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
	frameID int
	frames  []debugFrame
	scopes  []*widgets.TreeNode
	values  map[*widgets.TreeNode]debugVariable
}

type debugSidebar struct {
	debugInspection
	stale bool

	variables, stack, points *widgets.TreeWidget
	toolbar                  *widgets.HStackWidget
	adapter                  *ui.WidgetAdapter
	idle                     debugIdleAction
	controls                 debugControlState
	threadNodes              map[*widgets.TreeNode]int
	frameNodes               map[*widgets.TreeNode]debugFrame
	breakNodes               map[*widgets.TreeNode]debugLocation
	epoch                    uint64
}
type debugLocation struct {
	path string
	line int
}

func (a *App) ensureDebugPanel() {
	a.restoreBreakpoints()
	p := &a.debugger.panel
	if p.adapter != nil {
		return
	}
	a.wireDebuggerGutter(a.EditorGroup)
	config := widgets.TreeConfig{Indent: 2, OnExpand: a.expandDebugNode, OnCommand: func(cmd string, n *widgets.TreeNode) {
		if cmd != "activate" {
			return
		}
		if tid, ok := p.threadNodes[n]; ok {
			a.selectDebugThread(tid)
			return
		}
		if f, ok := p.frameNodes[n]; ok && !p.stale && a.activeDebugPeer() != nil && !a.activeDebugPeer().stepping {
			next := p.debugInspection
			a.beginDebugInspection()
			a.loadDebugFrame(a.activeDebugPeer(), &next, f)
		}
		if b, ok := p.breakNodes[n]; ok {
			a.revealDebugLocation(b.path, b.line)
		}
	}}
	p.variables = widgets.NewTreeWidget(config)
	p.variables.Config.OnMenu = a.showDebugVariableMenu
	p.stack = widgets.NewTreeWidget(config)
	p.points = widgets.NewTreeWidget(config)
	title := debugHeading(i18n.Text("debug.breakpoints"))
	more := widgets.NewDropdownWidget(widgets.DropdownConfig{
		Entries: []widgets.MenuEntry{{Label: i18n.Text("debug.clearBreakpoints"), Command: "debug.clearBreakpoints"}},
		OnMenu: func(entries []widgets.MenuEntry, x, y int) {
			a.showWidgetMenu(entries, x, y, func(cmd string) { a.Reg.Execute(cmd) })
		},
	})
	header := widgets.NewHStackWidget(title, more)
	header.FixedHeight = 1
	p.toolbar = widgets.NewHStackWidget()
	p.toolbar.Box.PaddingLeft, p.toolbar.Box.PaddingRight = 1, 1
	p.toolbar.FixedHeight = 1
	p.toolbar.Gap = 1
	section := func(heading ui.Widget, tree *widgets.TreeWidget) *widgets.VStackWidget {
		return widgets.NewVStackWidget(heading, widgets.NewDividerWidget(widgets.DividerConfig{}), tree)
	}
	variables := section(debugHeading(i18n.Text("debug.variables")), p.variables)
	stack := section(debugHeading(i18n.Text("debug.stack")), p.stack)
	inspection := debugSectionSplit(variables, stack, 1.0/3, 2)
	sections := debugSectionSplit(inspection, section(header, p.points), 0.25, 5)
	a.refreshDebugConfiguration()
	p.adapter = ui.NewWidgetAdapter(widgets.NewVStackWidget(p.toolbar, widgets.NewDividerWidget(widgets.DividerConfig{}), sections))
	a.Sidebar.AddPanel("debug", i18n.Text("debug.panel"), p.adapter)
	a.refreshDebugPanel()
}

func debugHeading(text string) *widgets.LabelWidget {
	label := widgets.NewLabelWidget(widgets.LabelConfig{Text: text, Style: term.StyleHoverBold})
	label.Box.PaddingLeft = 1
	return label
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
	active, paused, ready, fileReady, attached bool
	idle                                       debugIdleAction
}

func (a *App) refreshDebugControls() {
	p := &a.debugger.panel
	if p.toolbar == nil {
		return
	}
	state := debugControlState{idle: p.idle}
	if r := a.activeDebugPeer(); r != nil {
		state = debugControlState{active: true, attached: a.debugger.run.attached, paused: r.thread != nil, ready: r.session != nil && !r.busy}
	} else {
		_, state.fileReady = a.currentFileDebugTarget()
	}
	if p.controls == state && len(p.toolbar.Children) > 0 {
		return
	}
	if a.hover.button != nil {
		a.DismissHover()
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
	var stack []*widgets.TreeNode
	p.frameNodes = make(map[*widgets.TreeNode]debugFrame)
	frames := &stack
	p.threadNodes = make(map[*widgets.TreeNode]int)
	if r := a.activeDebugPeer(); r != nil && r.thread != nil && len(r.threads) > 1 {
		for _, t := range r.threads {
			node := &widgets.TreeNode{ID: fmt.Sprintf("thread:%d", t.ID), Label: t.Name, Expanded: t.ID == *r.thread}
			stack = append(stack, node)
			p.threadNodes[node] = t.ID
			if node.Expanded {
				frames = &node.Children
			}
		}
	}
	for _, f := range p.frames {
		node := &widgets.TreeNode{ID: fmt.Sprintf("frame:%d", f.ID), Label: f.Name + "  " + filepath.Base(f.Source.Path) + ":" + strconv.Itoa(f.Line)}
		*frames = append(*frames, node)
		p.frameNodes[node] = f
	}
	var points []*widgets.TreeNode
	p.breakNodes = make(map[*widgets.TreeNode]debugLocation)
	for _, path := range slices.Sorted(maps.Keys(a.debugger.points)) {
		for _, line := range slices.Sorted(maps.Keys(a.debugger.points[path])) {
			pt := a.debugger.points[path][line]
			label := fmt.Sprintf("%s:%d", filepath.Base(path), line)
			b := a.bindingFor(path, pt)
			switch {
			case pt.confirm:
				label += "  " + i18n.Text("debug.confirmLine")
			case a.debugger.edited[path] && pt.runtimeLine == 0:
				label += "  " + i18n.Text("debug.editedSource")
			default:
				if b.Line > 0 && (b.Line != pt.runtimeLine || b.Source.Path != "" && b.Source.Path != path) {
					dest := b.Source.Path
					if dest == "" {
						dest = path
					}
					label += fmt.Sprintf(" %c %s:%d", icons.Rune(icons.ArrowRight), filepath.Base(dest), b.Line)
				}
				if b.Message != "" {
					label += "  " + b.Message
				}
			}
			n := &widgets.TreeNode{ID: fmt.Sprintf("%s:%d", path, line), Label: label, IconName: a.breakpointIcon(path, pt), IconStyle: term.StyleDebugBreakpoint, SelectedIconStyle: term.StyleDebugBreakpoint}
			points = append(points, n)
			p.breakNodes[n] = debugLocation{path, line}
		}
	}
	p.points.SetItems(points)
	p.variables.SetItems(p.scopes)
	p.stack.SetItems(stack)
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
	if p.stack != nil && len(next.frames) > 0 {
		p.stack.SelectByID(fmt.Sprintf("frame:%d", next.frameID))
	}
}

func (a *App) clearDebugInspection() {
	a.beginDebugInspection()
	a.debugger.file = ""
	a.publishDebugInspection(&debugInspection{})
}

func (a *App) debugResumed(r *debugRun) {
	r.thread = nil
	if a.selectPausedPeer() {
		return
	}
	if r.stepping {
		a.beginDebugInspection()
		a.debugger.file = ""
		a.refreshDebugControls()
		return
	}
	a.clearDebugInspection()
}

// DAP frame and variable references expire as soon as execution resumes.
func debugQuery[T any](a *App, r *debugRun, method string, args any, apply func(T, error)) {
	epoch := a.debugger.panel.epoch
	post := a.tasks.post
	go func() {
		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
		defer cancel()
		var result T
		err := r.session.Call(ctx, method, args, &result)
		_ = post(func() {
			if a.activeDebugPeer() != r || a.debugger.panel.epoch != epoch || r.ctx.Err() != nil {
				return
			}
			if err != nil {
				a.StatusError(err.Error())
			}
			apply(result, err)
		})
	}()
}
func (a *App) loadDebugFrame(r *debugRun, next *debugInspection, f debugFrame) {
	next.frameID = f.ID
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
	}, _ error) {
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
	debugQuery(a, r, "variables", map[string]int{"variablesReference": ref}, func(result struct{ Variables []debugVariable }, _ error) {
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
	r := a.activeDebugPeer()
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

func (a *App) selectDebugThread(tid int) {
	r := a.activeDebugPeer()
	if a.debugger.panel.stale || r == nil || r.thread == nil || r.busy || r.stepping || *r.thread == tid {
		return
	}
	generation := r.generation
	previous := a.debugger.panel.debugInspection
	a.beginDebugInspection()
	debugQuery(a, r, "stackTrace", map[string]int{"threadId": tid, "levels": 50}, func(result struct{ StackFrames []debugFrame }, err error) {
		if r.generation != generation || r.busy || r.stepping {
			return
		}
		if err != nil {
			a.publishDebugInspection(&previous)
			return
		}
		for i := range result.StackFrames {
			p := result.StackFrames[i].Source.Path
			if p != "" && !filepath.IsAbs(p) {
				result.StackFrames[i].Source.Path = filepath.Join(r.folder, p)
			}
		}
		r.frames = result.StackFrames
		a.showDebugStop(r, tid, r.frames)
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

func (a *App) toggleDebugToolbar() {
	r := a.debugger.run
	if r == nil || a.Root.DebugToolbar != nil {
		a.Root.DebugToolbar = nil
		return
	}
	a.Root.DebugToolbar = &ui.DebugToolbar{
		Corner:       a.debugger.toolbarCorner,
		EditorBounds: func() ui.Rect { return a.EditorGroup.ContentRect() },
		Borders:      a.Borders,
		Attached:     r.attached,
		CanStep: func() bool {
			peer := a.activeDebugPeer()
			return peer != nil && peer.session != nil && peer.thread != nil && !peer.busy && !peer.stepping
		},
		OnCommand: func(cmd string) { a.Reg.Execute(cmd) },
		OnMenu:    a.showDebugToolbarDropdown,
	}
}

type debugToolbarPosition struct {
	corner         ui.ToolbarCorner
	command, label string
}

func debugToolbarPositions() []debugToolbarPosition {
	return []debugToolbarPosition{
		{ui.ToolbarTopLeft, "debug.dockTopLeft", i18n.Text("debug.dockTopLeft")},
		{ui.ToolbarTopRight, "debug.dockTopRight", i18n.Text("debug.dockTopRight")},
		{ui.ToolbarBottomLeft, "debug.dockBottomLeft", i18n.Text("debug.dockBottomLeft")},
		{ui.ToolbarBottomRight, "debug.dockBottomRight", i18n.Text("debug.dockBottomRight")},
	}
}

func (a *App) debugToolbarMenu() []ui.ContextMenuItem {
	label := i18n.Text("debug.showToolbar")
	if a.Root.DebugToolbar != nil {
		label = i18n.Text("debug.hideToolbar")
	}
	return []ui.ContextMenuItem{
		{Label: label, Command: "debug.toggleToolbar", Disabled: a.debugger.run == nil},
		{Label: i18n.Text("debug.toolbarPosition"), Submenu: a.debugToolbarPositionItems()},
	}
}

func (a *App) debugToolbarPositionItems() []ui.ContextMenuItem {
	var positions []ui.ContextMenuItem
	for _, pos := range debugToolbarPositions() {
		positions = append(positions, ui.ContextMenuItem{Label: pos.label, Command: pos.command, Checked: menuChecked(pos.corner == a.debugger.toolbarCorner)})
	}
	return positions
}

func (a *App) showDebugToolbarDropdown(x, y int) {
	items := append(a.debugToolbarPositionItems(), ui.MenuSep(), ui.ContextMenuItem{Label: i18n.Text("debug.toolbarHidden"), Command: "debug.toggleToolbar"})
	openContextMenu(a, items, x, y)
}
