package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// waitForChange blocks until a path is reported or the timeout elapses.
func waitForChange(t *testing.T, ch <-chan string, timeout time.Duration) (string, bool) {
	t.Helper()
	select {
	case p := <-ch:
		return p, true
	case <-time.After(timeout):
		return "", false
	}
}

func newTestWatcher(t *testing.T) (*Watcher, <-chan string) {
	t.Helper()
	ch := make(chan string, 8)
	w, err := New(func(path string) { ch <- path }, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	return w, ch
}

func newTestDirWatcher(t *testing.T) (*Watcher, <-chan string) {
	t.Helper()
	ch := make(chan string, 8)
	w, err := New(nil, func(dir string) { ch <- dir })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	return w, ch
}

func TestWatcherDetectsChange(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(file, []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w, ch := newTestWatcher(t)
	w.Sync([]string{file})

	if err := os.WriteFile(file, []byte("two\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, ok := waitForChange(t, ch, 3*time.Second)
	if !ok {
		t.Fatal("expected a change notification, got none")
	}
	if got != file {
		t.Errorf("expected path %q, got %q", file, got)
	}
}

func TestWatcherIgnoresUntrackedFiles(t *testing.T) {
	dir := t.TempDir()
	tracked := filepath.Join(dir, "tracked.txt")
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(tracked, []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("y\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w, ch := newTestWatcher(t)
	w.Sync([]string{tracked})

	// Modifying a sibling that is not tracked must not notify.
	if err := os.WriteFile(other, []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, ok := waitForChange(t, ch, 500*time.Millisecond); ok {
		t.Errorf("expected no notification for untracked file, got %q", got)
	}
}

func TestWatcherStopsAfterUntrack(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w, ch := newTestWatcher(t)
	w.Sync([]string{file})
	w.Sync(nil) // untrack everything

	if err := os.WriteFile(file, []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, ok := waitForChange(t, ch, 500*time.Millisecond); ok {
		t.Errorf("expected no notification after untracking, got %q", got)
	}
}

func TestWatcherDetectsDirEntryCreated(t *testing.T) {
	dir := t.TempDir()

	w, ch := newTestDirWatcher(t)
	w.SyncDirs([]string{dir})

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, ok := waitForChange(t, ch, 3*time.Second)
	if !ok {
		t.Fatal("expected a directory change notification, got none")
	}
	if got != dir {
		t.Errorf("expected dir %q, got %q", dir, got)
	}
}

func TestWatcherDetectsDirEntryRemoved(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "gone.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w, ch := newTestDirWatcher(t)
	w.SyncDirs([]string{dir})

	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}

	if _, ok := waitForChange(t, ch, 3*time.Second); !ok {
		t.Fatal("expected a directory change notification, got none")
	}
}

func TestWatcherIgnoresUnwatchedDirs(t *testing.T) {
	watched := t.TempDir()
	other := t.TempDir()

	w, ch := newTestDirWatcher(t)
	w.SyncDirs([]string{watched})

	if err := os.WriteFile(filepath.Join(other, "x.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, ok := waitForChange(t, ch, 500*time.Millisecond); ok {
		t.Errorf("expected no notification for unwatched dir, got %q", got)
	}
}

func TestWatcherStopsAfterDirUntrack(t *testing.T) {
	dir := t.TempDir()

	w, ch := newTestDirWatcher(t)
	w.SyncDirs([]string{dir})
	w.SyncDirs(nil)

	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, ok := waitForChange(t, ch, 500*time.Millisecond); ok {
		t.Errorf("expected no notification after untracking dir, got %q", got)
	}
}

func TestWatcherDebouncesBurst(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w, ch := newTestWatcher(t)
	w.Sync([]string{file})

	// Several rapid writes should coalesce into a single notification.
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(file, []byte("burst\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if _, ok := waitForChange(t, ch, 3*time.Second); !ok {
		t.Fatal("expected at least one notification")
	}
	// No second notification should follow from the same burst.
	if got, ok := waitForChange(t, ch, 500*time.Millisecond); ok {
		t.Errorf("expected burst to be debounced, got extra notification %q", got)
	}
}
