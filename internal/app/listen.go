package app

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// ListenAddr is the bind address for the --listen HTTP command server.
// Loopback-only: POST /exec runs arbitrary key/type/click input against the
// running editor, so this must never be reachable off the local machine.
const ListenAddr = "127.0.0.1:4242"

// NewExecHandler returns the http.Handler backing the --listen server's
// POST /exec endpoint. Split out from StartListenServer so tests can drive
// it through httptest.Server without binding a real socket.
func NewExecHandler(a *App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/exec", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
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
		RunExecScriptSep(a, script, sep)
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	})
	return mux
}

// StartListenServer starts the --listen HTTP command server. Blocks until
// the listener fails, so callers run it in a goroutine.
func StartListenServer(a *App) {
	ln, err := net.Listen("tcp", ListenAddr)
	if err != nil {
		slog.Error("listen: failed to bind", "addr", ListenAddr, "error", err)
		return
	}
	slog.Info("listen: HTTP command server started", "addr", ListenAddr)
	if err := http.Serve(ln, NewExecHandler(a)); err != nil {
		slog.Error("listen: server stopped", "error", err)
	}
}
