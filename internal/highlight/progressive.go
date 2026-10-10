package highlight

import (
	"sync"
	"sync/atomic"

	textmate "github.com/eugenioenko/textmate-go"
)

// progressiveSlack is how far below the prefix tokenized when a pass began a
// line may sit and still be highlighted synchronously: scrolling a page stays
// exact, while a jump deep into a file no longer stalls the frame.
const progressiveSlack = 64

const warmChunk = 16

var warming atomic.Int64

// Warming reports how many highlighters are tokenizing in the background, so
// a benchmark can time steady state instead of a diff that is still settling.
func Warming() int { return int(warming.Load()) }

// SetProgressive makes HighlightLineAt answer lines far below the tokenized
// prefix with single-line highlighting while a goroutine tokenizes down to
// them. notify is called from that goroutine once it has; it must be safe to
// call off the main thread. A nil notify turns the mode off.
func (h *Highlighter) SetProgressive(notify func()) {
	if h == nil {
		return
	}
	if notify == nil {
		if h.progress != nil {
			h.progress.cancel()
			h.progress = nil
		}
		return
	}
	if h.progress == nil {
		h.progress = &progressive{base: -1, settled: -1, warm: &warmer{reached: -1, target: -1}}
		h.dirty = true
	}
	h.progress.notify = notify
}

type progressive struct {
	notify     func()
	base       int
	catchingUp bool
	// settled is the last line whose start state the main thread knows is
	// materialized; lines are the ones handed to the document, to find how
	// much of that survives an edit.
	settled int
	lines   []string
	gen     uint64
	warm    *warmer
}

// BeginPass marks the start of a top-to-bottom render; the synchronous budget
// is measured from what was tokenized at this point.
// The worker waits while a pass runs: both take the document's lock, and the
// worker re-taking it between chunks would stall every synchronous line.
func (h *Highlighter) BeginPass() {
	if h != nil && h.progress != nil {
		p := h.progress
		p.base = p.settledLine()
		w := p.warm
		w.mu.Lock()
		w.pause++
		p.catchingUp = w.gen == p.gen && w.target > w.reached
		w.mu.Unlock()
	}
}

// EndPass starts tokenizing down to the deepest line the pass deferred, so a
// frame wakes one worker and earns one redraw.
func (h *Highlighter) EndPass() {
	if h == nil || h.progress == nil {
		return
	}
	p, w := h.progress, h.progress.warm
	w.mu.Lock()
	if w.pause > 0 {
		w.pause--
	}
	if w.pause == 0 {
		w.resume().Broadcast()
	}
	start := !w.running && w.gen == p.gen && w.target > w.reached
	w.running = w.running || start
	w.mu.Unlock()
	if start {
		warming.Add(1)
		go w.run(h.doc, p.notify)
	}
}

type warmer struct {
	mu      sync.Mutex
	gen     uint64
	reached int
	target  int
	running bool
	pause   int
	resumed *sync.Cond
}

func (w *warmer) resume() *sync.Cond {
	if w.resumed == nil {
		w.resumed = sync.NewCond(&w.mu)
	}
	return w.resumed
}

// slack is zero until anything is tokenized, so opening a file deep in a
// diff costs no more than highlighting the visible lines one by one, and
// while the worker is still catching up, so the frame doesn't redo its work.
func (p *progressive) slack() int {
	if p.base < 0 || p.catchingUp {
		return 0
	}
	return progressiveSlack
}

func (p *progressive) settledLine() int {
	p.warm.mu.Lock()
	defer p.warm.mu.Unlock()
	if p.warm.gen == p.gen && p.warm.reached > p.settled {
		p.settled = p.warm.reached
	}
	return p.settled
}

func (p *progressive) reached(idx int) {
	if p != nil && idx > p.settled {
		p.settled = idx
	}
}

// linesChanged keeps the settled prefix only as far as the new lines agree
// with the old: a line's start state depends on every line above it.
func (p *progressive) linesChanged(lines []string) {
	if p == nil {
		return
	}
	settled := p.settledLine()
	prefix := 0
	for prefix < len(lines) && prefix < len(p.lines) && lines[prefix] == p.lines[prefix] {
		prefix++
	}
	p.settled = min(settled, prefix)
	p.base = min(p.base, p.settled)
	p.lines = append(p.lines[:0], lines...)
	p.cancel()
}

func (p *progressive) cancel() {
	p.warm.mu.Lock()
	p.gen++
	p.warm.gen = p.gen
	p.warm.reached = p.settled
	p.warm.target = p.settled
	p.warm.resume().Broadcast()
	p.warm.mu.Unlock()
}

func (p *progressive) warmTo(idx int) {
	w := p.warm
	w.mu.Lock()
	if w.gen != p.gen {
		w.gen, w.reached, w.target = p.gen, p.settled, p.settled
	}
	w.target = max(w.target, idx)
	w.mu.Unlock()
}

func (w *warmer) run(doc *textmate.Document, notify func()) {
	for {
		w.mu.Lock()
		for w.pause > 0 && w.reached < w.target {
			w.resume().Wait()
		}
		gen, reached, target := w.gen, w.reached, w.target
		if reached >= target {
			w.running = false
			w.mu.Unlock()
			warming.Add(-1)
			notify()
			return
		}
		w.mu.Unlock()
		next := min(reached+warmChunk, target)
		doc.StateAt(next)
		w.mu.Lock()
		if w.gen == gen && next > w.reached {
			w.reached = next
		}
		w.mu.Unlock()
	}
}
