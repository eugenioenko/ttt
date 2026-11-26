package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/lsp"

	"github.com/gdamore/tcell/v3"
)

type LSPServerStopped struct {
	Server  string
	WorkDir string
	// Epoch is the restart generation that stopped the server. A later restart
	// bumps the epoch and reopens the documents itself, so this event must not.
	Epoch int
}

type LSPServerRestarted struct {
	Server string
	Err    error
}

func lspDiagnosticsSource(server string) string {
	return "lsp:" + server
}

// Pulled diagnostics have their own source: each source holds one list per
// file, so a server that both pushes and pulls would otherwise have each
// replace the other.
func lspPullDiagnosticsSource(server string) string {
	return "lsp:" + server + ":pull"
}

func (a *App) RestartLSPServer() {
	path, lang := a.editorPathLang()
	serverKey, _, ok := a.lspResolve(path, lang)
	if path == "" || !ok {
		a.StatusWarn(i18n.Text("status.noLanguageServerForTheActiveFile"))
		return
	}
	workDir := a.lspWorkDirFor(path, lang)
	// Raised before the old server stops, since a change made while it stops
	// would otherwise start the new server and reach it before the reopen.
	a.DocVersionsMu.Lock()
	a.lspEpochs[serverKey]++
	epoch := a.lspEpochs[serverKey]
	a.DocVersionsMu.Unlock()
	a.StatusNotify(i18n.Format("status.restarting", i18n.Args{"serverKey": serverKey}))
	go func() {
		a.LspManager.Stop(serverKey)
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&LSPServerStopped{Server: serverKey, WorkDir: workDir, Epoch: epoch}))
	}()
}

// Runs only after the old client's read loop has exited, so every diagnostic it
// posted is already queued ahead of this event and is cleared here rather than
// landing after the restart.
func (a *App) handleLSPServerStopped(ev *LSPServerStopped) {
	// A restart that began while this one was stopping has already repinned and
	// reopened every document under the newer epoch.
	if !a.lspEpochIsCurrent(ev.Server, ev.Epoch) {
		return
	}
	for _, group := range a.editorGroups() {
		group.ClearDiagnosticsSource(lspDiagnosticsSource(ev.Server))
		group.ClearDiagnosticsSource(lspPullDiagnosticsSource(ev.Server))
	}
	type openDoc struct {
		path, uri, langID, text, workDir string
		inFlight                         <-chan struct{}
	}
	var docs []openDoc
	// Restarting re-resolves every root: a marker created or removed since the
	// documents were opened (possibly in a directory the watcher does not see)
	// takes effect here.
	a.invalidateLSPRoots()
	for _, d := range a.openEditorDocuments() {
		key, langID, ok := a.lspResolve(d.Path, d.Language)
		if !ok || key != ev.Server {
			continue
		}
		a.unpinLSPRoot(d.Path)
		workDir := a.lspWorkDirFor(d.Path, d.Language)
		a.pinLSPRoot(d.Path, workDir)
		docs = append(docs, openDoc{
			path:    d.Path,
			uri:     FileURI(d.Path),
			langID:  langID,
			text:    d.Text,
			workDir: workDir,
		})
	}
	sent := make(chan struct{})
	a.DocVersionsMu.Lock()
	for i := range docs {
		docs[i].inFlight = a.lspSent[docs[i].path]
		a.DocVersions[docs[i].path] = 1
		a.lspSent[docs[i].path] = sent
	}
	a.DocVersionsMu.Unlock()

	go func() {
		// A send already past its restart check must reach the server before
		// the reopen, or it would replace the current text.
		for _, d := range docs {
			if d.inFlight != nil {
				<-d.inFlight
			}
		}
		// Reopen each document on the process rooted at its own project. A root
		// that fails must not keep the other roots' documents from reopening, so
		// its remaining documents are skipped instead of ending the loop.
		// Reopening is an open like any other: a later restart repins and reopens
		// the document itself, so this one must not start a client or send a
		// didOpen for a root it no longer uses.
		current := func(d openDoc) bool {
			return a.lspOpenStillCurrent(ev.Server, d.path, d.workDir, ev.Epoch)
		}
		clients := make(map[string]*lsp.Client)
		failed := make(map[string]bool)
		var firstErr error
		for _, d := range docs {
			if failed[d.workDir] || !current(d) {
				continue
			}
			client := clients[d.workDir]
			if client == nil {
				var err error
				client, err = a.LspManager.ClientForLanguageCurrent(ev.Server, d.workDir, func() bool { return current(d) })
				if err != nil {
					if errors.Is(err, context.Canceled) {
						continue
					}
					failed[d.workDir] = true
					a.LogOutputAsync("error", "lsp:"+ev.Server, err.Error())
					if firstErr == nil {
						firstErr = err
					}
					continue
				}
				clients[d.workDir] = client
			}
			if !current(d) {
				continue
			}
			if err := client.DidOpen(d.uri, d.langID, d.text); err != nil {
				err = fmt.Errorf("didOpen %s: %w", d.uri, err)
				failed[d.workDir] = true
				a.LogOutputAsync("error", "lsp:"+ev.Server, err.Error())
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		close(sent)
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&LSPServerRestarted{Server: ev.Server, Err: firstErr}))
	}()
}

func (a *App) handleLSPServerRestarted(ev *LSPServerRestarted) {
	if ev.Err != nil {
		a.StatusWarn(i18n.Format("status.failedToRestart", i18n.Args{"server": ev.Server, "err": ev.Err}))
		return
	}
	a.StatusNotify(i18n.Format("status.restarted", i18n.Args{"server": ev.Server}))
	a.SyncLanguageSegment()
	a.RefreshSymbols()
	a.ScheduleDiagnosticPull(true)
}
