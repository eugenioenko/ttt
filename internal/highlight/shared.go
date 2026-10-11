package highlight

import (
	"sync"

	textmate "github.com/eugenioenko/textmate-go"
)

type lineDoc interface {
	Len() int
	SetLines(lines []string)
	Line(index int) (textmate.LineResult, bool)
	StateAt(index int) (*textmate.StateStack, bool)
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
	mu       sync.Mutex
	m        map[tokenKey]textmate.LineResult
	order    []tokenKey
	next     int
	limit    int
	inflight map[tokenKey]chan struct{}
}

// claim returns a cached result, or reports that the caller must tokenize k
// and then call done. Both sides of a diff warm the same lines at once, so a
// key another goroutine is tokenizing is waited for rather than repeated.
func (c *tokenCache) claim(k tokenKey) (textmate.LineResult, bool) {
	c.mu.Lock()
	for {
		if r, ok := c.m[k]; ok {
			c.mu.Unlock()
			return r, true
		}
		wait, busy := c.inflight[k]
		if !busy {
			break
		}
		c.mu.Unlock()
		<-wait
		c.mu.Lock()
		if _, still := c.inflight[k]; still {
			continue
		}
		if r, ok := c.m[k]; ok {
			c.mu.Unlock()
			return r, true
		}
	}
	if c.inflight == nil {
		c.inflight = make(map[tokenKey]chan struct{})
	}
	c.inflight[k] = make(chan struct{})
	c.mu.Unlock()
	return textmate.LineResult{}, false
}

func (c *tokenCache) done(k tokenKey) {
	c.mu.Lock()
	ch := c.inflight[k]
	delete(c.inflight, k)
	c.mu.Unlock()
	close(ch)
}

func (c *tokenCache) put(k tokenKey, r textmate.LineResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	c.limit = max(c.limit, minSharedCache, 2*lines)
	c.mu.Unlock()
}

type sharedDoc struct {
	mu      sync.Mutex
	grammar *textmate.Grammar
	options textmate.TokenizeOptions
	lines   []string
	states  []*textmate.StateStack
	cache   *tokenCache
}

func (d *sharedDoc) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.lines)
}

func (d *sharedDoc) SetLines(lines []string) {
	owned := append([]string(nil), lines...)
	d.cache.reserve(len(owned))
	d.mu.Lock()
	defer d.mu.Unlock()
	keep := 0
	for keep < len(d.states)-1 && keep < len(owned) && keep < len(d.lines) && owned[keep] == d.lines[keep] {
		keep++
	}
	d.states = d.states[:keep+1]
	d.lines = owned
}

func (d *sharedDoc) Line(index int) (textmate.LineResult, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if index < 0 || index >= len(d.lines) {
		return textmate.LineResult{}, false
	}
	state := d.stateAtLocked(index)
	result := d.tokenize(d.lines[index], state)
	if len(d.states) == index+1 {
		d.states = append(d.states, result.RuleStack)
	}
	return result, true
}

func (d *sharedDoc) StateAt(index int) (*textmate.StateStack, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if index < 0 || index > len(d.lines) {
		return nil, false
	}
	return d.stateAtLocked(index), true
}

func (d *sharedDoc) stateAtLocked(index int) *textmate.StateStack {
	for len(d.states) <= index {
		i := len(d.states) - 1
		d.states = append(d.states, d.tokenize(d.lines[i], d.states[i]).RuleStack)
	}
	return d.states[index]
}

func (d *sharedDoc) tokenize(line string, state *textmate.StateStack) textmate.LineResult {
	key := tokenKey{line: line, state: state}
	if r, ok := d.cache.claim(key); ok {
		return r
	}
	r := d.grammar.TokenizeLineWithOptions(line, state, d.options)
	// A time stop can come from transient load; caching it would make a soft
	// budget permanent.
	if r.StoppedReason != textmate.StopReasonTimeLimit {
		d.cache.put(key, r)
	}
	d.cache.done(key)
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
