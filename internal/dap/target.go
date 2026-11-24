package dap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/jsonc"
)

type Target struct {
	Name      string
	Type      string
	Request   string
	Folder    string
	Arguments map[string]any
}

func LoadTargets(folder string) ([]Target, error) {
	for _, dir := range []string{".ttt", ".vscode"} {
		path := filepath.Join(folder, dir, "launch.json")
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var file struct{ Configurations []map[string]any }
		if err = json.Unmarshal(jsonc.ToJSON(data), &file); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		var targets []Target
		for i, args := range file.Configurations {
			name, _ := args["name"].(string)
			adapter, _ := args["type"].(string)
			request, _ := args["request"].(string)
			if name == "" || adapter == "" || request == "" {
				return nil, fmt.Errorf("%s: configuration %d requires name, type and request", path, i+1)
			}
			targets = append(targets, Target{name, adapter, request, folder, args})
		}
		return targets, nil
	}
	return nil, nil
}

func (t Target) LaunchArguments() (map[string]any, error) {
	if t.Type != "go" || t.Request != "launch" {
		return nil, fmt.Errorf("%s: unsupported %s %s configuration", t.Name, t.Type, t.Request)
	}
	out := map[string]any{"mode": "debug", "outputMode": "remote", "cwd": t.Folder}
	for key, value := range t.Arguments {
		switch key {
		case "name", "request", "type", "presentation":
			continue
		case "program", "cwd", "mode", "args", "env", "buildFlags", "stopOnEntry":
			expanded, err := expand(value, t.Folder)
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %w", t.Name, key, err)
			}
			out[key] = expanded
		default:
			return nil, fmt.Errorf("%s: unsupported configuration field %q", t.Name, key)
		}
	}
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
func expand(value any, folder string) (any, error) {
	switch v := value.(type) {
	case string:
		s := strings.ReplaceAll(v, "${workspaceFolder}", folder)
		if strings.Contains(s, "${") {
			return nil, fmt.Errorf("unsupported substitution in %q", v)
		}
		return s, nil
	case []any:
		dst := make([]any, len(v))
		for i, item := range v {
			x, err := expand(item, folder)
			if err != nil {
				return nil, err
			}
			dst[i] = x
		}
		return dst, nil
	case map[string]any:
		dst := make(map[string]any, len(v))
		for k, item := range v {
			x, err := expand(item, folder)
			if err != nil {
				return nil, err
			}
			dst[k] = x
		}
		return dst, nil
	default:
		return value, nil
	}
}
