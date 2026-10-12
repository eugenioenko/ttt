package asynctask

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
)

type Timer interface{ Stop() bool }

type Clock interface {
	Now() time.Time
	// AfterFunc must not call fn inline.
	AfterFunc(time.Duration, func()) Timer
}

type realClock struct{}

func (realClock) Now() time.Time                             { return time.Now() }
func (realClock) AfterFunc(d time.Duration, fn func()) Timer { return time.AfterFunc(d, fn) }

// Post must enqueue delivery on the owning goroutine. A nil Post runs work and
// delivery inline, ignoring delays, for headless and deterministic test use.
type Post func(func()) error

type TaskOptions struct {
	Delay time.Duration
	// MaxWait limits debounce; queued refreshes still wait for active delivery.
	MaxWait time.Duration
	Clock   Clock
	// KeepRunning applies the active refresh before starting the latest queued
	// request. Use it for refreshes of the same resource; Stop on identity changes.
	KeepRunning bool
	// Run must execute work outside the owning goroutine in production.
	Run func(func())
}

type request[T any] struct {
	ctx    context.Context
	cancel context.CancelFunc
	work   func(context.Context) (T, error)
	apply  func(T, error)
	due    time.Time
}

// Scheduling, lifecycle methods, and posted delivery belong to the same owning
// goroutine. Pending is safe to call from any goroutine. Work must use snapshots
// of UI state, since it runs on the executor, not on the owning goroutine.
type Task[T any] struct {
	mu       sync.Mutex
	options  TaskOptions
	post     Post
	current  *request[T]
	next     *request[T]
	timer    Timer
	waiting  bool
	deadline time.Time
	key      any
	hasKey   bool
	closed   bool
}

func New[T any](options TaskOptions, post Post) *Task[T] {
	if options.Clock == nil {
		options.Clock = realClock{}
	}
	if options.Run == nil {
		options.Run = func(fn func()) { go fn() }
	}
	return &Task[T]{options: options, post: post}
}

// SetPost cancels outstanding requests and forgets the key before rebinding
// delivery. Previously posted callbacks cannot apply through the old poster.
func (t *Task[T]) SetPost(post Post) {
	t.mu.Lock()
	t.stopLocked()
	t.key, t.hasKey, t.post = nil, false, post
	t.mu.Unlock()
}

func (t *Task[T]) Schedule(work func(context.Context) (T, error), apply func(T, error)) {
	t.schedule(nil, false, work, apply)
}

// ScheduleKey retains the key after completion, including errors. Invalidate
// permits a retry; Stop also clears the key. Keys must be comparable, including
// any values held in interface fields.
func (t *Task[T]) ScheduleKey(key any, work func(context.Context) (T, error), apply func(T, error)) {
	if key != nil && !reflect.ValueOf(key).Comparable() {
		panic("asynctask: key must be comparable")
	}
	t.schedule(key, true, work, apply)
}

func (t *Task[T]) schedule(key any, keyed bool, work func(context.Context) (T, error), apply func(T, error)) {
	t.mu.Lock()
	if t.closed || (keyed && t.hasKey && t.key == key) {
		t.mu.Unlock()
		return
	}
	deadline := t.deadline
	ctx, cancel := context.WithCancel(context.Background())
	r := &request[T]{ctx: ctx, cancel: cancel, work: work, apply: apply}
	if t.options.KeepRunning && t.current != nil && !t.waiting {
		if t.next != nil {
			t.next.cancel()
		}
		t.next = r
	} else {
		t.stopLocked()
		t.current, t.waiting = r, true
	}
	t.key, t.hasKey = key, keyed
	if t.post != nil && t.options.Delay > 0 {
		now := t.options.Clock.Now()
		r.due = now.Add(t.options.Delay)
		if t.options.MaxWait > 0 {
			if deadline.IsZero() {
				deadline = now.Add(t.options.MaxWait)
			}
			t.deadline = deadline
			if r.due.After(deadline) {
				r.due = deadline
			}
		}
	}
	t.mu.Unlock()
	t.arm(r)
}

func (t *Task[T]) arm(r *request[T]) {
	t.mu.Lock()
	if r == nil || t.current != r || !t.waiting || t.timer != nil {
		t.mu.Unlock()
		return
	}
	var delay time.Duration
	if !r.due.IsZero() {
		delay = r.due.Sub(t.options.Clock.Now())
	}
	if delay > 0 {
		t.timer = t.options.Clock.AfterFunc(delay, func() { t.start(r) })
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()
	t.start(r)
}

func (t *Task[T]) start(r *request[T]) {
	t.mu.Lock()
	if t.current != r || !t.waiting {
		t.mu.Unlock()
		return
	}
	if t.timer != nil {
		t.timer.Stop()
	}
	t.timer, t.waiting, t.deadline = nil, false, time.Time{}
	post := t.post
	t.mu.Unlock()
	run := func() {
		if r.ctx.Err() != nil {
			return
		}
		value, err := invoke(r.ctx, r.work)
		t.mu.Lock()
		current := t.current == r
		t.mu.Unlock()
		if !current {
			return
		}
		deliver := func() {
			t.mu.Lock()
			if t.current != r {
				t.mu.Unlock()
				return
			}
			next := t.finishLocked(r)
			t.mu.Unlock()
			r.apply(value, err)
			t.arm(next)
		}
		if post == nil {
			deliver()
		} else if err := post(deliver); err != nil {
			var next *request[T]
			t.mu.Lock()
			if t.current == r {
				next = t.finishLocked(r)
				// Failed delivery must permit retrying the same input.
				if next == nil {
					t.key, t.hasKey = nil, false
				}
			}
			t.mu.Unlock()
			t.arm(next)
		}
	}
	if post == nil {
		run()
	} else {
		t.options.Run(run)
	}
}

func invoke[T any](ctx context.Context, work func(context.Context) (T, error)) (value T, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("async task panic: %v", recovered)
		}
	}()
	return work(ctx)
}

func (t *Task[T]) Flush() {
	t.mu.Lock()
	r := t.current
	waiting := t.waiting
	if waiting {
		r.due = time.Time{}
	}
	if t.next != nil {
		t.next.due = time.Time{}
	}
	t.mu.Unlock()
	if waiting {
		t.start(r)
	}
}

// Invalidate forgets the key without cancelling current work.
func (t *Task[T]) Invalidate() {
	t.mu.Lock()
	t.key, t.hasKey = nil, false
	t.mu.Unlock()
}

func (t *Task[T]) Stop() {
	t.mu.Lock()
	t.stopLocked()
	t.key, t.hasKey = nil, false
	t.mu.Unlock()
}

func (t *Task[T]) Close() {
	t.mu.Lock()
	t.closed = true
	t.stopLocked()
	t.key, t.hasKey = nil, false
	t.mu.Unlock()
}

func (t *Task[T]) Pending() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.current != nil
}

func (t *Task[T]) finishLocked(r *request[T]) *request[T] {
	r.cancel()
	t.current, t.next = t.next, nil
	t.waiting = t.current != nil
	if t.current == nil {
		t.deadline = time.Time{}
	}
	return t.current
}

func (t *Task[T]) stopLocked() {
	if t.timer != nil {
		t.timer.Stop()
	}
	if t.current != nil {
		t.current.cancel()
	}
	if t.next != nil {
		t.next.cancel()
	}
	t.current, t.next, t.timer, t.waiting, t.deadline = nil, nil, nil, false, time.Time{}
}
