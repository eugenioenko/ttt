package dap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	_, targets, err := ReadLaunchFile(folder)
	return targets, err
}

func ReadLaunchFile(folder string) (string, []Target, error) {
	for _, dir := range []string{".ttt", ".vscode"} {
		path := filepath.Join(folder, dir, "launch.json")
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return path, nil, err
		}
		var file map[string]json.RawMessage
		if err = json.Unmarshal(jsonc.ToJSON(data), &file); err != nil {
			return path, nil, fmt.Errorf("%s: %w", path, err)
		}
		var configurations []map[string]any
		if err = json.Unmarshal(file["configurations"], &configurations); err != nil || configurations == nil {
			return path, nil, fmt.Errorf("%s: expected a configurations array", path)
		}
		var targets []Target
		for i, args := range configurations {
			name, _ := args["name"].(string)
			adapter, _ := args["type"].(string)
			request, _ := args["request"].(string)
			if name == "" || adapter == "" || request == "" {
				return path, nil, fmt.Errorf("%s: configuration %d requires name, type and request", path, i+1)
			}
			targets = append(targets, Target{name, adapter, request, folder, args})
		}
		return path, targets, nil
	}
	return "", nil, nil
}

func CreateLaunchFile(folder, source, adapter, provider string) (string, error) {
	path := filepath.Join(folder, ".ttt", "launch.json")
	configs := []map[string]string{}
	if program, err := filepath.Rel(folder, source); source != "" && err == nil && filepath.IsLocal(program) {
		if provider == "delve" {
			program = filepath.Dir(program)
		}
		if adapter != "" {
			configs = append(configs, map[string]string{"name": filepath.Base(source), "type": adapter, "request": "launch", "program": filepath.ToSlash(program)})
		}
	}
	data, err := json.MarshalIndent(map[string]any{"version": "0.2.0", "configurations": configs}, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, os.ErrExist) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	_, err = f.Write(append(data, '\n'))
	if err = errors.Join(err, f.Close()); err != nil {
		return "", errors.Join(err, os.Remove(path))
	}
	return path, nil
}

func Expand(value any, folder string) (any, error) {
	return expand(value, folder, false)
}
func expand(value any, folder string, pass bool) (any, error) {
	switch v := value.(type) {
	case string:
		s := strings.ReplaceAll(v, "${workspaceFolder}", folder)
		if strings.Contains(s, "${input:") || (!pass && strings.Contains(s, "${")) {
			return nil, fmt.Errorf("unsupported substitution in %q", v)
		}
		return s, nil
	case []any:
		dst := make([]any, len(v))
		for i, item := range v {
			x, err := expand(item, folder, pass)
			if err != nil {
				return nil, err
			}
			dst[i] = x
		}
		return dst, nil
	case map[string]any:
		dst := make(map[string]any, len(v))
		for k, item := range v {
			x, err := expand(item, folder, pass)
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
func (t Target) Fields(defaults map[string]any, allowed ...string) (map[string]any, error) {
	for key, v := range t.Arguments {
		if key == "request" || key == "name" || key == "type" || key == "presentation" {
			continue
		}
		if !slices.Contains(allowed, key) {
			return nil, fmt.Errorf("%s: unsupported %s %s field %q", t.Name, t.Type, t.Request, key)
		}
		x, err := Expand(v, t.Folder)
		if err != nil {
			return nil, err
		}
		defaults[key] = x
	}
	return defaults, nil
}

func (a Adapter) Arguments(t Target) (map[string]any, error) {
	if t.Request != "launch" && t.Request != "attach" {
		return nil, fmt.Errorf("unsupported request %q", t.Request)
	}
	if _, ok := t.Arguments["preLaunchTask"]; ok {
		return nil, fmt.Errorf("preLaunchTask is unsupported")
	}
	out, err := expand(t.Arguments, t.Folder, true)
	if err != nil {
		return nil, err
	}
	return out.(map[string]any), nil
}
