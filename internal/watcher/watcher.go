// Package watcher reports when files open in the editor are modified on disk
// by another process. It wraps fsnotify, watching the parent directories of
// tracked files (the portable, rename-safe pattern) and debouncing bursts of
// events into a single notification per file. SyncDirs additionally watches a
// set of directories for the workspace explorer.
package watcher

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// defaultDebounce coalesces the rapid bursts of events that a single logical
// write produces (including the temp-file + rename pattern the editor itself
// uses to save) into one notification.
const defaultDebounce = 150 * time.Millisecond

// Watcher tracks a set of files and invokes onChange when one of them changes
// on disk. onChange is called from an internal goroutine with the same path
// string that was passed to Sync. SyncDirs tracks a parallel set of directories
// whose entry creation/removal/rename fires onDirChange.
type Watcher struct {
	fsw         *fsnotify.Watcher
	onChange    func(path string)
	onDirChange func(dir string)
	debounce    time.Duration

	mu        sync.Mutex
	files     map[string]string // cleaned-abs path -> path as tracked
	watchDirs map[string]string // cleaned-abs dir -> path as tracked
	dirRefs   map[string]int    // fsnotify-watched dir -> refcount (file parents + watchDirs)
	timers    map[string]*time.Timer
	dirTimers map[string]*time.Timer
	closed    bool
}

// New creates a Watcher and starts its event loop. The callbacks run on an
// internal goroutine; either may be nil.
func New(onChange func(path string), onDirChange func(dir string)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fsw:         fsw,
		onChange:    onChange,
		onDirChange: onDirChange,
		debounce:    defaultDebounce,
		files:       make(map[string]string),
		watchDirs:   make(map[string]string),
		dirRefs:     make(map[string]int),
		timers:      make(map[string]*time.Timer),
		dirTimers:   make(map[string]*time.Timer),
	}
	go w.run()
	return w, nil
}

// Sync reconciles the tracked set with paths, adding watches for newly opened
// files and dropping them for closed ones. It is a no-op for paths already
// tracked, so it is cheap to call frequently.
func (w *Watcher) Sync(paths []string) {
	want := make(map[string]string, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		want[filepath.Clean(abs)] = p
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	for key := range w.files {
		if _, ok := want[key]; !ok {
			w.untrackLocked(key)
		}
	}
	for key, orig := range want {
		if _, ok := w.files[key]; !ok {
			w.trackLocked(key, orig)
		}
	}
}

// SyncDirs reconciles the tracked directory set with paths. onDirChange fires
// with the path as passed here when an entry in one of them is added, removed
// or renamed. Like Sync, it is cheap to call frequently.
func (w *Watcher) SyncDirs(paths []string) {
	want := make(map[string]string, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		want[filepath.Clean(abs)] = p
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	for key := range w.watchDirs {
		if _, ok := want[key]; !ok {
			delete(w.watchDirs, key)
			if t := w.dirTimers[key]; t != nil {
				t.Stop()
				delete(w.dirTimers, key)
			}
			w.releaseDirLocked(key)
		}
	}
	for key, orig := range want {
		if _, ok := w.watchDirs[key]; !ok {
			if w.retainDirLocked(key) {
				w.watchDirs[key] = orig
			}
		}
	}
}

// retainDirLocked bumps dir's fsnotify refcount, adding the watch on the first
// reference and reporting whether the directory ended up watched.
func (w *Watcher) retainDirLocked(dir string) bool {
	if w.dirRefs[dir] == 0 {
		if err := w.fsw.Add(dir); err != nil {
			return false
		}
	}
	w.dirRefs[dir]++
	return true
}

func (w *Watcher) releaseDirLocked(dir string) {
	if w.dirRefs[dir] > 0 {
		w.dirRefs[dir]--
		if w.dirRefs[dir] == 0 {
			delete(w.dirRefs, dir)
			_ = w.fsw.Remove(dir)
		}
	}
}

func (w *Watcher) trackLocked(key, orig string) {
	if !w.retainDirLocked(filepath.Dir(key)) {
		return
	}
	w.files[key] = orig
}

func (w *Watcher) untrackLocked(key string) {
	if _, ok := w.files[key]; !ok {
		return
	}
	delete(w.files, key)
	if t := w.timers[key]; t != nil {
		t.Stop()
		delete(w.timers, key)
	}
	w.releaseDirLocked(filepath.Dir(key))
}

func (w *Watcher) run() {
	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if ev.Op == fsnotify.Chmod {
				continue
			}
			key := filepath.Clean(ev.Name)
			w.handle(key)
			// A write to an existing file can't change a directory listing.
			if ev.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				w.handleDir(key)
			}
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) handle(key string) {
	if w.onChange == nil {
		return
	}
	w.mu.Lock()
	orig, tracked := w.files[key]
	if !tracked || w.closed {
		w.mu.Unlock()
		return
	}
	if t := w.timers[key]; t != nil {
		t.Stop()
	}
	w.timers[key] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		_, still := w.files[key]
		delete(w.timers, key)
		closed := w.closed
		w.mu.Unlock()
		if still && !closed {
			w.onChange(orig)
		}
	})
	w.mu.Unlock()
}

// handleDir debounces changes to a watched directory into one onDirChange call.
func (w *Watcher) handleDir(key string) {
	if w.onDirChange == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	// key is either an entry of a watched dir or a watched dir itself (removed).
	dirKey := filepath.Dir(key)
	orig, ok := w.watchDirs[dirKey]
	if !ok {
		if orig, ok = w.watchDirs[key]; ok {
			dirKey = key
		} else {
			return
		}
	}
	if t := w.dirTimers[dirKey]; t != nil {
		t.Stop()
	}
	w.dirTimers[dirKey] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.dirTimers, dirKey)
		_, still := w.watchDirs[dirKey]
		closed := w.closed
		w.mu.Unlock()
		if still && !closed {
			w.onDirChange(orig)
		}
	})
}

// Close stops watching and releases resources. The Watcher must not be used
// afterwards.
func (w *Watcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	for key, t := range w.timers {
		t.Stop()
		delete(w.timers, key)
	}
	for key, t := range w.dirTimers {
		t.Stop()
		delete(w.dirTimers, key)
	}
	w.mu.Unlock()
	return w.fsw.Close()
}
