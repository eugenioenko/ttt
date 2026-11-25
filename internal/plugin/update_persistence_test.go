package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestApplyUpdatePersistsStateBeforeUnloading(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, reduce := range []bool{false, true} {
			name := "disabled"
			if enabled {
				name = "enabled"
			}
			if reduce {
				name += "/reduced-permissions"
			}
			t.Run(name, func(t *testing.T) {
				m, pluginsDir := newTestManager(t)
				dir := writePluginDir(t, pluginsDir, "p")
				manifest := `{"name":"p","version":"1.0.0","entry":"main.ttt.lua","permissions":{"editor.read":true}}`
				if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0o644); err != nil {
					t.Fatal(err)
				}
				m.registry.AddOrUpdate("p", "", "", "", "1.0.0", PermissionSet{EditorRead: true})
				m.registry.SetEnabled("p", enabled)
				if err := m.registry.Save(); err != nil {
					t.Fatal(err)
				}
				m.LoadAll()
				t.Cleanup(m.Shutdown)
				old := m.FindPlugin("p")
				if enabled && (old == nil || !old.Loaded()) {
					t.Fatal("plugin not loaded")
				}
				before := *m.registry.Find("p")
				u := m.PrepareUpdate("p")
				var err error
				u.manifest, err = LoadManifest(dir)
				if err != nil {
					t.Fatal(err)
				}
				u.manifest.Version = "2.0.0"
				if reduce {
					u.manifest.Permissions = PermissionSet{}
				}

				saveErr := errors.New("registry unavailable")
				m.registry.saveHook = func() error { return saveErr }
				if _, _, err := m.ApplyUpdate(u); !errors.Is(err, saveErr) {
					t.Fatalf("update error = %v, want %v", err, saveErr)
				}
				if enabled && (m.FindPlugin("p") != old || !old.Loaded()) {
					t.Error("save failure replaced or unloaded the running plugin")
				}
				if got := *m.registry.Find("p"); !reflect.DeepEqual(got, before) {
					t.Errorf("save failure changed registry: %+v", got)
				}
				stored, err := LoadRegistry(m.registryPath)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(*stored.Find("p"), before) {
					t.Error("save failure changed persisted state")
				}

				saves := 0
				m.registry.saveHook = func() error { saves++; return nil }
				p, approval, err := m.ApplyUpdate(u)
				if err != nil || approval || p == nil {
					t.Fatalf("retry: plugin=%v approval=%v err=%v", p, approval, err)
				}
				if saves != 1 {
					t.Errorf("state saved %d times, want one version/permissions commit", saves)
				}
				if p.Loaded() != enabled {
					t.Errorf("loaded = %v, want %v", p.Loaded(), enabled)
				}
				if enabled && (p == old || old.Loaded()) {
					t.Error("successful retry did not replace the old plugin")
				}
				stored, err = LoadRegistry(m.registryPath)
				if err != nil {
					t.Fatal(err)
				}
				entry := stored.Find("p")
				if entry.Version != "2.0.0" || entry.Enabled != enabled || entry.Permissions.EditorRead == reduce {
					t.Errorf("unexpected persisted state: %+v", entry)
				}
			})
		}
	}
}
