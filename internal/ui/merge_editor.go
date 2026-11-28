package ui

import (
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type mergeButton struct {
	rect   Rect
	action func()
}

type MergeEditorWidget struct {
	BaseWidget
	preview       *EditorPaneWidget
	projection    mergeProjection
	session       *mergeSession
	sources       [2]*EditorPaneWidget
	result        *EditorPaneWidget
	View          string
	Identity      string
	active, focus int
	buttons       []mergeButton
	onAction      func(string)
	confirm       func(func())
	captured      *EditorPaneWidget
}

func (g *EditorGroupWidget) EndMerge() {
	if t := g.activeTab(); t != nil && t.Merge != nil {
		t.Merge.close()
		t.Merge = nil
	}
}
func (g *EditorGroupWidget) EndMergeSession(m *MergeEditorWidget) {
	for i := range g.tabs {
		if m != nil && g.tabs[i].Merge == m {
			m.close()
			g.tabs[i].Merge = nil
			return
		}
	}
}
func (g *EditorGroupWidget) visualMerge() *MergeEditorWidget {
	if m := g.MergeEditor(); m != nil && m.View != config.MergeText && g.ActiveBuffer() == m.session.buf {
		return m
	}
	return nil
}
func (g *EditorGroupWidget) HasMergeBanner() bool {
	return g.OnOpenMerge != nil && g.IsEditorActive() && g.activeInlineDiff() == nil && g.visualMerge() == nil && (g.MergeEditor() != nil || len(g.Editor.conflictBlocks()) > 0)
}
func (g *EditorGroupWidget) MergeEditor() *MergeEditorWidget {
	if t := g.activeTab(); t != nil {
		return t.Merge
	}
	return nil
}

func (g *EditorGroupWidget) OpenMerge(inputs scm.ConflictInputs, view string, action func(string), confirm func(func())) bool {
	t := g.activeTab()
	if t == nil || t.ReadOnly || t.Buf == nil || t.Content != nil || t.Virtual {
		return false
	}
	if t.Merge != nil && t.Merge.Identity == inputs.Identity && t.Merge.session.valid {
		t.Merge.View = view
		t.Merge.result = g.Editor
		t.Merge.project()
		return true
	}
	session, lines := newMergeSession(t.Buf, inputs)
	if session == nil {
		return false
	}
	if t.Merge != nil {
		t.Merge.close()
	}
	if t.Diff != nil {
		g.DisableInlineDiff(t.FilePath)
	}
	m := &MergeEditorWidget{session: session, result: g.Editor, View: view, Identity: inputs.Identity, focus: 2, onAction: action, confirm: confirm}
	m.preview = newDiffPane()
	m.preview.ReadOnly = false
	m.preview.Undo = nil
	m.preview.TabSize = t.TabSize
	m.preview.UseTabs = t.UseTabs
	if g.SyntaxHighlight {
		m.preview.Highlighter = highlight.New(t.FilePath)
	}
	m.project()
	for side := range m.sources {
		p := newDiffPane()
		p.Buf.SetLines(lines[side])
		if g.SyntaxHighlight {
			p.Highlighter = highlight.NewForLines(t.FilePath, p.Buf.Lines)
		}
		o := NewAnnotationOverlay()
		for _, b := range session.blocks {
			for line := b.anchors[side]; line < b.anchors[side]+max(1, len(b.sides[side])); line++ {
				o.LineBg[line] = term.StyleDiffAdded
			}
		}
		p.SetDiffOverlay(o)
		p.BookmarkOverlay = func(line int) (Bookmark, bool) {
			i, ok := session.sourceBlock(side, line)
			if !ok || line != session.blocks[i].anchors[side] {
				return Bookmark{}, false
			}
			icon := icons.MergeUnchecked
			if session.blocks[i].choice&(1<<side) != 0 && !session.blocks[i].custom {
				icon = icons.MergeChecked
			}
			return Bookmark{Icon: icons.Rune(icon), Style: term.StyleGutterBookmark}, true
		}
		p.OnGutterClick = func(line int) bool {
			i, ok := session.sourceBlock(side, line)
			if ok {
				m.flushProjection()
				m.selectBlock(i)
				m.toggle(side)
			}
			return ok
		}
		m.sources[side] = p
	}
	t.Merge = m
	t.Preview = false
	m.selectBlock(0)
	return true
}

func (m *MergeEditorWidget) close() {
	m.preview.Highlighter.Close()
	for _, p := range m.sources {
		p.Highlighter.Close()
	}
}
func (m *MergeEditorWidget) Remaining() int { return m.session.remaining() }
func (m *MergeEditorWidget) CanComplete() bool {
	m.flushProjection()
	if m.Remaining() != 0 || !m.session.valid {
		return false
	}
	for _, line := range m.session.buf.Lines {
		if k := gitMarker(line, m.session.markerSize); k == '<' || k == '>' || k == '|' {
			return false
		}
	}
	return true
}
func (m *MergeEditorWidget) pane(i int) *EditorPaneWidget {
	if i == 2 {
		return m.preview
	}
	return m.sources[i]
}
func (m *MergeEditorWidget) selectBlock(i int) {
	m.session.sync()
	if !m.session.valid || i < 0 || i >= len(m.session.blocks) {
		return
	}
	m.active = i
	b := m.session.blocks[i]
	for k := range 3 {
		p := m.pane(k)
		line := m.projection.blocks[i].start
		if k < 2 {
			line = b.anchors[k]
		}
		p.Cursor.Line, p.Cursor.Col = min(line, len(p.Buf.Lines)-1), 0
		p.Viewport.TopLine = max(0, p.Cursor.Line-2)
	}
}
func (m *MergeEditorWidget) Navigate(delta int) {
	if n := len(m.session.blocks); n > 0 {
		m.selectBlock((m.active + n + delta) % n)
	}
}
func (m *MergeEditorWidget) toggle(side int) {
	if m.active >= len(m.session.blocks) {
		return
	}
	c := m.session.blocks[m.active].choice
	c = (c & (mergeCurrent | mergeIncoming)) ^ (1 << side)
	m.choose(c)
}
func (m *MergeEditorWidget) ApplyAction(id string) {
	m.flushProjection()
	switch id {
	case "focusNext":
		m.focus = (m.focus + 1) % 3
	case "incoming":
		m.toggle(0)
	case "current":
		m.toggle(1)
	case "neither":
		m.choose(mergeNeither)
	case "manual":
		m.choose(mergeManual)
	case "swap":
		if m.active < len(m.session.blocks) {
			c := m.session.blocks[m.active].choice
			if c&(mergeIncoming|mergeCurrent) == mergeIncoming|mergeCurrent {
				m.choose(c ^ mergeReverse)
			}
		}
	}
}
func (m *MergeEditorWidget) choose(c mergeChoice) {
	s := m.session
	s.sync()
	i := m.active
	if !s.valid || i >= len(s.blocks) {
		return
	}
	ver := s.buf.Version()
	apply := func() {
		if m.result.Buf != s.buf || ver != s.buf.Version() {
			return
		}
		if cmd := s.edit(i, c); cmd != nil {
			m.result.ExecCommand(cmd)
			m.project()
			m.focus = 2
			m.selectBlock(i)
		}
	}
	if (s.blocks[i].custom || s.blocks[i].choice == mergeManual) && c != mergeManual {
		m.confirm(apply)
		return
	}
	apply()
}

func (m *MergeEditorWidget) Render(s Surface) {
	w, h := s.Size()
	r := m.GetRect()
	m.flushProjection()
	m.project()
	m.buttons = m.buttons[:0]
	x := 0
	button := func(label string, f func()) {
		if x >= w {
			return
		}
		end := s.DrawText(x, 0, "["+label+"]", w, term.StyleButton)
		m.buttons = append(m.buttons, mergeButton{Rect{X: r.X + x, Y: r.Y, W: end - x, H: 1}, f})
		x = end + 1
	}
	button(i18n.Text("merge.view"), func() { m.onAction("view") })
	button(i18n.Text("merge.actions"), func() { m.onAction("actions") })
	button(string(icons.Rune(icons.ChevronLeft)), func() { m.Navigate(-1) })
	button(string(icons.Rune(icons.ChevronRight)), func() { m.Navigate(1) })
	if m.CanComplete() {
		button(i18n.Text("merge.complete"), func() { m.onAction("complete") })
	}
	if x < w {
		s.DrawText(x, 0, i18n.Format("merge.remaining", i18n.Args{"count": m.Remaining()}), w, term.StyleStatusBar)
	}
	var rects [3]Rect
	if m.View == config.MergeThree {
		thirds := w / 3
		rects = [3]Rect{{X: 0, Y: 1, W: thirds, H: h - 1}, {X: w - thirds, Y: 1, W: thirds, H: h - 1}, {X: thirds, Y: 1, W: w - 2*thirds, H: h - 1}}
	} else {
		top := (h - 1) / 2
		rects = [3]Rect{{X: 0, Y: 1, W: w / 2, H: top}, {X: w / 2, Y: 1, W: w - w/2, H: top}, {X: 0, Y: 1 + top, W: w, H: h - 1 - top}}
	}
	labels := [3]string{i18n.Text("conflict.incoming"), i18n.Text("conflict.current"), i18n.Text("merge.result")}
	for k, box := range rects {
		p := m.pane(k)
		p.SetRect(Rect{X: r.X + box.X, Y: r.Y + box.Y + 1, W: box.W, H: max(0, box.H-1)})
		if box.W <= 0 || box.H <= 0 {
			continue
		}
		title := labels[k]
		if k < 2 && m.session.labels[k] != "" {
			title += " (" + m.session.labels[k] + ")"
		}
		style := term.StyleStatusBar
		p.Passive = m.focus != k || m.result.Passive
		if m.focus == k {
			style = term.StyleButtonFocused
		}
		sub := s.Sub(box)
		sub.ClearRect(0, 0, box.W, 1, style)
		sub.DrawText(0, 0, title, box.W, style)
		if box.H > 1 {
			p.Render(sub.Sub(Rect{Y: 1, W: box.W, H: box.H - 1}))
		}
	}
}
func (m *MergeEditorWidget) anchor(pane, block int) int {
	if pane == 2 {
		return m.projection.blocks[block].start
	}
	return m.session.blocks[block].anchors[pane]
}
func (m *MergeEditorWidget) route(pane int, ev tcell.Event) EventResult {
	p := m.pane(pane)
	top, line := p.Viewport.TopLine, p.Cursor.Line
	out := p.HandleEvent(ev)
	if p.Cursor.Line != line && m.focus == pane {
		for i := range m.session.blocks {
			start, count := m.anchor(pane, i), 0
			if pane == 2 {
				count = m.projection.blocks[i].end - start
			} else {
				count = len(m.session.blocks[i].sides[pane])
			}
			if p.Cursor.Line >= start && p.Cursor.Line < start+max(1, count) {
				m.active = i
				break
			}
		}
	}
	if top != p.Viewport.TopLine {
		for k := range 3 {
			if k == pane {
				continue
			}
			offset := 0
			for i := range m.session.blocks {
				a := m.anchor(pane, i)
				if a > p.Viewport.TopLine {
					break
				}
				offset = m.anchor(k, i) - a
			}
			other := m.pane(k)
			other.Viewport.TopLine = other.Buf.ClampLine(p.Viewport.TopLine + offset)
		}
	}
	return out
}
func (m *MergeEditorWidget) HandleEvent(ev tcell.Event) EventResult {
	defer m.flushProjection()
	if me, ok := ev.(*tcell.EventMouse); ok {
		if m.captured != nil {
			p := m.captured
			out := p.HandleEvent(ev)
			if !p.OwnsPointerCapture() {
				m.captured = nil
			}
			return out
		}
		x, y := me.Position()
		if me.Buttons() == tcell.Button1 {
			for _, b := range m.buttons {
				if b.rect.Contains(x, y) {
					b.action()
					return EventConsumed
				}
			}
		}
		for k := range 3 {
			p := m.pane(k)
			if !p.GetRect().Contains(x, y) {
				continue
			}
			if me.Buttons() == tcell.Button1 {
				m.focus = k
			}
			out := m.route(k, ev)
			if p.OwnsPointerCapture() {
				m.captured = p
			}
			return out
		}
		return EventIgnored
	}
	if key, ok := ev.(*tcell.EventKey); ok && (key.Key() == tcell.KeyF7 || (key.Key() == tcell.KeyTab && m.focus < 2)) {
		m.focus = (m.focus + 1) % 3
		return EventConsumed
	}
	if key, ok := ev.(*tcell.EventKey); ok && m.focus < 2 && key.Key() == tcell.KeyRune && key.Str() == " " {
		if i, ok := m.session.sourceBlock(m.focus, m.pane(m.focus).Cursor.Line); ok {
			m.active = i
			m.toggle(m.focus)
		}
		return EventConsumed
	}
	return m.route(m.focus, ev)
}
