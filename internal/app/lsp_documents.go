package app

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

// EnsureLSPOpen sends didOpen for a file that became active without going
// through OnFileOpen (tabs opened from CLI args before callbacks were wired).
// Servers like clangd reject requests for documents they never saw opened.
func (a *App) EnsureLSPOpen(path string) {
	if path == "" || !a.EditorGroup.IsEditorActive() || a.EditorGroup.IsActiveVirtual() {
		return
	}
	a.DocVersionsMu.Lock()
	_, opened := a.DocVersions[path]
	a.DocVersionsMu.Unlock()
	if opened {
		return
	}
	lang := a.EditorGroup.ActiveLanguage()
	a.NotifyLSPOpen(path, lang, a.EditorGroup.Editor.Buf.Lines)
}

func (a *App) NotifyLSPOpen(path, lang string, lines []string) {
	serverKey, langID, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}

	serverCfg := a.LspManager.ServerConfig(serverKey)
	if len(serverCfg.Command) > 0 {
		if _, err := exec.LookPath(serverCfg.Command[0]); err != nil {
			if !a.LspNotified[serverKey] && a.Settings.LSP.ShouldNotifyAvailability() {
				a.LspNotified[serverKey] = true
				msg := i18n.Format("lsp.installServerHint", i18n.Args{"lang": lang})
				anchor := serverKey
				a.Status.SetNotificationWithAction(msg, view.NotifyWarning, 10*time.Second, i18n.Text("status.docs"), func() {
					OpenURL("https://tttedit.dev/guides/lsp/#" + anchor)
				})
				a.Status.SecondaryLabel = i18n.Text("lsp.donTShowAgain")
				a.Status.SecondaryAction = func() {
					v := false
					a.Settings.LSP.NotifyAvailability = &v
					a.persistSettings()
				}
				time.AfterFunc(10*time.Second, func() {
					_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
				})
			}
			return
		}
	}

	text := strings.Join(lines, "\n")
	workDir := a.lspWorkDirFor(path, lang)
	a.pinLSPRoot(path, serverKey, workDir)
	sent := make(chan struct{})
	a.DocVersionsMu.Lock()
	epoch := a.lspEpochs[serverKey]
	a.DocVersions[path] = 1
	a.lspSent[path] = sent
	a.DocVersionsMu.Unlock()
	slog.Debug("lsp didOpen", "path", path, "language", langID)
	go func() {
		defer close(sent)
		// A restart repins the document and reopens it on the root it resolves
		// to now, so this open must not create a client for a root the document
		// has since left.
		current := func() bool { return a.lspOpenStillCurrent(serverKey, path, workDir, epoch) }
		client, err := a.LspManager.ClientForLanguageCurrent(serverKey, workDir, current)
		if err != nil {
			return
		}
		// The lookup may hand back a client another root started, so the open
		// still has to be current to reach that server.
		if !current() {
			return
		}
		if err := client.DidOpen(FileURI(path), langID, text); err != nil {
			a.LogOutputAsync("error", "lsp:"+serverKey, fmt.Sprintf("didOpen %s: %v", path, err))
		}
	}()
}

// lspEpochIsCurrent reports whether epoch is still the server's latest restart
// generation.
func (a *App) lspEpochIsCurrent(server string, epoch int) bool {
	a.DocVersionsMu.Lock()
	current := a.lspEpochs[server] == epoch
	a.DocVersionsMu.Unlock()
	return current
}

// lspOpenStillCurrent reports whether a didOpen resolved for workDir under
// epoch still targets the document's current root. A restart bumps the epoch
// and repins every open document, so an open queued before it is obsolete.
func (a *App) lspOpenStillCurrent(server, path, workDir string, epoch int) bool {
	if !a.lspEpochIsCurrent(server, epoch) {
		return false
	}
	root, pinned := a.pinnedLSPRoot(path)
	return pinned && root == workDir
}

// The returned channel closes once the didChange send attempt finishes,
// so callers that must not race ahead of it (e.g. a signature-help request
// for the same keystroke) can wait on it.
func (a *App) NotifyLSPChange(path, lang string, lines []string) <-chan struct{} {
	done := make(chan struct{})
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		close(done)
		return done
	}
	text := strings.Join(lines, "\n")
	workDir := a.lspWorkDirFor(path, lang)
	a.DocVersionsMu.Lock()
	a.DocVersions[path]++
	version := a.DocVersions[path]
	epoch := a.lspEpochs[serverKey]
	previous := a.lspSent[path]
	a.lspSent[path] = done
	a.DocVersionsMu.Unlock()
	go func() {
		defer close(done)
		// Each change carries the full text, so one sent out of order would
		// leave the server on an older version.
		if previous != nil {
			<-previous
		}
		// A restart reopens the document with its current text; a change from
		// before it would reach a server that has not opened the document yet.
		a.DocVersionsMu.Lock()
		restarted := a.lspEpochs[serverKey] != epoch
		a.DocVersionsMu.Unlock()
		if restarted {
			return
		}
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return
		}
		if err := client.DidChange(FileURI(path), text, version); err != nil {
			a.LogOutputAsync("error", "lsp:"+serverKey, fmt.Sprintf("didChange %s: %v", path, err))
		}
	}()
	return done
}

func (a *App) NotifyLSPSave(path, lang string, lines []string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		return
	}
	text := strings.Join(lines, "\n")
	workDir := a.lspWorkDirFor(path, lang)
	sent := make(chan struct{})
	a.DocVersionsMu.Lock()
	previous := a.lspSent[path]
	a.lspSent[path] = sent
	a.DocVersionsMu.Unlock()
	go func() {
		defer close(sent)
		if previous != nil {
			<-previous
		}
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return
		}
		if err := client.DidSave(FileURI(path), text); err != nil {
			a.LogOutputAsync("error", "lsp:"+serverKey, fmt.Sprintf("didSave %s: %v", path, err))
		}
	}()
}

func (a *App) NotifyLSPClose(path, lang string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		// A stale pin would send the next open to a root resolved before the
		// document was closed.
		a.unpinLSPRoot(path)
		return
	}
	workDir := a.lspWorkDirFor(path, lang)
	a.unpinLSPRoot(path)
	a.scheduleLSPIdleStop(serverKey, workDir)
	a.DocVersionsMu.Lock()
	delete(a.DocVersions, path)
	delete(a.lspSent, path)
	a.DocVersionsMu.Unlock()
	// Reopening starts the version over at 1, which would match this entry.
	delete(a.lastPulled, path)
	// A server that is not running has nothing to close; starting one here
	// would also revive a root whose idle server was stopped. An open still
	// waiting for startup sees the pin gone and skips its didOpen.
	client := a.LspManager.ReadyClient(serverKey, workDir)
	if client == nil {
		return
	}
	go func() {
		if err := client.DidClose(FileURI(path)); err != nil {
			a.LogOutputAsync("error", "lsp:"+serverKey, fmt.Sprintf("didClose %s: %v", path, err))
		}
	}()
}
