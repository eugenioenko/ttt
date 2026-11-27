package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/i18n"
)

// SupportedAPIVersion is the plugin API version this editor implements.
// Manifests without an "api" field are assumed to target version 1.
//
// v2 (ttt 1.1.0) adds the plugin key interceptor's precedence over Escape and
// chords, the ttt.command_line API, and plugin-namespaced settings. A plugin
// that needs any of these declares "api": 2; older builds (which only know v1)
// already reject it via the check in LoadManifest, so it fails cleanly instead
// of loading into a broken half-state.
const SupportedAPIVersion = 2

type Manifest struct {
	Name        string        `json:"name"`
	DisplayName string        `json:"displayName,omitempty"`
	Description string        `json:"description"`
	Version     string        `json:"version"`
	Author      string        `json:"author"`
	Entry       string        `json:"entry"`
	API         int           `json:"api,omitempty"`
	Permissions PermissionSet `json:"permissions"`
	Grammars    []Grammar     `json:"grammars,omitempty"`
	Themes      []Theme       `json:"themes,omitempty"`
	Languages   []Language    `json:"languages,omitempty"`
}

type Language struct {
	Locale       string `json:"locale"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	SourceCommit string `json:"sourceCommit,omitempty"`
	TTTVersion   string `json:"tttVersion,omitempty"`
}

func (p *Plugin) LanguageFile(path string) (string, error) {
	return p.contributionFile(path)
}

type Theme struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

func (t Theme) ID() string {
	if t.Name != "" {
		return t.Name
	}
	return strings.TrimSuffix(filepath.Base(t.Path), filepath.Ext(t.Path))
}

func (p *Plugin) ThemeFile(t Theme) (string, error) {
	return p.contributionFile(t.Path)
}

// Grammar is a TextMate grammar the plugin contributes. Path is relative to
// the plugin directory.
type Grammar struct {
	Path      string   `json:"path"`
	Language  string   `json:"language,omitempty"`
	FileTypes []string `json:"fileTypes,omitempty"`
}

// GrammarFile resolves symlinks so a grammar cannot point outside the plugin
// directory, which the lexical check in LoadManifest does not catch.
func (p *Plugin) GrammarFile(g Grammar) (string, error) {
	return p.contributionFile(g.Path)
}

func (p *Plugin) contributionFile(relative string) (string, error) {
	root, err := filepath.EvalSymlinks(p.Dir)
	if err != nil {
		return "", err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(p.Dir, relative))
	if err != nil {
		return "", err
	}
	if !withinDir(root, path) {
		return "", fmt.Errorf("contribution path %q resolves outside plugin directory", relative)
	}
	return path, nil
}

// Title is the human-facing plugin name shown in the UI: DisplayName when the
// author set one, otherwise the unique kebab-case Name identifier.
func (m Manifest) Title() string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	return m.Name
}

func LoadManifest(dir string) (Manifest, error) {
	path := filepath.Join(dir, "plugin.ttt.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}

	if m.Name == "" {
		return Manifest{}, fmt.Errorf("manifest missing required field: name")
	}
	if m.Entry == "" && len(m.Grammars) == 0 && len(m.Themes) == 0 && len(m.Languages) == 0 {
		return Manifest{}, fmt.Errorf("manifest missing required field: entry")
	}
	for _, g := range m.Grammars {
		if g.Path == "" {
			return Manifest{}, fmt.Errorf("grammar missing required field: path")
		}
		if !withinDir(dir, filepath.Join(dir, g.Path)) {
			return Manifest{}, fmt.Errorf("grammar path %q escapes plugin directory", g.Path)
		}
	}
	seenThemes := make(map[string]bool)
	for _, theme := range m.Themes {
		if !filepath.IsLocal(theme.Path) {
			return Manifest{}, fmt.Errorf("invalid theme path %q", theme.Path)
		}
		name := theme.ID()
		if name == "." || !filepath.IsLocal(name) || filepath.Base(name) != name {
			return Manifest{}, fmt.Errorf("invalid theme name %q", name)
		}
		if seenThemes[name] {
			return Manifest{}, fmt.Errorf("duplicate theme name %q", name)
		}
		seenThemes[name] = true
	}
	seenLocales := make(map[string]bool)
	for i := range m.Languages {
		contribution := &m.Languages[i]
		locale, err := i18n.NormalizeLocale(contribution.Locale)
		if err != nil || contribution.Locale == "" || locale == "en" {
			return Manifest{}, fmt.Errorf("invalid contributed locale %q", contribution.Locale)
		}
		if contribution.Name == "" || !filepath.IsLocal(contribution.Path) || (contribution.SourceCommit != "" && !validSourceCommit(contribution.SourceCommit)) {
			return Manifest{}, fmt.Errorf("invalid language contribution for %q", locale)
		}
		if seenLocales[locale] {
			return Manifest{}, fmt.Errorf("duplicate contributed locale %q", locale)
		}
		contribution.Locale = locale
		seenLocales[locale] = true
	}
	if err := m.Permissions.NetworkHTTP.Validate(); err != nil {
		return Manifest{}, err
	}
	if m.API == 0 {
		m.API = 1
	}
	if m.API < 1 || m.API > SupportedAPIVersion {
		return Manifest{}, fmt.Errorf("plugin %q requires plugin API v%d; this version of ttt supports v%d",
			m.Name, m.API, SupportedAPIVersion)
	}

	return m, nil
}
