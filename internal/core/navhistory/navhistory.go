package navhistory

// NavEntry represents a cursor position in a file.
type NavEntry struct {
	FilePath string
	Line     int
	Col      int
}

// NavHistory tracks navigation positions with back/forward support,
// similar to browser history.
type NavHistory struct {
	stack   []NavEntry
	current int // index into stack; -1 when empty
	maxSize int
}

// New creates a NavHistory with the given maximum stack size.
func New(maxSize int) *NavHistory {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &NavHistory{
		current: -1,
		maxSize: maxSize,
	}
}

// Push records a new navigation entry. Any forward history beyond the
// current position is truncated (browser-style). If the new entry matches
// the current top entry (same file/line), it is ignored to avoid duplicates.
func (h *NavHistory) Push(entry NavEntry) {
	if h.current >= 0 && h.current < len(h.stack) {
		top := h.stack[h.current]
		if top.FilePath == entry.FilePath && top.Line == entry.Line {
			return
		}
	}

	// Truncate forward history.
	h.stack = h.stack[:h.current+1]

	h.stack = append(h.stack, entry)
	h.current = len(h.stack) - 1

	// Evict oldest entries when the stack exceeds maxSize.
	if len(h.stack) > h.maxSize {
		excess := len(h.stack) - h.maxSize
		h.stack = h.stack[excess:]
		h.current -= excess
		if h.current < 0 {
			h.current = 0
		}
	}
}

// Back moves one step back in the history and returns that entry.
// Returns nil if there is no back history.
func (h *NavHistory) Back() *NavEntry {
	if !h.CanGoBack() {
		return nil
	}
	h.current--
	e := h.stack[h.current]
	return &e
}

// Forward moves one step forward in the history and returns that entry.
// Returns nil if there is no forward history.
func (h *NavHistory) Forward() *NavEntry {
	if !h.CanGoForward() {
		return nil
	}
	h.current++
	e := h.stack[h.current]
	return &e
}

// CanGoBack reports whether there is at least one entry before the current position.
func (h *NavHistory) CanGoBack() bool {
	return h.current > 0
}

// CanGoForward reports whether there is at least one entry after the current position.
func (h *NavHistory) CanGoForward() bool {
	return h.current >= 0 && h.current < len(h.stack)-1
}

// Len returns the total number of entries in the history stack.
func (h *NavHistory) Len() int {
	return len(h.stack)
}

// Current returns the entry at the current position, or nil if the history is empty.
func (h *NavHistory) Current() *NavEntry {
	if h.current < 0 || h.current >= len(h.stack) {
		return nil
	}
	e := h.stack[h.current]
	return &e
}

// CurrentIndex returns the current index into the stack.
func (h *NavHistory) CurrentIndex() int {
	return h.current
}
