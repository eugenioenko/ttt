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
	a := prepareListenTestApp(t)
	startListenTestEventLoop(t, a)
	return a
}

func prepareListenTestApp(t *testing.T) *App {
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

func startListenTestEventLoop(t *testing.T, a *App) {
	t.Helper()
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

func execHandlerDirect(t *testing.T, a *App, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	response := httptest.NewRecorder()
	NewExecHandler(a).ServeHTTP(response, req)
	return response
}

func TestExecHandlerAppliesQuitAfterWritingResponse(t *testing.T) {
	a := newListenTestApp(t)

	response := execHandlerDirect(t, a, "/exec", "quit")

	if response.Code != http.StatusOK || response.Body.String() != "ok" || !response.Flushed {
		t.Fatalf("response = status %d body %q flushed %v", response.Code, response.Body.String(), response.Flushed)
	}
	if *a.Running {
		t.Error("handler returned before applying quit")
	}
}

func TestExecHandlerShutdownAliasesQuit(t *testing.T) {
	a := newListenTestApp(t)

	response := execHandlerDirect(t, a, "/exec", "shutdown")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if response.Body.String() != "ok" || response.Header().Get("Content-Length") != "2" || !response.Flushed {
		t.Fatalf("response = %q length %q flushed %v, want flushed two-byte success", response.Body.String(), response.Header().Get("Content-Length"), response.Flushed)
	}
	if *a.Running {
		t.Error("handler returned before applying shutdown")
	}
}

func TestExecHandlerRepeatedShutdownReturnsAfterLoopStopped(t *testing.T) {
	a := newListenTestApp(t)
	first := execHandlerDirect(t, a, "/exec", "shutdown")
	if first.Code != http.StatusOK || first.Body.String() != "ok" {
		t.Fatalf("first response = status %d body %q, want 200 ok", first.Code, first.Body.String())
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	handler := NewExecHandler(a)
	go func() {
		request := httptest.NewRequest(http.MethodPost, "/exec", strings.NewReader("shutdown"))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		done <- response
	}()
	select {
	case second := <-done:
		if second.Code != http.StatusOK || second.Body.String() != "ok" {
			t.Fatalf("second response = status %d body %q, want 200 ok", second.Code, second.Body.String())
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("repeated HTTP shutdown blocked after event loop stopped")
	}
}

func TestListenAddressUsesLoopbackPortOverride(t *testing.T) {
	t.Setenv("TTT_LISTEN_PORT", "54321")
	if got := ListenAddress(); got != "127.0.0.1:54321" {
		t.Fatalf("ListenAddress() = %q, want loopback override", got)
	}

	for _, invalid := range []string{"", "0", "65536", "not-a-port"} {
		t.Run(invalid, func(t *testing.T) {
			t.Setenv("TTT_LISTEN_PORT", invalid)
			if got := ListenAddress(); got != ListenAddr {
				t.Fatalf("ListenAddress() = %q, want default %q", got, ListenAddr)
			}
		})
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

	response := execHandlerDirect(t, a, "/exec?sep=,", "wait 10,quit")

	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("response = status %d body %q, want 200 ok", response.Code, response.Body.String())
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
	var screen string
	if err := runExecMain(a, func() error {
		screen = a.Screenshot()
		return nil
	}); err != nil {
		t.Fatalf("capturing screen: %v", err)
	}
	if !strings.Contains(screen, "hello") {
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
