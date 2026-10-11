package highlight

import "time"

// DefaultSyncLines is how many lines a render tokenizes past a document's
// materialized lines before deferring. A deferred line is drawn with its last
// spans, or highlighted on its own, and Step catches up between frames. Edits
// usually reconverge with the old tail well within it.
const DefaultSyncLines = 200

var (
	wake    func()
	pending = map[*Highlighter]struct{}{}
)

// SetStepScheduler sets fn to be called, on the UI goroutine, when a render
// deferred lines; fn should arrange for Step to run between frames. Without a
// scheduler every line is highlighted synchronously.
func SetStepScheduler(fn func()) {
	wake = fn
}

// Pending reports how many highlighters have deferred lines.
func Pending() int {
	return len(pending)
}

// Step tokenizes toward deferred lines for about budget and reports whether
// any remain. It runs on the UI goroutine, between frames.
func Step(budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for h := range pending {
		for !h.materialize(h.wantLine, 1) {
			if time.Now().After(deadline) {
				return true
			}
		}
		h.stepping = false
		h.wantLine = 0
		delete(pending, h)
	}
	return len(pending) > 0
}

// Synchronous turns off deferral for h, for output rendered once, such as a
// markdown code block, that a later frame would not redraw.
func (h *Highlighter) Synchronous() {
	if h != nil {
		h.sync = true
	}
}

// SetSyncLines sets how many lines a render may tokenize past what h has
// materialized before deferring the rest to Step.
func (h *Highlighter) SetSyncLines(n int) {
	if h != nil {
		h.syncLines = max(n, 0)
	}
}

// Close drops h's deferred work.
func (h *Highlighter) Close() {
	if h == nil {
		return
	}
	delete(pending, h)
	h.stepping = false
	h.wantLine = 0
}

func (h *Highlighter) deferTo(idx int) {
	h.wantLine = max(h.wantLine, idx)
	if h.stepping {
		return
	}
	h.stepping = true
	pending[h] = struct{}{}
	if wake != nil {
		wake()
	}
}

// materialize tokenizes at most lines lines toward idx and reports whether
// idx's start state is now known.
func (h *Highlighter) materialize(idx, lines int) bool {
	for range lines {
		n := h.doc.MaterializedLines()
		if n > idx || n >= h.doc.Len() {
			return true
		}
		h.doc.StateAt(n)
	}
	n := h.doc.MaterializedLines()
	return n > idx || n >= h.doc.Len()
}

type lastLine struct {
	text  string
	spans []Span
}

func (h *Highlighter) remember(idx int, text string, spans []Span) {
	if e, ok := h.lastSpans[idx]; ok && e.text == text && sameSpans(e.spans, spans) {
		return
	}
	if h.lastSpans == nil || len(h.lastSpans) >= maxIsolatedCache {
		h.lastSpans = make(map[int]lastLine)
	}
	h.lastSpans[idx] = lastLine{text: text, spans: spans}
}

// deferred draws a line that is not tokenized yet: its last spans while its
// text is unchanged, else the line highlighted on its own.
func (h *Highlighter) deferred(idx int, text string) []Span {
	if e, ok := h.lastSpans[idx]; ok && e.text == text {
		return e.spans
	}
	return h.HighlightLine(text)
}

func sameSpans(a, b []Span) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// HasDeferredLines reports whether h is waiting for Step.
func (h *Highlighter) HasDeferredLines() bool {
	return h != nil && h.stepping
}
