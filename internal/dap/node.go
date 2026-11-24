package dap

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"os"
	"path/filepath"
	"time"
)

func (t Target) nodeArguments() (map[string]any, error) {
	if t.Request != "launch" && t.Request != "attach" {
		return nil, fmt.Errorf("%s: unsupported request %s", t.Name, t.Request)
	}
	out := map[string]any{"type": "pwa-node", "request": t.Request, "name": t.Name, "cwd": t.Folder, "__workspaceFolder": t.Folder, "rootPath": t.Folder, "sourceMaps": true, "autoAttachChildProcesses": false}
	if t.Request == "launch" {
		out["console"] = "internalConsole"
		out["outputCapture"] = "std"
	}
	for key, val := range t.Arguments {
		switch key {
		case "program", "args", "env", "runtimeArgs", "runtimeExecutable", "console":
			if t.Request != "launch" {
				return nil, fmt.Errorf("%s: %s requires Node launch", t.Name, key)
			}
		case "port", "address", "localRoot", "remoteRoot":
			if t.Request != "attach" {
				return nil, fmt.Errorf("%s: %s requires Node attach", t.Name, key)
			}
		}
		switch key {
		case "type", "name", "request", "presentation":
			continue
		case "processId":
			if t.Request != "attach" {
				return nil, fmt.Errorf("%s: processId requires attach", t.Name)
			}
			pid, err := pidValue(val)
			if err != nil {
				return nil, err
			}
			out[key] = pid
		case "debugServer":
			if _, err := nodeServerAddress(t); err != nil {
				return nil, err
			}
			continue
		case "cwd", "program", "args", "env", "runtimeExecutable", "runtimeArgs", "outFiles", "sourceMaps", "skipFiles", "stopOnEntry", "console", "port", "address", "timeout", "localRoot", "remoteRoot":
			x, err := expand(val, t.Folder)
			if err != nil {
				return nil, err
			}
			out[key] = x
		default:
			return nil, fmt.Errorf("%s: unsupported Node configuration field %q", t.Name, key)
		}
	}
	if port, ok := out["port"]; ok {
		n, err := pidValue(port)
		if err != nil || n > 65535 {
			return nil, fmt.Errorf("%s: invalid inspector port", t.Name)
		}
		out["port"] = n
	}
	if addr, ok := out["address"]; ok {
		host, valid := addr.(string)
		if !valid || host == "" {
			return nil, fmt.Errorf("%s: invalid inspector address", t.Name)
		}
		if out["processId"] != nil && host != "localhost" && host != "127.0.0.1" {
			return nil, fmt.Errorf("%s: PID attach requires a local inspector", t.Name)
		}
	}
	cwd, ok := out["cwd"].(string)
	if !ok || cwd == "" {
		return nil, fmt.Errorf("%s: invalid cwd", t.Name)
	}
	if !filepath.IsAbs(cwd) {
		out["cwd"] = filepath.Join(t.Folder, cwd)
	}
	if c, ok := out["console"]; ok && c != "internalConsole" {
		return nil, fmt.Errorf("%s: only internalConsole is supported", t.Name)
	}
	if t.Request == "launch" {
		p, ok := out["program"].(string)
		if !ok || p == "" {
			return nil, fmt.Errorf("%s: missing Node program", t.Name)
		}
		if !filepath.IsAbs(p) {
			out["program"] = filepath.Join(t.Folder, p)
		}
	}
	return out, nil
}
func nodeServerAddress(t Target) (string, error) {
	port, ok := t.Arguments["debugServer"].(float64)
	if !ok || port < 1 || port > 65535 || float64(int(port)) != port {
		return "", fmt.Errorf("%s: invalid DAP debugServer port", t.Name)
	}
	return fmt.Sprintf("127.0.0.1:%d", int(port)), nil
}
func openNodeAdapter(ctx context.Context, t Target, args map[string]any, onEvent func(Event)) (*Session, error) {
	var s *Session
	if _, external := t.Arguments["debugServer"]; external {
		addr, err := nodeServerAddress(t)
		if err != nil {
			return nil, err
		}
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		s = newSession(conn, conn, conn, func() {}, onEvent)
	} else {
		entry := os.Getenv("TTT_JS_DEBUG_SERVER")
		if entry == "" {
			return nil, fmt.Errorf("set TTT_JS_DEBUG_SERVER to the standalone dapDebugServer.js path")
		}
		var err error
		s, err = startListeningAdapter(ctx, "node", []string{entry, "0", "127.0.0.1"}, t.Folder, "Debug server listening at ", onEvent)
		if err != nil {
			return nil, err
		}
	}
	if pid, ok := args["processId"].(int); ok {
		port := 9229
		if p, ok := args["port"].(int); ok {
			port = p
		}
		if err := enableNodeInspector(ctx, pid, port); err != nil {
			s.Close(false)
			return nil, err
		}
		delete(args, "processId")
		args["address"] = "127.0.0.1"
		args["port"] = port
	}
	s.mu.Lock()
	s.jsAddr = s.conn.(net.Conn).RemoteAddr().String()
	s.mu.Unlock()
	return s, nil
}
func (s *Session) startChild(p packet) {
	err := s.attachChild(p.Arguments)
	response := map[string]any{"type": "response", "seq": s.nextSeq(), "command": p.Command, "request_seq": p.Seq, "success": err == nil}
	if err != nil {
		response["message"] = err.Error()
	}
	_ = s.codec.Send(response)
}
func (s *Session) attachChild(raw json.RawMessage) error {
	var req struct {
		Request       string
		Configuration map[string]any
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return err
	}
	id, _ := req.Configuration["__pendingTargetId"].(string)
	s.mu.Lock()
	addr := s.jsAddr
	s.mu.Unlock()
	if addr == "" || id == "" || (req.Request != "launch" && req.Request != "attach") {
		return fmt.Errorf("unsupported child debug request")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	child := newSession(conn, conn, conn, func() {}, s.onEvent)
	s.mu.Lock()
	if s.child != nil || s.closing {
		s.mu.Unlock()
		_ = conn.Close()
		return fmt.Errorf("debug target already active or closing")
	}
	points := maps.Clone(s.desired)
	s.child = child
	s.mu.Unlock()
	err = child.Start(ctx, "pwa-node", req.Request, req.Configuration, func() error {
		for path, lines := range points {
			if _, err := child.SetBreakpoints(ctx, path, lines); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = conn.Close()
		s.mu.Lock()
		if s.child == child {
			s.child = nil
		}
		s.mu.Unlock()
	}
	return err
}
