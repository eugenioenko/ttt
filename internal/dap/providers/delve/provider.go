package delve

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/dap"
)

type Provider struct{}

func (Provider) Arguments(t dap.Target) (map[string]any, error) {
	if t.Request == "attach" {
		if raw, ok := t.Arguments["processId"]; ok {
			pid, err := dap.ProcessID(raw)
			if err != nil {
				return nil, err
			}
			args, err := t.Fields(map[string]any{"mode": "local"}, "processId", "mode", "substitutePath")
			if err != nil {
				return nil, err
			}
			if args["mode"] != "local" {
				return nil, fmt.Errorf("%s: PID attach requires local mode", t.Name)
			}
			args["processId"] = pid
			return args, nil
		}
		if _, err := dap.Endpoint(t, t.Arguments); err != nil {
			return nil, err
		}
		args, err := t.Fields(map[string]any{"mode": "remote"}, "host", "port", "mode", "substitutePath")
		if err != nil {
			return nil, err
		}
		if args["mode"] != "remote" {
			return nil, fmt.Errorf("%s: address attach requires remote mode", t.Name)
		}
		delete(args, "host")
		delete(args, "port")
		return args, nil
	}
	if t.Request != "launch" {
		return nil, fmt.Errorf("%s: unsupported %s %s configuration", t.Name, t.Type, t.Request)
	}
	out, err := t.Fields(map[string]any{"mode": "debug", "outputMode": "remote", "cwd": t.Folder}, "program", "cwd", "mode", "args", "env", "buildFlags", "stopOnEntry", "console")
	if err != nil {
		return nil, err
	}
	switch out["console"] {
	case nil, "internalConsole":
	case "externalTerminal":
		out["outputMode"] = "local"
	default:
		return nil, fmt.Errorf("%s: unsupported Go console %v", t.Name, out["console"])
	}
	delete(out, "console")
	program, _ := out["program"].(string)
	if program == "" {
		return nil, fmt.Errorf("%s: missing program", t.Name)
	}
	for _, key := range []string{"program", "cwd"} {
		path, ok := out[key].(string)
		if !ok {
			return nil, fmt.Errorf("%s: invalid %s", t.Name, key)
		}
		if !filepath.IsAbs(path) {
			out[key] = filepath.Join(t.Folder, path)
		}
	}
	return out, nil
}

func (Provider) Open(ctx context.Context, t dap.Target, args map[string]any, onEvent func(dap.Event)) (*dap.Session, error) {
	if t.Request == "attach" && args["processId"] == nil {
		addr, err := dap.Endpoint(t, t.Arguments)
		if err != nil {
			return nil, err
		}
		return dap.Connect(ctx, addr, onEvent, dap.SessionOptions{ResumeBeforeDetach: true})
	}
	if t.Arguments["console"] == "externalTerminal" {
		return openExternal(ctx, t, onEvent)
	}
	return dap.ListenProcess(ctx, "dlv", []string{"dap", "--listen=127.0.0.1:0"}, t.Folder, "DAP server listening at: ", onEvent, dap.SessionOptions{})
}
