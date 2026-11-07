package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/plugin"
)

func TestLanguagePackInstallSelectionAndRestart(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = ""; i18n.SetCatalog(nil); initText() })
	a := buildTestApp(t, config.DefaultSettings())
	if !a.Settings.Language.IsAutoUpdateEnabled() {
		t.Fatal("auto-update must default on")
	}
	p := &plugin.Plugin{Name: "spanish", Dir: t.TempDir(), Repo: "https://github.com/example/languages", RepoPath: "packs/es", Manifest: plugin.Manifest{
		Languages: []plugin.Language{{Locale: "es", Name: "Español", Path: "messages.json", TTTVersion: "1.12.0"}},
	}}
	write := func(data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(p.Dir, "messages.json"), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"menu.file":"Archivo","general.save":"Guardar","settings.general":"General (es)","options.minimal":"Mínimo","help.click":"Clic","help.openFileInPreview":"Vista previa","welcome.openFolder":"Abrir carpeta"}`)
	a.installPluginLanguages(p)
	settings := config.LoadSettings()
	if len(settings.Language.Packs) != 1 || settings.Language.Locale != "" {
		t.Fatalf("install changed selection or missed pack: %+v", settings.Language)
	}
	if items := languageItemsFor(settings.Language); len(items) != 2 || items[1].ID != "es" {
		t.Fatalf("picker=%v", items)
	}
	a.SetDisplayLanguage("es")
	if i18n.Text("general.save") != "Save" || menuBarMenus[0][2].Label != "Save" {
		t.Fatal("selection applied before restart")
	}
	translated := buildTestApp(t, config.LoadSettings())
	if translated.MenuBar.Items[0].Name != "Archivo" || menuBarMenus[0][2].Label != "Guardar" {
		t.Fatalf("menu labels were cached in English: %+v", translated.MenuBar.Items)
	}
	if styleLabels["minimal"] != "Mínimo" {
		t.Fatalf("style label cached in English: %q", styleLabels["minimal"])
	}

	if entries := explorerHelpEntries; entries[0].Key != "Clic" || entries[0].Value != "Vista previa" || entries[2].Key != "Enter" {
		t.Fatalf("help table failed to resolve copy or preserved key notation incorrectly: %+v", entries)
	}
	if welcomeCommands[0].label != "Abrir carpeta" {
		t.Fatalf("welcome copy cached in English: %+v", welcomeCommands[0])
	}

	if i18n.Text("general.cancel") != "Cancel" {
		t.Fatal("missing translation did not fall back")
	}
	write(`invalid`)
	translated.installPluginLanguages(p)
	if _, err := loadDisplayLanguage(config.LoadSettings().Language); err != nil {
		t.Fatal("bad update destroyed cached pack", err)
	}
	write(`{"general.save":"stale translation"}`)
	p.Manifest.Languages[0].SourceCommit = strings.Repeat("a", 40)
	if err := os.MkdirAll(filepath.Join(config.UserConfigDir(), "languages", "sources"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.UserConfigDir(), plugin.LanguageSourcePath(strings.Repeat("a", 40))), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := translated.copyPluginLanguages(p); err == nil {
		t.Fatal("empty declared English source was installed")
	}
	if _, err := loadDisplayLanguage(config.LoadSettings().Language); err != nil || i18n.Text("general.save") != "Guardar" {
		t.Fatal("invalid source update destroyed cached pack", err)
	}
	p.Manifest.Languages[0].SourceCommit = ""
	write(`{"general.save":"Guardar actualizado"}`)
	translated.installPluginLanguages(p)
	if _, err := loadDisplayLanguage(config.LoadSettings().Language); err != nil || i18n.Text("general.save") != "Guardar actualizado" {
		t.Fatal("updated pack was not installed", err)
	}
	translated.uninstallPluginLanguages(p.Name)
	if config.LoadSettings().Language.Locale != "en" || len(config.LoadSettings().Language.Packs) != 0 {
		t.Fatal("uninstall did not restore English selection")
	}
	if _, err := os.Stat(languageFilePath(languageCachePath(p.Name, "es"))); !os.IsNotExist(err) {
		t.Fatal("uninstall left cached pack")
	}
}

func TestLanguageAutoUpdateDoesNotGetOverwrittenByOldPluginBundle(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = ""; i18n.SetCatalog(nil); initText() })
	a := buildTestApp(t, config.DefaultSettings())
	p := &plugin.Plugin{Name: "spanish", Dir: t.TempDir(), Repo: "https://github.com/example/languages", Manifest: plugin.Manifest{
		Languages: []plugin.Language{{Locale: "es", Name: "Español", Path: "messages.json", TTTVersion: "1.11.0"}},
	}}
	os.WriteFile(filepath.Join(p.Dir, "messages.json"), []byte(`{"general.save":"old"}`), 0644)
	a.installPluginLanguages(p)
	source := append([]config.LanguagePackSetting(nil), a.Settings.Language.Packs...)
	update := languageUpdateResult{source: source, update: plugin.LanguageUpdate{Languages: []plugin.Language{{Locale: "es", Name: "Español", TTTVersion: "1.12.0"}}, Files: map[string][]byte{"es.json": []byte(`{"general.save":"new"}`)}}}
	a.applyLanguageUpdate(update)
	a.installPluginLanguages(p)
	data, _, err := readLanguageFiles(a.Settings.Language.Packs[0])
	if err != nil || !strings.Contains(string(data), "new") || a.Settings.Language.Packs[0].TTTVersion != "1.12.0" {
		t.Fatal("startup overwrote automatic update", err, string(data))
	}
	a.applyLanguageUpdate(languageUpdateResult{source: append([]config.LanguagePackSetting(nil), a.Settings.Language.Packs...), err: fmt.Errorf("offline")})
	data, _, err = readLanguageFiles(a.Settings.Language.Packs[0])
	if err != nil || !strings.Contains(string(data), "new") {
		t.Fatal("failed download destroyed cached translation", err)
	}
	a.Settings.Language.Packs = nil
	a.applyLanguageUpdate(update)
	if len(a.Settings.Language.Packs) != 0 {
		t.Fatal("stale download reinstalled removed pack")
	}
}

func TestLanguageAutoUpdateDisabledAndMissingPack(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = ""; i18n.SetCatalog(nil); initText() })
	settings := config.DefaultSettings()
	settings.Language.Locale = "es"
	settings.Language.AutoUpdate = boolPtr(false)
	settings.Language.Packs = []config.LanguagePackSetting{{Locale: "es", Path: "missing.json", Plugin: "spanish", Repo: "https://github.com/example/languages"}}
	if err := config.SaveSettings(settings); err != nil || config.LoadSettings().Language.IsAutoUpdateEnabled() {
		t.Fatal("explicit auto-update false was not persisted", err)
	}
	a := buildTestApp(t, settings)
	a.Version = "1.12.0"
	a.CheckLanguagePackUpdates()
	if a.language.updates != nil {
		t.Fatal("disabled update created a task")
	}
	if a.MenuBar.Items[0].Name != "File" || menuBarMenus[0][2].Label != "Save" {
		t.Fatal("missing pack did not fall back to English")
	}
}

func TestLanguageBundleMetadataAndRemovedContribution(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = ""; i18n.SetCatalog(nil); initText() })
	a := buildTestApp(t, config.DefaultSettings())
	p := &plugin.Plugin{Name: "spanish", Dir: t.TempDir(), Manifest: plugin.Manifest{
		Languages: []plugin.Language{{Locale: "es", Name: "Español", Path: "messages.json", TTTVersion: "1.11.0"}},
	}}
	if err := os.WriteFile(filepath.Join(p.Dir, "messages.json"), []byte(`{"general.save":"Guardar"}`), 0644); err != nil {
		t.Fatal(err)
	}
	a.installPluginLanguages(p)
	p.Manifest.Languages[0].TTTVersion = "1.12.0"
	a.installPluginLanguages(p)
	if a.Settings.Language.Packs[0].TTTVersion != "1.12.0" {
		t.Fatal("unchanged messages prevented bundle metadata update")
	}
	a.SetDisplayLanguage("es")
	p.Manifest.Languages = nil
	a.installPluginLanguages(p)
	if config.LoadSettings().Language.Locale != "en" || len(a.Settings.Language.Packs) != 0 {
		t.Fatal("dropped contribution did not restore English selection")
	}
}
