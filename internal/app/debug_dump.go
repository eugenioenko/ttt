package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type DebugState struct {
	Screen      DebugScreen        `json:"screen"`
	Cursor      DebugCursor        `json:"cursor"`
	Buffer      *DebugBuffer       `json:"buffer"`
	Viewport    *DebugViewport     `json:"viewport,omitempty"`
	Focus       string             `json:"focus"`
	Sidebar     DebugPanel         `json:"sidebar"`
	BottomPanel DebugPanel         `json:"bottom_panel"`
	Tabs        []DebugTab         `json:"tabs"`
	ActiveTab   int                `json:"active_tab"`
	Overlay     *DebugOverlay      `json:"overlay"`
	Selection   DebugSelection     `json:"selection"`
	MultiCursor []DebugMultiCursor `json:"multi_cursor,omitempty"`
	Diagnostics []DebugDiagnostic  `json:"diagnostics"`
	Output      []string           `json:"output"`
	Terminals   []DebugTerminal    `json:"terminals,omitempty"`
	WidgetTree  *DebugWidgetNode   `json:"widget_tree"`
}

// DebugTerminal reports one integrated-terminal tab's raw PTY byte tail, so
// a terminal-emulation bug can be diagnosed from what the child process
// actually sent rather than from ttt's parsed/rendered state.
type DebugTerminal struct {
	ID      string `json:"id"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
	RawTail string `json:"raw_tail"`
}

// DebugDiagnostic reports one diagnostic on the active editor (LSP or plugin),
// so scripted tests can observe squiggles that a text screenshot can't show.
type DebugDiagnostic struct {
	StartLine int    `json:"start_line"`
	StartCol  int    `json:"start_col"`
	EndLine   int    `json:"end_line"`
	EndCol    int    `json:"end_col"`
	Severity  int    `json:"severity"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Styled    bool   `json:"styled"`
}

type DebugScreen struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type DebugCursor struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

type DebugBuffer struct {
	Path          string   `json:"path"`
	Lines         int      `json:"lines"`
	Modified      bool     `json:"modified"`
	Text          []string `json:"text"`
	TextTruncated bool     `json:"text_truncated,omitempty"`
}

// DebugViewport reports the visible region of the active editor, so
// scripted tests can assert scroll position directly.
type DebugViewport struct {
	TopLine int `json:"top_line"`
	LeftCol int `json:"left_col"`
	Width   int `json:"width"`
	Height  int `json:"height"`
}

// DebugMultiCursor reports one cursor of the multicursor set, so scripted
// tests can observe secondary cursors that the primary cursor/selection
// fields can't show.
type DebugMultiCursor struct {
	Line    int       `json:"line"`
	Col     int       `json:"col"`
	Primary bool      `json:"primary,omitempty"`
	SelFrom *DebugPos `json:"sel_from,omitempty"`
}

type DebugPanel struct {
	Visible bool     `json:"visible"`
	Active  string   `json:"active"`
	Panels  []string `json:"panels"`
}

type DebugTab struct {
	Path     string `json:"path"`
	Modified bool   `json:"modified"`
}

type DebugOverlay struct {
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
}

type DebugSelection struct {
	Active bool      `json:"active"`
	Start  *DebugPos `json:"start,omitempty"`
	End    *DebugPos `json:"end,omitempty"`
}

type DebugPos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

type DebugWidgetNode struct {
	Type     string             `json:"type"`
	Rect     DebugRect          `json:"rect"`
	Focused  bool               `json:"focused,omitempty"`
	Visible  bool               `json:"visible"`
	Props    map[string]any     `json:"props,omitempty"`
	Children []*DebugWidgetNode `json:"children,omitempty"`
}

type DebugRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

func (a *App) BuildDebugState() *DebugState {
	w, h := a.Screen.Size()
	state := &DebugState{
		Screen: DebugScreen{Width: w, Height: h},
	}

	if a.EditorGroup.Editor != nil {
		line, col := a.EditorGroup.ActiveCursor()
		state.Cursor = DebugCursor{Line: line, Col: col}
		if vp := a.EditorGroup.Editor.Viewport; vp != nil {
			state.Viewport = &DebugViewport{
				TopLine: vp.TopLine, LeftCol: vp.LeftCol,
				Width: vp.Width, Height: vp.Height,
			}
		}
	}

	if buf := a.EditorGroup.ActiveBuffer(); buf != nil {
		const maxTextLines = 1000
		text := buf.Lines
		truncated := false
		if len(text) > maxTextLines {
			text = text[:maxTextLines]
			truncated = true
		}
		state.Buffer = &DebugBuffer{
			Path:          a.EditorGroup.ActiveFilePath(),
			Lines:         len(buf.Lines),
			Modified:      buf.Dirty,
			Text:          text,
			TextTruncated: truncated,
		}
	}

	if ed := a.EditorGroup.Editor; ed != nil {
		for _, d := range ed.Diagnostics {
			state.Diagnostics = append(state.Diagnostics, DebugDiagnostic{
				StartLine: d.StartLine, StartCol: d.StartCol,
				EndLine: d.EndLine, EndCol: d.EndCol,
				Severity: int(d.Severity), Source: d.Source,
				Message: d.Message, Styled: d.Style != 0,
			})
		}
	}

	state.Focus = a.describeFocus()

	state.Sidebar = DebugPanel{
		Visible: a.Sidebar.Visible,
		Active:  a.Sidebar.ActivePanel,
		Panels:  a.Sidebar.PanelIDs(),
	}

	state.BottomPanel = DebugPanel{
		Visible: a.ContentSplit.ShowBottom,
		Active:  a.BottomPanel.ActivePanel,
		Panels:  a.BottomPanel.PanelIDs(),
	}

	for i := range a.EditorGroup.TabCount() {
		path, modified := a.EditorGroup.TabInfo(i)
		state.Tabs = append(state.Tabs, DebugTab{Path: path, Modified: modified})
	}
	state.ActiveTab = a.EditorGroup.ActiveTabIndex()

	if len(a.Root.Overlays) > 0 {
		top := a.Root.Overlays[len(a.Root.Overlays)-1]
		state.Overlay = describeOverlay(top.Widget)
	}

	if ed := a.EditorGroup.Editor; ed != nil && ed.Multi != nil && ed.Multi.IsMulti() {
		for i, c := range ed.Multi.Cursors {
			mc := DebugMultiCursor{
				Line:    c.Line,
				Col:     c.Col,
				Primary: i == ed.Multi.Primary,
			}
			if c.Sel.Active {
				mc.SelFrom = &DebugPos{Line: c.Sel.Anchor.Line, Col: c.Sel.Anchor.Col}
			}
			state.MultiCursor = append(state.MultiCursor, mc)
		}
	}

	if active, sl, sc, el, ec := a.EditorGroup.ActiveSelection(); active {
		state.Selection = DebugSelection{
			Active: true,
			Start:  &DebugPos{Line: sl, Col: sc},
			End:    &DebugPos{Line: el, Col: ec},
		}
	}

	if a.Output != nil {
		for _, line := range a.Output.Lines {
			state.Output = append(state.Output, line.Time+" ["+line.PluginName+"] "+line.Message)
		}
	}

	for _, tt := range a.Terminals {
		cols, rows := tt.Term.Size()
		state.Terminals = append(state.Terminals, DebugTerminal{
			ID:      tt.ID,
			Cols:    cols,
			Rows:    rows,
			RawTail: strconv.Quote(string(tt.Term.RawTail())),
		})
	}

	state.WidgetTree = walkWidget(a.Root.Main)

	return state
}

func (a *App) describeFocus() string {
	f := a.Root.Focused
	if f == nil {
		return ""
	}
	switch f.(type) {
	case *ui.EditorPaneWidget:
		return "editor"
	case *ui.SidebarWidget:
		return "sidebar"
	case *ui.BottomPanelWidget:
		return "bottom_panel"
	case *ui.SearchWidget:
		return "search"
	default:
		return "other"
	}
}

func describeOverlay(w widgets.Widget) *DebugOverlay {
	switch v := w.(type) {
	case *widgets.DialogWidget:
		return &DebugOverlay{Type: "dialog", Title: v.Title}
	case *widgets.DrawerWidget:
		return &DebugOverlay{Type: "drawer"}
	default:
		return &DebugOverlay{Type: "unknown"}
	}
}

func walkWidget(w widgets.Widget) *DebugWidgetNode {
	if w == nil {
		return nil
	}
	node := &DebugWidgetNode{
		Type:    widgetTypeName(w),
		Rect:    rectToDebug(w.GetRect()),
		Visible: true,
	}

	if fw, ok := w.(widgets.FocusableWidget); ok {
		node.Focused = fw.IsFocused()
	}

	switch v := w.(type) {
	case *widgets.VStackWidget:
		for _, child := range v.Children {
			node.Children = append(node.Children, walkWidget(child))
		}
	case *widgets.HStackWidget:
		for _, child := range v.Children {
			node.Children = append(node.Children, walkWidget(child))
		}
	case *widgets.BoxWidget:
		if v.Child != nil {
			node.Children = append(node.Children, walkWidget(v.Child))
		}
	case *widgets.ScrollViewWidget:
		if v.Child != nil {
			node.Children = append(node.Children, walkWidget(v.Child.(widgets.Widget)))
		}
	case *widgets.TreeWidget:
		node.Props = map[string]any{
			"items":    len(v.FlatList()),
			"selected": v.SelectedIndex(),
		}
	case *widgets.InputWidget:
		node.Props = map[string]any{
			"text":        v.Text(),
			"placeholder": v.Config.Placeholder,
		}
	case *widgets.LabelWidget:
		node.Props = map[string]any{
			"text": v.Config.Text,
		}
	case *widgets.ButtonWidget:
		node.Props = map[string]any{
			"label": v.Config.Label,
		}
	case *widgets.DividerWidget:
		// no props
	case *widgets.DropdownWidget:
		node.Props = map[string]any{
			"label": v.Config.Label,
		}
	case *widgets.DialogWidget:
		node.Props = map[string]any{
			"title": v.Title,
		}
		if v.Content != nil {
			node.Children = append(node.Children, walkWidget(v.Content))
		}
	case *widgets.DrawerWidget:
		node.Props = map[string]any{
			"width": v.Config.Width,
		}
		if v.Content != nil {
			node.Children = append(node.Children, walkWidget(v.Content))
		}
	case *widgets.TabbedWidget:
		activeIdx := -1
		for i, item := range v.Tabs.Config.Items {
			if item.Active {
				activeIdx = i
				break
			}
		}
		node.Props = map[string]any{
			"active": activeIdx,
			"count":  len(v.Children),
		}
		for _, child := range v.Children {
			node.Children = append(node.Children, walkWidget(child))
		}

	// UI-level widgets
	case *ui.SplitPanelWidget:
		node.Props = map[string]any{
			"show_left":   v.ShowLeft,
			"divider_pos": v.DividerPos,
		}
		if v.Left != nil {
			node.Children = append(node.Children, walkWidget(v.Left))
		}
		if v.Right != nil {
			node.Children = append(node.Children, walkWidget(v.Right))
		}
	case *ui.ContentSplitWidget:
		node.Props = map[string]any{
			"show_bottom": v.ShowBottom,
			"bottom_h":    v.BottomH,
		}
		if v.Top != nil {
			node.Children = append(node.Children, walkWidget(v.Top))
		}
		if v.Bottom != nil {
			node.Children = append(node.Children, walkWidget(v.Bottom))
		}
	case *ui.SidebarWidget:
		node.Props = map[string]any{
			"visible": v.Visible,
			"active":  v.ActivePanel,
			"panels":  v.PanelIDs(),
		}
		if aw := v.ActiveWidget(); aw != nil {
			node.Children = append(node.Children, walkWidget(aw))
		}
	case *ui.BottomPanelWidget:
		node.Props = map[string]any{
			"visible": v.Visible,
			"active":  v.ActivePanel,
			"panels":  v.PanelIDs(),
		}
		if aw := v.ActiveWidget(); aw != nil {
			node.Children = append(node.Children, walkWidget(aw))
		}
	case *ui.EditorGroupWidget:
		paths := make([]string, v.TabCount())
		for i := range v.TabCount() {
			paths[i], _ = v.TabInfo(i)
		}
		node.Props = map[string]any{
			"active_tab": v.ActiveTabIndex(),
			"tabs":       paths,
			"path":       v.ActiveFilePath(),
		}
		if v.Editor != nil {
			line, col := v.ActiveCursor()
			node.Props["cursor_line"] = line
			node.Props["cursor_col"] = col
		}
	case *ui.StatusBarWidget:
		node.Props = map[string]any{}
	case *ui.MenuBarWidget:
		node.Props = map[string]any{}
	case *ui.EditorPaneWidget:
		node.Props = map[string]any{}
	case *ui.WidgetAdapter:
		if inner := v.Inner(); inner != nil {
			node.Children = append(node.Children, walkWidget(inner))
		}
	}

	return node
}

func widgetTypeName(w widgets.Widget) string {
	s := fmt.Sprintf("%T", w)
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimPrefix(s, "*")
	s = strings.TrimSuffix(s, "Widget")
	return s
}

func rectToDebug(r widgets.Rect) DebugRect {
	return DebugRect{X: r.X, Y: r.Y, W: r.W, H: r.H}
}

func (a *App) Screenshot() string {
	w, h := a.Screen.Size()
	var lines []string
	for y := 0; y < h; y++ {
		var line strings.Builder
		for x := 0; x < w; {
			str, _, width := a.Screen.GetContent(x, y)
			if str == "" {
				str = " "
			}
			line.WriteString(str)
			if width < 1 {
				width = 1
			}
			x += width
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	return strings.Join(lines, "\n")
}

func (a *App) DumpDebugState(path string) error {
	state := a.BuildDebugState()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (a *App) DumpScreenshot(path string) error {
	return os.WriteFile(path, []byte(a.Screenshot()), 0644)
}
