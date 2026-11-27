package jsdebug

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/dap"
)

type Provider struct{ dap.Adapter }

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
			x, err := t.Expand(val)
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
	if c, ok := out["console"]; ok && c != "internalConsole" && c != "integratedTerminal" {
		return nil, fmt.Errorf("%s: expected internalConsole or integratedTerminal", t.Name)
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
	port, err := dap.ProcessID(t.Arguments["debugServer"])
	if err != nil || port > 65535 {
		return "", fmt.Errorf("%s: invalid DAP debugServer port", t.Name)
	}
	return fmt.Sprintf("127.0.0.1:%d", int(port)), nil
}
func (p Provider) Open(ctx context.Context, t dap.Target, args map[string]any, onEvent func(dap.Event), options dap.SessionOptions) (*dap.Session, error) {
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
		var err error
		s, err = p.Adapter.Open(ctx, t, args, onEvent, options)
		if err != nil {
			return nil, err
		}
	}

	if pid, ok := args["processId"].(int); ok {
		port := 9229
		if p, ok := args["port"].(int); ok {
			port = p
		}
		ws, err := inspectorWebsocket(ctx, pid, port)
		if err != nil {
			s.Close()
			return nil, err
		}
		delete(args, "processId")
		args["websocketAddress"] = ws
	}
	return s, nil
}
