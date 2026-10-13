package highlight

import (
	"slices"

	textmate "github.com/eugenioenko/textmate-go"
)

type lineDoc interface {
	Len() int
	SetLines(lines []string)
	ReplaceLines(start, end int, replacement []string) error
	Line(index int) (textmate.LineResult, bool)
	StateAt(index int) (*textmate.StateStack, bool)
	MaterializedLines() int
}

const minSharedCache = 20_000

type tokenKey struct {
	line  string
	state *textmate.StateStack
}

// tokenCache is keyed by line text and start state. A grammar interns its
// state stacks, so equal states share a pointer across documents and a result
// tokenized for one side of a diff is exact for the other.
type tokenCache struct {
	m     map[tokenKey]textmate.LineResult
	order []tokenKey
	next  int
	limit int
}

func (c *tokenCache) get(k tokenKey) (textmate.LineResult, bool) {
	r, ok := c.m[k]
	return r, ok
}

func (c *tokenCache) put(k tokenKey, r textmate.LineResult) {
	if c.m == nil {
		c.m = make(map[tokenKey]textmate.LineResult)
	}
	if _, ok := c.m[k]; ok {
		return
	}
	if len(c.order) < c.limit {
		c.order = append(c.order, k)
	} else {
		delete(c.m, c.order[c.next])
		c.order[c.next] = k
		c.next = (c.next + 1) % len(c.order)
	}
	c.m[k] = r
}

func (c *tokenCache) reserve(lines int) {
	c.limit = max(c.limit, minSharedCache, 2*lines)
}

// sharedDoc is used only on the UI goroutine, like every highlighter.
type sharedDoc struct {
	grammar *textmate.Grammar
	options textmate.TokenizeOptions
	lines   []string
	states  []*textmate.StateStack
	cache   *tokenCache

	// partial is the earliest index i > 0 whose states[i] came from a line
	// stopped by the time limit, or 0. As in textmate.Document, that state
	// and every one after it are recomputed on the next call, at most
	// maxTimeLimitRetries times in a row for the same line.
	partial   int
	retries   int
	retryLine int
}

const maxTimeLimitRetries = 3

func (d *sharedDoc) Len() int {
	return len(d.lines)
}

func (d *sharedDoc) MaterializedLines() int {
	d.retryPartial()
	return min(len(d.states), len(d.lines))
}

func (d *sharedDoc) SetLines(lines []string) {
	owned := append([]string(nil), lines...)
	d.cache.reserve(len(owned))
	keep := 0
	for keep < len(d.states)-1 && keep < len(owned) && keep < len(d.lines) && owned[keep] == d.lines[keep] {
		keep++
	}
	d.states = d.states[:keep+1]
	d.lines = owned
	if d.partial > keep {
		d.partial = 0
	}
	d.retries = 0
}

// ReplaceLines keeps the states above the edit, as SetLines does.
func (d *sharedDoc) ReplaceLines(start, end int, replacement []string) error {
	if start < 0 || start > end || end > len(d.lines) {
		return textmate.ErrInvalidLineRange
	}
	d.lines = slices.Replace(d.lines, start, end, replacement...)
	d.cache.reserve(len(d.lines))
	keep := min(start, len(d.states)-1)
	d.states = d.states[:keep+1]
	if d.partial > keep {
		d.partial = 0
	}
	d.retries = 0
	return nil
}

func (d *sharedDoc) Line(index int) (textmate.LineResult, bool) {
	if index < 0 || index >= len(d.lines) {
		return textmate.LineResult{}, false
	}
	d.retryPartial()
	state := d.stateAt(index)
	result := d.tokenize(d.lines[index], state)
	if len(d.states) == index+1 {
		d.states = append(d.states, result.RuleStack)
		d.noteState(index, result)
	}
	return result, true
}

func (d *sharedDoc) StateAt(index int) (*textmate.StateStack, bool) {
	if index < 0 || index > len(d.lines) {
		return nil, false
	}
	d.retryPartial()
	return d.stateAt(index), true
}

func (d *sharedDoc) stateAt(index int) *textmate.StateStack {
	for len(d.states) <= index {
		i := len(d.states) - 1
		result := d.tokenize(d.lines[i], d.states[i])
		d.states = append(d.states, result.RuleStack)
		d.noteState(i, result)
	}
	return d.states[index]
}

func (d *sharedDoc) noteState(i int, result textmate.LineResult) {
	if result.StoppedReason == textmate.StopReasonTimeLimit {
		if d.partial == 0 || i+1 < d.partial {
			d.partial = i + 1
		}
		return
	}
	if i == d.retryLine {
		d.retries = 0
	}
}

func (d *sharedDoc) retryPartial() {
	if d.partial == 0 || d.retries >= maxTimeLimitRetries {
		return
	}
	d.retries++
	d.retryLine = d.partial - 1
	d.states = d.states[:d.partial]
	d.partial = 0
}

func (d *sharedDoc) tokenize(line string, state *textmate.StateStack) textmate.LineResult {
	key := tokenKey{line: line, state: state}
	if r, ok := d.cache.get(key); ok {
		return r
	}
	r := d.grammar.TokenizeLineWithOptions(line, state, d.options)
	// A time stop can come from transient load; caching it would make a soft
	// budget permanent.
	if r.StoppedReason != textmate.StopReasonTimeLimit {
		d.cache.put(key, r)
	}
	return r
}

// ShareTokens makes h tokenize through a cache shared with peer, for two
// documents that hold mostly the same lines, such as the sides of a diff.
// Call it before h highlights anything.
func (h *Highlighter) ShareTokens(peer *Highlighter) {
	if h == nil {
		return
	}
	var cache *tokenCache
	if peer != nil && peer.grammar == h.grammar {
		if d, ok := peer.doc.(*sharedDoc); ok {
			cache = d.cache
		}
	}
	if cache == nil {
		cache = &tokenCache{limit: minSharedCache}
	}
	h.doc = &sharedDoc{grammar: h.grammar, options: tokenizeOptions, states: []*textmate.StateStack{nil}, cache: cache}
	h.dirty = true
}
