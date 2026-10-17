package plugin

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

const (
	FsWorkspace = "{workspace}"
	FsConfig    = "{config}"
	FsAnyPath   = "*"
)

// FsAccess is the fs.read or fs.write permission: `true` (the workspace) or an
// array of paths. An array grants only what it lists, so the workspace is
// excluded unless `{workspace}` is listed.
type FsAccess struct {
	Paths []string
}

func (f FsAccess) Enabled() bool { return len(f.Paths) > 0 }

func (f FsAccess) Covers(path string) bool {
	return slices.Contains(f.Paths, FsAnyPath) || slices.Contains(f.Paths, path)
}

func (f FsAccess) MarshalJSON() ([]byte, error) {
	if len(f.Paths) == 1 && f.Paths[0] == FsWorkspace {
		return json.Marshal(true)
	}
	if len(f.Paths) > 0 {
		return json.Marshal(f.Paths)
	}
	return json.Marshal(false)
}

func (f *FsAccess) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		f.Paths = nil
		if b {
			f.Paths = []string{FsWorkspace}
		}
		return nil
	}
	var paths []string
	if err := json.Unmarshal(data, &paths); err != nil {
		return fmt.Errorf("fs permissions must be a boolean or an array of paths: %w", err)
	}
	for _, p := range paths {
		if !validFsPath(p) {
			return fmt.Errorf("fs permission path %q must be %s, %s, an absolute path, or start with ~ or %s", p, FsAnyPath, FsWorkspace, FsConfig)
		}
	}
	f.Paths = paths
	return nil
}

func validFsPath(p string) bool {
	return p == FsAnyPath || p == FsWorkspace || hasPathPrefix(p, "~") || hasPathPrefix(p, FsConfig) || filepath.IsAbs(p)
}

func hasPathPrefix(p, prefix string) bool {
	return p == prefix || strings.HasPrefix(p, prefix+"/") || strings.HasPrefix(p, prefix+string(filepath.Separator))
}

type FsVars struct {
	Workspace []string
	Config    string
	Home      string
}

func (v FsVars) Expand(path string) string {
	if v.Home != "" && hasPathPrefix(path, "~") {
		return filepath.Join(v.Home, path[1:])
	}
	if v.Config != "" && hasPathPrefix(path, FsConfig) {
		return filepath.Join(v.Config, path[len(FsConfig):])
	}
	return path
}

func (f FsAccess) Resolve(v FsVars) (dirs []string, all bool) {
	for _, p := range f.Paths {
		switch p {
		case FsAnyPath:
			all = true
		case FsWorkspace:
			dirs = append(dirs, v.Workspace...)
		default:
			if e := v.Expand(p); filepath.IsAbs(e) {
				dirs = append(dirs, e)
			}
		}
	}
	return dirs, all
}

func fsPathLabel(p string) string {
	switch p {
	case FsWorkspace:
		return "workspace"
	case FsAnyPath:
		return "any path"
	}
	return p
}

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
	EditorDecorations bool        `json:"editor.decorations,omitempty"`
	FileHandlers      bool        `json:"editor.file_handlers,omitempty"`
	FsRead            FsAccess    `json:"fs.read,omitzero"`
	FsWrite           FsAccess    `json:"fs.write,omitzero"`
	SystemExec        []string    `json:"system.exec,omitempty"`
	SystemEnv         bool        `json:"system.env,omitempty"`
	NetworkHTTP       NetworkHTTP `json:"network.http,omitempty"`
	EventsFile        bool        `json:"events.file,omitempty"`
	EventsEditor      bool        `json:"events.editor,omitempty"`
	Settings          bool        `json:"settings,omitempty"`
	SettingsKeys      []string    `json:"settings_keys,omitempty"`
	Storage           bool        `json:"storage,omitempty"`
	ClipboardRead     bool        `json:"clipboard.read,omitempty"`
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
	check("editor.decorations", granted.EditorDecorations, requested.EditorDecorations)
	check("editor.file_handlers", granted.FileHandlers, requested.FileHandlers)
	checkFs := func(name string, g, r FsAccess) {
		for _, p := range r.Paths {
			if !g.Covers(p) {
				entries = append(entries, PermissionDiffEntry{Name: name, Value: p})
			}
		}
	}
	checkFs("fs.read", granted.FsRead, requested.FsRead)
	checkFs("fs.write", granted.FsWrite, requested.FsWrite)
	check("system.env", granted.SystemEnv, requested.SystemEnv)
	check("events.file", granted.EventsFile, requested.EventsFile)
	check("events.editor", granted.EventsEditor, requested.EventsEditor)
	check("settings", granted.Settings, requested.Settings)
	check("storage", granted.Storage, requested.Storage)
	check("clipboard.read", granted.ClipboardRead, requested.ClipboardRead)

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
	case "editor.decorations":
		allowed = ps.EditorDecorations
	case "editor.file_handlers":
		allowed = ps.FileHandlers
	case "fs.read":
		allowed = ps.FsRead.Enabled()
	case "fs.write":
		allowed = ps.FsWrite.Enabled()
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
	case "storage":
		allowed = ps.Storage
	case "clipboard.read":
		allowed = ps.ClipboardRead
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
	add("Editor decorations", ps.EditorDecorations)
	add("Handle file types", ps.FileHandlers)
	for _, p := range ps.FsRead.Paths {
		entries = append(entries, PermissionDiffEntry{Name: "Read files", Value: fsPathLabel(p)})
	}
	for _, p := range ps.FsWrite.Paths {
		entries = append(entries, PermissionDiffEntry{Name: "Write files", Value: fsPathLabel(p)})
	}
	add("Environment", ps.SystemEnv)
	add("File events", ps.EventsFile)
	add("Editor events", ps.EventsEditor)
	add("Plugin storage", ps.Storage)
	add("Clipboard read", ps.ClipboardRead)

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
