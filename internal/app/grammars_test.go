package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/plugin"
)

const fooGrammarJSON = `{
	"scopeName": "source.foo",
	"name": "Foo",
	"fileTypes": ["foo"],
	"patterns": [ { "match": "\\bfoo\\b", "name": "keyword.control.foo" } ]
}`

func newGrammarPlugin(t *testing.T, name string) *plugin.Plugin {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(dir, "syntaxes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "syntaxes", "foo.json"), []byte(fooGrammarJSON), 0644); err != nil {
		t.Fatal(err)
	}
	return &plugin.Plugin{
		Name: name,
		Dir:  dir,
		Manifest: plugin.Manifest{
			Name:     name,
			Grammars: []plugin.Grammar{{Path: "syntaxes/foo.json", Language: "Foo"}},
		},
	}
}

func activeLanguage(a *App) string {
	if a.EditorGroup.Editor.Highlighter == nil {
		return ""
	}
	return a.EditorGroup.Editor.Highlighter.Language()
}

func TestPluginGrammarInstallAndUninstall(t *testing.T) {
	cfgDir := t.TempDir()
	config.OverrideConfigDir = cfgDir
	t.Cleanup(func() {
		config.OverrideConfigDir = ""
		highlight.SetExternalGrammars(nil)
	})

	os.MkdirAll(filepath.Join(cfgDir, "grammars"), 0755)
	os.WriteFile(filepath.Join(cfgDir, "grammars", "manual.json"), []byte(`{"scopeName": "source.bar", "fileTypes": ["bar"], "patterns": []}`), 0644)
	manual := config.GrammarSetting{Path: "grammars/manual.json"}
	settings := config.DefaultSettings()
	settings.Editor.Grammars = []config.GrammarSetting{manual}
	a := buildTestApp(t, settings)

	src := filepath.Join(t.TempDir(), "main.foo")
	os.WriteFile(src, []byte("foo\n"), 0644)
	a.EditorGroup.OpenFile(src)
	if got := activeLanguage(a); got != "" {
		t.Fatalf("language before install = %q, want none", got)
	}

	a.installPluginGrammars(newGrammarPlugin(t, "lang-foo"))

	copied := filepath.Join(cfgDir, "grammars", "lang-foo", "syntaxes", "foo.json")
	if _, err := os.Stat(copied); err != nil {
		t.Fatalf("grammar not copied: %v", err)
	}
	want := []config.GrammarSetting{manual, {Path: filepath.Join("grammars", "lang-foo", "syntaxes", "foo.json"), Language: "Foo", Plugin: "lang-foo"}}
	saved := config.LoadSettings().Editor.Grammars
	if len(saved) != 2 || saved[0].Path != want[0].Path || saved[1].Path != want[1].Path || saved[1].Plugin != "lang-foo" {
		t.Fatalf("saved grammars = %+v, want %+v", saved, want)
	}
	if got := activeLanguage(a); got != "Foo" {
		t.Errorf("open tab language after install = %q, want Foo", got)
	}

	a.installPluginGrammars(newGrammarPlugin(t, "lang-foo"))
	if n := len(config.LoadSettings().Editor.Grammars); n != 2 {
		t.Errorf("reinstall left %d entries, want 2", n)
	}

	a.uninstallPluginGrammars("lang-foo")
	if _, err := os.Stat(copied); !os.IsNotExist(err) {
		t.Errorf("grammar file still present after uninstall: %v", err)
	}
	saved = config.LoadSettings().Editor.Grammars
	if len(saved) != 1 || saved[0].Path != manual.Path {
		t.Errorf("grammars after uninstall = %+v, want only the manual entry", saved)
	}
	if got := activeLanguage(a); got != "" {
		t.Errorf("open tab language after uninstall = %q, want none", got)
	}
}

func TestGrammarSettingsAppliedFromSettings(t *testing.T) {
	cfgDir := t.TempDir()
	config.OverrideConfigDir = cfgDir
	t.Cleanup(func() {
		config.OverrideConfigDir = ""
		highlight.SetExternalGrammars(nil)
	})
	os.MkdirAll(filepath.Join(cfgDir, "grammars"), 0755)
	os.WriteFile(filepath.Join(cfgDir, "grammars", "foo.json"), []byte(fooGrammarJSON), 0644)

	a := buildTestApp(t, config.DefaultSettings())
	src := filepath.Join(t.TempDir(), "main.foo")
	os.WriteFile(src, []byte("foo\n"), 0644)
	a.EditorGroup.OpenFile(src)

	s := *a.Settings
	s.Editor.Grammars = []config.GrammarSetting{{Path: "grammars/foo.json"}, {Path: "grammars/broken.json"}}
	a.ApplySettings(s)

	if got := activeLanguage(a); got != "Foo" {
		t.Errorf("language = %q, want Foo", got)
	}
	reported := false
	for _, line := range a.Output.Lines {
		if line.PluginName == "grammars" && strings.Contains(line.Message, "broken.json") {
			reported = true
		}
	}
	if !reported {
		t.Error("missing grammar file not reported in the Output panel")
	}
}

func TestPluginGrammarsDoNotCollide(t *testing.T) {
	cfgDir := t.TempDir()
	config.OverrideConfigDir = cfgDir
	t.Cleanup(func() {
		config.OverrideConfigDir = ""
		highlight.SetExternalGrammars(nil)
	})
	a := buildTestApp(t, config.DefaultSettings())

	first := newGrammarPlugin(t, "a")
	first.Manifest.Grammars[0].Path = "b-x.json"
	if err := os.Rename(filepath.Join(first.Dir, "syntaxes", "foo.json"), filepath.Join(first.Dir, "b-x.json")); err != nil {
		t.Fatal(err)
	}
	second := newGrammarPlugin(t, "a-b")
	second.Manifest.Grammars[0].Path = "x.json"
	if err := os.Rename(filepath.Join(second.Dir, "syntaxes", "foo.json"), filepath.Join(second.Dir, "x.json")); err != nil {
		t.Fatal(err)
	}

	a.installPluginGrammars(first)
	a.installPluginGrammars(second)
	for _, path := range []string{filepath.Join("a", "b-x.json"), filepath.Join("a-b", "x.json")} {
		if _, err := os.Stat(filepath.Join(cfgDir, "grammars", path)); err != nil {
			t.Fatalf("grammar %s not installed: %v", path, err)
		}
	}
	a.uninstallPluginGrammars("a")

	if _, err := os.Stat(filepath.Join(cfgDir, "grammars", "a-b", "x.json")); err != nil {
		t.Errorf("uninstalling plugin a removed a-b's grammar: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfgDir, "grammars", "a")); !os.IsNotExist(err) {
		t.Errorf("plugin a's grammar folder still present: %v", err)
	}
}

func TestPluginGrammarSymlinkOutsidePluginIsRejected(t *testing.T) {
	cfgDir := t.TempDir()
	config.OverrideConfigDir = cfgDir
	t.Cleanup(func() {
		config.OverrideConfigDir = ""
		highlight.SetExternalGrammars(nil)
	})
	a := buildTestApp(t, config.DefaultSettings())

	secret := filepath.Join(t.TempDir(), "secret.json")
	os.WriteFile(secret, []byte(fooGrammarJSON), 0644)
	p := newGrammarPlugin(t, "sneaky")
	link := filepath.Join(p.Dir, "syntaxes", "foo.json")
	os.Remove(link)
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	a.installPluginGrammars(p)

	if _, err := os.Stat(filepath.Join(cfgDir, "grammars", "sneaky", "syntaxes", "foo.json")); !os.IsNotExist(err) {
		t.Errorf("symlinked grammar outside the plugin was copied: %v", err)
	}
	if hasPluginGrammars(a.Settings.Editor.Grammars, "sneaky") {
		t.Error("settings entry added for a rejected grammar")
	}
}

func TestPluginNameCannotReachOutsideItsGrammarFolder(t *testing.T) {
	cfgDir := t.TempDir()
	config.OverrideConfigDir = cfgDir
	t.Cleanup(func() {
		config.OverrideConfigDir = ""
		highlight.SetExternalGrammars(nil)
	})
	a := buildTestApp(t, config.DefaultSettings())
	a.installPluginGrammars(newGrammarPlugin(t, "lang-foo"))
	kept := filepath.Join(cfgDir, "grammars", "lang-foo", "syntaxes", "foo.json")

	for _, name := range []string{".", "..", "a/b", ""} {
		p := newGrammarPlugin(t, "bad")
		p.Name = name
		a.Settings.Editor.Grammars = append(a.Settings.Editor.Grammars, config.GrammarSetting{Path: "grammars/x.json", Plugin: name})
		a.installPluginGrammars(p)
		a.uninstallPluginGrammars(name)
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("plugin name %q removed another plugin's grammar: %v", name, err)
		}
	}
}
