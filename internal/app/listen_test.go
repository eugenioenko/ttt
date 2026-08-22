package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	loopDone := make(chan struct{})
	go func() {
		RunEventLoop(a.Screen, a.Renderer, a, a.Running, a.CloseTerminal)
		close(loopDone)
	}()
	t.Cleanup(func() {
		select {
		case <-loopDone:
			return
		default:
		}
		_ = StopExecLoop(a)
		select {
		case <-loopDone:
		case <-time.After(time.Second):
			t.Error("event loop did not stop")
		}
	})

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
	// The handler returns only after the event loop has acknowledged quit.
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

func TestExecHandlerWaitsForAcknowledgedVisibleInput(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodPost, "/exec", `type hello; wait-for hello timeout=500`)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
	if !strings.Contains(a.Screenshot(), "hello") {
		t.Fatal("handler returned before typed text was visible")
	}
}

func TestExecHandlerRejectsInvalidAction(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodPost, "/exec", "not-an-action")
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(string(body), `unknown action "not-an-action"`) {
		t.Fatalf("body = %q, want useful invalid-action error", body)
	}
}

func TestExecHandlerReportsExecutionFailure(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodPost, "/exec", "screenshot /missing/parent/screen.txt")
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	if !strings.Contains(string(body), "screenshot") {
		t.Fatalf("body = %q, want screenshot failure", body)
	}
}

func TestExecHandlerReportsWaitForTimeout(t *testing.T) {
	a := newListenTestApp(t)

	resp := execHandlerRequest(t, a, http.MethodPost, "/exec", `wait-for "never visible" timeout=40`)
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want 408", resp.StatusCode)
	}
	if !strings.Contains(string(body), `screen text "never visible" not visible`) {
		t.Fatalf("body = %q, want timeout detail", body)
	}
}
