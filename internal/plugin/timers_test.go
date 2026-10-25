package plugin

import (
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// timerHarness simulates the main loop: posted async results are executed
// in order on the test goroutine.
type timerHarness struct {
	p       *Plugin
	results chan *PluginAsyncResult
}

func newTimerHarness(t *testing.T) *timerHarness {
	t.Helper()
	h := &timerHarness{results: make(chan *PluginAsyncResult, 64)}
	p := &Plugin{Name: "timer-test"}
	p.State = NewSandbox()
	t.Cleanup(func() {
		if p.State != nil {
			p.State.Close()
		}
	})
	p.PostAsync = func(r *PluginAsyncResult) { h.results <- r }
	setupTTTModule(p.State, p)
	h.p = p
	return h
}

// runOne waits for one posted callback and executes it, like the event loop.
func (h *timerHarness) runOne(t *testing.T, timeout time.Duration) bool {
	t.Helper()
	select {
	case r := <-h.results:
		if r.Callback != nil {
			r.Callback()
		}
		return true
	case <-time.After(timeout):
		return false
	}
}

func (h *timerHarness) luaGlobal(name string) lua.LValue {
	return h.p.State.GetGlobal(name)
}

func TestSetTimeoutFiresOnce(t *testing.T) {
	h := newTimerHarness(t)
	err := h.p.State.DoString(`
		local ttt = require("ttt")
		fired = 0
		ttt.set_timeout(10, function() fired = fired + 1 end)
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !h.runOne(t, 2*time.Second) {
		t.Fatal("timeout callback never posted")
	}
	if got := h.luaGlobal("fired").String(); got != "1" {
		t.Errorf("expected fired=1, got %s", got)
	}
	if h.runOne(t, 150*time.Millisecond) {
		t.Error("set_timeout fired more than once")
	}
}

func TestSetIntervalRepeatsUntilCleared(t *testing.T) {
	h := newTimerHarness(t)
	err := h.p.State.DoString(`
		local ttt = require("ttt")
		ticks = 0
		timer_id = ttt.set_interval(10, function() ticks = ticks + 1 end)
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 2; i++ {
		if !h.runOne(t, 2*time.Second) {
			t.Fatalf("interval tick %d never posted", i+1)
		}
	}
	if got := h.luaGlobal("ticks").String(); got != "2" {
		t.Errorf("expected ticks=2, got %s", got)
	}

	if err := h.p.State.DoString(`require("ttt").clear_interval(timer_id)`); err != nil {
		t.Fatalf("clear_interval: %v", err)
	}
	// Drain anything posted before the clear took effect, then expect silence.
	for h.runOne(t, 120*time.Millisecond) {
	}
	if h.runOne(t, 150*time.Millisecond) {
		t.Error("interval still ticking after clear_interval")
	}
}

func TestClearTimeoutCancels(t *testing.T) {
	h := newTimerHarness(t)
	err := h.p.State.DoString(`
		local ttt = require("ttt")
		fired = 0
		local id = ttt.set_timeout(50, function() fired = fired + 1 end)
		ttt.clear_timeout(id)
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.runOne(t, 200*time.Millisecond) {
		t.Error("cleared timeout still fired")
	}
	if got := h.luaGlobal("fired").String(); got != "0" {
		t.Errorf("expected fired=0, got %s", got)
	}
}

func TestDestroyStopsTimers(t *testing.T) {
	h := newTimerHarness(t)
	err := h.p.State.DoString(`
		local ttt = require("ttt")
		ttt.set_interval(10, function() end)
		ttt.set_timeout(30, function() end)
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	h.p.Destroy()

	// PostAsync is nilled by Destroy; nothing should arrive.
	if h.runOne(t, 150*time.Millisecond) {
		t.Error("timer posted after Destroy")
	}
	if h.p.timers != nil {
		t.Error("expected timers map cleared on Destroy")
	}
}

func TestTimerIDsAreDistinct(t *testing.T) {
	h := newTimerHarness(t)
	err := h.p.State.DoString(`
		local ttt = require("ttt")
		a = ttt.set_timeout(1000, function() end)
		b = ttt.set_interval(1000, function() end)
		c = ttt.set_timeout(1000, function() end)
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, b, c := h.luaGlobal("a").String(), h.luaGlobal("b").String(), h.luaGlobal("c").String()
	if a == b || b == c || a == c {
		t.Errorf("expected distinct ids, got %s %s %s", a, b, c)
	}
}

func TestDebounceRunsLastCallOnce(t *testing.T) {
	h := newTimerHarness(t)
	err := h.p.State.DoString(`
		local ttt = require("ttt")
		calls, last = 0, nil
		run, cancel_run = ttt.debounce(20, function(v) calls = calls + 1; last = v end)
		run("a"); run("b"); run("c")
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !h.runOne(t, 2*time.Second) {
		t.Fatal("debounced callback never posted")
	}
	if h.luaGlobal("calls").String() != "1" || h.luaGlobal("last").String() != "c" {
		t.Fatalf("calls=%s last=%s, want 1 call with the last argument", h.luaGlobal("calls"), h.luaGlobal("last"))
	}
	if h.runOne(t, 100*time.Millisecond) {
		t.Error("debounce fired more than once")
	}
}

func TestDebounceDropsQueuedStaleCallAndCancels(t *testing.T) {
	h := newTimerHarness(t)
	if err := h.p.State.DoString(`
		local ttt = require("ttt")
		last = nil
		run, cancel_run = ttt.debounce(1, function(v) last = v end)
		run("stale")
	`); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if err := h.p.State.DoString(`run("fresh")`); err != nil {
		t.Fatal(err)
	}
	h.runOne(t, time.Second)
	if got := h.luaGlobal("last"); got != lua.LNil {
		t.Fatalf("stale call ran: last=%s", got)
	}
	h.runOne(t, time.Second)
	if got := h.luaGlobal("last").String(); got != "fresh" {
		t.Fatalf("last=%s, want fresh", got)
	}
	if err := h.p.State.DoString(`run("cancelled"); cancel_run()`); err != nil {
		t.Fatal(err)
	}
	for h.runOne(t, 100*time.Millisecond) {
	}
	if got := h.luaGlobal("last").String(); got != "fresh" {
		t.Fatalf("cancelled call ran: last=%s", got)
	}
}
