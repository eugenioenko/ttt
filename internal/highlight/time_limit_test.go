package highlight

import (
	"testing"
	"time"

	textmate "github.com/eugenioenko/textmate-go"
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

// timeLimitedDoc reports every line as cut off by the time limit.
type timeLimitedDoc struct{ lines []string }

func (d *timeLimitedDoc) Len() int                              { return len(d.lines) }
func (d *timeLimitedDoc) SetLines(lines []string)               { d.lines = lines }
func (d *timeLimitedDoc) MaterializedLines() int                { return len(d.lines) }
func (d *timeLimitedDoc) ReplaceLines(int, int, []string) error { return nil }
func (d *timeLimitedDoc) StateAt(int) (*textmate.StateStack, bool) {
	return nil, true
}
func (d *timeLimitedDoc) Line(int) (textmate.LineResult, bool) {
	return textmate.LineResult{Stopped: true, StoppedReason: textmate.StopReasonTimeLimit}, true
}

func TestTimeLimitedLineRequestsOneRedrawPerEdit(t *testing.T) {
	wakes := recordWakes(t)
	h := New("x.go")
	h.doc = &timeLimitedDoc{}
	lines := []string{"package main"}

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

func TestSynchronousTimeLimitedLineRequestsNoRedraw(t *testing.T) {
	wakes := recordWakes(t)
	h := New("x.go")
	h.Synchronous()
	h.doc = &timeLimitedDoc{}

	h.HighlightLineAt([]string{"package main"}, 0)
	if *wakes != 0 {
		t.Fatalf("wakes = %d, want 0 for a synchronous highlighter", *wakes)
	}
}
