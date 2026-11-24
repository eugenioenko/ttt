package debugpy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/eugenioenko/ttt/internal/dap"
)

func defaultPython(t dap.Target) string {
	python := "python3"
	envPath := filepath.Join(t.Folder, ".venv", "bin", "python")
	if runtime.GOOS == "windows" {
		python = "python"
		envPath = filepath.Join(t.Folder, ".venv", "Scripts", "python.exe")
	}
	if _, err := os.Stat(envPath); err == nil {
		python = envPath
	}
	return python
}

type Provider struct{ dap.Adapter }

func (Provider) Arguments(t dap.Target) (map[string]any, error) {
	if t.Request == "attach" {
		if raw, ok := t.Arguments["processId"]; ok {
			pid, err := dap.ProcessID(raw)
			if err != nil {
				return nil, err
			}
			args, err := t.Fields(map[string]any{"python": defaultPython(t)}, "processId", "python", "justMyCode", "pathMappings")
			if err != nil {
				return nil, err
			}
			args["processId"] = pid
			if s, ok := args["python"].(string); !ok || s == "" {
				return nil, fmt.Errorf("%s: invalid Python interpreter", t.Name)
			}
			return args, nil
		}
		if _, err := endpoint(t); err != nil {
			return nil, err
		}
		return t.Fields(map[string]any{}, "connect", "justMyCode", "pathMappings")
	}
	if t.Request != "launch" {
		return nil, fmt.Errorf("%s: unsupported request %s", t.Name, t.Request)
	}

	out, err := t.Fields(map[string]any{"cwd": t.Folder, "python": defaultPython(t), "console": "internalConsole", "redirectOutput": true, "subProcess": false}, "cwd", "python", "program", "module", "args", "env", "justMyCode", "stopOnEntry", "console", "redirectOutput")
	if err != nil {
		return nil, err
	}
	if out["console"] != "internalConsole" && out["console"] != "integratedTerminal" {
		return nil, fmt.Errorf("%s: expected internalConsole or integratedTerminal", t.Name)
	}
	interpreter, ok := out["python"].(string)
	if !ok || interpreter == "" {
		return nil, fmt.Errorf("%s: expected a Python interpreter path", t.Name)
	}
	if !filepath.IsAbs(interpreter) && filepath.Base(interpreter) != interpreter {
		out["python"] = filepath.Join(t.Folder, interpreter)
	}
	for _, key := range []string{"cwd", "program"} {
		if v, exists := out[key]; exists {
			s, ok := v.(string)
			if !ok || s == "" {
				return nil, fmt.Errorf("%s: invalid %s", t.Name, key)
			}
			if !filepath.IsAbs(s) {
				out[key] = filepath.Join(t.Folder, s)
			}
		}
	}
	module, hasModule := out["module"]
	_, hasProgram := out["program"]
	if hasModule == hasProgram {
		return nil, fmt.Errorf("%s: specify exactly one of program or module", t.Name)
	}
	if hasModule {
		if s, ok := module.(string); !ok || s == "" {
			return nil, fmt.Errorf("%s: invalid module", t.Name)
		}
	}
	return out, nil
}

func endpoint(t dap.Target) (string, error) {
	fields, ok := t.Arguments["connect"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("%s: expected a connect object", t.Name)
	}
	for k := range fields {
		if k != "host" && k != "port" {
			return "", fmt.Errorf("%s: unsupported connect field %q", t.Name, k)
		}
	}
	return dap.Endpoint(t, fields)
}
func (p Provider) Open(ctx context.Context, t dap.Target, args map[string]any, onEvent func(dap.Event), options dap.SessionOptions) (*dap.Session, error) {
	if t.Request == "attach" && args["processId"] == nil {
		addr, err := endpoint(t)
		if err != nil {
			return nil, err
		}
		return dap.Connect(ctx, addr, onEvent, options)
	}
	python, _ := args["python"].(string)
	return p.Adapter.OpenWithVariables(ctx, t, onEvent, options, "${python}", python)
}
