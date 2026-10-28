package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
)

func (a *App) installPluginThemes(p *plugin.Plugin) {
	files := make(map[string][]byte)
	for _, theme := range p.Manifest.Themes {
		path, err := p.ThemeFile(theme)
		var data []byte
		if err == nil {
			data, err = os.ReadFile(path)
		}
		if err == nil {
			parsed := config.DefaultTheme()
			err = json.Unmarshal(data, &parsed)
			if err == nil && strings.TrimSpace(string(data)) == "null" {
				err = fmt.Errorf("theme must be a JSON object")
			}
		}
		if err != nil {
			a.LogOutput("error", "themes", fmt.Sprintf("install %s from %s: %v", theme.Path, p.Name, err))
			return
		}
		if !isPlainName(theme.ID()) {
			a.LogOutput("error", "themes", "invalid theme name: "+theme.ID())
			return
		}
		files[theme.ID()+".json"] = data
	}
	if err := replacePluginThemes(p.Name, files); err != nil {
		a.LogOutput("error", "themes", fmt.Sprintf("install themes from %s: %v", p.Name, err))
		return
	}
	a.refreshPluginTheme(p.Name)
}

func (a *App) uninstallPluginThemes(name string) {
	if err := replacePluginThemes(name, nil); err != nil {
		a.LogOutput("error", "themes", fmt.Sprintf("remove themes of %s: %v", name, err))
		return
	}
	a.refreshPluginTheme(name)
}

func replacePluginThemes(name string, files map[string][]byte) error {
	if !isPlainName(name) {
		return errInvalidPluginName
	}
	dir := filepath.Join("themes", "plugins", name)
	if len(files) == 0 {
		if _, err := os.Lstat(filepath.Join(config.UserConfigDir(), dir)); os.IsNotExist(err) {
			return nil
		}
	} else if err := os.MkdirAll(config.UserConfigDir(), 0755); err != nil {
		return err
	}
	root, err := os.OpenRoot(config.UserConfigDir())
	if err != nil {
		return err
	}
	defer root.Close()
	for _, dir := range []string{"themes", filepath.Join("themes", "plugins")} {
		if err := root.Mkdir(dir, 0755); err != nil && !os.IsExist(err) {
			return err
		}
		info, err := root.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("theme directory %s must not be a symlink", dir)
		}
	}
	if err := root.RemoveAll(dir); err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	if err := root.Mkdir(dir, 0755); err != nil {
		return err
	}
	for filename, data := range files {
		if err := root.WriteFile(filepath.Join(dir, filename), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) refreshPluginTheme(name string) {
	if !strings.HasPrefix(a.Settings.Theme, "plugins/"+name+"/") {
		return
	}
	s := *a.Settings
	if _, err := config.LoadTheme(s.Theme); err != nil {
		s.Theme = ""
		if err := config.SaveSettings(s); err != nil {
			a.LogOutput("error", "themes", "save settings: "+err.Error())
		}
	}
	a.ApplySettings(s)
}
