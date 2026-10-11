package highlight

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
)

func commentedFile(n int) []string {
	lines := []string{"package main", "/*"}
	for i := 0; i < n; i++ {
		lines = append(lines, fmt.Sprintf("x%d := %d", i, i))
	}
	return append(lines, "*/")
}

func waitNotified(t *testing.T, ch chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("progressive highlighting never reported")
	}
}

func TestProgressiveHighlightSettlesDeepLines(t *testing.T) {
	lines := commentedFile(600)
	deep := 500
	exact := New("main.go").HighlightLineAt(lines, deep)
	if styleAt(exact, 0) != term.StyleSyntaxComment {
		t.Fatal("fixture line is not inside the block comment")
	}

	h := New("main.go")
	notified := make(chan struct{}, 8)
	h.SetProgressive(func() { notified <- struct{}{} })
	h.BeginPass()
	first := h.HighlightLineAt(lines, deep)
	h.EndPass()
	if want := New("main.go").HighlightLine(lines[deep]); !reflect.DeepEqual(first, want) {
		t.Fatalf("deep line before settling = %v, want single-line spans %v", first, want)
	}
	waitNotified(t, notified)
	h.BeginPass()
	if got := h.HighlightLineAt(lines, deep); !reflect.DeepEqual(got, exact) {
		t.Fatalf("deep line after settling = %v, want %v", got, exact)
	}
	if got := h.HighlightLineAt(lines, 3); !reflect.DeepEqual(got, New("main.go").HighlightLineAt(lines, 3)) {
		t.Fatalf("near line = %v", got)
	}
}

func TestProgressiveHighlightNeverServesStaleState(t *testing.T) {
	lines := commentedFile(600)
	deep := 500
	h := New("main.go")
	notified := make(chan struct{}, 8)
	h.SetProgressive(func() { notified <- struct{}{} })
	h.BeginPass()
	h.HighlightLineAt(lines, deep)
	h.EndPass()
	waitNotified(t, notified)

	edited := append([]string(nil), lines...)
	edited[1] = "// no longer a block comment"
	h.ClearCache()
	h.BeginPass()
	got := h.HighlightLineAt(edited, deep)
	h.EndPass()
	exact := New("main.go").HighlightLineAt(edited, deep)
	single := New("main.go").HighlightLine(edited[deep])
	if !reflect.DeepEqual(got, exact) && !reflect.DeepEqual(got, single) {
		t.Fatalf("after closing the comment above, deep line = %v; want %v or %v", got, exact, single)
	}
	if styleAt(got, 0) == term.StyleSyntaxComment {
		t.Fatalf("deep line kept the removed block comment's state")
	}
	waitNotified(t, notified)
	h.BeginPass()
	if got := h.HighlightLineAt(edited, deep); !reflect.DeepEqual(got, exact) {
		t.Fatalf("after settling the edit, deep line = %v, want %v", got, exact)
	}
}

func TestProgressiveHighlightOffIsSynchronous(t *testing.T) {
	lines := commentedFile(600)
	h := New("main.go")
	h.SetProgressive(func() {})
	h.SetProgressive(nil)
	if got, want := h.HighlightLineAt(lines, 500), New("main.go").HighlightLineAt(lines, 500); !reflect.DeepEqual(got, want) {
		t.Fatalf("with progressive mode off, deep line = %v, want %v", got, want)
	}
}

func TestHeldWarmersWaitForTheFrame(t *testing.T) {
	lines := commentedFile(600)
	h := New("main.go")
	notified := make(chan struct{}, 8)
	h.SetProgressive(func() { notified <- struct{}{} })
	release := HoldWarmers()
	h.BeginPass()
	h.HighlightLineAt(lines, 500)
	h.EndPass()
	select {
	case <-notified:
		release()
		t.Fatal("a held worker tokenized during the frame")
	case <-time.After(50 * time.Millisecond):
	}
	release()
	waitNotified(t, notified)
}

func TestProgressivePassRequestsOneRedraw(t *testing.T) {
	lines := commentedFile(2000)
	h := New("main.go")
	var calls atomic.Int32
	notified := make(chan struct{}, 8)
	h.SetProgressive(func() {
		calls.Add(1)
		notified <- struct{}{}
	})
	h.BeginPass()
	for _, deep := range []int{900, 1200, 1500, 1800} {
		h.HighlightLineAt(lines, deep)
	}
	h.EndPass()
	waitNotified(t, notified)
	time.Sleep(50 * time.Millisecond)
	if n := calls.Load(); n != 1 {
		t.Fatalf("one pass deferring several deep lines requested %d redraws, want 1", n)
	}
}

func TestTurningProgressiveOffStopsTheWorker(t *testing.T) {
	waitIdle := func(limit time.Duration) time.Duration {
		start := time.Now()
		for Warming() != 0 {
			if time.Since(start) > limit {
				t.Fatalf("%d background workers still running after %v", Warming(), limit)
			}
			time.Sleep(time.Millisecond)
		}
		return time.Since(start)
	}
	waitIdle(30 * time.Second)

	lines := commentedFile(200000)
	full := New("main.go")
	full.SetProgressive(func() {})
	full.BeginPass()
	full.HighlightLineAt(lines, len(lines)-1)
	full.EndPass()
	settle := waitIdle(60 * time.Second)

	h := New("main.go")
	h.SetProgressive(func() {})
	release := HoldWarmers()
	h.BeginPass()
	h.HighlightLineAt(lines, len(lines)-1)
	h.EndPass()
	if Warming() == 0 {
		release()
		t.Fatal("no background worker started for a deep line")
	}
	h.SetProgressive(nil)
	release()
	if stop := waitIdle(30 * time.Second); stop > settle/4 {
		t.Fatalf("worker took %v to stop after progressive mode was turned off; tokenizing the whole file takes %v", stop, settle)
	}
}
