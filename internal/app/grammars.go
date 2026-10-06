package app

import (
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
		rel := filepath.Join(grammarsDirName, p.Name+"-"+filepath.Base(g.Path))
		if err := copyGrammarFile(filepath.Join(p.Dir, g.Path), resolveGrammarPath(rel)); err != nil {
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

// dropPluginGrammars deletes the plugin's copied grammar files and returns the
// settings entries that remain.
func (a *App) dropPluginGrammars(name string) []config.GrammarSetting {
	var kept []config.GrammarSetting
	for _, g := range a.Settings.Editor.Grammars {
		if g.Plugin != name {
			kept = append(kept, g)
			continue
		}
		if err := os.Remove(resolveGrammarPath(g.Path)); err != nil && !os.IsNotExist(err) {
			a.LogOutput("error", "grammars", fmt.Sprintf("remove %s: %v", g.Path, err))
		}
	}
	return kept
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
