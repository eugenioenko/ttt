package app

import (
	"context"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/view"
)

func TestTaskEventLoopDeliveryAndShutdown(t *testing.T) {
	a := prepareListenTestApp(t)
	work := make(chan func(), 1)
	task := NewTask[string](a, asynctask.TaskOptions{Run: func(fn func()) { work <- fn }})
	startListenTestEventLoop(t, a)
	var pending *asynctask.Task[int]
	if err := runExecMain(a, func() error {
		task.Schedule(func(context.Context) (string, error) { return "task delivered", nil },
			func(text string, err error) {
				if err != nil {
					t.Error(err)
				}
				a.Status.SetSegment(view.StatusSegment{ID: "task", Text: text})
			})
		pending = NewTask[int](a, asynctask.TaskOptions{Delay: time.Hour})
		pending.Schedule(func(context.Context) (int, error) {
			t.Error("shutdown did not stop delayed work")
			return 0, nil
		}, func(int, error) { t.Error("shutdown task applied") })
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case fn := <-work:
		fn()
	case <-time.After(time.Second):
		t.Fatal("task created before initialization stayed synchronous")
	}
	if _, err := RunExecScript(a, `wait-for "task delivered" timeout=2000`); err != nil {
		t.Fatal(err)
	}
	if err := StopExecLoop(a); err != nil {
		t.Fatal(err)
	}
	<-a.eventLoopDoneSignal()
	if pending.Pending() {
		t.Fatal("shutdown left task pending")
	}
	late := NewTask[int](a, asynctask.TaskOptions{})
	late.Schedule(func(context.Context) (int, error) {
		t.Error("task started after shutdown")
		return 0, nil
	}, func(int, error) { t.Error("late task applied") })
}
