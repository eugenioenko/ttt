package plugin

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCheckExecPathRequest(t *testing.T) {
	ps := PermissionSet{SystemExec: []string{"psql", "/usr/bin/sqlite3"}}
	tests := []struct {
		path string
		ok   bool
	}{
		{"/opt/pg16/bin/psql", true},
		{"/home/me/.local/bin/sqlite3", true},
		{"psql", false},
		{"bin/psql", false},
		{"/opt/pg16/../evil/psql", false},
		{"/opt/pg16//bin/psql", false},
		{"/opt/pg16/bin/psql/", false},
		{"/bin/sh", false},
		{"/opt/pg16/bin/psql-evil", false},
	}
	for _, tt := range tests {
		if err := ps.CheckExecPathRequest(tt.path); (err == nil) != tt.ok {
			t.Errorf("CheckExecPathRequest(%q) = %v, want ok %v", tt.path, err, tt.ok)
		}
	}
}

func TestExecPathGrantDoesNotMatchByName(t *testing.T) {
	ps := PermissionSet{SystemExec: []string{"psql", "/opt/pg16/bin/psql"}}
	if err := ps.CheckExec("/tmp/psql"); err == nil {
		t.Error("a path grant covered another file with the same name")
	}
}

func writeExecPlugin(t *testing.T, pluginsDir, regPath string, exec string, execPaths string) {
	t.Helper()
	dir := writePluginDir(t, pluginsDir, "db")
	manifest := `{"name":"db","version":"1.0.0","entry":"main.ttt.lua","permissions":{"system.exec":` + exec + `}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if regPath == "" {
		return
	}
	registry := `[{"name":"db","version":"1.0.0","enabled":true,"grantsVersion":2,"permissions":{"system.exec":["psql","sqlite3"]},"execPaths":` + execPaths + `}]`
	if err := os.WriteFile(regPath, []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGrantExecPathPersistsAcrossLoads(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	writeExecPlugin(t, pluginsDir, regPath, `["psql","sqlite3"]`, `[]`)
	m := NewManager(pluginsDir, regPath)
	t.Cleanup(m.Shutdown)
	if pending := m.LoadAll(); len(pending) != 0 {
		t.Fatalf("pending = %d, want 0", len(pending))
	}
	p := m.FindPlugin("db")
	if p == nil {
		t.Fatal("plugin not loaded")
	}

	if err := m.GrantExecPath(p, "/bin/sh"); err == nil {
		t.Fatal("granted a path whose name the manifest does not allow")
	}
	if err := m.GrantExecPath(p, "/opt/pg16/bin/psql"); err != nil {
		t.Fatal(err)
	}
	if err := m.GrantExecPath(p, "/opt/pg16/bin/psql"); err != nil {
		t.Fatal(err)
	}
	if err := p.Granted.CheckExec("/opt/pg16/bin/psql"); err != nil {
		t.Errorf("running plugin not granted the path: %v", err)
	}

	m2 := NewManager(pluginsDir, regPath)
	t.Cleanup(m2.Shutdown)
	m2.LoadAll()
	entry := m2.registry.Find("db")
	if !slices.Equal(entry.ExecPaths, []string{"/opt/pg16/bin/psql"}) {
		t.Errorf("persisted ExecPaths = %v", entry.ExecPaths)
	}
	if !slices.Equal(entry.Permissions.SystemExec, []string{"psql", "sqlite3"}) {
		t.Errorf("path grant leaked into the manifest permissions: %v", entry.Permissions.SystemExec)
	}
	if err := m2.FindPlugin("db").Granted.CheckExec("/opt/pg16/bin/psql"); err != nil {
		t.Errorf("grant not restored on load: %v", err)
	}
}

func TestGrantExecPathRequiresARegistryEntry(t *testing.T) {
	m, _ := newTestManager(t)
	p := &Plugin{Name: "debug", Granted: PermissionSet{SystemExec: []string{"psql"}}}
	if err := m.GrantExecPath(p, "/opt/pg16/bin/psql"); err == nil {
		t.Fatal("granted a path to a plugin with no registry entry")
	}
}

func TestExecPathDroppedWithItsManifestName(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	writeExecPlugin(t, pluginsDir, regPath, `["sqlite3"]`, `["/opt/pg16/bin/psql","/opt/bin/sqlite3"]`)
	m := NewManager(pluginsDir, regPath)
	t.Cleanup(m.Shutdown)
	m.LoadAll()
	p := m.FindPlugin("db")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	if err := p.Granted.CheckExec("/opt/pg16/bin/psql"); err == nil {
		t.Error("path grant applied after the manifest dropped psql")
	}
	if err := p.Granted.CheckExec("/opt/bin/sqlite3"); err != nil {
		t.Errorf("unrelated path grant lost: %v", err)
	}
	if got := m.registry.Find("db").ExecPaths; !slices.Equal(got, []string{"/opt/bin/sqlite3"}) {
		t.Errorf("registry ExecPaths = %v, want the stale grant removed", got)
	}
}

func TestApproveAndLoadKeepsExecPaths(t *testing.T) {
	pluginsDir := t.TempDir()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	writeExecPlugin(t, pluginsDir, regPath, `["psql","sqlite3","pg_dump"]`, `["/opt/pg16/bin/psql"]`)
	m := NewManager(pluginsDir, regPath)
	t.Cleanup(m.Shutdown)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want the new pg_dump to need approval", len(pending))
	}
	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatal(err)
	}
	if err := m.FindPlugin("db").Granted.CheckExec("/opt/pg16/bin/psql"); err != nil {
		t.Errorf("re-approval lost the path grant: %v", err)
	}
}

func TestSystemRequestExec(t *testing.T) {
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, &mockSystemAPI{})
	defer cleanup()
	var asked []string
	var answer func(bool)
	p.RequestExecPath = func(path string, done func(bool)) error {
		asked = append(asked, path)
		answer = done
		return nil
	}

	err := p.State.DoString(`
		local sys = require("ttt.system")
		local ok, err = sys.request_exec("/bin/sh", function() _G.bad = true end)
		assert(ok == nil and err:find("system.exec"), tostring(err))
		ok, err = sys.request_exec("psql", function() end)
		assert(ok == nil and err:find("absolute"), tostring(err))
		assert(sys.request_exec("/opt/pg16/bin/psql", function(granted) _G.granted = granted end) == true)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(asked, []string{"/opt/pg16/bin/psql"}) {
		t.Fatalf("asked = %v", asked)
	}
	p.Granted.SystemExec = append(p.Granted.SystemExec, "/opt/pg16/bin/psql")
	answer(true)
	if p.State.GetGlobal("granted").String() != "true" || p.State.GetGlobal("bad").String() != "nil" {
		t.Fatalf("granted = %s", p.State.GetGlobal("granted"))
	}
	if err := p.State.DoString(`assert(require("ttt.system").exec("/opt/pg16/bin/psql", {}).exit_code == 0)`); err != nil {
		t.Fatal(err)
	}
}

func TestSystemRequestExecDenied(t *testing.T) {
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, &mockSystemAPI{})
	defer cleanup()
	p.RequestExecPath = func(_ string, done func(bool)) error {
		done(false)
		return nil
	}
	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.request_exec("/opt/pg16/bin/psql", function(granted) _G.granted = granted end)
		assert(_G.granted == false)
		local r, err = sys.exec("/opt/pg16/bin/psql", {})
		assert(r == nil and err ~= nil)
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSystemRequestExecAlreadyGranted(t *testing.T) {
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql", "/opt/pg16/bin/psql"}}, &mockSystemAPI{})
	defer cleanup()
	ch := make(chan func(), 1)
	p.PostAsync = func(r *PluginAsyncResult) { ch <- r.Callback }
	p.RequestExecPath = func(string, func(bool)) error {
		t.Error("asked again for a granted path")
		return nil
	}
	if err := p.State.DoString(`require("ttt.system").request_exec("/opt/pg16/bin/psql", function(g) _G.granted = g end)`); err != nil {
		t.Fatal(err)
	}
	if p.State.GetGlobal("granted").String() != "nil" {
		t.Fatal("callback ran inside request_exec")
	}
	(<-ch)()
	if p.State.GetGlobal("granted").String() != "true" {
		t.Errorf("granted = %s", p.State.GetGlobal("granted"))
	}
}

func TestSystemRequestExecDialogUnavailable(t *testing.T) {
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, &mockSystemAPI{})
	defer cleanup()
	err := p.State.DoString(`
		local ok, err = require("ttt.system").request_exec("/opt/pg16/bin/psql", function() end)
		assert(ok == nil and err ~= nil)
	`)
	if err != nil {
		t.Fatal(err)
	}
}
