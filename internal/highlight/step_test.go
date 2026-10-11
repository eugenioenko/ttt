package highlight

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
)

func goLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("var value%d = %q", i, fmt.Sprint(i))
	}
	return lines
}

func commentedFile(n int) []string {
	lines := []string{"package main", "/*"}
	for i := 0; i < n; i++ {
		lines = append(lines, fmt.Sprintf("x%d := %d", i, i))
	}
	return append(lines, "*/")
}

// recordWakes installs a scheduler that counts wake-ups, and leaves nothing
// pending for the next test.
func recordWakes(t testing.TB) *int {
	t.Helper()
	wakes := 0
	previous := wake
	SetStepScheduler(func() { wakes++ })
	t.Cleanup(func() {
		for h := range pending {
			h.Close()
		}
		wake = previous
	})
	return &wakes
}

func stepUntilDone(t testing.TB) {
	t.Helper()
	for range 10_000 {
		if !Step(time.Hour) {
			return
		}
	}
	t.Fatal("Step never finished")
}

// wantSpans highlights lines[idx] synchronously, with every line above it.
func wantSpans(lines []string, idx int) []Span {
	previous := wake
	wake = nil
	defer func() { wake = previous }()
	return New("x.go").HighlightLineAt(lines, idx)
}

func TestHighlightLineAtDefersFarLines(t *testing.T) {
	wakes := recordWakes(t)
	lines := goLines(1000)
	h := New("x.go")

	if got, want := h.HighlightLineAt(lines, 900), New("x.go").HighlightLine(lines[900]); !reflect.DeepEqual(got, want) {
		t.Fatalf("far line during the render = %v, want the line highlighted on its own %v", got, want)
	}
	if got := h.doc.MaterializedLines(); got > DefaultSyncLines+1 {
		t.Fatalf("render tokenized %d lines, want at most %d", got, DefaultSyncLines)
	}
	if *wakes != 1 || Pending() != 1 {
		t.Fatalf("wakes = %d, pending = %d, want one each", *wakes, Pending())
	}

	stepUntilDone(t)

	if Pending() != 0 {
		t.Fatalf("pending after catching up = %d", Pending())
	}
	if got, want := h.HighlightLineAt(lines, 900), wantSpans(lines, 900); !reflect.DeepEqual(got, want) {
		t.Fatalf("spans after catching up = %v, want %v", got, want)
	}
}

func TestDeferredLineSettlesToSynchronousOutput(t *testing.T) {
	recordWakes(t)
	lines := commentedFile(600)
	deep := 500
	exact := wantSpans(lines, deep)
	if styleAt(exact, 0) != term.StyleSyntaxComment {
		t.Fatal("fixture line is not inside the block comment")
	}

	h := New("main.go")
	if first := h.HighlightLineAt(lines, deep); styleAt(first, 0) == term.StyleSyntaxComment {
		t.Fatal("fixture: the first answer should be the line on its own, outside the comment")
	}
	stepUntilDone(t)
	if got := h.HighlightLineAt(lines, deep); !reflect.DeepEqual(got, exact) {
		t.Fatalf("deep line after settling = %v, want %v", got, exact)
	}
}

func TestHighlightLineAtNearLinesStaySynchronous(t *testing.T) {
	wakes := recordWakes(t)
	lines := goLines(1000)
	h := New("x.go")

	if got, want := h.HighlightLineAt(lines, DefaultSyncLines-1), wantSpans(lines, DefaultSyncLines-1); !reflect.DeepEqual(got, want) {
		t.Fatalf("near line = %v, want %v", got, want)
	}
	if *wakes != 0 {
		t.Fatalf("woke the scheduler %d times for a near line, want 0", *wakes)
	}
}

func TestHighlightLineAtWithoutSyncLinesDefersEveryUnreachedLine(t *testing.T) {
	wakes := recordWakes(t)
	lines := commentedFile(100)
	h := New("x.go")
	h.SetSyncLines(0)

	h.HighlightLineAt(lines, 0)
	if *wakes != 0 || h.doc.MaterializedLines() != 2 {
		t.Fatalf("first line: wakes = %d, materialized = %d; want 0 and 2", *wakes, h.doc.MaterializedLines())
	}
	if got, want := h.HighlightLineAt(lines, 50), h.HighlightLine(lines[50]); !reflect.DeepEqual(got, want) {
		t.Fatalf("unreached line = %v, want its single-line spans %v", got, want)
	}
	if *wakes != 1 || h.doc.MaterializedLines() != 2 {
		t.Fatalf("unreached line: wakes = %d, materialized = %d; want 1 and 2", *wakes, h.doc.MaterializedLines())
	}
	stepUntilDone(t)
	if got, want := h.HighlightLineAt(lines, 50), wantSpans(lines, 50); !reflect.DeepEqual(got, want) {
		t.Fatalf("after steps line = %v, want %v", got, want)
	}
}

func TestStepWorksOnlyTowardLinesRenderedSinceAnEdit(t *testing.T) {
	recordWakes(t)
	lines := goLines(5000)
	h := New("x.go")
	h.HighlightLineAt(lines, 4000)

	edited := append([]string(nil), lines...)
	edited[1] = "var edited = 1"
	h.ClearCache()
	h.HighlightLineAt(edited, 10)
	stepUntilDone(t)
	if got := h.doc.MaterializedLines(); got > 1000 {
		t.Fatalf("steps materialized %d lines after the edit; only line 10 was rendered since", got)
	}
}

// Typing never defers: lines around an edit are within the synchronous range.
func TestHighlightLineAtTypingStaysSynchronous(t *testing.T) {
	wakes := recordWakes(t)
	lines := goLines(1000)
	h := New("x.go")
	h.HighlightLineAt(lines, 500)
	stepUntilDone(t)
	*wakes = 0

	edited := append([]string(nil), lines...)
	edited[490] = `var value490 = "edited"`
	h.ClearCache()
	for idx := 480; idx < 530; idx++ {
		if got, want := h.HighlightLineAt(edited, idx), wantSpans(edited, idx); !reflect.DeepEqual(got, want) {
			t.Fatalf("line %d after an edit at line 490 = %v, want %v", idx, got, want)
		}
	}
	if *wakes != 0 {
		t.Fatalf("woke the scheduler %d times while typing, want 0", *wakes)
	}
}

// An edit far above reconverges with the old tail within a few lines, so a
// line below it stays synchronous too.
func TestHighlightLineAtEditAboveReconverges(t *testing.T) {
	wakes := recordWakes(t)
	lines := goLines(1000)
	h := New("x.go")
	h.HighlightLineAt(lines, 900)
	stepUntilDone(t)
	*wakes = 0

	edited := append([]string(nil), lines...)
	edited[0] = `var value0 = "edited"`
	h.ClearCache()
	if got, want := h.HighlightLineAt(edited, 900), wantSpans(edited, 900); !reflect.DeepEqual(got, want) {
		t.Fatalf("line below a reconverged edit = %v, want %v", got, want)
	}
	if *wakes != 0 {
		t.Fatalf("woke the scheduler %d times after a converging edit, want 0", *wakes)
	}
}

// When an edit does not reconverge, a far line keeps its previous colors
// until Step catches up, rather than changing twice.
func TestHighlightLineAtKeepsLastSpansWhileCatchingUp(t *testing.T) {
	wakes := recordWakes(t)
	lines := goLines(1000)
	h := New("x.go")
	h.HighlightLineAt(lines, 900)
	stepUntilDone(t)
	before := h.HighlightLineAt(lines, 900)
	*wakes = 0

	edited := append([]string(nil), lines...)
	edited[0] = "/* an unclosed comment changes every following line"
	h.ClearCache()
	if got := h.HighlightLineAt(edited, 900); !reflect.DeepEqual(got, before) {
		t.Fatalf("spans while catching up = %v, want the previous %v", got, before)
	}
	if *wakes != 1 {
		t.Fatalf("woke the scheduler %d times, want 1", *wakes)
	}

	stepUntilDone(t)
	if got, want := h.HighlightLineAt(edited, 900), wantSpans(edited, 900); !reflect.DeepEqual(got, want) {
		t.Fatalf("spans after catching up = %v, want %v", got, want)
	}
}

func TestHighlightLineAtSchedulesOneStepChain(t *testing.T) {
	wakes := recordWakes(t)
	lines := goLines(2000)
	h := New("x.go")

	h.HighlightLineAt(lines, 900)
	h.HighlightLineAt(lines, 1900)
	if *wakes != 1 {
		t.Fatalf("woke the scheduler %d times, want one chain for both lines", *wakes)
	}
	stepUntilDone(t)
	if got := h.doc.MaterializedLines(); got <= 1900 {
		t.Fatalf("steps stopped at %d lines, want past 1900", got)
	}

	h.HighlightLineAt(lines, 1950)
	if *wakes != 1 {
		t.Fatalf("woke the scheduler again for a line that is ready")
	}
}

func TestStepRespectsBudget(t *testing.T) {
	recordWakes(t)
	lines := goLines(5000)
	h := New("x.go")
	h.HighlightLineAt(lines, 4900)
	before := h.doc.MaterializedLines()

	if !Step(0) {
		t.Fatal("a zero budget finished 5,000 lines")
	}
	if got := h.doc.MaterializedLines() - before; got > 2 {
		t.Fatalf("a zero budget tokenized %d lines, want at most one", got)
	}
}

func TestCloseDropsDeferredWork(t *testing.T) {
	recordWakes(t)
	h := New("x.go")
	h.HighlightLineAt(goLines(1000), 900)
	if Pending() != 1 {
		t.Fatalf("fixture: pending = %d, want 1", Pending())
	}
	h.Close()
	if Pending() != 0 {
		t.Fatalf("pending after Close = %d, want 0", Pending())
	}
	if Step(time.Hour) {
		t.Fatal("Step reported work left after the only highlighter closed")
	}
}

func TestSynchronousHighlighterNeverDefers(t *testing.T) {
	wakes := recordWakes(t)
	lines := commentedFile(600)
	h := New("main.go")
	h.Synchronous()
	if got, want := h.HighlightLineAt(lines, 500), wantSpans(lines, 500); !reflect.DeepEqual(got, want) {
		t.Fatalf("synchronous deep line = %v, want %v", got, want)
	}
	if *wakes != 0 || Pending() != 0 {
		t.Fatalf("a synchronous highlighter deferred work (wakes %d, pending %d)", *wakes, Pending())
	}
}

func TestNoSchedulerMeansSynchronous(t *testing.T) {
	previous := wake
	wake = nil
	t.Cleanup(func() { wake = previous })
	lines := commentedFile(600)
	if got, want := New("main.go").HighlightLineAt(lines, 500), wantSpans(lines, 500); !reflect.DeepEqual(got, want) {
		t.Fatalf("without a scheduler, deep line = %v, want %v", got, want)
	}
	if Pending() != 0 {
		t.Fatalf("pending without a scheduler = %d", Pending())
	}
}
