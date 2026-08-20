package plugin

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// NetworkHTTP is the network.http permission. In a manifest it is either a
// boolean (`true` = any host) or an array of allowed hostnames
// (`["api.github.com"]`). A missing field or `false` means no network access.
type NetworkHTTP struct {
	All   bool
	Hosts []string
}

// Enabled reports whether the plugin may make any HTTP requests at all.
func (n NetworkHTTP) Enabled() bool { return n.All || len(n.Hosts) > 0 }

// AllowsHost reports whether requests to host are permitted. Matching is
// exact and case-insensitive; `All` permits any host.
func (n NetworkHTTP) AllowsHost(host string) bool {
	if n.All {
		return true
	}
	host = strings.ToLower(host)
	for _, h := range n.Hosts {
		if strings.ToLower(h) == host {
			return true
		}
	}
	return false
}

func (n NetworkHTTP) MarshalJSON() ([]byte, error) {
	if n.All {
		return json.Marshal(true)
	}
	if len(n.Hosts) > 0 {
		return json.Marshal(n.Hosts)
	}
	return json.Marshal(false)
}

func (n *NetworkHTTP) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		n.All = b
		n.Hosts = nil
		return nil
	}
	var hosts []string
	if err := json.Unmarshal(data, &hosts); err != nil {
		return fmt.Errorf("network.http must be a boolean or an array of hostnames: %w", err)
	}
	n.All = false
	n.Hosts = hosts
	return nil
}

type PermissionSet struct {
	PanelSidebar      bool        `json:"panel.sidebar,omitempty"`
	PanelBottom       bool        `json:"panel.bottom,omitempty"`
	PanelDrawer       bool        `json:"panel.drawer,omitempty"`
	PanelEditor       bool        `json:"panel.editor,omitempty"`
	Commands          bool        `json:"commands,omitempty"`
	Keybindings       bool        `json:"keybindings,omitempty"`
	EditorRead        bool        `json:"editor.read,omitempty"`
	EditorWrite       bool        `json:"editor.write,omitempty"`
	EditorDiagnostics bool        `json:"editor.diagnostics,omitempty"`
	EditorBookmarks   bool        `json:"editor.bookmarks,omitempty"`
	FsRead            bool        `json:"fs.read,omitempty"`
	FsWrite           bool        `json:"fs.write,omitempty"`
	SystemExec        []string    `json:"system.exec,omitempty"`
	SystemEnv         bool        `json:"system.env,omitempty"`
	NetworkHTTP       NetworkHTTP `json:"network.http,omitempty"`
	EventsFile        bool        `json:"events.file,omitempty"`
	EventsEditor      bool        `json:"events.editor,omitempty"`
	Settings          bool        `json:"settings,omitempty"`
	SettingsKeys      []string    `json:"settings_keys,omitempty"`
}

type PermissionDiffEntry struct {
	Name  string
	Value string
}

type PermissionDiff struct {
	Entries []PermissionDiffEntry
}

func DiffPermissions(granted, requested PermissionSet) PermissionDiff {
	var entries []PermissionDiffEntry

	check := func(name string, g, r bool) {
		if r && !g {
			entries = append(entries, PermissionDiffEntry{Name: name, Value: "required"})
		}
	}

	check("panel.sidebar", granted.PanelSidebar, requested.PanelSidebar)
	check("panel.bottom", granted.PanelBottom, requested.PanelBottom)
	check("panel.drawer", granted.PanelDrawer, requested.PanelDrawer)
	check("panel.editor", granted.PanelEditor, requested.PanelEditor)
	check("commands", granted.Commands, requested.Commands)
	check("keybindings", granted.Keybindings, requested.Keybindings)
	check("editor.read", granted.EditorRead, requested.EditorRead)
	check("editor.write", granted.EditorWrite, requested.EditorWrite)
	check("editor.diagnostics", granted.EditorDiagnostics, requested.EditorDiagnostics)
	check("editor.bookmarks", granted.EditorBookmarks, requested.EditorBookmarks)
	check("fs.read", granted.FsRead, requested.FsRead)
	check("fs.write", granted.FsWrite, requested.FsWrite)
	check("system.env", granted.SystemEnv, requested.SystemEnv)
	check("events.file", granted.EventsFile, requested.EventsFile)
	check("events.editor", granted.EventsEditor, requested.EventsEditor)
	check("settings", granted.Settings, requested.Settings)

	grantedSettingsKeys := make(map[string]bool)
	for _, k := range granted.SettingsKeys {
		grantedSettingsKeys[k] = true
	}
	for _, k := range requested.SettingsKeys {
		if !grantedSettingsKeys[k] {
			entries = append(entries, PermissionDiffEntry{Name: "settings_keys", Value: k})
		}
	}

	grantedExec := make(map[string]bool)
	for _, b := range granted.SystemExec {
		grantedExec[b] = true
	}
	for _, b := range requested.SystemExec {
		if !grantedExec[b] {
			entries = append(entries, PermissionDiffEntry{Name: "system.exec", Value: b})
		}
	}

	if requested.NetworkHTTP.All && !granted.NetworkHTTP.All {
		entries = append(entries, PermissionDiffEntry{Name: "network.http", Value: "required"})
	} else if !granted.NetworkHTTP.All {
		for _, h := range requested.NetworkHTTP.Hosts {
			if !granted.NetworkHTTP.AllowsHost(h) {
				entries = append(entries, PermissionDiffEntry{Name: "network.http", Value: h})
			}
		}
	}

	return PermissionDiff{Entries: entries}
}

func (d PermissionDiff) IsEmpty() bool {
	return len(d.Entries) == 0
}

func (ps PermissionSet) Check(perm string) error {
	allowed := false
	switch perm {
	case "panel.sidebar":
		allowed = ps.PanelSidebar
	case "panel.bottom":
		allowed = ps.PanelBottom
	case "panel.drawer":
		allowed = ps.PanelDrawer
	case "panel.editor":
		allowed = ps.PanelEditor
	case "commands":
		allowed = ps.Commands
	case "keybindings":
		allowed = ps.Keybindings
	case "editor.read":
		allowed = ps.EditorRead
	case "editor.write":
		allowed = ps.EditorWrite
	case "editor.diagnostics":
		allowed = ps.EditorDiagnostics
	case "editor.bookmarks":
		allowed = ps.EditorBookmarks
	case "fs.read":
		allowed = ps.FsRead
	case "fs.write":
		allowed = ps.FsWrite
	case "system.env":
		allowed = ps.SystemEnv
	case "network.http":
		allowed = ps.NetworkHTTP.Enabled()
	case "events.file":
		allowed = ps.EventsFile
	case "events.editor":
		allowed = ps.EventsEditor
	case "settings":
		allowed = ps.Settings
	default:
		return fmt.Errorf("unknown permission: %s", perm)
	}
	if !allowed {
		return fmt.Errorf("permission denied: %s", perm)
	}
	return nil
}

func (ps PermissionSet) CheckSettingsKey(key string) error {
	if !ps.Settings {
		return fmt.Errorf("permission denied: settings")
	}
	for _, pattern := range ps.SettingsKeys {
		if pattern == "*" || pattern == key {
			return nil
		}
		if strings.HasSuffix(pattern, ".*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(key, prefix) {
				return nil
			}
		}
	}
	return fmt.Errorf("permission denied: settings key %q", key)
}

func (ps PermissionSet) CheckExec(binary string) error {
	if slices.Contains(ps.SystemExec, binary) {
		return nil
	}
	return fmt.Errorf("permission denied: system.exec %q", binary)
}

// CheckHost reports whether the plugin may make an HTTP request to host.
func (ps PermissionSet) CheckHost(host string) error {
	if ps.NetworkHTTP.AllowsHost(host) {
		return nil
	}
	return fmt.Errorf("permission denied: network.http host %q not in the plugin's allowed hosts", host)
}

func (ps PermissionSet) DisplayEntries() []PermissionDiffEntry {
	var entries []PermissionDiffEntry

	add := func(name string, val bool) {
		if val {
			entries = append(entries, PermissionDiffEntry{Name: name, Value: "yes"})
		}
	}

	add("Sidebar panel", ps.PanelSidebar)
	add("Bottom panel", ps.PanelBottom)
	add("Drawer", ps.PanelDrawer)
	add("Editor tab", ps.PanelEditor)
	add("Commands", ps.Commands)
	add("Keybindings", ps.Keybindings)
	add("Read editor", ps.EditorRead)
	add("Write editor", ps.EditorWrite)
	add("Editor diagnostics", ps.EditorDiagnostics)
	add("Editor bookmarks", ps.EditorBookmarks)
	add("Read files", ps.FsRead)
	add("Write files", ps.FsWrite)
	add("Environment", ps.SystemEnv)
	add("File events", ps.EventsFile)
	add("Editor events", ps.EventsEditor)

	for _, b := range ps.SystemExec {
		entries = append(entries, PermissionDiffEntry{Name: "Run binary", Value: b})
	}

	if ps.NetworkHTTP.All {
		entries = append(entries, PermissionDiffEntry{Name: "HTTP requests", Value: "any host"})
	}
	for _, h := range ps.NetworkHTTP.Hosts {
		entries = append(entries, PermissionDiffEntry{Name: "HTTP host", Value: h})
	}

	add("Settings", ps.Settings)
	for _, k := range ps.SettingsKeys {
		entries = append(entries, PermissionDiffEntry{Name: "Settings key", Value: k})
	}

	return entries
}
