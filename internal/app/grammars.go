package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/plugin"
)

const grammarsDirName = "grammars"

func configDir() string {
	return filepath.Dir(config.ConfigFilePath("settings.json"))
}

func resolveGrammarPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(configDir(), path)
}

func loadGrammarSettings(list []config.GrammarSetting) []error {
	external := make([]highlight.ExternalGrammar, 0, len(list))
	for _, g := range list {
		external = append(external, highlight.ExternalGrammar{
			Path:      resolveGrammarPath(g.Path),
			Language:  g.Language,
			FileTypes: g.FileTypes,
		})
	}
	return highlight.SetExternalGrammars(external)
}

func (a *App) reportGrammarErrors(errs []error) {
	for _, err := range errs {
		a.LogOutput("error", "grammars", err.Error())
	}
}

func (a *App) applyGrammarSettings(list []config.GrammarSetting) {
	if reflect.DeepEqual(list, a.loadedGrammars) {
		return
	}
	a.reloadGrammars(list)
}

func (a *App) reloadGrammars(list []config.GrammarSetting) {
	a.loadedGrammars = slices.Clone(list)
	a.reportGrammarErrors(loadGrammarSettings(list))
	a.EditorGroup.RefreshHighlighters()
}

// installPluginGrammars copies the plugin's grammars into the config grammars
// folder and replaces the plugin's entries in settings, so the grammars stay
// available independent of the plugin directory.
func (a *App) installPluginGrammars(p *plugin.Plugin) {
	if len(p.Manifest.Grammars) == 0 && !hasPluginGrammars(a.Settings.Editor.Grammars, p.Name) {
		return
	}
	entries := a.dropPluginGrammars(p.Name)
	for _, g := range p.Manifest.Grammars {
		rel := filepath.Join(grammarsDirName, p.Name, filepath.Clean(g.Path))
		err := errInvalidPluginName
		if isPlainName(p.Name) {
			var src string
			if src, err = p.GrammarFile(g); err == nil {
				err = copyGrammarFile(src, resolveGrammarPath(rel))
			}
		}
		if err != nil {
			a.LogOutput("error", "grammars", fmt.Sprintf("install %s from %s: %v", g.Path, p.Name, err))
			continue
		}
		entries = append(entries, config.GrammarSetting{
			Path:      rel,
			Language:  g.Language,
			FileTypes: g.FileTypes,
			Plugin:    p.Name,
		})
	}
	a.saveGrammarSettings(entries)
}

func (a *App) uninstallPluginGrammars(name string) {
	if !hasPluginGrammars(a.Settings.Editor.Grammars, name) {
		return
	}
	a.saveGrammarSettings(a.dropPluginGrammars(name))
}

// dropPluginGrammars deletes the plugin's grammar folder and returns the
// settings entries that remain.
func (a *App) dropPluginGrammars(name string) []config.GrammarSetting {
	var kept []config.GrammarSetting
	for _, g := range a.Settings.Editor.Grammars {
		if g.Plugin != name {
			kept = append(kept, g)
		}
	}
	if isPlainName(name) {
		if err := os.RemoveAll(resolveGrammarPath(filepath.Join(grammarsDirName, name))); err != nil {
			a.LogOutput("error", "grammars", fmt.Sprintf("remove grammars of %s: %v", name, err))
		}
	}
	return kept
}

var errInvalidPluginName = errors.New("plugin name is not a valid folder name")

// isPlainName guards RemoveAll and copy targets against plugin names such as
// ".." or "a/b", which would reach outside the grammars folder.
func isPlainName(name string) bool {
	return filepath.IsLocal(name) && filepath.Base(name) == name
}

func (a *App) saveGrammarSettings(entries []config.GrammarSetting) {
	a.Settings.Editor.Grammars = entries
	if err := config.SaveSettings(*a.Settings); err != nil {
		a.LogOutput("error", "grammars", "save settings: "+err.Error())
	}
	a.reloadGrammars(entries)
}

func grammarLabel(g plugin.Grammar) string {
	label := g.Language
	if label == "" {
		label = filepath.Base(g.Path)
	}
	if len(g.FileTypes) > 0 {
		label += " (" + strings.Join(g.FileTypes, ", ") + ")"
	}
	return label
}

func hasPluginGrammars(list []config.GrammarSetting, name string) bool {
	return slices.ContainsFunc(list, func(g config.GrammarSetting) bool { return g.Plugin == name })
}

func copyGrammarFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
