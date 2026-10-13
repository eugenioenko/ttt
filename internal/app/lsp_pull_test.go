package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestApplyPulledDiagnosticsDropsSupersededResults(t *testing.T) {
	const path = "/tmp/a.ts"
	pulled := pulledDiagnostics{replace: true, diags: []lsp.Diagnostic{{Message: "pulled"}}}
	existing := []ui.Diagnostic{{Message: "existing"}}

	tests := []struct {
		name  string
		setup func(a *App)
		res   pulledDiagnostics
		err   error
		want  string
	}{
		{name: "current document applies", res: pulled, want: "pulled"},
		{name: "empty full report clears", res: pulledDiagnostics{replace: true}, want: ""},
		{name: "unchanged report keeps existing", res: pulledDiagnostics{}, want: "existing"},
		{name: "error keeps existing", res: pulled, err: errors.New("boom"), want: "existing"},
		{name: "edited since the request", setup: func(a *App) { a.DocVersions[path] = 4 }, res: pulled, want: "existing"},
		{name: "closed since the request", setup: func(a *App) { delete(a.DocVersions, path) }, res: pulled, want: "existing"},
		{name: "server restarted since the request", setup: func(a *App) { a.lspEpochs["typescript"]++ }, res: pulled, want: "existing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := buildTestApp(t, config.DefaultSettings())
			a.DocVersions[path] = 3
			source := lspPullDiagnosticsSource("typescript")
			a.EditorGroup.SetDiagnosticsSource(source, path, existing)
			key := pullKey{path: path, version: 3, epoch: a.lspEpochs["typescript"]}
			if tt.setup != nil {
				tt.setup(a)
			}

			a.applyPulledDiagnostics("typescript", key, tt.res, tt.err)

			got := a.EditorGroup.DiagnosticsByPath()[path]
			var msg string
			if len(got) > 0 {
				msg = got[0].Message
			}
			if len(got) > 1 || msg != tt.want {
				t.Errorf("diagnostics = %+v, want message %q", got, tt.want)
			}
		})
	}
}

func TestFailedDiagnosticPullCanBeRetried(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	key := pullKey{path: "/tmp/a.ts", version: 1}
	runs := 0
	schedule := func(err error) {
		a.diagPull.ScheduleKey(key, func(context.Context) (pulledDiagnostics, error) {
			runs++
			return pulledDiagnostics{}, err
		}, func(res pulledDiagnostics, err error) {
			a.applyPulledDiagnostics("typescript", key, res, err)
		})
	}

	schedule(nil)
	schedule(nil)
	if runs != 1 {
		t.Fatalf("same key after success ran %d times, want 1", runs)
	}
	a.diagPull.Invalidate()
	schedule(errors.New("timed out"))
	schedule(nil)
	if runs != 3 {
		t.Fatalf("same key after failure ran %d times in total, want 3", runs)
	}
}

func TestDidChangeWaitsForThePreviousSend(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"go": {Command: []string{"ttt-test-missing-server"}, Languages: map[string]string{".go": "go"}},
	}})
	const path = "/tmp/a.go"
	previous := make(chan struct{})
	a.DocVersions[path] = 1
	a.lspSent[path] = previous

	done := a.NotifyLSPChange(path, "go", []string{"x"})
	select {
	case <-done:
		t.Fatal("didChange was sent before the previous notification")
	case <-time.After(50 * time.Millisecond):
	}
	close(previous)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("didChange never sent after the previous notification")
	}
}
