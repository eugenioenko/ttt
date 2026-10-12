package app

import (
	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type taskDelivery struct{ run func() }

type taskOwner struct {
	post   asynctask.Post
	tasks  []interface{ Close() }
	closed bool
}

// Create UI tasks after Init wires delivery. Without initialization, tasks run
// synchronously, even when a simulation screen is present in the E2E harness.
func NewTask[T any](a *App, options asynctask.TaskOptions) *asynctask.Task[T] {
	task := asynctask.New[T](options, a.tasks.post)
	if a.tasks.closed {
		task.Close()
	} else {
		a.tasks.tasks = append(a.tasks.tasks, task)
	}
	return task
}

func (a *App) initTaskDelivery(screen *term.TcellScreen) {
	a.tasks.post = func(run func()) error {
		return screen.PostEvent(tcell.NewEventInterrupt(&taskDelivery{run: run}))
	}
}

func (a *App) CloseTasks() {
	a.tasks.closed = true
	for _, task := range a.tasks.tasks {
		task.Close()
	}
	a.tasks.tasks = nil
}
