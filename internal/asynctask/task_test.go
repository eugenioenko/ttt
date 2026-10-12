package asynctask

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeTimer struct {
	at      time.Time
	fn      func()
	stopped bool
}

func (t *fakeTimer) Stop() bool {
	active := !t.stopped
	t.stopped = true
	return active
}

type taskRig struct {
	now    time.Time
	timers []*fakeTimer
	work   []func()
	posts  []func()
	values []int
}

func (r *taskRig) Now() time.Time { return r.now }
func (r *taskRig) AfterFunc(d time.Duration, fn func()) Timer {
	timer := &fakeTimer{at: r.now.Add(d), fn: fn}
	r.timers = append(r.timers, timer)
	return timer
}
func (r *taskRig) advance(d time.Duration) {
	r.now = r.now.Add(d)
	for _, timer := range r.timers {
		if !timer.stopped && !timer.at.After(r.now) {
			timer.stopped = true
			timer.fn()
		}
	}
}
func (r *taskRig) drain() {
	for len(r.work) > 0 {
		fn := r.work[0]
		r.work = r.work[1:]
		fn()
	}
	for len(r.posts) > 0 {
		fn := r.posts[0]
		r.posts = r.posts[1:]
		fn()
	}
}
func newTaskRig(delay, maxWait time.Duration) (*Task[int], *taskRig) {
	r := &taskRig{now: time.Unix(1, 0)}
	task := New[int](TaskOptions{Delay: delay, MaxWait: maxWait, Clock: r,
		Run: func(fn func()) { r.work = append(r.work, fn) }}, func(fn func()) error {
		r.posts = append(r.posts, fn)
		return nil
	})
	return task, r
}
func (r *taskRig) read(value int) func(context.Context) (int, error) {
	return func(context.Context) (int, error) { return value, nil }
}
func (r *taskRig) apply(value int, err error) {
	if err != nil {
		panic(err)
	}
	r.values = append(r.values, value)
}

func TestDebounceLatestAndPending(t *testing.T) {
	task, r := newTaskRig(10*time.Second, 0)
	task.Schedule(r.read(1), r.apply)
	r.advance(8 * time.Second)
	task.Schedule(r.read(2), r.apply)
	r.timers[0].fn() // A stopped timer may already be firing.
	r.advance(9 * time.Second)
	if len(r.work) != 0 || !task.Pending() {
		t.Fatal("work ran before the latest delay")
	}
	r.advance(time.Second)
	if len(r.work) != 1 || !task.Pending() {
		t.Fatal("expected one pending run")
	}
	r.work[0]()
	r.work = nil
	if len(r.values) != 0 || !task.Pending() {
		t.Fatal("delivery must remain queued and pending")
	}
	r.drain()
	if len(r.values) != 1 || r.values[0] != 2 || task.Pending() {
		t.Fatalf("values = %v, pending = %v", r.values, task.Pending())
	}
}

func TestMaxWaitAndFlushRunLatest(t *testing.T) {
	for _, flush := range []bool{false, true} {
		t.Run(map[bool]string{false: "max wait", true: "flush"}[flush], func(t *testing.T) {
			task, r := newTaskRig(10*time.Second, 25*time.Second)
			for value := 1; value <= 4; value++ {
				task.Schedule(r.read(value), r.apply)
				if value < 4 {
					r.advance(8 * time.Second)
				}
			}
			if flush {
				task.Flush()
				task.Flush()
			}
			r.advance(time.Second)
			if len(r.work) != 1 {
				t.Fatalf("runs = %d, want 1", len(r.work))
			}
			r.drain()
			task.Flush()
			task.Schedule(r.read(5), r.apply)
			r.advance(9 * time.Second)
			if len(r.work) != 0 {
				t.Fatal("new burst reused previous deadline")
			}
			r.advance(time.Second)
			r.drain()
			if len(r.values) != 2 || r.values[0] != 4 || r.values[1] != 5 {
				t.Fatalf("values = %v", r.values)
			}
		})
	}
}

func TestKeyDedupThroughCompletionAndInvalidation(t *testing.T) {
	task, r := newTaskRig(time.Second, 0)
	key := struct{ File string }{"file.go"}
	same := func() { task.ScheduleKey(key, r.read(1), r.apply) }
	same()
	same()
	if len(r.timers) != 1 {
		t.Fatal("same key restarted debounce")
	}
	r.advance(time.Second)
	same()
	r.work[0]()
	r.work = nil
	same()
	r.drain()
	same()
	if task.Pending() || len(r.values) != 1 {
		t.Fatal("same key restarted completed work")
	}
	task.Invalidate()
	same()
	task.Flush()
	r.drain()
	task.ScheduleKey(nil, r.read(2), r.apply)
	task.Flush()
	r.drain()
	task.ScheduleKey(nil, r.read(3), r.apply)
	if task.Pending() {
		t.Fatal("nil key was not retained")
	}
	task.Schedule(r.read(4), r.apply)
	task.Flush()
	r.drain()
	if len(r.values) != 4 || r.values[3] != 4 {
		t.Fatalf("values = %v", r.values)
	}
}

func TestQueuedDeliveryRejectedAfterLifecycleChange(t *testing.T) {
	for _, action := range []string{"supersede", "stop", "close"} {
		t.Run(action, func(t *testing.T) {
			task, r := newTaskRig(0, 0)
			task.ScheduleKey("key", r.read(1), r.apply)
			r.work[0]()
			r.work = nil
			switch action {
			case "supersede":
				task.ScheduleKey("new", r.read(2), r.apply)
			case "stop":
				task.Stop()
			case "close":
				task.Close()
			}
			r.posts[0]()
			r.posts = nil
			if len(r.values) != 0 {
				t.Fatal("stale queued result applied")
			}
			if action != "supersede" && task.Pending() {
				t.Fatal("cancelled task is pending")
			}
			task.ScheduleKey("key", r.read(3), r.apply)
			r.drain()
			if action == "close" {
				if len(r.values) != 0 {
					t.Fatal("closed task restarted")
				}
			} else if len(r.values) != 1 || r.values[0] != 3 {
				t.Fatalf("values = %v", r.values)
			}
		})
	}
}

func TestCancellationReachesRunningWork(t *testing.T) {
	started, exited := make(chan context.Context, 1), make(chan struct{})
	task := New[int](TaskOptions{}, func(func()) error {
		t.Error("cancelled work posted a result")
		return nil
	})
	t.Cleanup(task.Close)
	task.Schedule(func(ctx context.Context) (int, error) {
		started <- ctx
		<-ctx.Done()
		close(exited)
		return 0, ctx.Err()
	}, func(int, error) { t.Error("cancelled work applied") })
	select {
	case ctx := <-started:
		task.Stop()
		if ctx.Err() != context.Canceled || task.Pending() {
			t.Fatal("Stop did not cancel running work")
		}
	case <-time.After(time.Second):
		t.Fatal("work did not start")
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("work did not exit")
	}
}

func TestInlineErrorsPanicRecoveryAndReentrantApply(t *testing.T) {
	wantErr := errors.New("read failed")
	for _, panicWork := range []bool{false, true} {
		task := New[int](TaskOptions{Delay: time.Hour}, nil)
		applied := false
		task.Schedule(func(context.Context) (int, error) {
			if panicWork {
				panic("broken worker")
			}
			return 0, wantErr
		}, func(_ int, err error) {
			applied = true
			if task.Pending() || (!panicWork && !errors.Is(err, wantErr)) ||
				(panicWork && (err == nil || !strings.Contains(err.Error(), "broken worker"))) {
				t.Fatalf("pending = %v, error = %v", task.Pending(), err)
			}
			task.Stop()
		})
		if !applied {
			t.Fatal("nil poster did not apply inline")
		}
	}
}

func TestPostFailureAllowsRetry(t *testing.T) {
	posts := 0
	task := New[int](TaskOptions{Run: func(fn func()) { fn() }}, func(func()) error {
		posts++
		return errors.New("queue closed")
	})
	for range 2 {
		task.ScheduleKey("key", func(context.Context) (int, error) { return 1, nil },
			func(int, error) { t.Fatal("failed post applied") })
		if task.Pending() {
			t.Fatal("failed post remained pending")
		}
	}
	if posts != 2 {
		t.Fatal("failed post retained its key")
	}
}
