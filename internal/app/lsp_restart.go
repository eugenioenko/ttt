package app

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
)

type LSPServerStopped struct {
	Server  string
	WorkDir string
}

type LSPServerRestarted struct {
	Server string
	Err    error
}

func lspDiagnosticsSource(server string) string {
	return "lsp:" + server
}

func (a *App) RestartLSPServer() {
	path, lang := a.editorPathLang()
	serverKey, _, ok := a.lspResolve(path, lang)
	if path == "" || !ok {
		a.StatusWarn("No language server for the active file")
		return
	}
	workDir := a.lspWorkDir(path)
	a.StatusNotify(fmt.Sprintf("Restarting %s…", serverKey))
	go func() {
		a.LspManager.Stop(serverKey)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&LSPServerStopped{Server: serverKey, WorkDir: workDir}))
	}()
}

// Runs only after the old client's read loop has exited, so every diagnostic it
// posted is already queued ahead of this event and is cleared here rather than
// landing after the restart.
func (a *App) handleLSPServerStopped(ev *LSPServerStopped) {
	a.EditorGroup.ClearDiagnosticsSource(lspDiagnosticsSource(ev.Server))

	type openDoc struct{ uri, langID, text string }
	var docs []openDoc
	a.DocVersionsMu.Lock()
	for _, d := range a.EditorGroup.OpenDocuments() {
		key, langID, ok := a.lspResolve(d.Path, d.Language)
		if !ok || key != ev.Server {
			continue
		}
		a.DocVersions[d.Path] = 1
		docs = append(docs, openDoc{uri: FileURI(d.Path), langID: langID, text: d.Text})
	}
	a.DocVersionsMu.Unlock()

	go func() {
		client, err := a.LspManager.ClientForLanguage(ev.Server, ev.WorkDir)
		if err == nil {
			for _, d := range docs {
				client.DidOpen(d.uri, d.langID, d.text)
			}
		}
		a.Screen.PostEvent(tcell.NewEventInterrupt(&LSPServerRestarted{Server: ev.Server, Err: err}))
	}()
}

func (a *App) handleLSPServerRestarted(ev *LSPServerRestarted) {
	if ev.Err != nil {
		a.StatusWarn(fmt.Sprintf("%s failed to restart: %v", ev.Server, ev.Err))
		return
	}
	a.StatusNotify(ev.Server + " restarted")
	a.SyncLanguageSegment()
	a.RefreshSymbols()
}
