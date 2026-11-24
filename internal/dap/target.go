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
	for _, dir := range []string{".ttt", ".vscode"} {
		path := filepath.Join(folder, dir, "launch.json")
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var file map[string]json.RawMessage
		if err = json.Unmarshal(jsonc.ToJSON(data), &file); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for key := range file {
			if key != "version" && key != "$schema" && key != "configurations" {
				return nil, fmt.Errorf("%s: unsupported launch file field %q", path, key)
			}
		}
		var configurations []map[string]any
		if err = json.Unmarshal(file["configurations"], &configurations); err != nil || configurations == nil {
			return nil, fmt.Errorf("%s: expected a configurations array", path)
		}
		var targets []Target
		for i, args := range configurations {
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

func Expand(value any, folder string) (any, error) {
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
			x, err := Expand(item, folder)
			if err != nil {
				return nil, err
			}
			dst[i] = x
		}
		return dst, nil
	case map[string]any:
		dst := make(map[string]any, len(v))
		for k, item := range v {
			x, err := Expand(item, folder)
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
