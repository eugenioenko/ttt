package app

import (
	"context"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/dap"
)

func (a *App) runDebugTerminal(ctx context.Context, r *debugRun, req dap.TerminalRequest, deliver func(func())) (int, error) {
	type result struct {
		pid int
		err error
	}
	done := make(chan result, 1)
	deliver(func() {
		if ctx.Err() != nil || r.ctx.Err() != nil {
			return
		}
		var env []string
		for key, value := range req.Env {
			entry := key
			if value != nil {
				entry += "=" + *value
			}
			env = append(env, entry)
		}
		cwd := req.Cwd
		if !filepath.IsAbs(cwd) {
			cwd = filepath.Join(r.folder, cwd)
		}
		id, err := a.SpawnTerminalSpec(TerminalSpec{Shell: req.Args[0], Args: req.Args[1:], Env: env, Cwd: cwd, Label: req.Title})
		var pid int
		if err == nil {
			pid = a.terminalByID(id).Term.PID()
			if !r.attached {
				r.terminalIDs = append(r.terminalIDs, id)
			}
		}
		done <- result{pid, err}
	})
	select {
	case res := <-done:
		return res.pid, res.err
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	}
}

func (a *App) closeDebugTerminals(r *debugRun) {
	for _, id := range r.terminalIDs {
		if tab := a.terminalByID(id); tab != nil && !tab.Term.Exited() {
			a.CloseTerminal(id)
		}
	}
	r.terminalIDs = nil
}
