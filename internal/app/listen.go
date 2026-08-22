package app

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// ListenAddr is the bind address for the --listen HTTP command server.
// Loopback-only: POST /exec runs arbitrary key/type/click input against the
// running editor, so this must never be reachable off the local machine.
const ListenAddr = "127.0.0.1:4242"

const maxExecBodySize = 1 << 20 // 1MB

func ListenAddress() string {
	port := os.Getenv("TTT_LISTEN_PORT")
	value, err := strconv.ParseUint(port, 10, 16)
	if port == "" || err != nil || value == 0 {
		return ListenAddr
	}
	return net.JoinHostPort("127.0.0.1", port)
}

// NewExecHandler is split out from StartListenServer so tests can drive it
// through httptest.Server. Concurrent /exec requests are not synchronized:
// --listen is a single-operator debug tool, not a public API.
func NewExecHandler(a *App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/exec", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxExecBodySize)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		script := string(body)
		if strings.TrimSpace(script) == "" {
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}
		sep := r.URL.Query().Get("sep")
		if sep == "" {
			sep = DefaultExecSeparator
		}
		result, err := RunExecScriptSep(a, script, sep)
		if err != nil {
			status := http.StatusUnprocessableEntity
			var execErr *ExecError
			if errors.As(err, &execErr) {
				switch execErr.Kind {
				case ExecErrorInvalid:
					status = http.StatusBadRequest
				case ExecErrorTimeout:
					status = http.StatusRequestTimeout
				}
			}
			http.Error(w, err.Error(), status)
			return
		}
		w.Header().Set("Content-Length", "2")
		w.WriteHeader(http.StatusOK)
		written, err := io.WriteString(w, "ok")
		if err != nil || written != 2 {
			slog.Error("listen: failed to write exec response", "written", written, "error", err)
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		if result.ShutdownRequested {
			if err := StopExecLoop(a); err != nil {
				slog.Error("listen: failed to apply exec shutdown", "error", err)
			}
		}
	})
	return mux
}

// StartListenServer starts the --listen HTTP command server. Blocks until
// the listener fails, so callers run it in a goroutine.
func StartListenServer(a *App) {
	addr := ListenAddress()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("listen: failed to bind", "addr", addr, "error", err)
		return
	}
	slog.Info("listen: HTTP command server started", "addr", addr)
	if err := http.Serve(ln, NewExecHandler(a)); err != nil {
		slog.Error("listen: server stopped", "error", err)
	}
}
