package app

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/lsp"
)

type symbolsReply struct {
	symbols []lsp.DocumentSymbol
	status  string
}

func (a *App) RequestDocumentSymbols(path, lang string) {
	a.stopSymbolsTask()
	samePath := a.Symbols.Path == path
	a.Symbols.Path = path
	fallback := a.fallbackSymbols(path, lang)
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if len(fallback) == 0 && lang != "" && !strings.EqualFold(lang, "plaintext") {
			a.Symbols.SetStatus("No language server for " + lang)
			return
		}
		a.ApplySymbols(fallback)
		return
	}
	serverCfg := a.LspManager.ServerConfig(serverKey)
	if len(serverCfg.Command) > 0 {
		if _, err := exec.LookPath(serverCfg.Command[0]); err != nil {
			if len(fallback) > 0 {
				a.ApplySymbols(fallback)
			} else {
				a.Symbols.SetStatus(serverCfg.Command[0] + " is not installed")
			}
			return
		}
	}
	// Keep the current outline on same-file refreshes to avoid flicker on save.
	if len(fallback) > 0 {
		a.ApplySymbols(fallback)
	} else if !samePath || a.Symbols.Tree.ItemCount() == 0 {
		a.Symbols.SetStatus("Loading symbols…")
	}
	workDir := a.lspWorkDir(path)
	manager := a.LspManager
	a.scheduleSymbols(path, func(ctx context.Context) (symbolsReply, error) {
		client, err := manager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return symbolsReply{symbols: fallback, status: "Language server unavailable"}, nil
		}
		symbols, err := readSymbolsWithRetry(ctx, func(ctx context.Context) ([]lsp.DocumentSymbol, error) {
			return client.DocumentSymbols(ctx, FileURI(path))
		})
		if err != nil {
			if ctx.Err() != nil {
				return symbolsReply{}, ctx.Err()
			}
			slog.Error("lsp documentSymbol", "err", err)
			return symbolsReply{symbols: fallback, status: lspStatusMessage(err)}, nil
		}
		return symbolsReply{symbols: symbols}, nil
	})
}

func (a *App) scheduleSymbols(path string, work func(context.Context) (symbolsReply, error)) {
	if a.symbolsTask == nil {
		a.symbolsTask = NewTask[symbolsReply](a, asynctask.TaskOptions{})
	}
	// Save, restart, and panel activation must refresh even an unchanged file.
	a.symbolsTask.Schedule(work, func(reply symbolsReply, err error) {
		if err == nil {
			a.applySymbolsReply(path, reply)
		}
	})
}

func (a *App) applySymbolsReply(path string, reply symbolsReply) {
	if path != a.EditorGroup.ActiveFilePath() {
		return
	}
	if len(reply.symbols) == 0 && reply.status != "" {
		a.Symbols.SetStatus(reply.status)
	} else {
		a.ApplySymbols(reply.symbols)
	}
}

func (a *App) stopSymbolsTask() {
	if a.symbolsTask != nil {
		a.symbolsTask.Stop()
	}
}

func readSymbolsWithRetry(ctx context.Context, read func(context.Context) ([]lsp.DocumentSymbol, error)) ([]lsp.DocumentSymbol, error) {
	symbols, err := read(ctx)
	if err == nil {
		return symbols, nil
	}
	// One retry absorbs cold-start latency and didOpen races.
	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return read(ctx)
	}
}
