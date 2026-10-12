package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/eugenioenko/ttt/internal/lsp"
)

const pullDiagnosticsDelay = 500 * time.Millisecond

// pullKey identifies the document state a pull answers for. epoch is needed
// because a server restart resets document versions to 1.
type pullKey struct {
	path    string
	version int
	epoch   int
}

type pulledDiagnostics struct {
	diags []lsp.Diagnostic
	// replace is false when the existing diagnostics stand: an "unchanged"
	// report, or a server that only pushes.
	replace bool
}

// Background documents keep their last pulled diagnostics until they become
// active again.
func (a *App) ScheduleDiagnosticPull(immediate bool) {
	path, lang := a.editorPathLang()
	serverKey, _, ok := a.lspResolve(path, lang)
	if path == "" || !ok || a.diagPull == nil {
		return
	}
	a.DocVersionsMu.Lock()
	version, opened := a.DocVersions[path]
	sent := a.lspSent[path]
	a.DocVersionsMu.Unlock()
	if !opened {
		return
	}
	key := pullKey{path: path, version: version, epoch: a.lspEpochs[serverKey]}
	workDir := a.lspWorkDir(path)
	a.diagPull.ScheduleKey(key, func(ctx context.Context) (pulledDiagnostics, error) {
		// The server must have this version before it is asked about it.
		if sent != nil {
			select {
			case <-sent:
			case <-ctx.Done():
				return pulledDiagnostics{}, ctx.Err()
			}
		}
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			return pulledDiagnostics{}, err
		}
		if !client.SupportsPullDiagnostics() {
			return pulledDiagnostics{}, nil
		}
		diags, replace, err := client.Diagnostic(ctx, FileURI(path))
		return pulledDiagnostics{diags: diags, replace: replace}, err
	}, func(res pulledDiagnostics, err error) {
		a.applyPulledDiagnostics(serverKey, key, res, err)
	})
	if immediate {
		a.diagPull.Flush()
	}
}

func (a *App) applyPulledDiagnostics(server string, key pullKey, res pulledDiagnostics, err error) {
	if err != nil {
		slog.Debug("lsp diagnostic pull failed", "server", server, "path", key.path, "err", err)
		// The task keeps a failed key, which would dedupe every retry of the
		// same document version.
		a.diagPull.Invalidate()
		if errors.Is(err, lsp.ErrRetriggerDiagnostic) {
			a.ScheduleDiagnosticPull(false)
		}
		return
	}
	if !res.replace {
		return
	}
	a.DocVersionsMu.Lock()
	version, open := a.DocVersions[key.path]
	a.DocVersionsMu.Unlock()
	if !open || version != key.version || a.lspEpochs[server] != key.epoch {
		return
	}
	a.EditorGroup.SetDiagnosticsSource(lspPullDiagnosticsSource(server), key.path, LspToUIDiagnostics(res.diags))
}
