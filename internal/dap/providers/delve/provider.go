package delve

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/dap"
)

type Provider struct{ dap.Adapter }

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
	out, err := t.Fields(map[string]any{"mode": "debug", "outputMode": "remote", "cwd": t.Folder}, "program", "cwd", "mode", "args", "env", "buildFlags", "stopOnEntry", "console", "output")
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

func (p Provider) Open(ctx context.Context, t dap.Target, args map[string]any, onEvent func(dap.Event), options dap.SessionOptions) (*dap.Session, error) {
	if t.Request == "attach" && args["processId"] == nil {
		addr, err := dap.Endpoint(t, t.Arguments)
		if err != nil {
			return nil, err
		}
		options.ResumeBeforeDetach = true
		return dap.Connect(ctx, addr, onEvent, options)
	}
	dir, err := os.MkdirTemp("", "ttt-delve-")
	if err != nil {
		return nil, err
	}
	if t.Request == "launch" && args["output"] == nil && (args["mode"] == "debug" || args["mode"] == "test") {
		args["output"] = filepath.Join(dir, "debuggee")
	}
	var session *dap.Session
	if t.Arguments["console"] != "externalTerminal" {
		session, err = p.Adapter.Open(ctx, t, args, onEvent, options)
	} else {
		session, err = dap.ReverseProcess(ctx, filepath.Join(dir, "adapter.sock"), t.Folder, func(address string) (*exec.Cmd, bool, error) {
			cmd, err := p.Process(t.Folder)
			if err != nil {
				return nil, false, err
			}
			args := cmd.Args[:1]
			for i := 1; i < len(cmd.Args); i++ {
				arg := cmd.Args[i]
				switch {
				case strings.HasPrefix(arg, "--listen="):
					continue
				case arg == "--listen":
					i++
					continue
				}
				args = append(args, arg)
			}
			cmd.Args = append(args, "--client-addr", address)
			return externalCommand(cmd)
		}, onEvent, options)
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	session.OnCleanup(func() { _ = os.RemoveAll(dir) })
	return session, nil
}
