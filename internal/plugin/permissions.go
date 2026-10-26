package plugin

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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
// boolean (`true` = any destination, local ones included) or an array of
// hosts or IPs, each with an optional port (`["api.github.com",
// "127.0.0.1:4250"]`). A missing field or `false` means no network access.
type NetworkHTTP struct {
	All   bool
	Hosts []string
}

// Enabled reports whether the plugin may make any HTTP requests at all.
func (n NetworkHTTP) Enabled() bool { return n.All || len(n.Hosts) > 0 }

// AllowsHost reports whether the grant covers entry, a host or IP with an
// optional port. An entry without a port covers every port on its host.
// Hostnames match case-insensitively and IPs by value; `All` covers anything.
func (n NetworkHTTP) AllowsHost(entry string) bool {
	if n.All {
		return true
	}
	want, err := parseHostEntry(entry)
	return err == nil && n.allows(want.host, want.port)
}

// AllowsURL reports whether the grant covers u's host and port; a URL
// without a port uses its scheme's default.
func (n NetworkHTTP) AllowsURL(u *url.URL) bool {
	if n.All {
		return true
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	return n.allows(u.Hostname(), port)
}

func (n NetworkHTTP) allows(host, port string) bool {
	for _, h := range n.Hosts {
		e, err := parseHostEntry(h)
		if err == nil && sameHost(e.host, host) && (e.port == "" || e.port == port) {
			return true
		}
	}
	return false
}

// Validate rejects entries that could never match a request.
func (n NetworkHTTP) Validate() error {
	for _, h := range n.Hosts {
		if _, err := parseHostEntry(h); err != nil {
			return err
		}
	}
	return nil
}

type hostEntry struct {
	host string
	port string // empty: any port
}

// parseHostEntry accepts a hostname, an IPv4 address or a bracketed IPv6
// address, each with an optional port. Bare IPv6 is refused: "::1:4250"
// would be read as an address, not a port.
func parseHostEntry(s string) (hostEntry, error) {
	bad := fmt.Errorf(`network.http entry %q: want a hostname or IP with an optional port, like "api.github.com", "127.0.0.1:4250" or "[::1]:4250"`, s)
	var host, port string
	if rest, ok := strings.CutPrefix(s, "["); ok {
		end := strings.Index(rest, "]")
		if end < 0 {
			return hostEntry{}, bad
		}
		host = rest[:end]
		if ip := net.ParseIP(host); ip == nil || ip.To4() != nil {
			return hostEntry{}, bad
		}
		if after := rest[end+1:]; after != "" {
			p, ok := strings.CutPrefix(after, ":")
			if !ok {
				return hostEntry{}, bad
			}
			port = p
		}
		if strings.Contains(s, "]:") && port == "" {
			return hostEntry{}, bad
		}
	} else {
		host = s
		if i := strings.LastIndex(s, ":"); i >= 0 {
			host, port = s[:i], s[i+1:]
			if port == "" || strings.Contains(host, ":") {
				return hostEntry{}, bad
			}
		}
		if net.ParseIP(host) == nil && !validHostname(host) {
			return hostEntry{}, bad
		}
	}
	if port != "" {
		p, err := strconv.Atoi(port)
		if strings.Trim(port, "0123456789") != "" || err != nil || p < 1 || p > 65535 {
			return hostEntry{}, bad
		}
		port = strconv.Itoa(p)
	}
	return hostEntry{host: host, port: port}, nil
}

func validHostname(h string) bool {
	if h == "" || len(h) > 253 {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for _, r := range label {
			if r != '-' && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return false
			}
		}
	}
	return true
}

// sameHost compares IPs by value and ASCII hostnames case-insensitively.
// Non-ASCII names must match exactly: Unicode case folding equates names
// (σ and ς) that IDNA sends to different domains.
func sameHost(a, b string) bool {
	if ipA := net.ParseIP(a); ipA != nil {
		return ipA.Equal(net.ParseIP(b))
	}
	if isASCII(a) && isASCII(b) {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
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

// NetworkRPC is the network.rpc permission: the endpoints a plugin may open
// RPC connections to. Each is a host or IP with a required port
// ("127.0.0.1:4250") or "unix:" and a socket path that is absolute or starts
// with ~ or {config}.
type NetworkRPC struct {
	Endpoints []string
	// All is for plugins loaded with --plugin; a manifest cannot request it.
	All bool
}

const UnixEndpointPrefix = "unix:"

func (n NetworkRPC) Enabled() bool { return n.All || len(n.Endpoints) > 0 }

func (n NetworkRPC) Allows(endpoint string) bool {
	want, err := parseRPCEndpoint(endpoint)
	if err != nil {
		return false
	}
	if n.All {
		return true
	}
	for _, e := range n.Endpoints {
		if have, err := parseRPCEndpoint(e); err == nil && have.matches(want) {
			return true
		}
	}
	return false
}

type rpcEndpoint struct {
	socket string
	host   hostEntry
}

func (e rpcEndpoint) matches(o rpcEndpoint) bool {
	if e.socket != "" || o.socket != "" {
		return e.socket == o.socket
	}
	return sameHost(e.host.host, o.host.host) && e.host.port == o.host.port
}

func parseRPCEndpoint(s string) (rpcEndpoint, error) {
	if path, ok := strings.CutPrefix(s, UnixEndpointPrefix); ok {
		if path == FsAnyPath || path == FsWorkspace || !validFsPath(path) {
			return rpcEndpoint{}, fmt.Errorf(`network.rpc entry %q: a socket path must be absolute or start with ~ or %s`, s, FsConfig)
		}
		return rpcEndpoint{socket: path}, nil
	}
	h, err := parseHostEntry(s)
	if err != nil || h.port == "" {
		return rpcEndpoint{}, fmt.Errorf(`network.rpc entry %q: want a host with a port, like "127.0.0.1:4250", or a socket, like "unix:~/.cache/hub.sock"`, s)
	}
	return rpcEndpoint{host: h}, nil
}

func (n NetworkRPC) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.Endpoints)
}

func (n *NetworkRPC) UnmarshalJSON(data []byte) error {
	var endpoints []string
	if err := json.Unmarshal(data, &endpoints); err != nil {
		return fmt.Errorf("network.rpc must be an array of endpoints: %w", err)
	}
	for _, e := range endpoints {
		if _, err := parseRPCEndpoint(e); err != nil {
			return err
		}
	}
	n.Endpoints = endpoints
	return nil
}

// The approval dialog has room for about 24 characters of value.
const anyDestination = "any host, even local"

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
	TerminalControl   bool        `json:"terminal.control,omitempty"`
	TerminalRead      bool        `json:"terminal.read,omitempty"`
	NetworkHTTP       NetworkHTTP `json:"network.http,omitempty"`
	NetworkRPC        NetworkRPC  `json:"network.rpc,omitzero"`
	EventsFile        bool        `json:"events.file,omitempty"`
	EventsEditor      bool        `json:"events.editor,omitempty"`
	Settings          bool        `json:"settings,omitempty"`
	SettingsKeys      []string    `json:"settings_keys,omitempty"`
	Storage           bool        `json:"storage,omitempty"`
	ClipboardRead     bool        `json:"clipboard.read,omitempty"`
	Input             bool        `json:"input,omitempty"`
	ScreenRead        bool        `json:"screen.read,omitempty"`
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
	check("terminal.control", granted.TerminalControl, requested.TerminalControl)
	check("terminal.read", granted.TerminalRead, requested.TerminalRead)
	check("events.file", granted.EventsFile, requested.EventsFile)
	check("events.editor", granted.EventsEditor, requested.EventsEditor)
	check("settings", granted.Settings, requested.Settings)
	check("storage", granted.Storage, requested.Storage)
	check("clipboard.read", granted.ClipboardRead, requested.ClipboardRead)
	check("input", granted.Input, requested.Input)
	check("screen.read", granted.ScreenRead, requested.ScreenRead)

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
		entries = append(entries, PermissionDiffEntry{Name: "network.http", Value: anyDestination})
	} else if !granted.NetworkHTTP.All {
		for _, h := range requested.NetworkHTTP.Hosts {
			if !granted.NetworkHTTP.AllowsHost(h) {
				entries = append(entries, PermissionDiffEntry{Name: "network.http", Value: h})
			}
		}
	}

	for _, e := range requested.NetworkRPC.Endpoints {
		if !granted.NetworkRPC.Allows(e) {
			entries = append(entries, PermissionDiffEntry{Name: "network.rpc", Value: e})
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
	case "terminal.control":
		allowed = ps.TerminalControl
	case "terminal.read":
		allowed = ps.TerminalRead
	case "network.http":
		allowed = ps.NetworkHTTP.Enabled()
	case "network.rpc":
		allowed = ps.NetworkRPC.Enabled()
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
	case "input":
		allowed = ps.Input
	case "screen.read":
		allowed = ps.ScreenRead
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

// CheckExecPathRequest reports whether a plugin may ask the user to approve
// path: an absolute, clean path whose file name is one of the plugin's
// system.exec entries.
func (ps PermissionSet) CheckExecPathRequest(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("system.exec path %q must be absolute and clean", path)
	}
	name := execName(path)
	for _, b := range ps.SystemExec {
		if execName(b) == name {
			return nil
		}
	}
	return fmt.Errorf("permission denied: system.exec does not list %q", filepath.Base(path))
}

// execPathsFor keeps the user-approved paths whose file name is still in
// the declared allowlist.
func execPathsFor(paths, declared []string) []string {
	var out []string
	for _, p := range paths {
		if (PermissionSet{SystemExec: declared}).CheckExecPathRequest(p) == nil {
			out = append(out, p)
		}
	}
	return out
}

func execName(binary string) string {
	name := filepath.Base(binary)
	if runtime.GOOS == "windows" {
		name = strings.TrimSuffix(strings.ToLower(name), ".exe")
	}
	return name
}

func (ps PermissionSet) CheckRPCEndpoint(endpoint string) error {
	if ps.NetworkRPC.Allows(endpoint) {
		return nil
	}
	return fmt.Errorf("permission denied: network.rpc does not list %q", endpoint)
}

// CheckURL reports whether the plugin may make an HTTP request to u.
func (ps PermissionSet) CheckURL(u *url.URL) error {
	if ps.NetworkHTTP.AllowsURL(u) {
		return nil
	}
	return fmt.Errorf("permission denied: network.http does not list %q", u.Host)
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
	add("Control terminals", ps.TerminalControl)
	add("Read terminals", ps.TerminalRead)
	add("Environment", ps.SystemEnv)
	add("File events", ps.EventsFile)
	add("Editor events", ps.EventsEditor)
	add("Plugin storage", ps.Storage)
	add("Clipboard read", ps.ClipboardRead)
	add("Simulate input", ps.Input)
	add("Read screen", ps.ScreenRead)

	for _, b := range ps.SystemExec {
		entries = append(entries, PermissionDiffEntry{Name: "Run binary", Value: b})
	}

	if ps.NetworkHTTP.All {
		entries = append(entries, PermissionDiffEntry{Name: "HTTP requests", Value: anyDestination})
	}
	for _, h := range ps.NetworkHTTP.Hosts {
		entries = append(entries, PermissionDiffEntry{Name: "HTTP host", Value: h})
	}

	for _, e := range ps.NetworkRPC.Endpoints {
		entries = append(entries, PermissionDiffEntry{Name: "Accept commands from", Value: e})
	}

	add("Settings", ps.Settings)
	for _, k := range ps.SettingsKeys {
		entries = append(entries, PermissionDiffEntry{Name: "Settings key", Value: k})
	}

	return entries
}

// IntersectPermissions returns the granted permissions the manifest still
// declares, so a permission dropped from the manifest is no longer granted.
func IntersectPermissions(granted, declared PermissionSet) PermissionSet {
	return PermissionSet{
		PanelSidebar:      granted.PanelSidebar && declared.PanelSidebar,
		PanelBottom:       granted.PanelBottom && declared.PanelBottom,
		PanelDrawer:       granted.PanelDrawer && declared.PanelDrawer,
		PanelEditor:       granted.PanelEditor && declared.PanelEditor,
		Commands:          granted.Commands && declared.Commands,
		Keybindings:       granted.Keybindings && declared.Keybindings,
		EditorRead:        granted.EditorRead && declared.EditorRead,
		EditorWrite:       granted.EditorWrite && declared.EditorWrite,
		EditorDiagnostics: granted.EditorDiagnostics && declared.EditorDiagnostics,
		EditorBookmarks:   granted.EditorBookmarks && declared.EditorBookmarks,
		EditorDecorations: granted.EditorDecorations && declared.EditorDecorations,
		FileHandlers:      granted.FileHandlers && declared.FileHandlers,
		FsRead:            intersectFs(granted.FsRead, declared.FsRead),
		FsWrite:           intersectFs(granted.FsWrite, declared.FsWrite),
		SystemExec:        intersectStrings(granted.SystemExec, declared.SystemExec),
		SystemEnv:         granted.SystemEnv && declared.SystemEnv,
		TerminalControl:   granted.TerminalControl && declared.TerminalControl,
		TerminalRead:      granted.TerminalRead && declared.TerminalRead,
		NetworkHTTP:       intersectNetwork(granted.NetworkHTTP, declared.NetworkHTTP),
		NetworkRPC:        intersectRPC(granted.NetworkRPC, declared.NetworkRPC),
		EventsFile:        granted.EventsFile && declared.EventsFile,
		EventsEditor:      granted.EventsEditor && declared.EventsEditor,
		Settings:          granted.Settings && declared.Settings,
		SettingsKeys:      intersectStrings(granted.SettingsKeys, declared.SettingsKeys),
		Storage:           granted.Storage && declared.Storage,
		ClipboardRead:     granted.ClipboardRead && declared.ClipboardRead,
		Input:             granted.Input && declared.Input,
		ScreenRead:        granted.ScreenRead && declared.ScreenRead,
	}
}

func intersectStrings(granted, declared []string) []string {
	var out []string
	for _, s := range declared {
		if slices.Contains(granted, s) {
			out = append(out, s)
		}
	}
	return out
}

func intersectFs(granted, declared FsAccess) FsAccess {
	if declared.Covers(FsAnyPath) {
		return granted
	}
	var paths []string
	for _, p := range declared.Paths {
		if granted.Covers(p) {
			paths = append(paths, p)
		}
	}
	return FsAccess{Paths: paths}
}

func intersectRPC(granted, declared NetworkRPC) NetworkRPC {
	var endpoints []string
	for _, e := range declared.Endpoints {
		if granted.Allows(e) {
			endpoints = append(endpoints, e)
		}
	}
	return NetworkRPC{Endpoints: endpoints}
}

func intersectNetwork(granted, declared NetworkHTTP) NetworkHTTP {
	if declared.All {
		return granted
	}
	var hosts []string
	for _, h := range declared.Hosts {
		if granted.AllowsHost(h) {
			hosts = append(hosts, h)
			continue
		}
		// A grant for some ports of a host the manifest now lists without a
		// port still holds for those ports.
		want, err := parseHostEntry(h)
		if err != nil || want.port != "" {
			continue
		}
		for _, g := range granted.Hosts {
			if e, err := parseHostEntry(g); err == nil && e.port != "" && sameHost(e.host, want.host) {
				hosts = append(hosts, g)
			}
		}
	}
	return NetworkHTTP{Hosts: hosts}
}
