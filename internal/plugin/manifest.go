package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SupportedAPIVersion is the plugin API version this editor implements.
// Manifests without an "api" field are assumed to target version 1.
//
// v2 (ttt 1.1.0) adds the plugin key interceptor's precedence over Escape and
// chords, the ttt.command_line API, and plugin-namespaced settings. A plugin
// that needs any of these declares "api": 2; older builds (which only know v1)
// already reject it via the check in LoadManifest, so it fails cleanly instead
// of loading into a broken half-state.
//
// v3 (ttt 1.4.0) is the settings/editor-API surface used by vim-mode-style
// plugins: editor.* settings in settings_keys and the plugin editor API
// additions shipping with 1.4.0.
const SupportedAPIVersion = 3

type Manifest struct {
	Name        string        `json:"name"`
	DisplayName string        `json:"displayName,omitempty"`
	Description string        `json:"description"`
	Version     string        `json:"version"`
	Author      string        `json:"author"`
	Entry       string        `json:"entry"`
	API         int           `json:"api,omitempty"`
	Permissions PermissionSet `json:"permissions"`
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
	if m.Entry == "" {
		return Manifest{}, fmt.Errorf("manifest missing required field: entry")
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
