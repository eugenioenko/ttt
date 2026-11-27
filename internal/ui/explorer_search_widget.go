package ui

import (
	"context"
	"strings"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type ExplorerSearchWidget struct {
	widgets.BaseWidget
	Tree      *widgets.TreeWidget
	Input     *widgets.InputWidget
	Debounce  *asynctask.Task[struct{}]
	query     string
	visible   bool
	editing   bool
	focused   bool
	emptyText string
}

func NewExplorerSearchWidget(tree *widgets.TreeWidget, iconMode string) *ExplorerSearchWidget {
	s := &ExplorerSearchWidget{Tree: tree, Input: newPanelInput(), emptyText: tree.Config.EmptyText}
	s.Input.Config.Placeholder = i18n.Text("explorer.searchInFolders")
	s.Input.Config.OnChange = s.scheduleFilter
	s.Input.Actions = []widgets.InputAction{{Label: icons.Get(iconMode, icons.Close), OnClick: s.HideSearch}}
	s.SetFocused(false)
	return s
}

func (s *ExplorerSearchWidget) Height() int     { return 0 }
func (s *ExplorerSearchWidget) Width() int      { return 0 }
func (s *ExplorerSearchWidget) Focusable() bool { return true }
func (s *ExplorerSearchWidget) IsFocused() bool { return s.focused }
func (s *ExplorerSearchWidget) SetFocused(f bool) {
	s.focused = f
	s.Input.SetFocused(f && s.visible && s.editing)
	s.Tree.SetFocused(f)
}
func (s *ExplorerSearchWidget) FocusedInput() *widgets.InputWidget {
	if s.visible && s.editing {
		return s.Input
	}
	return nil
}
func (s *ExplorerSearchWidget) CursorPosition() (int, int, bool) {
	if input := s.FocusedInput(); input != nil {
		return input.CursorPosition()
	}
	return 0, 0, false
}
func (s *ExplorerSearchWidget) ShowSearch() {
	s.visible, s.editing = true, true
	s.SetFocused(s.focused)
}
func (s *ExplorerSearchWidget) HideSearch() {
	s.CancelPointerCapture()
	s.Input.SetText("")
	s.visible, s.editing = false, false
	s.SetFocused(s.focused)
}
func (s *ExplorerSearchWidget) matches(node *widgets.TreeNode) bool {
	return strings.Contains(strings.ToLower(node.Label), s.query)
}
func (s *ExplorerSearchWidget) RefreshSearch() { s.scheduleFilter(s.Input.Text()) }
func (s *ExplorerSearchWidget) scheduleFilter(text string) {
	if s.Debounce == nil {
		s.applyFilter(text)
		return
	}
	if text == "" {
		s.Debounce.Stop()
		s.applyFilter(text)
		return
	}
	s.Debounce.Schedule(func(context.Context) (struct{}, error) {
		return struct{}{}, nil
	}, func(_ struct{}, err error) {
		if err == nil {
			s.applyFilter(text)
		}
	})
}
func (s *ExplorerSearchWidget) flushFilter() {
	if s.Debounce != nil {
		s.Debounce.Stop()
	}
	if s.query != strings.ToLower(s.Input.Text()) {
		s.applyFilter(s.Input.Text())
	}
}
func (s *ExplorerSearchWidget) applyFilter(text string) {
	s.query = strings.ToLower(text)
	if text == "" {
		s.Tree.Config.EmptyText = s.emptyText
		s.Tree.SetFilter(nil)
		return
	}
	s.Tree.Config.EmptyText = i18n.Text("search.noResults")
	s.Tree.SetFilter(s.matches)
	if selected := s.Tree.Selected(); selected == nil || !s.matches(selected) {
		s.selectMatch(0, 1)
	}
}
func (s *ExplorerSearchWidget) selectMatch(start, step int) {
	nodes := s.Tree.FlatList()
	for i := start; i >= 0 && i < len(nodes); i += step {
		if s.matches(nodes[i]) {
			s.Tree.SetSelectedIndex(i)
			return
		}
	}
}
func (s *ExplorerSearchWidget) Render(surface Surface) {
	w, h := surface.Size()
	y := 0
	if s.visible && h > 0 {
		s.Input.Actions[0].Label = string(icons.Rune(icons.Close))
		renderPanelInput(surface, s.Input, 0, 0, w)
		y = min(2, h)
		if h > 1 {
			for x := 0; x < w; x++ {
				surface.SetCell(x, 1, term.Cell{Ch: '─', Style: term.StyleBorder})
			}
		}
	}
	sub := surface.Sub(Rect{Y: y, W: w, H: h - y})
	ox, oy := sub.Origin()
	s.Tree.SetRect(Rect{X: ox, Y: oy, W: w, H: h - y})
	s.Tree.Render(sub)
}
func (s *ExplorerSearchWidget) HandleEvent(ev tcell.Event) widgets.EventResult {
	if key, ok := ev.(*tcell.EventKey); ok {
		if key.Key() == tcell.KeyRune && key.Modifiers() == tcell.ModNone && term.KeyRune(key) == '/' && (!s.visible || !s.editing) {
			s.ShowSearch()
			return widgets.EventConsumed
		}
		if s.visible {
			switch key.Key() {
			case tcell.KeyEscape:
				s.HideSearch()
				return widgets.EventConsumed
			case tcell.KeyUp, tcell.KeyDown:
				s.flushFilter()
				step := 1
				if key.Key() == tcell.KeyUp {
					step = -1
				}
				s.selectMatch(s.Tree.SelectedIndex()+step, step)
				return widgets.EventConsumed
			case tcell.KeyEnter:
				s.flushFilter()
				s.Tree.ActivateSelected()
				return widgets.EventConsumed
			}
			if s.editing {
				return s.Input.HandleEvent(ev)
			}
		}
	}
	if mouse, ok := ev.(*tcell.EventMouse); ok && s.visible {
		if s.Tree.OwnsPointerCapture() {
			return s.Tree.HandleEvent(ev)
		}
		if s.Input.OwnsPointerCapture() {
			return s.Input.HandleEvent(ev)
		}
		x, y := mouse.Position()
		r := s.Input.GetRect()
		if x >= r.X && x < r.X+r.W && y == r.Y {
			if mouse.Buttons()&tcell.Button1 != 0 {
				s.editing = true
				s.SetFocused(s.focused)
			}
			return s.Input.HandleEvent(ev)
		}
		if mouse.Buttons()&tcell.Button1 != 0 {
			s.editing = false
			s.SetFocused(s.focused)
		}
	}
	return s.Tree.HandleEvent(ev)
}
func (s *ExplorerSearchWidget) CancelPointerCapture() bool {
	input := s.Input.CancelPointerCapture()
	return s.Tree.CancelPointerCapture() || input
}
func (s *ExplorerSearchWidget) OwnsPointerCapture() bool {
	return s.Input.OwnsPointerCapture() || s.Tree.OwnsPointerCapture()
}
func (s *ExplorerSearchWidget) InvalidatePointerInteraction() bool { return s.CancelPointerCapture() }
func (s *ExplorerSearchWidget) SetPointerCaptureInvalidated(fn func()) {
	s.Input.SetPointerCaptureInvalidated(fn)
	s.Tree.SetPointerCaptureInvalidated(fn)
}

func (s *ExplorerSearchWidget) PointerShapeAt(mx, my int) string {
	if s.visible {
		if shape := widgets.PointerShapeAt(s.Input, mx, my); shape != "" {
			return shape
		}
	}
	return widgets.PointerShapeAt(s.Tree, mx, my)
}
