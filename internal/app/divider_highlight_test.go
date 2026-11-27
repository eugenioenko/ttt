package app

import (
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

func TestDividerHighlightWaitsForThePointerToRest(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	settings := config.DefaultSettings()
	settings.Appearance.HighlightDividers = true
	a := buildTestApp(t, settings)
	t.Cleanup(a.CloseTasks)
	a.releaseTask(a.Root.DividerHoverTask)
	clock := gitGutterTestClock{now: time.Unix(0, 0)}
	var deliveries []func()
	a.tasks.post = func(fn func()) error { deliveries = append(deliveries, fn); return nil }
	a.Root.DividerHoverTask = NewTask[struct{}](a, asynctask.TaskOptions{
		Clock: &clock, Run: func(fn func()) { fn() }, Delay: ui.DividerHighlightDelay,
	})
	a.SplitPanel.ShowLeft = true
	a.SplitPanel.OnResize = func(int) {}
	a.Root.SetSize(100, 30)
	var renderer render.Renderer
	lit := func() bool {
		cells := renderer.NextFrame(100, 30)
		a.Root.Render(cells)
		for _, row := range cells {
			for _, c := range row {
				if c.Style == term.StyleBorderHover {
					return true
				}
			}
		}
		return false
	}
	mouse := func(x, y int, btn tcell.ButtonMask) {
		a.Root.HandleEvent(tcell.NewEventMouse(x, y, btn, tcell.ModNone))
	}
	advance := func(d time.Duration) {
		clock.advance(d)
		for len(deliveries) > 0 {
			fn := deliveries[0]
			deliveries = deliveries[1:]
			fn()
		}
	}
	lit()
	divX := a.SplitPanel.DividerScreenX()
	y := a.SplitPanel.GetRect().Y + 10

	mouse(divX, y, tcell.ButtonNone)
	if lit() {
		t.Fatal("divider lit before the pointer rested")
	}
	advance(ui.DividerHighlightDelay - time.Millisecond)
	if lit() {
		t.Fatal("divider lit before the delay")
	}
	advance(time.Millisecond)
	if !lit() {
		t.Fatal("divider did not light after the delay")
	}

	mouse(divX+10, y, tcell.ButtonNone)
	if lit() {
		t.Fatal("divider stayed lit after the pointer left")
	}
	mouse(divX, y, tcell.ButtonNone)
	lit()
	advance(ui.DividerHighlightDelay / 2)
	mouse(divX+10, y, tcell.ButtonNone)
	lit()
	advance(ui.DividerHighlightDelay)
	mouse(divX, y, tcell.ButtonNone)
	if lit() {
		t.Fatal("crossing the divider briefly lit it on return")
	}

	mouse(divX, y, tcell.Button1)
	if !lit() {
		t.Fatal("dragging did not light the divider immediately")
	}
	mouse(divX, y, tcell.ButtonNone)
	if !lit() {
		t.Fatal("releasing on the divider turned the highlight off")
	}
}
