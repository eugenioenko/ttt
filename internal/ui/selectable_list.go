package ui

import "github.com/gdamore/tcell/v2"

type ListAction int

const (
	ListActionNone     ListAction = iota
	ListActionActivate            // click or Enter
	ListActionContext             // right-click
)

type ListEventResult struct {
	Result  EventResult
	Action  ListAction
	ScreenX int
	ScreenY int
}

type SelectableList struct {
	Selected  int
	ScrollTop int
}

func (sl *SelectableList) EnsureVisible(visibleHeight int) {
	if sl.Selected < sl.ScrollTop {
		sl.ScrollTop = sl.Selected
	}
	if sl.Selected >= sl.ScrollTop+visibleHeight {
		sl.ScrollTop = sl.Selected - visibleHeight + 1
	}
}

func (sl *SelectableList) ClampSelected(itemCount int) {
	if sl.Selected >= itemCount {
		sl.Selected = itemCount - 1
	}
	if sl.Selected < 0 {
		sl.Selected = 0
	}
}

func (sl *SelectableList) HandleListEvent(ev tcell.Event, rect Rect, itemCount int) ListEventResult {
	none := ListEventResult{Result: EventIgnored}

	switch tev := ev.(type) {
	case *tcell.EventMouse:
		btn := tev.Buttons()
		if btn&tcell.Button1 != 0 {
			_, my := tev.Position()
			idx := sl.ScrollTop + (my - rect.Y)
			if idx >= 0 && idx < itemCount {
				sl.Selected = idx
				return ListEventResult{Result: EventConsumed, Action: ListActionActivate}
			}
			return ListEventResult{Result: EventConsumed}
		}
		if btn&tcell.Button2 != 0 {
			mx, my := tev.Position()
			idx := sl.ScrollTop + (my - rect.Y)
			if idx >= 0 && idx < itemCount {
				sl.Selected = idx
				return ListEventResult{Result: EventConsumed, Action: ListActionContext, ScreenX: mx, ScreenY: my}
			}
			return ListEventResult{Result: EventConsumed}
		}
		if btn&tcell.WheelUp != 0 {
			sl.ScrollTop -= 3
			if sl.ScrollTop < 0 {
				sl.ScrollTop = 0
			}
			return ListEventResult{Result: EventConsumed}
		}
		if btn&tcell.WheelDown != 0 {
			max := itemCount - rect.H
			if max < 0 {
				max = 0
			}
			sl.ScrollTop += 3
			if sl.ScrollTop > max {
				sl.ScrollTop = max
			}
			return ListEventResult{Result: EventConsumed}
		}

	case *tcell.EventKey:
		switch tev.Key() {
		case tcell.KeyUp:
			if sl.Selected > 0 {
				sl.Selected--
			}
			return ListEventResult{Result: EventConsumed}
		case tcell.KeyDown:
			if sl.Selected < itemCount-1 {
				sl.Selected++
			}
			return ListEventResult{Result: EventConsumed}
		case tcell.KeyEnter:
			return ListEventResult{Result: EventConsumed, Action: ListActionActivate}
		}
	}

	return none
}
