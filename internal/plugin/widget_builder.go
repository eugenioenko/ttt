package plugin

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type WidgetState struct {
	keys  []string
	items []widgets.Widget
	root  *widgets.VStackWidget
	focus *widgets.FocusManager
}

func NewWidgetState() *WidgetState {
	ws := &WidgetState{
		focus: widgets.NewFocusManager(),
	}
	ws.focus.OnFocusChange = func(w widgets.FocusableWidget) {
		if ws.root != nil {
			widgets.ScrollIntoView(ws.root, w)
		}
	}
	return ws
}

func (ws *WidgetState) Reconcile(descs []WidgetDesc, p *Plugin) *widgets.VStackWidget {
	newKeys := make([]string, len(descs))
	newWidgets := make([]widgets.Widget, len(descs))

	for i, desc := range descs {
		newKeys[i] = desc.Key

		if i < len(ws.keys) && ws.keys[i] == desc.Key && widgetMatchesKind(ws.items[i], desc.Kind) {
			updateWidget(ws.items[i], desc, p)
			newWidgets[i] = ws.items[i]
		} else {
			newWidgets[i] = createWidget(desc, p)
		}
	}

	ws.keys = newKeys
	ws.items = newWidgets
	ws.root = widgets.NewVStackWidget(newWidgets...)
	ws.focus.Collect(ws.root)
	return ws.root
}

func createWidget(desc WidgetDesc, p *Plugin) widgets.Widget {
	switch desc.Kind {
	case WidgetLabel:
		return createLabelWidget(desc, p)
	case WidgetTitle:
		return createTitleWidget(desc, p)
	case WidgetKeyValue:
		return createKeyValueWidget(desc)
	case WidgetTree:
		return createTreeWidget(desc, p)
	case WidgetList:
		return createListWidget(desc, p)
	case WidgetButton:
		return createButtonWidget(desc, p)
	case WidgetInput:
		return createInputWidget(desc, p)
	case WidgetVStack:
		return createVStackWidget(desc, p)
	case WidgetBox:
		return createBoxWidget(desc, p)
	case WidgetScrollView:
		return createScrollViewWidget(desc, p)
	case WidgetHStack:
		return createHStackWidget(desc, p)
	case WidgetDivider:
		return createDividerWidget(desc)
	case WidgetDropdown:
		return createDropdownWidget(desc, p)
	case WidgetProgress:
		return createProgressWidget(desc)
	case WidgetTable:
		return createTableWidget(desc, p)
	case WidgetMarkdown:
		return createMarkdownWidget(desc, p)
	case WidgetCheckbox:
		return createCheckboxWidget(desc, p)
	}
	return widgets.NewLabelWidget(widgets.LabelConfig{Text: "unknown widget"})
}

func updateWidget(w widgets.Widget, desc WidgetDesc, p *Plugin) {
	switch desc.Kind {
	case WidgetLabel:
		if lw, ok := w.(*widgets.LabelWidget); ok {
			lw.Config.Text = desc.Text
			lw.Config.Badge = desc.Badge
			lw.FixedWidth = desc.FixedWidth
			if desc.TextStyle != "" {
				lw.Config.Style = resolveStyleName(desc.TextStyle)
			}
			applyBoxModel(&lw.Box, desc)
			applyBoxBorders(&lw.Box, desc, p)
		}
	case WidgetTitle:
		if tw, ok := w.(*widgets.TitleWidget); ok {
			config := widgets.TitleConfig{
				Title:  desc.Text,
				Badge:  desc.Badge,
				Menu:   desc.Entries,
				Icon:   desc.Icon,
				Padded: desc.Padded,
			}
			wireTitleMenu(&config, desc, p)
			tw.UpdateConfig(config)
			applyBoxModel(&tw.Box, desc)
		}
	case WidgetKeyValue:
		if kv, ok := w.(*widgets.KeyValueListWidget); ok {
			kv.Entries = desc.KeyValueEntries
			applyBoxModel(&kv.Box, desc)
		}
	case WidgetTree, WidgetList:
		if tw, ok := w.(*widgets.TreeWidget); ok {
			expanded := map[string]bool{}
			tw.CollectExpanded(expanded)
			for _, item := range tw.Config.Items {
				if item.Expanded {
					expanded[item.ID] = true
				}
			}
			savedIdx := tw.SelectedIndex()
			tw.SetItems(desc.Items)
			for _, item := range tw.Config.Items {
				if expanded[item.ID] && item.Expandable {
					item.Expanded = true
				}
			}
			tw.RestoreExpandedSilent(expanded)
			tw.SetSelectedIndex(savedIdx)
			wireTreeCallbacks(tw, desc, p)
		}
	case WidgetButton:
		if bw, ok := w.(*widgets.ButtonWidget); ok {
			_ = bw
			// Button label cannot be changed after construction due to accelerator parsing.
			// Rewire callback only.
			wireButtonCallback(bw, desc, p)
		}
	case WidgetInput:
		if iw, ok := w.(*widgets.InputWidget); ok {
			iw.Config.Placeholder = desc.Placeholder
			iw.Config.Prefix = desc.Prefix
			wireInputCallbacks(iw, desc, p)
		}
	case WidgetVStack:
		if vs, ok := w.(*widgets.VStackWidget); ok {
			vs.Children = reconcileChildren(vs.Children, desc.Children, p)
			applyBoxModel(&vs.Box, desc)
		}
	case WidgetHStack:
		if hs, ok := w.(*widgets.HStackWidget); ok {
			hs.Children = reconcileChildren(hs.Children, desc.Children, p)
			applyBoxModel(&hs.Box, desc)
		}
	case WidgetDivider:
		// nothing to update
	case WidgetDropdown:
		if dd, ok := w.(*widgets.DropdownWidget); ok {
			dd.UpdateConfig(widgets.DropdownConfig{
				Label:   desc.Label,
				Entries: desc.Entries,
				Box:     &widgets.BoxModel{PaddingLeft: 1, PaddingRight: 1},
			})
			wireDropdownCallback(dd, desc, p)
			applyBoxModel(&dd.Box, desc)
		}
	case WidgetScrollView:
		if sv, ok := w.(*widgets.ScrollViewWidget); ok {
			if vs, ok := sv.Child.(*widgets.VStackWidget); ok {
				vs.Children = reconcileChildren(vs.Children, desc.Children, p)
			}
			applyBoxModel(&sv.Box, desc)
		}
	case WidgetBox:
		if bw, ok := w.(*widgets.BoxWidget); ok {
			applyBoxModel(&bw.Box, desc)
			applyBoxBorders(&bw.Box, desc, p)
			if desc.FixedHeight > 0 {
				bw.FixedHeight = desc.FixedHeight
			}
			if len(desc.Children) > 0 {
				if vs, ok := bw.Child.(*widgets.VStackWidget); ok {
					vs.Children = reconcileChildren(vs.Children, desc.Children, p)
				} else {
					bw.Child = createVStackFromDescs(desc.Children, p)
				}
			}
		}
	case WidgetProgress:
		if pw, ok := w.(*widgets.ProgressWidget); ok {
			pw.Config.Value = desc.Value
			if desc.Char != 0 {
				pw.Config.Char = desc.Char
			}
			if desc.StyleName != "" {
				pw.Config.Style = resolveStyleName(desc.StyleName)
			}
		}
	case WidgetTable:
		if tw, ok := w.(*widgets.TableWidget); ok {
			tw.Config.Columns = desc.Columns
			tw.Config.Rows = desc.Rows
			tw.Config.OnSelect = desc.OnSelectIndex
			tw.Config.OnCommand = desc.OnCommandStr
			tw.Config.NodeMenu = desc.NodeMenu
			tw.Config.KeyCommands = desc.KeyCommands
			wireTableMenu(tw, p)
			applyBoxModel(&tw.Box, desc)
		}
	case WidgetMarkdown:
		if sv, ok := w.(*widgets.ScrollViewWidget); ok {
			if md, ok := sv.Child.(*widgets.MarkdownWidget); ok {
				md.SetContent(desc.MarkdownContent)
			}
		}
	case WidgetCheckbox:
		if cw, ok := w.(*widgets.CheckboxWidget); ok {
			cw.Config.Label = desc.Label
			cw.Config.Checked = desc.Checked
			wireCheckboxCallback(cw, desc, p)
			if desc.StyleName != "" {
				cw.Config.Style = resolveStyleName(desc.StyleName)
			}
			applyBoxModel(&cw.Box, desc)
		}
	}
}

func widgetMatchesKind(w widgets.Widget, kind WidgetKind) bool {
	switch kind {
	case WidgetLabel:
		_, ok := w.(*widgets.LabelWidget)
		return ok
	case WidgetTitle:
		_, ok := w.(*widgets.TitleWidget)
		return ok
	case WidgetKeyValue:
		_, ok := w.(*widgets.KeyValueListWidget)
		return ok
	case WidgetTree, WidgetList:
		_, ok := w.(*widgets.TreeWidget)
		return ok
	case WidgetButton:
		_, ok := w.(*widgets.ButtonWidget)
		return ok
	case WidgetInput:
		_, ok := w.(*widgets.InputWidget)
		return ok
	case WidgetVStack:
		_, ok := w.(*widgets.VStackWidget)
		return ok
	case WidgetHStack:
		_, ok := w.(*widgets.HStackWidget)
		return ok
	case WidgetScrollView:
		_, ok := w.(*widgets.ScrollViewWidget)
		return ok
	case WidgetBox:
		_, ok := w.(*widgets.BoxWidget)
		return ok
	case WidgetDivider:
		_, ok := w.(*widgets.DividerWidget)
		return ok
	case WidgetDropdown:
		_, ok := w.(*widgets.DropdownWidget)
		return ok
	case WidgetProgress:
		_, ok := w.(*widgets.ProgressWidget)
		return ok
	case WidgetTable:
		_, ok := w.(*widgets.TableWidget)
		return ok
	case WidgetMarkdown:
		_, ok := w.(*widgets.ScrollViewWidget)
		return ok
	case WidgetCheckbox:
		_, ok := w.(*widgets.CheckboxWidget)
		return ok
	}
	return false
}

func reconcileChildren(old []widgets.Widget, descs []WidgetDesc, p *Plugin) []widgets.Widget {
	children := make([]widgets.Widget, len(descs))
	for i, cd := range descs {
		if i < len(old) && widgetMatchesKind(old[i], cd.Kind) {
			updateWidget(old[i], cd, p)
			children[i] = old[i]
		} else {
			children[i] = createWidget(cd, p)
		}
	}
	return children
}

func applyBoxModel(box *widgets.BoxModel, desc WidgetDesc) {
	box.MarginTop = desc.MarginTop
	box.MarginBottom = desc.MarginBottom
	box.MarginLeft = desc.MarginLeft
	box.MarginRight = desc.MarginRight
	box.PaddingTop = desc.PaddingTop
	box.PaddingBottom = desc.PaddingBottom
	box.PaddingLeft = desc.PaddingLeft
	box.PaddingRight = desc.PaddingRight
}

func applyBoxBorders(box *widgets.BoxModel, desc WidgetDesc, p *Plugin) {
	hasSideBorders := desc.BorderTop || desc.BorderBottom || desc.BorderLeft || desc.BorderRight
	if desc.Border {
		box.BorderTop = true
		box.BorderBottom = true
		box.BorderLeft = true
		box.BorderRight = true
	} else {
		box.BorderTop = desc.BorderTop
		box.BorderBottom = desc.BorderBottom
		box.BorderLeft = desc.BorderLeft
		box.BorderRight = desc.BorderRight
	}
	if desc.Border || hasSideBorders {
		borders := term.SingleBorderSet()
		if p.Borders != nil {
			borders = *p.Borders
		}
		box.Borders = borders
	}
}

func createLabelWidget(desc WidgetDesc, p *Plugin) *widgets.LabelWidget {
	style := term.StyleDefault
	if desc.TextStyle != "" {
		style = resolveStyleName(desc.TextStyle)
	}
	lw := widgets.NewLabelWidget(widgets.LabelConfig{
		Text:  desc.Text,
		Badge: desc.Badge,
		Style: style,
	})
	lw.FixedWidth = desc.FixedWidth
	applyBoxModel(&lw.Box, desc)
	applyBoxBorders(&lw.Box, desc, p)
	return lw
}

func createTitleWidget(desc WidgetDesc, p *Plugin) *widgets.TitleWidget {
	config := widgets.TitleConfig{
		Title:  desc.Text,
		Badge:  desc.Badge,
		Menu:   desc.Entries,
		Icon:   desc.Icon,
		Padded: desc.Padded,
	}
	wireTitleMenu(&config, desc, p)
	tw := widgets.NewTitleWidget(config)
	applyBoxModel(&tw.Box, desc)
	return tw
}

func wireTitleMenu(config *widgets.TitleConfig, desc WidgetDesc, p *Plugin) {
	config.OnMenu = nil
	if p.ShowContextMenu != nil && len(desc.Entries) > 0 {
		config.OnMenu = func(entries []widgets.MenuEntry, screenX, screenY int) {
			p.ShowContextMenu(entries, screenX, screenY, func(cmd string) {
				if desc.OnMenu != nil {
					desc.OnMenu(cmd)
				}
			})
		}
	}
}

func createKeyValueWidget(desc WidgetDesc) *widgets.KeyValueListWidget {
	kv := widgets.NewKeyValueListWidget(desc.KeyValueEntries)
	applyBoxModel(&kv.Box, desc)
	return kv
}

func createTreeWidget(desc WidgetDesc, p *Plugin) *widgets.TreeWidget {
	tw := widgets.NewTreeWidget(widgets.TreeConfig{
		Items:         desc.Items,
		Indent:        desc.Indent,
		NodeMenu:      desc.NodeMenu,
		SelectOnClick: desc.SelectOnClick,
		TruncateLeft:  desc.TruncateLeft,
	})
	wireTreeCallbacks(tw, desc, p)
	applyBoxModel(&tw.Box, desc)
	return tw
}

func createListWidget(desc WidgetDesc, p *Plugin) *widgets.TreeWidget {
	tw := widgets.NewTreeWidget(widgets.TreeConfig{
		Items:         desc.Items,
		NodeMenu:      desc.NodeMenu,
		SelectOnClick: desc.SelectOnClick,
		TruncateLeft:  desc.TruncateLeft,
	})
	wireTreeCallbacks(tw, desc, p)
	applyBoxModel(&tw.Box, desc)
	return tw
}

func createVStackFromDescs(descs []WidgetDesc, p *Plugin) *widgets.VStackWidget {
	children := make([]widgets.Widget, len(descs))
	for i, cd := range descs {
		children[i] = createWidget(cd, p)
	}
	return widgets.NewVStackWidget(children...)
}

func createVStackWidget(desc WidgetDesc, p *Plugin) *widgets.VStackWidget {
	vs := createVStackFromDescs(desc.Children, p)
	vs.Gap = desc.Gap
	applyBoxModel(&vs.Box, desc)
	return vs
}

func createHStackWidget(desc WidgetDesc, p *Plugin) *widgets.HStackWidget {
	children := make([]widgets.Widget, len(desc.Children))
	for i, cd := range desc.Children {
		children[i] = createWidget(cd, p)
	}
	hs := widgets.NewHStackWidget(children...)
	hs.Gap = desc.Gap
	hs.FixedHeight = desc.FixedHeight
	applyBoxModel(&hs.Box, desc)
	return hs
}

func createDividerWidget(_ WidgetDesc) *widgets.DividerWidget {
	return widgets.NewDividerWidget(widgets.DividerConfig{})
}

func createScrollViewWidget(desc WidgetDesc, p *Plugin) *widgets.ScrollViewWidget {
	child := createVStackFromDescs(desc.Children, p)
	// Measure grow widgets by content height so the stack doesn't collapse to zero.
	child.MeasureGrow = true
	sv := widgets.NewScrollViewWidget(child)
	applyBoxModel(&sv.Box, desc)
	return sv
}

func createBoxWidget(desc WidgetDesc, p *Plugin) *widgets.BoxWidget {
	box := widgets.NewBoxWidget(widgets.BoxModel{})
	applyBoxModel(&box.Box, desc)
	applyBoxBorders(&box.Box, desc, p)
	if desc.FixedHeight > 0 {
		box.FixedHeight = desc.FixedHeight
	}
	if len(desc.Children) > 0 {
		box.Child = createVStackFromDescs(desc.Children, p)
	}
	return box
}

func createDropdownWidget(desc WidgetDesc, p *Plugin) *widgets.DropdownWidget {
	dd := widgets.NewDropdownWidget(widgets.DropdownConfig{
		Label:   desc.Label,
		Entries: desc.Entries,
		Box:     &widgets.BoxModel{PaddingLeft: 1, PaddingRight: 1},
	})
	wireDropdownCallback(dd, desc, p)
	applyBoxModel(&dd.Box, desc)
	return dd
}

func wireDropdownCallback(dd *widgets.DropdownWidget, desc WidgetDesc, p *Plugin) {
	dd.Config.OnMenu = nil
	if p != nil && p.ShowContextMenu != nil && len(desc.Entries) > 0 {
		dd.Config.OnMenu = func(entries []widgets.MenuEntry, screenX, screenY int) {
			p.ShowContextMenu(entries, screenX, screenY, func(cmd string) {
				if desc.OnMenu != nil {
					desc.OnMenu(cmd)
				}
			})
		}
	}
}

func wireTreeCallbacks(tw *widgets.TreeWidget, desc WidgetDesc, p *Plugin) {
	tw.Config.OnSelect = desc.OnSelect
	tw.Config.OnExpand = desc.OnExpand
	tw.Config.OnCommand = desc.OnCommand
	tw.Config.NodeMenu = desc.NodeMenu
	tw.Config.OnMenu = nil
	if len(desc.NodeMenu) > 0 {
		if p != nil && p.ShowContextMenu != nil {
			tw.Config.OnMenu = func(entries []widgets.MenuEntry, node *widgets.TreeNode, sx, sy int) {
				p.ShowContextMenu(entries, sx, sy, func(cmd string) {
					if tw.Config.OnCommand != nil {
						tw.Config.OnCommand(cmd, node)
					}
				})
			}
		}
	}
	tw.Config.OnKey = nil
	if len(desc.KeyCommands) > 0 {
		kc := desc.KeyCommands
		tw.Config.OnKey = func(ev *tcell.EventKey, node *widgets.TreeNode) bool {
			if ev.Key() == tcell.KeyRune {
				if cmd, ok := kc[term.KeyRune(ev)]; ok && tw.Config.OnCommand != nil {
					tw.Config.OnCommand(cmd, node)
					return true
				}
			}
			return false
		}
	}
}

func createButtonWidget(desc WidgetDesc, p *Plugin) *widgets.ButtonWidget {
	bw := widgets.NewButtonWidget(widgets.ButtonConfig{
		Label: desc.Label,
	})
	wireButtonCallback(bw, desc, p)
	applyBoxModel(&bw.Box, desc)
	return bw
}

func wireButtonCallback(bw *widgets.ButtonWidget, desc WidgetDesc, _ *Plugin) {
	if desc.OnClick != nil {
		bw.Config.OnClick = desc.OnClick
	}
}

func createCheckboxWidget(desc WidgetDesc, p *Plugin) *widgets.CheckboxWidget {
	cw := widgets.NewCheckboxWidget(widgets.CheckboxConfig{
		Label:   desc.Label,
		Checked: desc.Checked,
	})
	if desc.StyleName != "" {
		cw.Config.Style = resolveStyleName(desc.StyleName)
	}
	wireCheckboxCallback(cw, desc, p)
	applyBoxModel(&cw.Box, desc)
	return cw
}

func wireCheckboxCallback(cw *widgets.CheckboxWidget, desc WidgetDesc, _ *Plugin) {
	if desc.OnChangeBool != nil {
		cw.Config.OnChange = desc.OnChangeBool
	}
}

func createInputWidget(desc WidgetDesc, p *Plugin) *widgets.InputWidget {
	iw := widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: desc.Placeholder,
		Prefix:      desc.Prefix,
	})
	wireInputCallbacks(iw, desc, p)
	applyBoxModel(&iw.Box, desc)
	return iw
}

func wireInputCallbacks(iw *widgets.InputWidget, desc WidgetDesc, _ *Plugin) {
	if desc.OnChange != nil {
		iw.Config.OnChange = desc.OnChange
	}
	if desc.OnSubmit != nil {
		onSubmit := desc.OnSubmit
		clearOnSubmit := desc.ClearOnSubmit
		iw.Config.OnSubmit = func(text string) {
			onSubmit(text)
			if clearOnSubmit {
				iw.Clear()
			}
		}
	}
}

func createProgressWidget(desc WidgetDesc) *widgets.ProgressWidget {
	style := resolveStyleName(desc.StyleName)
	ch := desc.Char
	if ch == 0 {
		ch = '▄'
	}
	pw := widgets.NewProgressWidget(widgets.ProgressConfig{
		Value: desc.Value,
		Style: style,
		Char:  ch,
	})
	applyBoxModel(&pw.Box, desc)
	return pw
}

func createMarkdownWidget(desc WidgetDesc, p *Plugin) *widgets.ScrollViewWidget {
	md := widgets.NewMarkdownWidget()
	if p != nil && p.Markdown.WrapWidth > 0 {
		md.MaxWidth = p.Markdown.WrapWidth
	}
	md.SetContent(desc.MarkdownContent)
	applyBoxModel(&md.Box, desc)
	sv := widgets.NewScrollViewWidget(md)
	md.SetScrollParent(sv)
	return sv
}

func createTableWidget(desc WidgetDesc, p *Plugin) *widgets.TableWidget {
	tw := widgets.NewTableWidget(widgets.TableConfig{
		Columns:     desc.Columns,
		Rows:        desc.Rows,
		OnSelect:    desc.OnSelectIndex,
		OnCommand:   desc.OnCommandStr,
		NodeMenu:    desc.NodeMenu,
		KeyCommands: desc.KeyCommands,
	})
	wireTableMenu(tw, p)
	applyBoxModel(&tw.Box, desc)
	return tw
}

func wireTableMenu(tw *widgets.TableWidget, p *Plugin) {
	tw.Config.OnMenu = nil
	if p == nil || p.ShowContextMenu == nil {
		return
	}
	tw.Config.OnMenu = func(entries []widgets.MenuEntry, rowIdx int, sx, sy int) {
		p.ShowContextMenu(entries, sx, sy, func(cmd string) {
			if tw.Config.OnCommand != nil {
				tw.Config.OnCommand(cmd, rowIdx)
			}
		})
	}
}
