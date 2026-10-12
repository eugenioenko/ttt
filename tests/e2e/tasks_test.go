package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/view"
)

func TestTaskAppliesSynchronouslyWithSimulationScreen(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	task := app.NewTask[string](h.app, asynctask.TaskOptions{Delay: time.Hour})
	task.Schedule(func(context.Context) (string, error) { return "task delivered", nil },
		func(text string, err error) {
			if err != nil {
				t.Fatal(err)
			}
			h.app.Status.SetSegment(view.StatusSegment{ID: "task", Text: text})
		})
	h.redraw()
	h.assertContains("task delivered")
	if task.Pending() {
		t.Fatal("synchronous task remained pending")
	}
}
