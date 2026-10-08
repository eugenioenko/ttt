package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func newStoragePlugin(t *testing.T, dir, name string, perms PermissionSet) *Plugin {
	t.Helper()
	p := &Plugin{Name: name, Granted: perms, StorageDir: dir}
	p.State = NewSandbox()
	setupStorageModule(p.State, p)
	t.Cleanup(func() { p.State.Close() })
	return p
}

func TestStorageRoundTripsAcrossPluginInstances(t *testing.T) {
	dir := t.TempDir()
	p := newStoragePlugin(t, dir, "marks", PermissionSet{Storage: true})
	err := p.State.DoString(`
		local storage = require("ttt.storage")
		storage.set("files", { ["/a.go"] = { { line = 12, icon = "*" } } })
		storage.set("count", 3)
		storage.set("gone", true)
		storage.remove("gone")
	`)
	if err != nil {
		t.Fatalf("DoString: %v", err)
	}

	q := newStoragePlugin(t, dir, "marks", PermissionSet{Storage: true})
	err = q.State.DoString(`
		local storage = require("ttt.storage")
		local files = storage.get("files")
		line = files["/a.go"][1].line
		icon = files["/a.go"][1].icon
		count = storage.get("count")
		gone = storage.get("gone")
		keys = table.concat(storage.keys(), ",")
	`)
	if err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if got := q.State.GetGlobal("line").String(); got != "12" {
		t.Errorf("line = %q, want 12", got)
	}
	if got := q.State.GetGlobal("icon").String(); got != "*" {
		t.Errorf("icon = %q, want *", got)
	}
	if got := q.State.GetGlobal("count").String(); got != "3" {
		t.Errorf("count = %q, want 3", got)
	}
	if got := q.State.GetGlobal("gone").String(); got != "nil" {
		t.Errorf("removed key = %q, want nil", got)
	}
	if got := q.State.GetGlobal("keys").String(); got != "count,files" {
		t.Errorf("keys = %q, want count,files", got)
	}
}

func TestStorageIsScopedPerPlugin(t *testing.T) {
	dir := t.TempDir()
	a := newStoragePlugin(t, dir, "alpha", PermissionSet{Storage: true})
	if err := a.State.DoString(`require("ttt.storage").set("k", "secret")`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	b := newStoragePlugin(t, dir, "beta", PermissionSet{Storage: true})
	if err := b.State.DoString(`v = require("ttt.storage").get("k")`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if got := b.State.GetGlobal("v").String(); got != "nil" {
		t.Errorf("beta read alpha's value: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "alpha.json")); err != nil {
		t.Errorf("alpha.json missing: %v", err)
	}
}

func TestStorageRequiresPermission(t *testing.T) {
	p := newStoragePlugin(t, t.TempDir(), "marks", PermissionSet{})
	if err := p.State.DoString(`has_set = require("ttt.storage").set ~= nil`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if p.State.GetGlobal("has_set").String() != "false" {
		t.Error("storage.set exposed without the storage permission")
	}
}

func TestStorageRejectsWritesOverLimit(t *testing.T) {
	dir := t.TempDir()
	p := newStoragePlugin(t, dir, "marks", PermissionSet{Storage: true})
	if err := p.State.DoString(`require("ttt.storage").set("small", "ok")`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	p.State.SetGlobal("big", lua.LString(strings.Repeat("x", maxStorageBytes)))
	err := p.State.DoString(`require("ttt.storage").set("big", big)`)
	if err == nil || !strings.Contains(err.Error(), "limit exceeded") {
		t.Fatalf("oversized write err = %v, want limit error", err)
	}
	if err := p.State.DoString(`small = require("ttt.storage").get("small"); big_after = require("ttt.storage").get("big")`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if p.State.GetGlobal("small").String() != "ok" || p.State.GetGlobal("big_after").String() != "nil" {
		t.Error("failed write changed stored data")
	}
}

func TestStorageRemovingLastKeyDeletesFile(t *testing.T) {
	dir := t.TempDir()
	p := newStoragePlugin(t, dir, "marks", PermissionSet{Storage: true})
	if err := p.State.DoString(`local s = require("ttt.storage"); s.set("k", 1); s.remove("k")`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "marks.json")); !os.IsNotExist(err) {
		t.Errorf("storage file still exists: %v", err)
	}
}

func TestStoragePathRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../x", "a/b"} {
		if _, err := storagePath("/tmp/store", name); err == nil {
			t.Errorf("storagePath accepted %q", name)
		}
	}
}

func TestUninstallDeletesPluginStorage(t *testing.T) {
	root := t.TempDir()
	m := NewManager(filepath.Join(root, "plugins"), filepath.Join(root, "plugins.ttt.json"))
	m.registry = &Registry{path: filepath.Join(root, "plugins.ttt.json")}
	path := filepath.Join(m.StorageDir(), "marks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"k":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall("marks"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("storage file survived uninstall: %v", err)
	}
}

func TestStorageSetRejectsUnsupportedValues(t *testing.T) {
	p := newStoragePlugin(t, t.TempDir(), "marks", PermissionSet{Storage: true})
	if err := p.State.DoString(`require("ttt.storage").set("k", "keep")`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if err := p.State.DoString(`require("ttt.storage").set("k", function() end)`); err == nil {
		t.Fatal("set accepted a function")
	}
	if err := p.State.DoString(`v = require("ttt.storage").get("k")`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if got := p.State.GetGlobal("v").String(); got != "keep" {
		t.Errorf("rejected set changed the key: %q", got)
	}
}

func TestStorageDoesNotOverwriteOtherInstanceKeys(t *testing.T) {
	dir := t.TempDir()
	a := newStoragePlugin(t, dir, "marks", PermissionSet{Storage: true})
	b := newStoragePlugin(t, dir, "marks", PermissionSet{Storage: true})
	if err := a.State.DoString(`require("ttt.storage").get("x")`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if err := b.State.DoString(`require("ttt.storage").set("from_b", 1)`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if err := a.State.DoString(`require("ttt.storage").set("from_a", 1)`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if err := b.State.DoString(`keys = table.concat(require("ttt.storage").keys(), ",")`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if got := b.State.GetGlobal("keys").String(); got != "from_a,from_b" {
		t.Errorf("keys = %q, want from_a,from_b", got)
	}
}
