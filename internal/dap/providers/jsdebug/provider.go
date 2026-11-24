package jsdebug

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/eugenioenko/ttt/internal/dap"
)

type Provider struct{}

func (Provider) Arguments(t dap.Target) (map[string]any, error) {
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
			pid, err := dap.ProcessID(val)
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
			x, err := dap.Expand(val, t.Folder)
			if err != nil {
				return nil, err
			}
			out[key] = x
		default:
			return nil, fmt.Errorf("%s: unsupported Node configuration field %q", t.Name, key)
		}
	}
	if port, ok := out["port"]; ok {
		n, err := dap.ProcessID(port)
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
func nodeServerAddress(t dap.Target) (string, error) {
	port, ok := t.Arguments["debugServer"].(float64)
	if !ok || port < 1 || port > 65535 || float64(int(port)) != port {
		return "", fmt.Errorf("%s: invalid DAP debugServer port", t.Name)
	}
	return fmt.Sprintf("127.0.0.1:%d", int(port)), nil
}
func (Provider) Open(ctx context.Context, t dap.Target, args map[string]any, onEvent func(dap.Event)) (*dap.Session, error) {
	options := dap.SessionOptions{ChildTarget: childTarget}
	var s *dap.Session
	if _, external := t.Arguments["debugServer"]; external {
		addr, err := nodeServerAddress(t)
		if err != nil {
			return nil, err
		}
		s, err = dap.Connect(ctx, addr, onEvent, options)
		if err != nil {
			return nil, err
		}
	} else {
		entry := os.Getenv("TTT_JS_DEBUG_SERVER")
		if entry == "" {
			return nil, fmt.Errorf("set TTT_JS_DEBUG_SERVER to the standalone dapDebugServer.js path")
		}
		addr := "0"
		var dir string
		var err error
		if runtime.GOOS != "windows" {
			dir, err = os.MkdirTemp("", "ttt-jsdebug-")
			if err != nil {
				return nil, err
			}
			addr = filepath.Join(dir, "debug.sock")
		}
		s, err = dap.ListenProcess(ctx, "node", []string{entry, addr, "127.0.0.1"}, t.Folder, "Debug server listening at ", onEvent, options)
		if err != nil {
			if dir != "" {
				_ = os.RemoveAll(dir)
			}
			return nil, err
		}
		if dir != "" {
			s.OnCleanup(func() { _ = os.RemoveAll(dir) })
		}
	}
	if pid, ok := args["processId"].(int); ok {
		port := 9229
		if p, ok := args["port"].(int); ok {
			port = p
		}
		ws, err := inspectorWebsocket(ctx, pid, port)
		if err != nil {
			s.Close(false)
			return nil, err
		}
		delete(args, "processId")
		args["websocketAddress"] = ws
	}
	return s, nil
}
func childTarget(raw json.RawMessage) (dap.Target, error) {
	var req struct {
		Request       string
		Configuration map[string]any
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return dap.Target{}, err
	}
	id, _ := req.Configuration["__pendingTargetId"].(string)
	if id == "" || (req.Request != "launch" && req.Request != "attach") {
		return dap.Target{}, fmt.Errorf("unsupported child debug request")
	}
	return dap.Target{Type: "pwa-node", Request: req.Request, Arguments: req.Configuration}, nil
}
