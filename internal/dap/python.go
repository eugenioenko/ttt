package dap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func (t Target) pythonArguments() (map[string]any, error) {
	python := "python3"
	envPath := filepath.Join(t.Folder, ".venv", "bin", "python")
	if runtime.GOOS == "windows" {
		python = "python"
		envPath = filepath.Join(t.Folder, ".venv", "Scripts", "python.exe")
	}
	if _, err := os.Stat(envPath); err == nil {
		python = envPath
	}
	out := map[string]any{"cwd": t.Folder, "python": python, "console": "internalConsole", "redirectOutput": true, "subProcess": false}
	for key, val := range t.Arguments {
		switch key {
		case "name", "request", "type", "presentation":
			continue
		case "cwd", "python", "program", "module", "args", "env", "justMyCode", "stopOnEntry", "console", "redirectOutput":
			x, err := expand(val, t.Folder)
			if err != nil {
				return nil, err
			}
			out[key] = x
		default:
			return nil, fmt.Errorf("%s: unsupported debugpy configuration field %q", t.Name, key)
		}
	}
	if out["console"] != "internalConsole" {
		return nil, fmt.Errorf("%s: debugpy requires internalConsole", t.Name)
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

func OpenAdapter(ctx context.Context, t Target, args map[string]any, onEvent func(Event)) (*Session, error) {
	if t.Request == "attach" {
		return connectAdapter(ctx, t, onEvent)
	}
	if t.Type == "go" {
		return StartDelve(ctx, "dlv", t.Folder, onEvent)
	}
	if t.Type != "debugpy" {
		return nil, fmt.Errorf("unsupported adapter %q", t.Type)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	python, ok := args["python"].(string)
	if !ok {
		return nil, fmt.Errorf("missing Python interpreter")
	}
	cmd := exec.Command(python, "-m", "debugpy.adapter")
	cmd.Dir = t.Folder
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	cleanup := func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); _ = stdout.Close() }
	return newSession(stdout, stdin, stdin, cleanup, onEvent), nil
}
