package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNavigateBackAfterGoToLine(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	// Create a file with many lines
	lines := ""
	for i := 1; i <= 50; i++ {
		if i > 1 {
			lines += "\n"
		}
		lines += "line content"
	}
	f := filepath.Join(h.dir, "nav.txt")
	os.WriteFile(f, []byte(lines), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	// Verify cursor starts at line 0
	line, _ := h.app.EditorGroup.ActiveCursor()
	if line != 0 {
		t.Fatalf("expected cursor at line 0, got %d", line)
	}

	// Push history and go to line 25
	h.app.PushNavHistory()
	h.app.EditorGroup.GoToLine(25)
	h.redraw()

	line, _ = h.app.EditorGroup.ActiveCursor()
	if line != 24 { // GoToLine is 1-based, cursor is 0-based
		t.Fatalf("expected cursor at line 24 after GoToLine(25), got %d", line)
	}

	// Navigate back
	h.exec("navigate.back")

	line, _ = h.app.EditorGroup.ActiveCursor()
	if line != 0 {
		t.Errorf("expected cursor back at line 0, got %d", line)
	}
}

func TestNavigateForwardAfterBack(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	lines := ""
	for i := 1; i <= 50; i++ {
		if i > 1 {
			lines += "\n"
		}
		lines += "line content"
	}
	f := filepath.Join(h.dir, "navfwd.txt")
	os.WriteFile(f, []byte(lines), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	// Push history at line 0, then jump to line 30
	h.app.PushNavHistory()
	h.app.EditorGroup.GoToLine(30)
	h.redraw()

	// Go back to line 0
	h.exec("navigate.back")
	line, _ := h.app.EditorGroup.ActiveCursor()
	if line != 0 {
		t.Fatalf("expected line 0 after back, got %d", line)
	}

	// Go forward to line 29
	h.exec("navigate.forward")
	line, _ = h.app.EditorGroup.ActiveCursor()
	if line != 29 {
		t.Errorf("expected line 29 after forward, got %d", line)
	}
}

func TestNavigateBackAcrossFiles(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	fileA := filepath.Join(h.dir, "a.txt")
	fileB := filepath.Join(h.dir, "b.txt")
	os.WriteFile(fileA, []byte("file a content\nline 2\nline 3"), 0644)
	os.WriteFile(fileB, []byte("file b content\nline 2\nline 3"), 0644)

	// Open file A
	h.app.EditorGroup.OpenFile(fileA)
	h.redraw()

	if h.app.EditorGroup.ActiveFilePath() != fileA {
		t.Fatalf("expected fileA active, got %s", h.app.EditorGroup.ActiveFilePath())
	}

	// Push history, then open file B (simulating a go-to-definition jump)
	h.app.PushNavHistory()
	h.app.EditorGroup.OpenFile(fileB)
	h.app.EditorGroup.GoToLine(2)
	h.redraw()

	if h.app.EditorGroup.ActiveFilePath() != fileB {
		t.Fatalf("expected fileB active, got %s", h.app.EditorGroup.ActiveFilePath())
	}

	// Navigate back should return to file A
	h.exec("navigate.back")

	if h.app.EditorGroup.ActiveFilePath() != fileA {
		t.Errorf("expected fileA after navigate back, got %s", h.app.EditorGroup.ActiveFilePath())
	}
}

func TestNavigateBackNoHistoryNoop(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	// Without any navigation, back/forward should be no-ops
	line, _ := h.app.EditorGroup.ActiveCursor()
	h.exec("navigate.back")
	afterLine, _ := h.app.EditorGroup.ActiveCursor()
	if afterLine != line {
		t.Errorf("navigate.back on empty history should be a no-op, cursor moved from %d to %d", line, afterLine)
	}

	h.exec("navigate.forward")
	afterLine, _ = h.app.EditorGroup.ActiveCursor()
	if afterLine != line {
		t.Errorf("navigate.forward on empty history should be a no-op, cursor moved from %d to %d", line, afterLine)
	}
}

func TestGoToLinePushesHistory(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	lines := ""
	for i := 1; i <= 50; i++ {
		if i > 1 {
			lines += "\n"
		}
		lines += "line content"
	}
	f := filepath.Join(h.dir, "gotoline.txt")
	os.WriteFile(f, []byte(lines), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	// Simulate what happens when user uses the go-to-line command:
	// the palette callback calls PushNavHistory then GoToLine
	h.app.PushNavHistory()
	h.app.EditorGroup.GoToLine(10)
	h.redraw()

	h.app.PushNavHistory()
	h.app.EditorGroup.GoToLine(40)
	h.redraw()

	// Navigate back to line 9 (0-based)
	h.exec("navigate.back")
	line, _ := h.app.EditorGroup.ActiveCursor()
	if line != 9 {
		t.Errorf("expected line 9 after first back, got %d", line)
	}

	// Navigate back to line 0
	h.exec("navigate.back")
	line, _ = h.app.EditorGroup.ActiveCursor()
	if line != 0 {
		t.Errorf("expected line 0 after second back, got %d", line)
	}
}
