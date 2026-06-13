package navhistory

import "testing"

func TestNewDefaults(t *testing.T) {
	h := New(0)
	if h.maxSize != 100 {
		t.Errorf("expected default maxSize=100, got %d", h.maxSize)
	}
	if h.Len() != 0 {
		t.Errorf("expected empty history, got len=%d", h.Len())
	}
	if h.CanGoBack() {
		t.Error("should not be able to go back on empty history")
	}
	if h.CanGoForward() {
		t.Error("should not be able to go forward on empty history")
	}
}

func TestPushAndBack(t *testing.T) {
	h := New(50)

	h.Push(NavEntry{FilePath: "a.go", Line: 10, Col: 5})
	h.Push(NavEntry{FilePath: "b.go", Line: 20, Col: 3})
	h.Push(NavEntry{FilePath: "c.go", Line: 30, Col: 0})

	if h.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", h.Len())
	}
	if !h.CanGoBack() {
		t.Fatal("should be able to go back")
	}
	if h.CanGoForward() {
		t.Fatal("should not be able to go forward at end")
	}

	e := h.Back()
	if e == nil {
		t.Fatal("Back() returned nil")
	}
	if e.FilePath != "b.go" || e.Line != 20 {
		t.Errorf("expected b.go:20, got %s:%d", e.FilePath, e.Line)
	}

	e = h.Back()
	if e == nil {
		t.Fatal("Back() returned nil")
	}
	if e.FilePath != "a.go" || e.Line != 10 {
		t.Errorf("expected a.go:10, got %s:%d", e.FilePath, e.Line)
	}

	if h.CanGoBack() {
		t.Error("should not be able to go back past first entry")
	}
	if h.Back() != nil {
		t.Error("Back() should return nil when at start")
	}
}

func TestForward(t *testing.T) {
	h := New(50)

	h.Push(NavEntry{FilePath: "a.go", Line: 1})
	h.Push(NavEntry{FilePath: "b.go", Line: 2})
	h.Push(NavEntry{FilePath: "c.go", Line: 3})

	h.Back()
	h.Back()

	if !h.CanGoForward() {
		t.Fatal("should be able to go forward")
	}

	e := h.Forward()
	if e.FilePath != "b.go" {
		t.Errorf("expected b.go, got %s", e.FilePath)
	}

	e = h.Forward()
	if e.FilePath != "c.go" {
		t.Errorf("expected c.go, got %s", e.FilePath)
	}

	if h.CanGoForward() {
		t.Error("should not be able to go forward past end")
	}
	if h.Forward() != nil {
		t.Error("Forward() should return nil at end")
	}
}

func TestPushTruncatesForwardHistory(t *testing.T) {
	h := New(50)

	h.Push(NavEntry{FilePath: "a.go", Line: 1})
	h.Push(NavEntry{FilePath: "b.go", Line: 2})
	h.Push(NavEntry{FilePath: "c.go", Line: 3})

	// Go back to a.go
	h.Back()
	h.Back()

	// Push a new entry -- forward history (b.go, c.go) should be truncated
	h.Push(NavEntry{FilePath: "d.go", Line: 4})

	if h.Len() != 2 {
		t.Fatalf("expected 2 entries after truncation, got %d", h.Len())
	}
	if h.CanGoForward() {
		t.Error("forward history should have been truncated")
	}

	e := h.Back()
	if e.FilePath != "a.go" {
		t.Errorf("expected a.go, got %s", e.FilePath)
	}
}

func TestDuplicateSuppressionSameFileLine(t *testing.T) {
	h := New(50)

	h.Push(NavEntry{FilePath: "a.go", Line: 10, Col: 5})
	h.Push(NavEntry{FilePath: "a.go", Line: 10, Col: 20}) // same file+line, different col

	if h.Len() != 1 {
		t.Errorf("expected 1 entry (duplicate suppressed), got %d", h.Len())
	}
}

func TestDuplicateAllowsDifferentLine(t *testing.T) {
	h := New(50)

	h.Push(NavEntry{FilePath: "a.go", Line: 10})
	h.Push(NavEntry{FilePath: "a.go", Line: 20})

	if h.Len() != 2 {
		t.Errorf("expected 2 entries (different lines), got %d", h.Len())
	}
}

func TestMaxSizeEviction(t *testing.T) {
	h := New(3)

	h.Push(NavEntry{FilePath: "a.go", Line: 1})
	h.Push(NavEntry{FilePath: "b.go", Line: 2})
	h.Push(NavEntry{FilePath: "c.go", Line: 3})
	h.Push(NavEntry{FilePath: "d.go", Line: 4})

	if h.Len() != 3 {
		t.Fatalf("expected 3 entries (oldest evicted), got %d", h.Len())
	}

	// The oldest entry (a.go) should have been evicted
	h.Back()
	h.Back()
	e := h.Current()
	if e.FilePath != "b.go" {
		t.Errorf("expected oldest remaining to be b.go, got %s", e.FilePath)
	}
}

func TestCurrent(t *testing.T) {
	h := New(50)

	if h.Current() != nil {
		t.Error("Current() should return nil on empty history")
	}

	h.Push(NavEntry{FilePath: "a.go", Line: 1})
	e := h.Current()
	if e == nil || e.FilePath != "a.go" {
		t.Errorf("expected current to be a.go, got %v", e)
	}

	h.Push(NavEntry{FilePath: "b.go", Line: 2})
	e = h.Current()
	if e == nil || e.FilePath != "b.go" {
		t.Errorf("expected current to be b.go, got %v", e)
	}

	h.Back()
	e = h.Current()
	if e == nil || e.FilePath != "a.go" {
		t.Errorf("expected current to be a.go after Back, got %v", e)
	}
}

func TestBackForwardRoundTrip(t *testing.T) {
	h := New(50)

	h.Push(NavEntry{FilePath: "a.go", Line: 1})
	h.Push(NavEntry{FilePath: "b.go", Line: 2})

	e := h.Back()
	if e.FilePath != "a.go" {
		t.Errorf("back: expected a.go, got %s", e.FilePath)
	}

	e = h.Forward()
	if e.FilePath != "b.go" {
		t.Errorf("forward: expected b.go, got %s", e.FilePath)
	}
}

func TestSingleEntryNoBack(t *testing.T) {
	h := New(50)
	h.Push(NavEntry{FilePath: "a.go", Line: 1})

	if h.CanGoBack() {
		t.Error("single entry should not allow going back")
	}
	if h.CanGoForward() {
		t.Error("single entry should not allow going forward")
	}
}
