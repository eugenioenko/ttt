package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func loadDisplayLanguage(settings config.LanguageSettings) (string, error) {
	i18n.SetCatalog(nil)
	locale, err := i18n.NormalizeLocale(settings.Locale)
	if err != nil {
		return "en", err
	}
	if locale == "en" {
		return locale, nil
	}
	for _, pack := range settings.Packs {
		if pack.Locale != locale {
			continue
		}
		data, hashes, err := readLanguageFiles(pack)
		if err != nil {
			return "en", err
		}
		catalog, err := i18n.ParseCatalog(data, hashes)
		if err != nil {
			return "en", err
		}
		i18n.SetCatalog(catalog)
		return locale, nil
	}
	return "en", fmt.Errorf("display language %q is not installed", locale)
}

func languageFilePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(config.UserConfigDir(), path)
}

func readLanguageFiles(pack config.LanguagePackSetting) ([]byte, []byte, error) {
	data, err := plugin.ReadLanguageData(languageFilePath(pack.Path))
	if err != nil {
		return nil, nil, err
	}
	var hashes []byte
	if pack.SourceHashes != "" {
		hashes, err = plugin.ReadLanguageData(languageFilePath(pack.SourceHashes))
	}
	return data, hashes, err
}

func languageItemsFor(settings config.LanguageSettings) []widgets.SelectItem {
	items := []widgets.SelectItem{{ID: "en", Label: i18n.Text("language.english")}}
	seen := map[string]bool{"en": true}
	for _, pack := range settings.Packs {
		if !seen[pack.Locale] {
			items = append(items, widgets.SelectItem{ID: pack.Locale, Label: pack.Name + " (" + pack.Locale + ")"})
			seen[pack.Locale] = true
		}
	}
	slices.SortFunc(items[1:], func(a, b widgets.SelectItem) int {
		if a.Label < b.Label {
			return -1
		}
		if a.Label > b.Label {
			return 1
		}
		return 0
	})
	return items
}

func languageItems() []widgets.SelectItem {
	return languageItemsFor(config.LoadSettings().Language)
}

func (a *App) ShowDisplayLanguagePicker() {
	if a.Root.HasOverlay() {
		return
	}
	a.ShowSelectDialog(i18n.Text("settings.displayLanguage"), languageItemsFor(a.Settings.Language), a.SetDisplayLanguage, nil)
}

func (a *App) SetDisplayLanguage(locale string) {
	normalized, err := i18n.NormalizeLocale(locale)
	if err != nil {
		a.StatusError(err.Error())
		return
	}
	s := *a.Settings
	s.Language.Locale = normalized
	if err := config.SaveSettings(s); err != nil {
		a.StatusError(err.Error())
		return
	}
	a.ApplySettings(s)
}

func languageCachePath(name, locale string) string {
	return filepath.Join("languages", "plugins", name, locale+".json")
}

func (a *App) installPluginLanguages(p *plugin.Plugin) {
	if len(p.Manifest.Languages) == 0 && !slices.ContainsFunc(a.Settings.Language.Packs, func(pack config.LanguagePackSetting) bool { return pack.Plugin == p.Name }) {
		return
	}
	if err := a.copyPluginLanguages(p); err != nil {
		a.LogOutput("error", "languages", fmt.Sprintf("install languages from %s: %v", p.Name, err))
	}
}

func (a *App) copyPluginLanguages(p *plugin.Plugin) error {
	if !isPlainName(p.Name) {
		return errInvalidPluginName
	}
	kept := slices.DeleteFunc(slices.Clone(a.Settings.Language.Packs), func(pack config.LanguagePackSetting) bool { return pack.Plugin == p.Name })
	files := make(map[string][]byte)
	var contributed []config.LanguagePackSetting
	seen := make(map[string]bool)
	cacheMissing := false
	for _, language := range p.Manifest.Languages {
		locale, err := i18n.NormalizeLocale(language.Locale)
		if err != nil || locale != language.Locale || locale == "en" || language.Name == "" || seen[locale] {
			return fmt.Errorf("invalid language contribution %q", language.Locale)
		}
		seen[locale] = true
		for _, existing := range kept {
			if existing.Locale == language.Locale {
				return fmt.Errorf("locale %s is already provided by %s", language.Locale, existing.Plugin)
			}
		}
		path, err := p.LanguageFile(language.Path)
		if err != nil {
			return err
		}
		data, err := plugin.ReadLanguageData(path)
		if err != nil {
			return err
		}
		var hashes []byte
		if language.SourceHashes != "" {
			path, err = p.LanguageFile(language.SourceHashes)
			if err != nil {
				return err
			}
			hashes, err = plugin.ReadLanguageData(path)
			if err != nil {
				return err
			}
		}
		if _, err := i18n.ParseCatalog(data, hashes); err != nil {
			return err
		}
		digest := sha256.New()
		digest.Write(data)
		digest.Write([]byte{0})
		digest.Write(hashes)
		metadata, err := json.Marshal(language)
		if err != nil {
			return err
		}
		digest.Write(metadata)
		pack := config.LanguagePackSetting{
			Locale: language.Locale, Name: language.Name, Path: languageCachePath(p.Name, language.Locale),
			TTTVersion: language.TTTVersion, Plugin: p.Name, Repo: p.Repo, RepoPath: p.RepoPath,
			BundleHash: hex.EncodeToString(digest.Sum(nil)),
		}
		if len(hashes) > 0 {
			pack.SourceHashes = languageCachePath(p.Name, language.Locale+".source")
		}
		cacheReady := false
		for _, installed := range a.Settings.Language.Packs {
			if installed.Plugin == p.Name && installed.Locale == pack.Locale && installed.BundleHash == pack.BundleHash && installed.Repo == pack.Repo && installed.RepoPath == pack.RepoPath {
				if cached, cachedHashes, err := readLanguageFiles(installed); err == nil {
					if _, err := i18n.ParseCatalog(cached, cachedHashes); err == nil {
						pack, data, hashes = installed, cached, cachedHashes
						cacheReady = true
					}
				}
			}
		}
		cacheMissing = cacheMissing || !cacheReady
		files[language.Locale+".json"] = data
		if pack.SourceHashes != "" {
			files[language.Locale+".source.json"] = hashes
		}
		contributed = append(contributed, pack)
	}
	next := append(kept, contributed...)
	if !cacheMissing && reflect.DeepEqual(next, a.Settings.Language.Packs) {
		return nil
	}
	if err := replacePluginContributionFiles("languages", p.Name, files); err != nil {
		return err
	}
	a.Settings.Language.Packs = next
	return config.SaveSettings(*a.Settings)
}

func (a *App) uninstallPluginLanguages(name string) {
	if !isPlainName(name) {
		return
	}
	if err := replacePluginContributionFiles("languages", name, nil); err != nil {
		a.LogOutput("error", "languages", err.Error())
		return
	}
	a.Settings.Language.Packs = slices.DeleteFunc(a.Settings.Language.Packs, func(pack config.LanguagePackSetting) bool { return pack.Plugin == name })
	if a.Settings.Language.Locale != "" && a.Settings.Language.Locale != "en" && !slices.ContainsFunc(a.Settings.Language.Packs, func(pack config.LanguagePackSetting) bool { return pack.Locale == a.Settings.Language.Locale }) {
		a.Settings.Language.Locale = "en"
		a.StatusNotify(i18n.Text("status.languageRestartRequired"))
	}
	if err := config.SaveSettings(*a.Settings); err != nil {
		a.LogOutput("error", "languages", err.Error())
	}
}
