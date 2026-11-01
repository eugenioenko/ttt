package plugin

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writePluginDir creates a minimal valid plugin (manifest + entry) in a new
// subdir of parent and returns its path.
func writePluginDir(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"` + name + `","version":"1.0.0","entry":"main.ttt.lua"}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ttt.lua"), []byte("-- noop"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	m := NewManager(pluginsDir, regPath)
	m.LoadAll() // initializes m.registry
	return m, pluginsDir
}

// TestDenyPluginDeletesFreshInstall: a plugin ttt cloned into its own
// pluginsDir with no registry entry must be removed on deny (#358).
func TestDenyPluginDeletesFreshInstall(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	dir := writePluginDir(t, pluginsDir, "fresh")

	p := &Plugin{Name: "fresh", Dir: dir, Manifest: Manifest{Name: "fresh", Version: "1.0.0"}}
	if err := m.DenyPlugin(p); err != nil {
		t.Fatalf("DenyPlugin: %v", err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("denied fresh install should be deleted from disk")
	}
	if m.registry.Find("fresh") != nil {
		t.Fatal("a deleted fresh install should leave no registry entry")
	}
}

// TestDenyPluginDisablesOutsideDir: a plugin living outside pluginsDir (e.g.
// workspace-local) must never be deleted; it's disabled in the registry instead.
func TestDenyPluginDisablesOutsideDir(t *testing.T) {
	m, _ := newTestManager(t)
	externalRoot := t.TempDir()
	dir := writePluginDir(t, externalRoot, "local")

	p := &Plugin{Name: "local", Dir: dir, Manifest: Manifest{Name: "local", Version: "1.0.0"}}
	if err := m.DenyPlugin(p); err != nil {
		t.Fatalf("DenyPlugin: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("workspace-local plugin must not be deleted: %v", err)
	}
	entry := m.registry.Find("local")
	if entry == nil {
		t.Fatal("denied on-disk plugin should get a registry entry")
	}
	if entry.Enabled {
		t.Fatal("denied plugin's registry entry must be disabled")
	}
}

// TestDenyStopsReprompt is the end-to-end regression for #358: after
// denying a rediscovered on-disk plugin, a fresh LoadAll must not surface it
// for approval again.
func TestDenyStopsReprompt(t *testing.T) {
	m, _ := newTestManager(t)
	externalRoot := t.TempDir()
	dir := writePluginDir(t, externalRoot, "local")
	m.extraDirs = []string{externalRoot}

	needs := m.LoadAll()
	if len(needs) != 1 || needs[0].Name != "local" {
		t.Fatalf("expected 'local' to need approval on first discovery, got %v", needs)
	}

	if err := m.DenyPlugin(needs[0]); err != nil {
		t.Fatalf("DenyPlugin: %v", err)
	}

	if needs := m.LoadAll(); len(needs) != 0 {
		t.Fatalf("denied plugin must not re-prompt, got %d pending", len(needs))
	}
	_ = dir
}

func TestWithinDir(t *testing.T) {
	base := filepath.Join("home", "user", "plugins")
	cases := []struct {
		child string
		want  bool
	}{
		{filepath.Join(base, "foo"), true},
		{filepath.Join(base, "foo", "bar"), true},
		{base, false}, // the dir itself
		{filepath.Join("home", "user", "other"), false}, // sibling
		{filepath.Join("home", "user"), false},          // parent
		{filepath.Join(base, "..", "escape"), false},    // traversal
	}
	for _, c := range cases {
		if got := withinDir(base, c.child); got != c.want {
			t.Errorf("withinDir(%q, %q) = %v, want %v", base, c.child, got, c.want)
		}
	}
}

// A plugin that fails to compile reports the failure from inside Init, and is
// never added to m.plugins for a later SetLogFactory backfill to reach. If the
// log sink is not wired before Init, the error is lost entirely (#395).
func TestLoadAllLogsInitFailure(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")

	dir := writePluginDir(t, pluginsDir, "broken")
	entry := filepath.Join(dir, "main.ttt.lua")
	if err := os.WriteFile(entry, []byte("this is not lua !!"), 0644); err != nil {
		t.Fatal(err)
	}
	reg := `[{"name":"broken","version":"1.0.0","enabled":true,"permissions":{}}]`
	if err := os.WriteFile(regPath, []byte(reg), 0644); err != nil {
		t.Fatal(err)
	}

	var logged []string
	m := NewManager(pluginsDir, regPath)
	m.SetLogFactory(func(name string) func(string, string) {
		return func(level, message string) {
			logged = append(logged, name+" "+level+" "+message)
		}
	})
	m.LoadAll()

	if len(logged) == 0 {
		t.Fatal("a plugin that fails to compile logged nothing")
	}
	if !strings.Contains(logged[0], "broken error init:") {
		t.Errorf("unexpected log line: %q", logged[0])
	}
	if len(m.Plugins()) != 0 {
		t.Errorf("a plugin that failed to init should not be loaded, got %d", len(m.Plugins()))
	}
}

// An update whose new code fails to init stays listed with its error, so it can
// be reloaded once fixed, but must not leave panel registrations behind.
func TestApplyUpdateInitFailureDropsRegistrations(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	dir := writePluginDir(t, pluginsDir, "panel")
	entry := filepath.Join(dir, "main.ttt.lua")
	registerSidebar := `require("ttt").register({ sidebar = { title = "P", render = function() end } })`
	if err := os.WriteFile(entry, []byte(registerSidebar), 0644); err != nil {
		t.Fatal(err)
	}
	reg := `[{"name":"panel","version":"1.0.0","enabled":true,"permissions":{"panel.sidebar":true}}]`
	manifestJSON := `{"name":"panel","version":"1.0.0","entry":"main.ttt.lua","permissions":{"panel.sidebar":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifestJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, []byte(reg), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(pluginsDir, regPath)
	m.LoadAll()
	old := m.FindPlugin("panel")
	if old == nil || len(m.SidebarPanels) != 1 {
		t.Fatalf("plugin not loaded with its sidebar: %v, %d panels", old, len(m.SidebarPanels))
	}
	defer m.Shutdown()

	if err := os.WriteFile(entry, []byte(registerSidebar+"\nerror('boom')"), 0644); err != nil {
		t.Fatal(err)
	}
	u := m.PrepareUpdate("panel")
	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	u.manifest = manifest
	if _, _, err := m.ApplyUpdate(u); err == nil {
		t.Fatal("ApplyUpdate succeeded with failing init code")
	}

	if old.Loaded() {
		t.Error("old plugin still loaded")
	}
	if len(m.SidebarPanels) != 0 {
		t.Errorf("sidebar registrations = %d, want 0", len(m.SidebarPanels))
	}
	if p := m.FindPlugin("panel"); p == nil || p.Loaded() || p.LastError == nil {
		t.Errorf("failed plugin should stay listed, unloaded, with its error: %v", p)
	}
}

// A plugin that loads cleanly must still get its log sink, so ttt.log works
// from the plugin's own init.
func TestLoadAllWiresLogForHealthyPlugin(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")

	dir := writePluginDir(t, pluginsDir, "healthy")
	entry := filepath.Join(dir, "main.ttt.lua")
	if err := os.WriteFile(entry, []byte(`local ttt = require("ttt")`+"\n"+`ttt.log("hello from init")`), 0644); err != nil {
		t.Fatal(err)
	}
	reg := `[{"name":"healthy","version":"1.0.0","enabled":true,"permissions":{}}]`
	if err := os.WriteFile(regPath, []byte(reg), 0644); err != nil {
		t.Fatal(err)
	}

	var logged []string
	m := NewManager(pluginsDir, regPath)
	m.SetLogFactory(func(name string) func(string, string) {
		return func(level, message string) { logged = append(logged, message) }
	})
	m.LoadAll()

	if len(logged) != 1 || logged[0] != "hello from init" {
		t.Errorf("logged = %v, want a single %q", logged, "hello from init")
	}
}

// A plugin's AppVersion must be set before Init runs, so ttt.version() works
// from the plugin's own init during startup LoadAll.
func TestLoadAllWiresAppVersionForHealthyPlugin(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")

	dir := writePluginDir(t, pluginsDir, "healthy")
	entry := filepath.Join(dir, "main.ttt.lua")
	if err := os.WriteFile(entry, []byte(`local ttt = require("ttt")`+"\n"+`ttt.log(ttt.version())`), 0644); err != nil {
		t.Fatal(err)
	}
	reg := `[{"name":"healthy","version":"1.0.0","enabled":true,"permissions":{}}]`
	if err := os.WriteFile(regPath, []byte(reg), 0644); err != nil {
		t.Fatal(err)
	}

	var logged []string
	m := NewManager(pluginsDir, regPath)
	m.SetLogFactory(func(name string) func(string, string) {
		return func(level, message string) { logged = append(logged, message) }
	})
	m.SetAppVersion("9.9.9")
	m.LoadAll()

	if len(logged) != 1 || logged[0] != "9.9.9" {
		t.Errorf("logged = %v, want a single %q", logged, "9.9.9")
	}
}

// A pending plugin's AppVersion must be set before Init runs in ApproveAndLoad,
// so ttt.version() is available during the entry point before any application
// wiring runs (#997).
func TestApproveAndLoadWiresAppVersionDuringInit(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")

	dir := writePluginDir(t, pluginsDir, "pending")
	entry := filepath.Join(dir, "main.ttt.lua")
	if err := os.WriteFile(entry, []byte(`local ttt = require("ttt")`+"\n"+`ttt.log(ttt.version())`), 0644); err != nil {
		t.Fatal(err)
	}

	var logged []string
	m := NewManager(pluginsDir, regPath)
	m.SetLogFactory(func(name string) func(string, string) {
		return func(level, message string) { logged = append(logged, message) }
	})
	m.SetAppVersion("1.2.3")

	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	if len(logged) != 0 {
		t.Fatalf("logged before approval = %q, want none", logged)
	}

	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatal(err)
	}

	if len(logged) != 1 || logged[0] != "1.2.3" {
		t.Errorf("logged = %q, want a single %q", logged, "1.2.3")
	}
}

// A permission dropped from the manifest is no longer granted, and asking for
// it again needs approval (#915).
func TestDroppedPermissionIsRevoked(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	dir := writePluginDir(t, pluginsDir, "p")
	reg := `[{"name":"p","version":"1.0.0","enabled":true,"permissions":{"editor.read":true,"fs.write":true}}]`
	if err := os.WriteFile(regPath, []byte(reg), 0644); err != nil {
		t.Fatal(err)
	}
	manifestJSON := `{"name":"p","version":"1.0.0","entry":"main.ttt.lua","permissions":{"editor.read":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifestJSON), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(pluginsDir, regPath)
	m.LoadAll()
	defer m.Shutdown()
	assertRevoked := func(when string) {
		t.Helper()
		p := m.FindPlugin("p")
		if p == nil {
			t.Fatalf("after %s: plugin not loaded", when)
		}
		if p.Granted.Check("fs.write") == nil || p.Granted.Check("editor.read") != nil {
			t.Fatalf("after %s: want editor.read only, got %+v", when, p.Granted)
		}
		stored, err := os.ReadFile(regPath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(stored), "fs.write") {
			t.Fatalf("after %s: fs.write still stored: %s", when, stored)
		}
	}
	assertRevoked("LoadAll")

	m.registry.Find("p").Permissions.FsWrite = FsAccess{Paths: []string{FsWorkspace}}
	u := m.PrepareUpdate("p")
	u.manifest, _ = LoadManifest(dir)
	if _, _, err := m.ApplyUpdate(u); err != nil {
		t.Fatal(err)
	}
	assertRevoked("ApplyUpdate")

	u.manifest.Permissions.FsWrite = FsAccess{Paths: []string{FsWorkspace}}
	if DiffPermissions(m.registry.Find("p").Permissions, u.manifest.Permissions).IsEmpty() {
		t.Error("requesting fs.write again should need approval")
	}
}

func TestDisabledPluginUpdateRevokesDroppedPermission(t *testing.T) {
	m, _ := newTestManager(t)
	m.registry.AddOrUpdate("p", "", "", "", "1.0.0", PermissionSet{EditorRead: true})
	m.registry.SetEnabled("p", false)
	if err := m.registry.Save(); err != nil {
		t.Fatal(err)
	}
	u := m.PrepareUpdate("p")
	u.manifest = Manifest{Name: "p", Version: "2.0.0", Entry: "main.ttt.lua"}
	m.registry.saveHook = func() error { return errors.New("simulated save error") }
	if _, _, err := m.ApplyUpdate(u); err == nil {
		t.Fatal("update succeeded despite a failed permission save")
	}
	entry := m.registry.Find("p")
	if !entry.Permissions.EditorRead || entry.Version != "1.0.0" {
		t.Fatalf("failed update changed registry state: %+v", entry)
	}
	m.registry.saveHook = nil
	p, needsApproval, err := m.ApplyUpdate(u)
	if err != nil || needsApproval || p == nil || p.Loaded() {
		t.Fatalf("disabled update: plugin %v, needs approval %v, err %v", p, needsApproval, err)
	}
	stored, err := LoadRegistry(m.registryPath)
	if err != nil {
		t.Fatal(err)
	}
	entry = stored.Find("p")
	if entry.Permissions.EditorRead || entry.Enabled || entry.Version != "2.0.0" {
		t.Fatalf("updated registry retained dropped permission or enabled plugin: %+v", entry)
	}
	u.manifest.Permissions.EditorRead = true
	if _, needsApproval, err := m.ApplyUpdate(u); err != nil || !needsApproval {
		t.Fatalf("requesting the dropped permission again: needs approval %v, err %v", needsApproval, err)
	}
}

// Denying a request for more permissions must not record them as approved:
// enabling the plugin afterwards would load it with them.
func TestDenyKeepsThePreviouslyApprovedPermissions(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	dir := writePluginDir(t, pluginsDir, "grower")
	manifest := `{"name":"grower","version":"2.0.0","entry":"main.ttt.lua","permissions":{"commands":true,"system.exec":["curl"]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	registry := `[{"name": "grower", "version": "1.0.0", "enabled": true, "permissions": {"commands": true}}]`
	if err := os.WriteFile(regPath, []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(pluginsDir, regPath)
	t.Cleanup(m.Shutdown)

	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	if err := m.DenyPlugin(pending[0]); err != nil {
		t.Fatal(err)
	}
	if got := m.registry.Find("grower").Permissions.SystemExec; len(got) != 0 {
		t.Errorf("denied system.exec recorded as approved: %v", got)
	}
	p, err := m.SetEnabled("grower", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Granted.CheckExec("curl"); err == nil {
		t.Error("enabling after a deny granted the denied system.exec")
	}
}

func TestDenyRecordsNoPermissionsForANewPlugin(t *testing.T) {
	m, _ := newTestManager(t)
	dir := writePluginDir(t, t.TempDir(), "local")
	p := &Plugin{Name: "local", Dir: dir, Manifest: Manifest{Name: "local", Version: "1.0.0", Permissions: PermissionSet{Commands: true}}}
	if err := m.DenyPlugin(p); err != nil {
		t.Fatal(err)
	}
	if got := m.registry.Find("local").Permissions; got.Commands {
		t.Errorf("denied plugin recorded as approved: %+v", got)
	}
}

func TestDenyRevokesDroppedPermissionsBeforeTheyAreRequestedAgain(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	t.Cleanup(m.Shutdown)
	dir := writePluginDir(t, pluginsDir, "mixed")
	manifestPath := filepath.Join(dir, "plugin.ttt.json")
	denied := `{"name":"mixed","version":"2.0.0","entry":"main.ttt.lua","permissions":{"commands":true,"editor.read":true}}`
	if err := os.WriteFile(manifestPath, []byte(denied), 0644); err != nil {
		t.Fatal(err)
	}
	m.registry.AddOrUpdate("mixed", "", "", "", "1.0.0", PermissionSet{Commands: true, SystemExec: []string{"curl"}})
	if err := m.registry.Save(); err != nil {
		t.Fatal(err)
	}
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	if err := m.DenyPlugin(pending[0]); err != nil {
		t.Fatal(err)
	}
	stored, err := LoadRegistry(m.registryPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := stored.Find("mixed")
	if entry == nil {
		t.Fatal("denied plugin is missing from the registry")
	}
	if entry.Enabled || !entry.Permissions.Commands || entry.Permissions.EditorRead || len(entry.Permissions.SystemExec) != 0 {
		t.Errorf("denial must retain only approved permissions still declared: %+v", entry)
	}
	requestedAgain := `{"name":"mixed","version":"3.0.0","entry":"main.ttt.lua","permissions":{"commands":true,"system.exec":["curl"]}}`
	if err := os.WriteFile(manifestPath, []byte(requestedAgain), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := m.SetEnabled("mixed", true)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Granted.Commands || p.Granted.CheckExec("curl") == nil {
		t.Errorf("re-enabling restored a dropped permission: %+v", p.Granted)
	}
	m.Shutdown()
	again := NewManager(pluginsDir, m.registryPath)
	t.Cleanup(again.Shutdown)
	if pending := again.LoadAll(); len(pending) != 1 || pending[0].Name != "mixed" {
		t.Error("requesting the dropped permission again must require approval after restart")
	}
}

// A plugin waiting for approval must still be asked about after a restart,
// even when something else saved the registry in the meantime.
func TestPendingApprovalSurvivesARegistrySave(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	for name, perms := range map[string]string{
		"asks":    `{"commands": true, "system.exec": ["curl"]}`,
		"narrows": `{"commands": true}`,
	} {
		dir := writePluginDir(t, pluginsDir, name)
		manifest := `{"name":"` + name + `","version":"1.0.0","entry":"main.ttt.lua","permissions":` + perms + `}`
		if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// narrows' grant has a permission its manifest dropped, so loading it saves the registry.
	registry := `[
		{"name": "asks", "version": "1.0.0", "enabled": true, "permissions": {"commands": true}},
		{"name": "narrows", "version": "1.0.0", "enabled": true, "permissions": {"commands": true, "editor.read": true}}
	]`
	if err := os.WriteFile(regPath, []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}
	first := NewManager(pluginsDir, regPath)
	if pending := first.LoadAll(); len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	first.Shutdown()

	second := NewManager(pluginsDir, regPath)
	t.Cleanup(second.Shutdown)
	if pending := second.LoadAll(); len(pending) != 1 || pending[0].Name != "asks" {
		t.Errorf("after a restart pending = %d, want asks again", len(pending))
	}
}

// An update that needs approval must still be asked about after a restart.
func TestUpdateAwaitingApprovalSurvivesARestart(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	dir := writePluginDir(t, pluginsDir, "grower")
	registry := `[{"name": "grower", "version": "1.0.0", "enabled": true, "permissions": {}}]`
	if err := os.WriteFile(regPath, []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(pluginsDir, regPath)
	if pending := m.LoadAll(); len(pending) != 0 {
		t.Fatalf("pending = %d before the update", len(pending))
	}
	manifest := `{"name":"grower","version":"2.0.0","entry":"main.ttt.lua","permissions":{"system.exec":["curl"]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	u := m.PrepareUpdate("grower")
	u.manifest, _ = LoadManifest(dir)
	if _, needsApproval, err := m.ApplyUpdate(u); err != nil || !needsApproval {
		t.Fatalf("ApplyUpdate: needsApproval %v, err %v", needsApproval, err)
	}
	m.Shutdown()

	again := NewManager(pluginsDir, regPath)
	t.Cleanup(again.Shutdown)
	if pending := again.LoadAll(); len(pending) != 1 {
		t.Errorf("after a restart pending = %d, want the update asked about again", len(pending))
	}
}

type hostCheckedNetworkStub struct {
	mockNetworkAPI
	checks []func(*url.URL) error
}

func (s *hostCheckedNetworkStub) ForPlugin(allow func(*url.URL) error) NetworkAPI {
	s.checks = append(s.checks, allow)
	return &s.mockNetworkAPI
}

// A plugin's init code runs before the app wires it, so the manager must
// already give it a network API that knows its hosts.
func TestManagerGivesEachPluginANetworkCheckedAgainstItsHosts(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	stub := &hostCheckedNetworkStub{}
	m.SetNetworkAPI(stub)
	dir := writePluginDir(t, pluginsDir, "netty")
	manifest := `{"name":"netty","version":"1.0.0","entry":"main.ttt.lua","permissions":{"network.http":["api.example.com"]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatal(err)
	}
	if len(stub.checks) != 1 {
		t.Fatalf("ForPlugin calls = %d, want 1", len(stub.checks))
	}
	if pending[0].Network != &stub.mockNetworkAPI {
		t.Error("the plugin does not use the copy ForPlugin returned")
	}
	if err := stub.checks[0](&url.URL{Scheme: "https", Host: "api.example.com"}); err != nil {
		t.Errorf("listed host refused: %v", err)
	}
	if err := stub.checks[0](&url.URL{Scheme: "https", Host: "other.example.com"}); err == nil {
		t.Error("unlisted host allowed")
	}
}

// network.http used to stop short of local services, so a grant made before
// that changed, true or a list whose hostnames can resolve to a local address,
// has to be asked for again.
func TestManagerAsksAgainForANetworkGrantFromBeforeLocalAccess(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	perms := map[string]string{
		"old":            `{"network.http": true}`,
		"old-listed":     `{"network.http": ["api.example.com"]}`,
		"current":        `{"network.http": true}`,
		"current-listed": `{"network.http": ["api.example.com"]}`,
	}
	for name, p := range perms {
		dir := writePluginDir(t, pluginsDir, name)
		manifest := `{"name":"` + name + `","version":"1.0.0","entry":"main.ttt.lua","permissions":` + p + `}`
		if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}
	}
	registry := `[
		{"name": "old", "version": "1.0.0", "enabled": true, "permissions": {"network.http": true}},
		{"name": "old-listed", "version": "1.0.0", "enabled": true, "permissions": {"network.http": ["api.example.com"]}},
		{"name": "current", "version": "1.0.0", "enabled": true, "permissions": {"network.http": true}, "grantsVersion": 2},
		{"name": "current-listed", "version": "1.0.0", "enabled": true, "permissions": {"network.http": ["api.example.com"]}, "grantsVersion": 2}
	]`
	if err := os.WriteFile(regPath, []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(pluginsDir, regPath)
	t.Cleanup(m.Shutdown)

	pending := m.LoadAll()
	names := []string{}
	for _, p := range pending {
		names = append(names, p.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"old", "old-listed"}) {
		t.Fatalf("pending = %v, want [old old-listed]", names)
	}
	for _, p := range pending {
		if err := m.ApproveAndLoad(p); err != nil {
			t.Fatal(err)
		}
		if m.registry.Find(p.Name).GrantsVersion != grantsVersion {
			t.Errorf("%s: the new approval does not record the current grants version", p.Name)
		}
	}
	if pending := NewManager(pluginsDir, regPath).LoadAll(); len(pending) != 0 {
		t.Errorf("approved again but still pending after a restart: %d", len(pending))
	}
}

// A denied plugin must not get its old network grant back. The migration
// clears a grant from before local access was allowed, and the denial has to
// keep it cleared (#989).
func TestDenyingALegacyNetworkGrantDoesNotRestoreIt(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	dir := writePluginDir(t, pluginsDir, "old-listed")
	manifest := `{"name":"old-listed","version":"1.0.0","entry":"main.ttt.lua","permissions":{"network.http":["api.example.com"]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	// A grant from before the local-access migration: no grants version.
	registry := `[{"name": "old-listed", "version": "1.0.0", "enabled": true, "permissions": {"network.http": ["api.example.com"]}}]`
	if err := os.WriteFile(regPath, []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(pluginsDir, regPath)
	t.Cleanup(m.Shutdown)

	pending := m.LoadAll()
	if len(pending) != 1 || pending[0].Name != "old-listed" {
		t.Fatalf("pending = %v, want the legacy grant to be asked for again", pending)
	}
	if err := m.DenyPlugin(pending[0]); err != nil {
		t.Fatal(err)
	}
	entry := m.registry.Find("old-listed")
	if entry == nil {
		t.Fatal("the denied plugin is missing from the registry")
	}
	if entry.Permissions.NetworkHTTP.Enabled() {
		t.Errorf("the denied plugin kept its network grant: %+v", entry.Permissions.NetworkHTTP)
	}
	// Read the file without LoadRegistry's migration: the denial itself must
	// have been persisted, not just the in-memory grant cleared.
	savedBytes, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved []RegistryEntry
	if err := json.Unmarshal(savedBytes, &saved); err != nil {
		t.Fatal(err)
	}
	savedEntry := findRegistryEntry(saved, "old-listed")
	if savedEntry == nil {
		t.Fatal("the denied plugin is missing from the saved registry")
	}
	if savedEntry.Enabled {
		t.Error("the denied plugin is still enabled in the saved registry")
	}
	if savedEntry.Permissions.NetworkHTTP.Enabled() {
		t.Errorf("the saved registry kept the denied network grant: %+v", savedEntry.Permissions.NetworkHTTP)
	}
	if savedEntry.GrantsVersion != grantsVersion {
		t.Errorf("saved grants version = %d, want %d", savedEntry.GrantsVersion, grantsVersion)
	}
	// A restart must neither bring the old grant back nor ask for it again.
	again := NewManager(pluginsDir, regPath)
	t.Cleanup(again.Shutdown)
	if pending := again.LoadAll(); len(pending) != 0 {
		t.Errorf("the denied plugin was asked for again after a restart: %v", pending)
	}
	if entry := again.registry.Find("old-listed"); entry != nil && entry.Permissions.NetworkHTTP.Enabled() {
		t.Errorf("the old network grant came back after a restart: %+v", entry.Permissions.NetworkHTTP)
	}
}

func findRegistryEntry(entries []RegistryEntry, name string) *RegistryEntry {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

func TestAnAnyDestinationGrantApprovedNowIsNotAskedForAgain(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	dir := writePluginDir(t, pluginsDir, "fresh")
	manifest := `{"name":"fresh","version":"1.0.0","entry":"main.ttt.lua","permissions":{"network.http":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(pluginsDir, regPath)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatal(err)
	}
	m.Shutdown()

	again := NewManager(pluginsDir, regPath)
	t.Cleanup(again.Shutdown)
	if pending := again.LoadAll(); len(pending) != 0 {
		t.Errorf("approved under the current rules but pending again after a restart: %d", len(pending))
	}
}

func TestDenyingAnOldAnyDestinationGrantDoesNotGrantIt(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	dir := writePluginDir(t, pluginsDir, "old")
	manifest := `{"name":"old","version":"1.0.0","entry":"main.ttt.lua","permissions":{"network.http":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, []byte(`[{"name": "old", "version": "1.0.0", "enabled": true, "permissions": {"network.http": true}}]`), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(pluginsDir, regPath)
	t.Cleanup(m.Shutdown)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	if err := m.DenyPlugin(pending[0]); err != nil {
		t.Fatal(err)
	}
	p, err := m.SetEnabled("old", true)
	if err != nil {
		t.Fatal(err)
	}
	if p.Granted.NetworkHTTP.Enabled() {
		t.Errorf("enabled after a deny with network.http %+v", p.Granted.NetworkHTTP)
	}
}

func TestSetEnabledRequestsApprovalForAdditionalPermissions(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	dir := filepath.Join(pluginsDir, "grower")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	entrySrc := `
		local ttt = require("ttt")
		ttt.log("info", "entry executed")
		ttt.register({
			commands = {
				{ id = "grower.hello", title = "Grower: Hello", handler = function() end },
			},
		})
	`
	if err := os.WriteFile(filepath.Join(dir, "main.ttt.lua"), []byte(entrySrc), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"grower","version":"2.0.0","entry":"main.ttt.lua","permissions":{"commands":true,"system.exec":["curl"]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	registry := `[{"name": "grower", "version": "1.0.0", "enabled": true, "permissions": {"commands": true, "editor.read": true}}]`
	if err := os.WriteFile(regPath, []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}

	var logged []string
	m := NewManager(pluginsDir, regPath)
	t.Cleanup(m.Shutdown)
	m.SetLogFactory(func(string) func(string, string) {
		return func(_, msg string) { logged = append(logged, msg) }
	})

	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	if err := m.DenyPlugin(pending[0]); err != nil {
		t.Fatal(err)
	}

	p, err := m.SetEnabled("grower", true)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Loaded() || p.Enabled || m.FindPlugin("grower") != nil {
		t.Fatalf("SetEnabled initialized plugin before approval: p=%+v loaded=%v inManager=%v", p, p != nil && p.Loaded(), m.FindPlugin("grower"))
	}
	if len(logged) != 0 {
		t.Fatalf("entry point executed before approval: logged=%v", logged)
	}

	// Pending approval stays discoverable after a restart without executing the entry point.
	restarted := NewManager(pluginsDir, regPath)
	t.Cleanup(restarted.Shutdown)
	if restartPending := restarted.LoadAll(); len(restartPending) != 1 || restartPending[0].Name != "grower" {
		t.Fatalf("pending after restart = %v, want [grower]", restartPending)
	}

	// Denying again leaves it disabled and preserves only approved permissions still declared.
	if err := m.DenyPlugin(p); err != nil {
		t.Fatal(err)
	}
	entry := m.registry.Find("grower")
	if entry == nil || entry.Enabled || !entry.Permissions.Commands || entry.Permissions.EditorRead || len(entry.Permissions.SystemExec) != 0 {
		t.Fatalf("registry entry after deny = %+v, want disabled with commands only", entry)
	}

	// Enabling again and approving initializes the plugin with the full approved grant.
	p, err = m.SetEnabled("grower", true)
	if err != nil {
		t.Fatal(err)
	}
	if p.Loaded() {
		t.Fatal("SetEnabled loaded plugin before ApproveAndLoad")
	}
	if err := m.ApproveAndLoad(p); err != nil {
		t.Fatal(err)
	}
	if !p.Loaded() || !p.Enabled || m.FindPlugin("grower") != p {
		t.Fatal("ApproveAndLoad did not initialize and register plugin")
	}
	if !p.Granted.Commands || p.Granted.CheckExec("curl") != nil {
		t.Fatalf("approved grant = %+v, want commands and curl", p.Granted)
	}
	if len(logged) != 1 || logged[0] != "entry executed" {
		t.Fatalf("logged after approval = %v, want [entry executed]", logged)
	}

	// Ordinary disable and re-enable with unchanged permissions loads immediately.
	if _, err := m.SetEnabled("grower", false); err != nil {
		t.Fatal(err)
	}
	if m.FindPlugin("grower") != nil {
		t.Fatal("plugin still loaded after disable")
	}
	p2, err := m.SetEnabled("grower", true)
	if err != nil {
		t.Fatal(err)
	}
	if p2 == nil || !p2.Loaded() || !p2.Enabled || m.FindPlugin("grower") != p2 {
		t.Fatalf("SetEnabled with unchanged permissions did not load plugin: %+v", p2)
	}
	if !p2.Granted.Commands || p2.Granted.CheckExec("curl") != nil {
		t.Fatalf("re-enabled grant = %+v, want commands and curl", p2.Granted)
	}
}

func TestApproveAndLoadRejectsStaleApprovalWhenDisabled(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	m := NewManager(pluginsDir, regPath)

	pluginDir := filepath.Join(pluginsDir, "stale-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"name": "stale-plugin",
		"version": "1.0.0",
		"entry": "main.ttt.lua",
		"permissions": {
			"commands": true,
			"system.exec": ["curl"]
		}
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "main.ttt.lua"), []byte("-- noop"), 0644); err != nil {
		t.Fatal(err)
	}

	// Registered with initial commands permission only.
	registry := `[{"name": "stale-plugin", "version": "1.0.0", "enabled": false, "permissions": {"commands": true}}]`
	if err := os.WriteFile(regPath, []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}
	m.LoadAll()

	// 1. Enable requests approval for additional system.exec permission.
	pending, err := m.SetEnabled("stale-plugin", true)
	if err != nil {
		t.Fatal(err)
	}
	if pending == nil || pending.Loaded() {
		t.Fatal("expected uninitialized pending plugin awaiting approval")
	}

	// 2. Plugin is disabled before approval is resolved.
	if _, err := m.SetEnabled("stale-plugin", false); err != nil {
		t.Fatal(err)
	}
	entry := m.registry.Find("stale-plugin")
	if entry == nil || entry.Enabled {
		t.Fatalf("expected disabled registry entry, got %+v", entry)
	}

	// 3. Stale approval must be rejected.
	if err := m.ApproveAndLoad(pending); err == nil {
		t.Fatal("ApproveAndLoad on disabled plugin should fail, got nil")
	}

	// 4. Stale approval must not grant additional permissions or execute entry.
	entry = m.registry.Find("stale-plugin")
	if entry == nil || entry.Enabled {
		t.Fatalf("registry entry must remain disabled, got: %+v", entry)
	}
	if len(entry.Permissions.SystemExec) != 0 {
		t.Fatalf("stale approval granted permissions: %+v", entry.Permissions)
	}
	if pending.Loaded() || m.FindPlugin("stale-plugin") != nil {
		t.Fatal("stale plugin must not be loaded or registered in active plugins")
	}

	// 5. First-time installation with no registry entry must still succeed.
	freshDir := writePluginDir(t, pluginsDir, "fresh-plugin")
	freshMf, err := LoadManifest(freshDir)
	if err != nil {
		t.Fatal(err)
	}
	freshPlugin := &Plugin{
		Name:     freshMf.Name,
		Dir:      freshDir,
		Manifest: freshMf,
	}
	if err := m.ApproveAndLoad(freshPlugin); err != nil {
		t.Fatalf("first-time install failed: %v", err)
	}
	freshEntry := m.registry.Find("fresh-plugin")
	if freshEntry == nil || !freshEntry.Enabled {
		t.Fatalf("fresh plugin should be enabled in registry, got %+v", freshEntry)
	}
}

func TestApproveAndLoadEnabledPluginWithAdditionalPermissions(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	m := NewManager(pluginsDir, regPath)

	pluginDir := filepath.Join(pluginsDir, "grower")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"name": "grower",
		"version": "1.0.0",
		"entry": "main.ttt.lua",
		"permissions": {
			"commands": true,
			"system.exec": ["curl"]
		}
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "main.ttt.lua"), []byte("-- noop"), 0644); err != nil {
		t.Fatal(err)
	}

	// Registered as enabled with initial commands permission only.
	registry := `[{"name": "grower", "version": "1.0.0", "enabled": true, "permissions": {"commands": true}}]`
	if err := os.WriteFile(regPath, []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}

	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatalf("enabled approval failed: %v", err)
	}

	entry := m.registry.Find("grower")
	if entry == nil || !entry.Enabled || len(entry.Permissions.SystemExec) != 1 || m.FindPlugin("grower") == nil {
		t.Fatalf("enabled approval did not persist permission or load plugin: %+v", entry)
	}
}

// --- symlinked plugin directories (#1029) ---

// A plugin installed as a directory symlink outside the plugins dir must load
// like a real directory; os.ReadDir reports the link type, not the target.
func TestLoadAllFollowsSymlinkedPluginDir(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	external := writePluginDir(t, t.TempDir(), "linked")
	if err := os.Symlink(external, filepath.Join(pluginsDir, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	reg := `[{"name": "linked", "version": "1.0.0", "enabled": true, "permissions": {}}]`
	if err := os.WriteFile(regPath, []byte(reg), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(pluginsDir, regPath)
	defer m.Shutdown()
	m.LoadAll()

	if m.FindPlugin("linked") == nil {
		t.Fatal("a symlinked plugin directory was not loaded")
	}
}

// A dangling symlink names nothing, so it must be skipped rather than made to
// look like a plugin that failed to load.
func TestLoadAllSkipsDanglingSymlink(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(pluginsDir, "dangling")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if pending := m.LoadAll(); len(pending) != 0 {
		t.Fatalf("pending = %d, want 0 for a dangling symlink", len(pending))
	}
}

// A symlink pointing at a sibling inside the plugins dir resolves to a
// directory the scan already visits, so following it would load the same
// plugin twice.
func TestLoadAllSkipsSymlinkInsidePluginsDir(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	writePluginDir(t, pluginsDir, "real")
	if err := os.Symlink(filepath.Join(pluginsDir, "real"), filepath.Join(pluginsDir, "alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1 (only the real directory)", len(pending))
	}
	if pending[0].Name != "real" {
		t.Fatalf("pending[0].Name = %q, want the real directory", pending[0].Name)
	}
	if pending[0].Dir != filepath.Join(pluginsDir, "real") {
		t.Fatalf("pending[0].Dir = %q, want the real directory", pending[0].Dir)
	}
}

// A symlink to a regular file keeps the pre-existing behavior: only
// directories are plugin candidates.
func TestLoadAllSkipsSymlinkToFile(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	target := filepath.Join(t.TempDir(), "plugin.ttt.json")
	if err := os.WriteFile(target, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(pluginsDir, "filelink")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if pending := m.LoadAll(); len(pending) != 0 {
		t.Fatalf("pending = %d, want 0 for a symlink to a file", len(pending))
	}
}

// makeRegistryUnwritable makes the registry file read-only, so it can still
// be read but a save fails the way a permission or quota error would. A
// privileged user can still write such a file, so the write is probed and
// the test skipped when the filesystem does not enforce it.
func makeRegistryUnwritable(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0644) })
	probe, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err == nil {
		probe.Close()
		t.Skip("this filesystem still allows writing a read-only file")
	}
}

// A dropped permission is only really dropped once the reduction reaches the
// registry on disk, so a failed save must be reported and must leave the
// in-memory permissions retryable.
func TestGrantReportsAFailedRegistrySave(t *testing.T) {
	m, _ := newTestManager(t)
	m.registry.AddOrUpdate("mixed", "", "", "", "1.0.0", PermissionSet{Commands: true, EditorRead: true})
	if err := m.registry.Save(); err != nil {
		t.Fatal(err)
	}
	makeRegistryUnwritable(t, m.registry.path)
	entry := m.registry.Find("mixed")

	granted, err := m.grant(entry, PermissionSet{Commands: true})
	if err == nil {
		t.Fatal("grant reported success although the registry could not be saved")
	}
	if granted.Commands || granted.EditorRead {
		t.Errorf("a failed grant must not report permissions, got %+v", granted)
	}
	if !entry.Permissions.Commands || !entry.Permissions.EditorRead {
		t.Errorf("a failed save must keep the in-memory permissions, got %+v", entry.Permissions)
	}
}

// A retry after a failed save must persist the reduction: the in-memory
// permissions must not already look like the reduced grant, or the save is
// skipped and the dropped permission stays in the registry forever.
func TestGrantRetriesTheSaveAfterAFailure(t *testing.T) {
	m, _ := newTestManager(t)
	m.registry.AddOrUpdate("mixed", "", "", "", "1.0.0", PermissionSet{Commands: true, EditorRead: true})
	if err := m.registry.Save(); err != nil {
		t.Fatal(err)
	}
	path := m.registry.path
	makeRegistryUnwritable(t, path)
	entry := m.registry.Find("mixed")

	if _, err := m.grant(entry, PermissionSet{Commands: true}); err == nil {
		t.Fatal("grant reported success although the registry could not be saved")
	}

	// The registry can be written again; the retry must persist the reduction.
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	granted, err := m.grant(entry, PermissionSet{Commands: true})
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if granted.EditorRead {
		t.Errorf("retry granted the dropped permission: %+v", granted)
	}
	if !granted.Commands {
		t.Errorf("retry must still grant what the manifest declares: %+v", granted)
	}
	stored, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	storedEntry := stored.Find("mixed")
	if storedEntry == nil {
		t.Fatal("plugin missing from the saved registry")
	}
	if storedEntry.Permissions.EditorRead {
		t.Errorf("the dropped permission is still in the saved registry: %+v", storedEntry.Permissions)
	}
}

// A plugin whose reduced permissions could not be persisted must not be
// loaded: the registry on disk still grants what the manifest dropped.
func TestLoadAllSkipsAPluginWhosePermissionsCannotBePersisted(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	dir := writePluginDir(t, pluginsDir, "mixed")
	manifest := `{"name":"mixed","version":"1.0.0","entry":"main.ttt.lua","permissions":{"commands":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	m.registry.AddOrUpdate("mixed", "", "", "", "1.0.0", PermissionSet{Commands: true, EditorRead: true})
	if err := m.registry.Save(); err != nil {
		t.Fatal(err)
	}
	makeRegistryUnwritable(t, m.registryPath)

	m.LoadAll()

	if p := m.FindPlugin("mixed"); p != nil {
		t.Errorf("plugin loaded although its reduced permissions were not persisted: %+v", p.Granted)
	}
}

// Reloading a plugin must report a failed permission reduction instead of
// reporting success and running the plugin with the stale grant.
func TestReloadReportsAFailedPermissionSave(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	dir := writePluginDir(t, pluginsDir, "mixed")
	wide := `{"name":"mixed","version":"1.0.0","entry":"main.ttt.lua","permissions":{"commands":true,"editor.read":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(wide), 0644); err != nil {
		t.Fatal(err)
	}
	m.registry.AddOrUpdate("mixed", "", "", "", "1.0.0", PermissionSet{Commands: true, EditorRead: true})
	if err := m.registry.Save(); err != nil {
		t.Fatal(err)
	}
	m.LoadAll()
	if m.FindPlugin("mixed") == nil {
		t.Fatal("plugin not loaded before the reload")
	}

	// The manifest drops a permission, so the reload has to persist a
	// reduction that the registry file cannot accept.
	narrow := `{"name":"mixed","version":"1.0.0","entry":"main.ttt.lua","permissions":{"commands":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(narrow), 0644); err != nil {
		t.Fatal(err)
	}
	makeRegistryUnwritable(t, m.registryPath)

	if _, err := m.Reload("mixed"); err == nil {
		t.Fatal("Reload reported success although the registry could not be saved")
	}
}

// Applying an update must report a failed permission reduction too.
func TestApplyUpdateReportsAFailedPermissionSave(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	dir := writePluginDir(t, pluginsDir, "mixed")
	wide := `{"name":"mixed","version":"1.0.0","entry":"main.ttt.lua","permissions":{"commands":true,"editor.read":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(wide), 0644); err != nil {
		t.Fatal(err)
	}
	m.registry.AddOrUpdate("mixed", "", "", "", "1.0.0", PermissionSet{Commands: true, EditorRead: true})
	if err := m.registry.Save(); err != nil {
		t.Fatal(err)
	}
	m.LoadAll()
	if m.FindPlugin("mixed") == nil {
		t.Fatal("plugin not loaded before the update")
	}
	old := m.FindPlugin("mixed")

	// The new manifest drops a permission, so the update has to persist a
	// reduction that the registry file cannot accept.
	narrow := `{"name":"mixed","version":"2.0.0","entry":"main.ttt.lua","permissions":{"commands":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(narrow), 0644); err != nil {
		t.Fatal(err)
	}
	makeRegistryUnwritable(t, m.registryPath)

	u := m.PrepareUpdate("mixed")
	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	u.manifest = manifest

	if _, _, err := m.ApplyUpdate(u); err == nil {
		t.Fatal("ApplyUpdate reported success although the registry could not be saved")
	}
	if !old.Loaded() {
		t.Error("the running plugin was destroyed although the reduction was not persisted")
	}
}

// Enabling a plugin must report a failed permission reduction instead of
// reporting success and running the plugin with the stale grant.
func TestSetEnabledReportsAFailedPermissionSave(t *testing.T) {
	m, pluginsDir := newTestManager(t)
	dir := writePluginDir(t, pluginsDir, "mixed")
	manifest := `{"name":"mixed","version":"1.0.0","entry":"main.ttt.lua","permissions":{"commands":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	m.registry.AddOrUpdate("mixed", "", "", "", "1.0.0", PermissionSet{Commands: true, EditorRead: true})
	m.registry.SetEnabled("mixed", false)
	if err := m.registry.Save(); err != nil {
		t.Fatal(err)
	}

	// The manifest drops a permission. SetEnabled first saves the enabled
	// state, then persists the permission reduction during grant. Let the
	// initial enabled-state save succeed, but fail the reduction save.
	saves := 0
	m.registry.saveHook = func() error {
		saves++
		if saves == 2 {
			return errors.New("simulated save error")
		}
		return nil
	}

	p, err := m.SetEnabled("mixed", true)
	if err == nil {
		t.Fatal("SetEnabled reported success although the reduced permissions could not be saved")
	}
	if p != nil {
		t.Errorf("SetEnabled returned a plugin instance on failure: %+v", p)
	}
	if saves < 2 {
		t.Fatalf("expected at least 2 save attempts (enabled state, then permission reduction), got %d", saves)
	}
	if loaded := m.FindPlugin("mixed"); loaded != nil {
		t.Errorf("plugin loaded although its reduced permissions were not persisted: %+v", loaded.Granted)
	}
	if len(m.plugins) != 0 {
		t.Errorf("expected 0 loaded plugins, got %d", len(m.plugins))
	}

	// Retrying after restoring persistence must save the reduced
	// permissions and enable the plugin.
	m.registry.saveHook = nil

	p, err = m.SetEnabled("mixed", true)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if p == nil {
		t.Fatal("retry succeeded but returned nil plugin")
	}
	if !p.Loaded() {
		t.Error("plugin was not loaded after retry")
	}
	if p.Granted.EditorRead {
		t.Errorf("retry granted the dropped permission: %+v", p.Granted)
	}
	if !p.Granted.Commands {
		t.Errorf("retry must still grant what the manifest declares: %+v", p.Granted)
	}
	if m.FindPlugin("mixed") == nil {
		t.Fatal("plugin not found in manager after successful retry")
	}

	stored, err := LoadRegistry(m.registryPath)
	if err != nil {
		t.Fatal(err)
	}
	storedEntry := stored.Find("mixed")
	if storedEntry == nil {
		t.Fatal("plugin missing from the saved registry")
	}
	if !storedEntry.Enabled {
		t.Error("plugin is not marked enabled in the saved registry")
	}
	if storedEntry.Permissions.EditorRead {
		t.Errorf("the dropped permission is still in the saved registry: %+v", storedEntry.Permissions)
	}
	if !storedEntry.Permissions.Commands {
		t.Errorf("the kept permission is missing from the saved registry: %+v", storedEntry.Permissions)
	}
}

func TestFailedOnInstallUndoesAFreshInstall(t *testing.T) {
	pluginsDir := t.TempDir()
	dir := writePluginDir(t, pluginsDir, "broken")
	manifest := `{"name":"broken","version":"1.0.0","entry":"main.ttt.lua","permissions":{"storage":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	entry := `require("ttt").on_install(function()
		require("ttt.storage").set("half", "done")
		error("no write access")
	end)`
	if err := os.WriteFile(filepath.Join(dir, "main.ttt.lua"), []byte(entry), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(pluginsDir, filepath.Join(t.TempDir(), "registry.json"))
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}

	err := m.ApproveAndLoad(pending[0])
	if err == nil || !strings.Contains(err.Error(), "no write access") {
		t.Fatalf("err = %v, want the on_install error", err)
	}
	if m.FindPlugin("broken") != nil {
		t.Error("plugin loaded after its on_install failed")
	}
	if m.Registry().Find("broken") != nil {
		t.Error("registry still records the failed install")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("plugin directory left behind: %v", err)
	}
	if path, _ := storagePath(m.StorageDir(), "broken"); path != "" {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("storage written by the failed on_install left behind: %v", err)
		}
	}
}

func TestRevertApprovalReportsAFailedSave(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	pluginsDir := t.TempDir()
	dir := writePluginDir(t, pluginsDir, "p")
	regDir := t.TempDir()
	m := NewManager(pluginsDir, filepath.Join(regDir, "registry.json"))
	m.LoadAll()
	if err := os.Chmod(regDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(regDir, 0o700) })

	err := m.revertApproval(&Plugin{Name: "p", Dir: dir, Manifest: Manifest{Name: "p"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "undo approval") {
		t.Fatalf("err = %v, want the failed registry save", err)
	}
}

// A failed on_install on re-approval keeps the plugin's files and its previous
// grants, so the next startup asks again and reruns the hook.
func TestFailedOnInstallRestoresThePreviousApproval(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	dir := writePluginDir(t, pluginsDir, "p")
	manifest := `{"name":"p","version":"1.0.0","entry":"main.ttt.lua","permissions":{"commands":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ttt.lua"), []byte(`require("ttt").on_install(function() error("boom") end)`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, []byte(`[{"name":"p","version":"0.9.0","enabled":true,"permissions":{}}]`), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(pluginsDir, regPath)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}

	if err := m.ApproveAndLoad(pending[0]); err == nil {
		t.Fatal("ApproveAndLoad succeeded although on_install failed")
	}
	entry := m.Registry().Find("p")
	if entry == nil || entry.Version != "0.9.0" || entry.Permissions.Commands {
		t.Fatalf("registry entry = %+v, want the previous approval", entry)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("plugin directory removed: %v", err)
	}
	if again := NewManager(pluginsDir, regPath).LoadAll(); len(again) != 1 {
		t.Errorf("pending on next startup = %d, want 1", len(again))
	}
}
