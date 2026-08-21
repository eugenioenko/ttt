package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
)

func newListenTestApp(t *testing.T) *App {
	t.Helper()
	a := buildTestApp(t, config.DefaultSettings())

	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(80, 24)
	a.Screen = term.NewTcellScreenFrom(sim)
	a.Renderer = &render.Renderer{}
	a.Reg = command.NewRegistry()
	running := true
	a.Running = &running
	RegisterCommands(a)
	a.Root.SetSize(80, 24)

	return a
}

func execHandlerRequest(t *testing.T, a *App, method, path, body string) *http.Response {
	t.Helper()
	srv := httptest.NewServer(NewExecHandler(a))
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestExecHandlerRunsScriptSynchronously(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodPost, "/exec", "quit")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// execQuit sets *a.Running = false before posting its interrupt event, so
	// by the time RunExecScriptSep (and therefore the handler) returns, the
	// script's effect is already observable -- proof it ran synchronously.
	if *a.Running {
		t.Error("expected quit script to have set Running to false")
	}
}

func TestExecHandlerShutdownAliasesQuit(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodPost, "/exec", "shutdown")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if *a.Running {
		t.Error("expected shutdown script to have set Running to false")
	}
}

func TestExecHandlerRejectsEmptyBody(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodPost, "/exec", "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestExecHandlerRejectsOversizedBody(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodPost, "/exec", strings.Repeat("a", maxExecBodySize+1))

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}

func TestExecHandlerRejectsNonPost(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodGet, "/exec", "")

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

func TestExecHandlerRespectsSepQueryParam(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodPost, "/exec?sep=,", "wait 10,quit")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if *a.Running {
		t.Error("expected script split on custom separator to run both commands, including quit")
	}
}
