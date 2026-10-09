package highlight

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func goLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("var value%d = %q", i, fmt.Sprint(i))
	}
	return lines
}

// recordSteps collects the highlighters that asked for a step.
func recordSteps(t *testing.T) *[]*Highlighter {
	t.Helper()
	var scheduled []*Highlighter
	previous := scheduleStep
	SetStepScheduler(func(h *Highlighter) { scheduled = append(scheduled, h) })
	t.Cleanup(func() { scheduleStep = previous })
	return &scheduled
}

func stepUntilDone(t *testing.T, h *Highlighter) {
	t.Helper()
	for range 10_000 {
		if !h.Step(time.Hour) {
			return
		}
	}
	t.Fatal("Step never finished")
}

// wantSpans highlights lines[idx] with every line above it tokenized first.
func wantSpans(lines []string, idx int) []Span {
	h := New("x.go")
	for i := 0; i < idx; i += syncLines {
		h.HighlightLineAt(lines, i)
	}
	return h.HighlightLineAt(lines, idx)
}

func TestHighlightLineAtDefersFarLines(t *testing.T) {
	scheduled := recordSteps(t)
	lines := goLines(1000)
	h := New("x.go")

	if spans := h.HighlightLineAt(lines, 900); spans != nil {
		t.Fatalf("far line highlighted during the render: %v", spans)
	}
	if got := h.doc.MaterializedLines(); got > syncLines+1 {
		t.Fatalf("render tokenized %d lines, want at most %d", got, syncLines)
	}
	if len(*scheduled) != 1 || (*scheduled)[0] != h {
		t.Fatalf("scheduled %d steps, want one for this highlighter", len(*scheduled))
	}

	stepUntilDone(t, h)

	if got, want := h.HighlightLineAt(lines, 900), wantSpans(lines, 900); !reflect.DeepEqual(got, want) {
		t.Fatalf("spans after catching up = %v, want %v", got, want)
	}
}

func TestHighlightLineAtNearLinesStaySynchronous(t *testing.T) {
	scheduled := recordSteps(t)
	lines := goLines(1000)
	h := New("x.go")

	if spans := h.HighlightLineAt(lines, syncLines-1); spans == nil {
		t.Fatal("line within the synchronous range was left uncolored")
	}
	if len(*scheduled) != 0 {
		t.Fatalf("scheduled %d steps for a near line, want 0", len(*scheduled))
	}
}

// Typing never defers: lines around an edit are within the synchronous range.
func TestHighlightLineAtTypingStaysSynchronous(t *testing.T) {
	scheduled := recordSteps(t)
	lines := goLines(1000)
	h := New("x.go")
	h.HighlightLineAt(lines, 500)
	stepUntilDone(t, h)
	*scheduled = nil

	edited := append([]string(nil), lines...)
	edited[490] = `var value490 = "edited"`
	h.ClearCache()
	for idx := 480; idx < 530; idx++ {
		if spans := h.HighlightLineAt(edited, idx); spans == nil {
			t.Fatalf("line %d uncolored after an edit at line 490", idx)
		}
	}
	if len(*scheduled) != 0 {
		t.Fatalf("scheduled %d steps while typing, want 0", len(*scheduled))
	}
}

// An edit far above reconverges with the old tail within a few lines, so a
// line below it stays synchronous too.
func TestHighlightLineAtEditAboveReconverges(t *testing.T) {
	scheduled := recordSteps(t)
	lines := goLines(1000)
	h := New("x.go")
	h.HighlightLineAt(lines, 900)
	stepUntilDone(t, h)
	*scheduled = nil

	edited := append([]string(nil), lines...)
	edited[0] = `var value0 = "edited"`
	h.ClearCache()
	if spans := h.HighlightLineAt(edited, 900); spans == nil {
		t.Fatal("line below a reconverged edit was left uncolored")
	}
	if len(*scheduled) != 0 {
		t.Fatalf("scheduled %d steps after a converging edit, want 0", len(*scheduled))
	}
}

// When an edit does not reconverge, a far line keeps its previous colors
// until Step catches up, rather than flashing uncolored.
func TestHighlightLineAtKeepsLastSpansWhileCatchingUp(t *testing.T) {
	scheduled := recordSteps(t)
	lines := goLines(1000)
	h := New("x.go")
	h.HighlightLineAt(lines, 900)
	stepUntilDone(t, h)
	before := h.HighlightLineAt(lines, 900)
	*scheduled = nil

	edited := append([]string(nil), lines...)
	edited[0] = "/* an unclosed comment changes every following line"
	h.ClearCache()
	if got := h.HighlightLineAt(edited, 900); !reflect.DeepEqual(got, before) {
		t.Fatalf("spans while catching up = %v, want the previous %v", got, before)
	}
	if len(*scheduled) != 1 {
		t.Fatalf("scheduled %d steps, want 1", len(*scheduled))
	}

	stepUntilDone(t, h)
	if got, want := h.HighlightLineAt(edited, 900), wantSpans(edited, 900); !reflect.DeepEqual(got, want) {
		t.Fatalf("spans after catching up = %v, want %v", got, want)
	}
}

func TestHighlightLineAtSchedulesOneStepChain(t *testing.T) {
	scheduled := recordSteps(t)
	lines := goLines(2000)
	h := New("x.go")

	h.HighlightLineAt(lines, 900)
	h.HighlightLineAt(lines, 1900)
	if len(*scheduled) != 1 {
		t.Fatalf("scheduled %d steps, want one chain for both lines", len(*scheduled))
	}
	stepUntilDone(t, h)
	if got := h.doc.MaterializedLines(); got <= 1900 {
		t.Fatalf("steps stopped at %d lines, want past 1900", got)
	}

	h.HighlightLineAt(lines, 1950)
	if len(*scheduled) != 1 {
		t.Fatalf("scheduled another step for a line that is ready, want none")
	}
}

func TestStepRespectsBudget(t *testing.T) {
	recordSteps(t)
	lines := goLines(5000)
	h := New("x.go")
	h.HighlightLineAt(lines, 4900)
	before := h.doc.MaterializedLines()

	if !h.Step(0) {
		t.Fatal("a zero budget finished 5,000 lines")
	}
	if got := h.doc.MaterializedLines() - before; got > 2 {
		t.Fatalf("a zero budget tokenized %d lines, want at most one step", got)
	}
}
