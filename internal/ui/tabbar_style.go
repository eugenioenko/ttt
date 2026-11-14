package ui

import (
	"context"
	"time"
)

const (
	TabBarAutoShowDelay = 50 * time.Millisecond
	TabBarAutoHideDelay = 100 * time.Millisecond
)

func (t *TabBarWidget) SetStyle(style string) {
	if t.Style == style {
		return
	}
	t.InvalidatePointerInteraction()
	t.Style = style
	t.autoHideExpanded = false
	t.autoHideWantExpanded = false
}

func (t *TabBarWidget) UpdateAutoHide(x, y int) bool {
	if t.Style != "auto-hide" {
		return false
	}
	r := t.GetRect()
	if t.autoHideExpanded {
		r.X--
		r.Y--
		r.W += 2
		r.H += 2
	}
	expand := r.Contains(x, y)
	t.autoHideWantExpanded = expand
	if expand == t.autoHideExpanded {
		t.cancelAutoHide()
		return false
	}
	task, opposite := t.AutoShowTask, t.AutoHideTask
	if !expand {
		task, opposite = opposite, task
	}
	if opposite != nil {
		opposite.Stop()
	}
	apply := func(value bool, err error) {
		if err != nil || t.Style != "auto-hide" || t.autoHideWantExpanded != value {
			return
		}
		if t.AutoHideBlocked != nil && t.AutoHideBlocked() {
			if task != nil {
				task.Stop()
			}
			return
		}
		t.InvalidatePointerInteraction()
		t.autoHideExpanded = value
		if t.OnAutoHideChange != nil {
			t.OnAutoHideChange()
		}
	}
	before := t.autoHideExpanded
	if task == nil {
		apply(expand, nil)
	} else {
		task.ScheduleKey(expand, func(context.Context) (bool, error) { return expand, nil }, apply)
	}
	return before != t.autoHideExpanded
}

func (t *TabBarWidget) cancelAutoHide() {
	if t.AutoShowTask != nil {
		t.AutoShowTask.Stop()
	}
	if t.AutoHideTask != nil {
		t.AutoHideTask.Stop()
	}
}

func (t *TabBarWidget) effectiveStyle() string {
	if t.Style == "auto-hide" {
		if t.autoHideExpanded {
			return "extended"
		}
		return "hidden"
	}
	return t.Style
}

func (t *TabBarWidget) Height() int {
	switch t.effectiveStyle() {
	case "hidden":
		return 1
	default:
		return 3
	}
}

func (t *TabBarWidget) hasTabs() bool {
	return t.effectiveStyle() != "hidden"
}

func (t *TabBarWidget) baselineY() (int, bool) {
	r := t.GetRect()
	if r.H == 0 {
		return 0, false
	}
	switch t.effectiveStyle() {
	case "hidden":
		return r.Y, true
	default:
		return r.Y + r.H - 1, true
	}
}

func (t *TabBarWidget) topBorder() (int, bool) {
	switch t.effectiveStyle() {
	case "hidden":
		return 0, true
	default:
		return 2, true
	}
}
