package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
)

func symbolsTestApp(t *testing.T, name, content string) (*App, *[]func()) {
	t.Helper()
	a := completionTestApp(t, name, content)
	var deliveries []func()
	a.tasks.post = func(fn func()) error {
		deliveries = append(deliveries, fn)
		return nil
	}
	a.symbolsTask = NewTask[symbolsReply](a, asynctask.TaskOptions{Run: func(fn func()) { fn() }})
	return a, &deliveries
}

func symbolReply(name string) symbolsReply {
	return symbolsReply{symbols: []lsp.DocumentSymbol{{Name: name, Kind: lsp.SKFunction}}}
}

func queueSymbolsReply(a *App, path string, reply symbolsReply) {
	a.scheduleSymbols(path, func(context.Context) (symbolsReply, error) { return reply, nil })
}

func assertOutlineName(t *testing.T, a *App, want string) {
	t.Helper()
	items := a.Symbols.Tree.Config.Items
	if len(items) != 1 || items[0].Label != want {
		t.Fatalf("outline = %v, want one symbol named %q", items, want)
	}
}

func TestSymbolsStaleReplyAfterSwitchingAwayAndBack(t *testing.T) {
	a, deliveries := symbolsTestApp(t, "a.md", "# fallback\n")
	pathA := a.EditorGroup.ActiveFilePath()
	queueSymbolsReply(a, pathA, symbolReply("old A"))
	a.EditorGroup.OpenFile(filepath.Join(filepath.Dir(pathA), "b.txt"))
	queueSymbolsReply(a, a.EditorGroup.ActiveFilePath(), symbolReply("B"))
	a.EditorGroup.OpenFile(pathA)
	queueSymbolsReply(a, pathA, symbolReply("new A"))
	if len(*deliveries) != 3 {
		t.Fatalf("queued replies = %d, want 3", len(*deliveries))
	}
	(*deliveries)[2]()
	assertOutlineName(t, a, "new A")
	(*deliveries)[1]()
	(*deliveries)[0]()
	assertOutlineName(t, a, "new A")
}

func TestSymbolsSameFileRefreshAfterError(t *testing.T) {
	a, deliveries := symbolsTestApp(t, "a.md", "# fallback\n")
	path := a.EditorGroup.ActiveFilePath()
	version := a.EditorGroup.ActiveBuffer().Version()
	queueSymbolsReply(a, path, symbolsReply{status: "LSP: temporary error"})
	(*deliveries)[0]()
	if a.Symbols.Tree.Config.EmptyText != "LSP: temporary error" {
		t.Fatal("error status was not applied")
	}
	for _, name := range []string{"after error", "same version refresh"} {
		queueSymbolsReply(a, path, symbolReply(name))
		(*deliveries)[len(*deliveries)-1]()
		assertOutlineName(t, a, name)
	}
	if a.EditorGroup.ActiveBuffer().Version() != version {
		t.Fatal("refresh test changed the buffer version")
	}
}

func TestSymbolsSynchronousPathsCancelPreviousReply(t *testing.T) {
	for _, tc := range []struct {
		name, file, content, lang, want string
		missing                         bool
	}{
		{name: "fallback", file: "a.md", content: "# fallback\n", lang: "markdown", want: "fallback"},
		{name: "no server", file: "a.js", lang: "javascript", want: "No language server for javascript"},
		{name: "missing server with fallback", file: "a.md", content: "# fallback\n", lang: "markdown", want: "fallback", missing: true},
		{name: "missing server", file: "a.js", lang: "javascript", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, deliveries := symbolsTestApp(t, tc.file, tc.content)
			path := a.EditorGroup.ActiveFilePath()
			a.LspManager = lsp.NewManager(&config.LSPSettings{})
			if tc.missing {
				command := filepath.Join(t.TempDir(), "missing-server")
				a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
					tc.lang: {Command: []string{command}, Languages: map[string]string{filepath.Ext(path): tc.lang}},
				}})
				if tc.want == "" {
					tc.want = command + " is not installed"
				}
			}
			queueSymbolsReply(a, path, symbolReply("stale"))
			a.RequestDocumentSymbols(path, tc.lang)
			(*deliveries)[0]()
			if tc.want == "fallback" {
				assertOutlineName(t, a, tc.want)
			} else if len(a.Symbols.Tree.Config.Items) != 0 || a.Symbols.Tree.Config.EmptyText != tc.want {
				t.Fatalf("outline status = %q, want %q", a.Symbols.Tree.Config.EmptyText, tc.want)
			}
		})
	}
}

func TestSymbolsReplyInvalidation(t *testing.T) {
	for _, action := range []string{"other file", "hidden panel", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			a, deliveries := symbolsTestApp(t, "a.md", "# fallback\n")
			path := a.EditorGroup.ActiveFilePath()
			queueSymbolsReply(a, path, symbolReply("stale"))
			switch action {
			case "other file":
				a.EditorGroup.OpenFile(filepath.Join(filepath.Dir(path), "b.txt"))
			case "hidden panel":
				a.Sidebar.ActivePanel = "explorer"
				a.RefreshSymbols()
			case "shutdown":
				a.CloseTasks()
			}
			(*deliveries)[0]()
			if a.Symbols.Tree.ItemCount() != 0 {
				t.Fatal("invalidated reply replaced the outline")
			}
		})
	}
}

func TestSymbolsRetryCancelsWithoutWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	called := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := readSymbolsWithRetry(ctx, func(context.Context) ([]lsp.DocumentSymbol, error) {
			close(called)
			return nil, errors.New("temporary error")
		})
		done <- err
	}()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("initial request did not run")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("retry error = %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled request waited for the 1.5 second retry timer")
	}
}
