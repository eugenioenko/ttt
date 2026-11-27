package dap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tidwall/jsonc"

	"github.com/eugenioenko/ttt/internal/config/debugadapter"
	"github.com/eugenioenko/ttt/internal/i18n"
)

type Adapter debugadapter.Config

func LookupAdapter(adapters map[string]debugadapter.Config, name string) (Adapter, error) {
	settings, ok := adapters[name]
	if !ok {
		return Adapter{}, errors.New(i18n.Format("debug.missingAdapter", i18n.Args{"type": name}))
	}
	switch settings.Provider {
	case "", "generic":
		settings.Provider = "generic"
	case "delve", "jsdebug", "debugpy":
	default:
		return Adapter{}, fmt.Errorf("unknown debug provider %q for adapter %q", settings.Provider, name)
	}
	return Adapter(settings), nil
}

type Target struct {
	Name      string
	Type      string
	Request   string
	Folder    string
	file      string
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
		decoder := json.NewDecoder(bytes.NewReader(file["configurations"]))
		decoder.UseNumber()
		if err = decoder.Decode(&configurations); err != nil || configurations == nil {
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
			targets = append(targets, Target{Name: name, Type: adapter, Request: request, Folder: folder, Arguments: args})
		}
		return path, targets, nil
	}
	return "", nil, nil
}

func (a Adapter) FileTarget(folder, file, name string) (Target, bool) {
	if a.FileLaunch == nil {
		return Target{}, false
	}
	args := a.FileLaunch
	for _, override := range a.FileLaunchOverrides {
		if match, _ := path.Match(override.Pattern, filepath.Base(file)); match {
			args = maps.Clone(args)
			maps.Copy(args, override.Arguments)
			break
		}
	}
	return Target{Name: filepath.Base(file), Folder: folder, Type: name, Request: "launch", file: file, Arguments: args}, true
}

func CreateLaunchFile(folder, source, adapter string, settings Adapter) (string, error) {
	path := filepath.Join(folder, ".ttt", "launch.json")
	configs := []map[string]any{}
	if program, err := filepath.Rel(folder, source); source != "" && adapter != "" && err == nil && filepath.IsLocal(program) {
		config := map[string]any{"program": filepath.ToSlash(program)}
		if t, ok := settings.FileTarget("${workspaceFolder}", "${workspaceFolder}/"+filepath.ToSlash(program), adapter); ok {
			expanded, err := t.expand(t.Arguments)
			if err != nil {
				return "", err
			}
			config = expanded.(map[string]any)
		}
		config["name"], config["type"], config["request"] = filepath.Base(source), adapter, "launch"
		configs = append(configs, config)
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

func (t Target) expand(value any) (any, error) {
	switch v := value.(type) {
	case string:
		if strings.Contains(v, "${input:") {
			return nil, fmt.Errorf("unsupported substitution in %q", v)
		}
		var s string
		if t.file == "" {
			s = strings.ReplaceAll(v, "${workspaceFolder}", t.Folder)
		} else {
			s = strings.NewReplacer("${file}", t.file, "${fileDirname}", filepath.Dir(t.file), "${workspaceFolder}", t.Folder).Replace(v)
		}
		return s, nil
	case []any:
		dst := make([]any, len(v))
		for i, item := range v {
			x, err := t.expand(item)
			if err != nil {
				return nil, err
			}
			dst[i] = x
		}
		return dst, nil
	case map[string]any:
		dst := make(map[string]any, len(v))
		for k, item := range v {
			x, err := t.expand(item)
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
func (a Adapter) Arguments(t Target) (map[string]any, error) {
	if t.Request != "launch" && t.Request != "attach" {
		return nil, fmt.Errorf("unsupported request %q", t.Request)
	}
	if _, ok := t.Arguments["preLaunchTask"]; ok {
		return nil, fmt.Errorf("preLaunchTask is unsupported")
	}
	out, err := t.expand(t.Arguments)
	if err != nil {
		return nil, err
	}
	return out.(map[string]any), nil
}
