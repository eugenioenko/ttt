package highlight

import (
	"testing"
	"time"
)

func TestSetTimeLimit(t *testing.T) {
	t.Cleanup(func() { SetTimeLimit(0) })
	SetTimeLimit(2 * time.Second)
	if tokenizeOptions.TimeLimit != 2*time.Second {
		t.Fatalf("time limit = %v, want 2s", tokenizeOptions.TimeLimit)
	}
	SetTimeLimit(0)
	if tokenizeOptions.TimeLimit != tokenizeTimeLimit {
		t.Fatalf("time limit after reset = %v, want the default %v", tokenizeOptions.TimeLimit, tokenizeTimeLimit)
	}
}

func TestTimeLimitedLineRequestsOneRedrawPerEdit(t *testing.T) {
	wakes := recordWakes(t)
	t.Cleanup(func() { SetTimeLimit(0) })
	SetTimeLimit(time.Nanosecond)
	h := New("x.go")
	lines := []string{`package main; import "fmt"; func main() { fmt.Println("hi") }`}

	h.HighlightLineAt(lines, 0)
	h.HighlightLineAt(lines, 0)
	if *wakes != 1 {
		t.Fatalf("wakes after two frames = %d, want 1", *wakes)
	}

	h.ClearCache()
	h.HighlightLineAt(lines, 0)
	if *wakes != 2 {
		t.Fatalf("wakes after an edit = %d, want 2", *wakes)
	}
}
