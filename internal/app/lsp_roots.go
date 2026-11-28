package app

import (
	"path/filepath"
	"time"

	"github.com/eugenioenko/ttt/internal/lsp"

	"github.com/gdamore/tcell/v3"
)

func (a *App) lspWorkDir(path string) string {
	workDir := a.Workspace.Primary()
	if folder := a.Workspace.FolderForFile(path); folder != nil {
		workDir = folder.Path
	}
	if workDir == "" && path != "" {
		// Single-file launch: no workspace folder exists. Root the server at
		// the file's directory instead of an empty (invalid) rootURI.
		workDir = filepath.Dir(path)
	}
	return workDir
}

func (a *App) lspWorkDirFor(path, lang string) string {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return a.lspWorkDir(path)
	}
	return a.lspWorkDirForServer(path, serverKey)
}

func (a *App) lspWorkDirForServer(path, serverKey string) string {
	workDir := a.lspWorkDir(path)
	if path == "" || a.LspManager == nil {
		return workDir
	}
	// A client only knows the documents opened on it, so a document stays on
	// the root it was opened on: re-resolving mid-lifecycle would send its
	// changes and requests to a client that never received its didOpen.
	if root, pinned := a.pinnedLSPRoot(path); pinned {
		return root
	}
	if root := lsp.FindRoot(filepath.Dir(path), a.LspManager.ServerConfig(serverKey).RootMarkers); root != "" {
		return root
	}
	return workDir
}

type lspPin struct {
	server, root string
}

func (a *App) pinLSPRoot(path, server, root string) {
	pin := lspPin{server: server, root: root}
	a.lspDocRootsMu.Lock()
	if a.lspDocRoots == nil {
		a.lspDocRoots = make(map[string]lspPin)
		a.lspIdleGens = make(map[lspPin]uint64)
	}
	a.lspDocRoots[path] = pin
	a.lspIdleGens[pin]++
	a.lspDocRootsMu.Unlock()
}

func (a *App) pinnedLSPRoot(path string) (string, bool) {
	a.lspDocRootsMu.Lock()
	pin, ok := a.lspDocRoots[path]
	a.lspDocRootsMu.Unlock()
	return pin.root, ok
}

func (a *App) unpinLSPRoot(path string) {
	a.lspDocRootsMu.Lock()
	delete(a.lspDocRoots, path)
	a.lspDocRootsMu.Unlock()
}

// lspIdleTimeout is how long a root's server outlives its last open document,
// so closing and reopening a file does not pay for a cold start.
const lspIdleTimeout = time.Minute

// lspAfterFunc schedules idle stops; tests replace it to fire them on demand.
var lspAfterFunc = func(d time.Duration, f func()) { time.AfterFunc(d, f) }

type LSPRootStopped struct {
	Server, Root string
}

// scheduleLSPIdleStop starts the root's idle period. Pinning a document on the
// root bumps its generation, so a timer from an earlier period, even one
// already firing, cannot stop the server.
func (a *App) scheduleLSPIdleStop(server, root string) {
	pin := lspPin{server: server, root: root}
	manager := a.LspManager
	a.lspDocRootsMu.Lock()
	defer a.lspDocRootsMu.Unlock()
	for _, p := range a.lspDocRoots {
		if p == pin {
			return
		}
	}
	if a.lspIdleGens == nil {
		a.lspIdleGens = make(map[lspPin]uint64)
	}
	a.lspIdleGens[pin]++
	gen := a.lspIdleGens[pin]
	lspAfterFunc(lspIdleTimeout, func() {
		idle := func() bool {
			a.lspDocRootsMu.Lock()
			defer a.lspDocRootsMu.Unlock()
			return a.lspIdleGens[pin] == gen
		}
		if manager.StopIdle(server, root, idle) {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&LSPRootStopped{Server: server, Root: root}))
		}
	})
}

// handleLSPRootStopped clears what the stopped server published, which no
// server would otherwise replace.
func (a *App) handleLSPRootStopped(ev *LSPRootStopped) {
	for key, root := range a.lspDiagRoots {
		if key.server != ev.Server || root != ev.Root {
			continue
		}
		delete(a.lspDiagRoots, key)
		for _, group := range a.editorGroups() {
			group.SetDiagnosticsSource(key.source, key.path, nil)
		}
	}
}

type lspDiagKey struct {
	server, source, path string
}

func (a *App) recordLSPDiagRoot(server, source, path, root string, has bool) {
	key := lspDiagKey{server: server, source: source, path: path}
	if !has {
		delete(a.lspDiagRoots, key)
		return
	}
	if a.lspDiagRoots == nil {
		a.lspDiagRoots = make(map[lspDiagKey]string)
	}
	a.lspDiagRoots[key] = root
}

// lspDiagnosticsStillCurrent reports whether diagnostics published by a client
// rooted at root may be applied to path. A document that moved roots has its
// diagnostics owned by the server now rooted at its pin, so the old root's
// list must not replace that one. Documents without a pin (never opened here,
// or closed) keep whatever their server publishes.
func (a *App) lspDiagnosticsStillCurrent(path, root string) bool {
	pinned, ok := a.pinnedLSPRoot(path)
	return !ok || pinned == root
}
