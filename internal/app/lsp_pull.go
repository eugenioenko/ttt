package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
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
	root    string
}

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
	workDir := a.lspWorkDirFor(path, lang)
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
		return pulledDiagnostics{diags: diags, replace: replace, root: workDir}, err
	}, func(res pulledDiagnostics, err error) {
		a.applyPulledDiagnostics(serverKey, key, res, err)
	})
	if immediate {
		a.diagPull.Flush()
	}
}

type LSPDiagnosticRefresh struct {
	Server string
}

type backgroundPull struct {
	key pullKey
	res pulledDiagnostics
	err error
}

type pullCandidate struct {
	key       pullKey
	sent      <-chan struct{}
	unchanged bool
	workDir   string
}

func (a *App) backgroundPullTask(server string) *asynctask.Task[[]backgroundPull] {
	if a.diagBackground == nil {
		return nil
	}
	task := a.diagBackground[server]
	if task == nil {
		task = NewTask[[]backgroundPull](a, asynctask.TaskOptions{Delay: pullDiagnosticsDelay})
		a.diagBackground[server] = task
	}
	return task
}

// cancelBackgroundPull stops the background pull of the edited document's
// server. Its documents count as pulled only once a result is applied, so the
// pull after the next active one covers them.
func (a *App) cancelBackgroundPull(path, lang string) {
	if server, _, ok := a.lspResolve(path, lang); ok {
		if task := a.diagBackground[server]; task != nil {
			task.Stop()
		}
	}
}

func (a *App) handleDiagnosticRefresh(server string) {
	for _, d := range a.openEditorFiles() {
		if key, _, ok := a.lspResolve(d.Path, d.Language); ok && key == server {
			delete(a.lastPulled, d.Path)
		}
	}
	// An active pull from before the refresh would otherwise apply its result
	// and replace this run with one that skips the active document.
	if path, lang := a.editorPathLang(); path != "" {
		if key, _, ok := a.lspResolve(path, lang); ok && key == server {
			a.diagPull.Stop()
		}
	}
	a.pullOpenDocuments(server, "", true)
}

// pullOpenDocuments pulls the documents open on server other than exclude. A
// server without interFileDependencies only gets the ones that changed since
// their last applied pull.
func (a *App) pullOpenDocuments(server, exclude string, immediate bool) {
	task := a.backgroundPullTask(server)
	if task == nil {
		return
	}
	files := a.openEditorFiles()
	// FindRoot reads the filesystem; resolve each document's root before
	// taking the versions lock the UI thread also uses.
	workDirs := make(map[string]string, len(files))
	for _, d := range files {
		if d.Path == exclude {
			continue
		}
		if key, _, ok := a.lspResolve(d.Path, d.Language); ok && key == server {
			workDirs[d.Path] = a.lspWorkDirFor(d.Path, d.Language)
		}
	}
	var candidates []pullCandidate
	a.DocVersionsMu.Lock()
	epoch := a.lspEpochs[server]
	for _, d := range files {
		if d.Path == exclude {
			continue
		}
		key, _, ok := a.lspResolve(d.Path, d.Language)
		if !ok || key != server {
			continue
		}
		version, opened := a.DocVersions[d.Path]
		if !opened {
			continue
		}
		k := pullKey{path: d.Path, version: version, epoch: epoch}
		candidates = append(candidates, pullCandidate{
			key:       k,
			sent:      a.lspSent[d.Path],
			unchanged: a.lastPulled[d.Path] == k,
			workDir:   workDirs[d.Path],
		})
	}
	a.DocVersionsMu.Unlock()
	if len(candidates) == 0 {
		return
	}
	task.Schedule(func(ctx context.Context) ([]backgroundPull, error) {
		// A restart raises the epoch before stopping the server, which
		// ClientForLanguage would otherwise start again ahead of the reopen.
		a.DocVersionsMu.Lock()
		restarted := a.lspEpochs[server] != epoch
		a.DocVersionsMu.Unlock()
		if restarted {
			return nil, nil
		}
		// Each root has its own client; a document must be pulled through the
		// server process that has its root registered. A root that fails to
		// initialize must not keep the other roots' documents from being pulled,
		// so its error is reported per document instead of ending the task.
		type rootClient struct {
			client *lsp.Client
			err    error
		}
		clients := make(map[string]rootClient)
		for _, c := range candidates {
			if _, ok := clients[c.workDir]; ok {
				continue
			}
			client, err := a.LspManager.ClientForLanguage(server, c.workDir)
			if err != nil {
				clients[c.workDir] = rootClient{err: err}
				continue
			}
			if !client.SupportsPullDiagnostics() {
				clients[c.workDir] = rootClient{}
				continue
			}
			clients[c.workDir] = rootClient{client: client}
		}
		var pulls []backgroundPull
		for _, c := range candidates {
			if err := ctx.Err(); err != nil {
				return pulls, err
			}
			root := clients[c.workDir]
			if root.err != nil {
				pulls = append(pulls, backgroundPull{key: c.key, err: root.err})
				continue
			}
			client := root.client
			if client == nil {
				continue
			}
			if c.unchanged && !client.InterFileDependencies() {
				continue
			}
			if c.sent != nil {
				select {
				case <-c.sent:
				case <-ctx.Done():
					return pulls, ctx.Err()
				}
			}
			diags, replace, err := client.Diagnostic(ctx, FileURI(c.key.path))
			if ctx.Err() != nil {
				return pulls, ctx.Err()
			}
			pulls = append(pulls, backgroundPull{key: c.key, res: pulledDiagnostics{diags: diags, replace: replace, root: c.workDir}, err: err})
		}
		return pulls, nil
	}, func(pulls []backgroundPull, err error) {
		if err != nil {
			slog.Debug("lsp background diagnostic pull failed", "server", server, "err", err)
		}
		retrigger := false
		for _, p := range pulls {
			if p.err != nil {
				slog.Debug("lsp diagnostic pull failed", "server", server, "path", p.key.path, "err", p.err)
				retrigger = retrigger || errors.Is(p.err, lsp.ErrRetriggerDiagnostic)
				continue
			}
			a.setPulledDiagnostics(server, p.key, p.res)
		}
		if retrigger {
			a.pullOpenDocuments(server, exclude, false)
		}
	})
	if immediate {
		task.Flush()
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
	if a.setPulledDiagnostics(server, key, res) {
		a.pullOpenDocuments(server, key.path, true)
	}
}

func (a *App) setPulledDiagnostics(server string, key pullKey, res pulledDiagnostics) bool {
	if !res.replace {
		return false
	}
	a.DocVersionsMu.Lock()
	version, open := a.DocVersions[key.path]
	a.DocVersionsMu.Unlock()
	if !open || version != key.version || a.lspEpochs[server] != key.epoch {
		return false
	}
	a.lastPulled[key.path] = key
	source := lspPullDiagnosticsSource(server)
	a.recordLSPDiagRoot(server, source, key.path, res.root, len(res.diags) > 0)
	for _, group := range a.editorGroups() {
		group.SetDiagnosticsSource(source, key.path, LspToUIDiagnostics(res.diags, a.lspLines(key.path)))
	}
	return true
}
