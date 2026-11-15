package app

import (
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestTabAutoHideDelaysCancellationAndGraceArea(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	bar := a.EditorGroup.TabBar
	a.releaseTask(bar.AutoShowTask)
	a.releaseTask(bar.AutoHideTask)
	clock := gitGutterTestClock{now: time.Unix(0, 0)}
	var deliveries []func()
	a.tasks.post = func(fn func()) error { deliveries = append(deliveries, fn); return nil }
	options := asynctask.TaskOptions{Clock: &clock, Run: func(fn func()) { fn() }, Delay: ui.TabBarAutoShowDelay}
	bar.AutoShowTask = NewTask[bool](a, options)
	options.Delay = ui.TabBarAutoHideDelay
	bar.AutoHideTask = NewTask[bool](a, options)
	bar.SetStyle("auto-hide")
	redraw := func() { bar.SetRect(ui.Rect{X: 10, Y: 10, W: 60, H: bar.Height()}) }
	deliver := func() {
		for len(deliveries) > 0 {
			fn := deliveries[0]
			deliveries = deliveries[1:]
			fn()
		}
		redraw()
	}
	advance := func(d time.Duration) { clock.advance(d); deliver() }
	redraw()
	bar.UpdateAutoHide(20, 10)
	advance(ui.TabBarAutoShowDelay - time.Millisecond)
	if bar.Height() != 1 {
		t.Fatal("tabs showed before the delay")
	}
	bar.UpdateAutoHide(21, 10)
	clock.advance(time.Millisecond)
	if bar.Height() != 1 || len(deliveries) != 1 {
		t.Fatal("show delay restarted on movement or mutated the UI before delivery")
	}
	deliver()
	if bar.Height() != 3 {
		t.Fatal("tabs did not show after the delay")
	}
	for _, point := range [][2]int{{9, 11}, {70, 11}, {20, 9}, {20, 13}} {
		bar.UpdateAutoHide(point[0], point[1])
		advance(ui.TabBarAutoHideDelay)
		if bar.Height() != 3 {
			t.Fatalf("one-cell grace area hid the tabs at %v", point)
		}
	}
	bar.UpdateAutoHide(71, 11)
	advance(ui.TabBarAutoHideDelay - time.Millisecond)
	if bar.Height() != 3 {
		t.Fatal("tabs hid before the delay")
	}
	bar.UpdateAutoHide(70, 11)
	advance(time.Millisecond)
	if bar.Height() != 3 {
		t.Fatal("returning to the grace area did not cancel hiding")
	}
	bar.UpdateAutoHide(20, 14)
	advance(ui.TabBarAutoHideDelay)
	if bar.Height() != 1 {
		t.Fatal("tabs did not hide outside the grace area")
	}
	bar.UpdateAutoHide(20, 10)
	advance(ui.TabBarAutoShowDelay / 2)
	bar.UpdateAutoHide(0, 0)
	advance(ui.TabBarAutoShowDelay)
	if bar.Height() != 1 {
		t.Fatal("briefly crossing the edge did not cancel showing")
	}
	bar.UpdateAutoHide(20, 10)
	clock.advance(ui.TabBarAutoShowDelay)
	bar.UpdateAutoHide(0, 0)
	deliver()
	if bar.Height() != 1 {
		t.Fatal("a queued show reopened the tabs after leaving")
	}
	bar.UpdateAutoHide(20, 10)
	clock.advance(ui.TabBarAutoShowDelay)
	bar.SetStyle("hidden")
	deliver()
	if bar.Height() != 1 {
		t.Fatal("a queued show changed the newly selected style")
	}
	bar.SetStyle("auto-hide")
	bar.UpdateAutoHide(20, 10)
	advance(ui.TabBarAutoShowDelay)
	blocked := true
	bar.AutoHideBlocked = func() bool { return blocked }
	bar.UpdateAutoHide(0, 0)
	advance(ui.TabBarAutoHideDelay)
	if bar.Height() != 3 {
		t.Fatal("a timer hid tabs during a menu or drag")
	}
	blocked = false
	bar.UpdateAutoHide(0, 0)
	advance(ui.TabBarAutoHideDelay)
	if bar.Height() != 1 {
		t.Fatal("hiding did not resume after the menu or drag ended")
	}
}
