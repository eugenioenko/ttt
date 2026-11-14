package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func systemPluginFixture(t *testing.T) (string, string) {
	t.Helper()
	system := t.TempDir()
	dir := writePluginDir(t, system, "shared")
	manifest := `{"name":"shared","version":"1.0.0","entry":"main.ttt.lua","permissions":{"editor.read":true}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	reg := &Registry{path: filepath.Join(system, "plugins.ttt.json")}
	reg.AddOrUpdate("shared", "Shared", "", "", "1.0.0", PermissionSet{EditorRead: true})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}
	return system, reg.path
}

func systemManager(t *testing.T, configDir, system string, extra ...string) *Manager {
	t.Helper()
	m := NewManager(filepath.Join(configDir, "plugins"), filepath.Join(configDir, "plugins.ttt.json"), extra...)
	m.SetSystemDir(system)
	t.Cleanup(m.Shutdown)
	return m
}

func TestSystemApprovalAndPerUserDisable(t *testing.T) {
	system, registryPath := systemPluginFixture(t)
	original, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	user := t.TempDir()
	m := systemManager(t, user, system)
	if pending := m.LoadAll(); len(pending) != 0 || m.FindPlugin("shared") == nil {
		t.Fatalf("admin-approved plugin not loaded: pending=%v", pending)
	}
	if !m.FindPlugin("shared").Granted.EditorRead {
		t.Fatal("admin permission not granted")
	}
	if m.registry.Find("shared").Permissions.EditorRead {
		t.Fatal("admin grant copied into user approvals")
	}
	if _, err := m.Reload("shared"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetEnabled("shared", false); err != nil {
		t.Fatal(err)
	}
	restarted := systemManager(t, user, system)
	if pending := restarted.LoadAll(); len(pending) != 0 || restarted.FindPlugin("shared") != nil {
		t.Fatal("disabled plugin loaded or prompted on restart")
	}
	other := systemManager(t, t.TempDir(), system)
	if pending := other.LoadAll(); len(pending) != 0 || other.FindPlugin("shared") == nil {
		t.Fatal("one user's disable affected another user")
	}
	if p, err := restarted.SetEnabled("shared", true); err != nil || p == nil || !p.Loaded() {
		t.Fatalf("re-enable: p=%v err=%v", p, err)
	}
	if err := restarted.RunUninstallHook("shared"); err == nil {
		t.Fatal("system uninstall hook allowed")
	}
	if err := restarted.Uninstall("shared"); err == nil {
		t.Fatal("system uninstall allowed")
	}
	if err := restarted.PrepareUpdate("shared").Fetch(); err == nil {
		t.Fatal("system update allowed")
	}
	after, err := os.ReadFile(registryPath)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatal("admin registry changed")
	}
}

func TestSystemApprovalDoesNotAuthorizeOverrides(t *testing.T) {
	for _, location := range []string{"user", "workspace"} {
		t.Run(location, func(t *testing.T) {
			system, _ := systemPluginFixture(t)
			user, workspace := t.TempDir(), t.TempDir()
			m := systemManager(t, user, system, workspace)
			m.LoadAll()
			root := workspace
			if location == "user" {
				root = m.pluginsDir
			}
			override := writePluginDir(t, root, "shared")
			restarted := systemManager(t, user, system, workspace)
			pending := restarted.LoadAll()
			if len(pending) != 1 || pending[0].Dir != override || restarted.FindPlugin("shared") != nil {
				t.Fatalf("override inherited admin approval: pending=%v", pending)
			}
			if err := restarted.ApproveAndLoad(pending[0]); err != nil {
				t.Fatal(err)
			}
			if restarted.registry.Find("shared").SystemApproval {
				t.Fatal("explicit approval still marked inherited")
			}
		})
	}
}

func TestSystemApprovalRevocationAndExpansion(t *testing.T) {
	for _, change := range []string{"removed", "disabled", "expanded", "corrupt"} {
		t.Run(change, func(t *testing.T) {
			system, registryPath := systemPluginFixture(t)
			user := t.TempDir()
			systemManager(t, user, system).LoadAll()
			switch change {
			case "removed":
				if err := os.Remove(registryPath); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				reg, err := LoadRegistry(registryPath)
				if err != nil {
					t.Fatal(err)
				}
				reg.SetEnabled("shared", false)
				if err := reg.Save(); err != nil {
					t.Fatal(err)
				}
			case "expanded":
				manifest := `{"name":"shared","version":"2.0.0","entry":"main.ttt.lua","permissions":{"editor.write":true}}`
				if err := os.WriteFile(filepath.Join(system, "shared", "plugin.ttt.json"), []byte(manifest), 0644); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(registryPath, []byte("invalid"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			restarted := systemManager(t, user, system)
			errors := 0
			restarted.OnDiscoveryError = func(string, error) { errors++ }
			if pending := restarted.LoadAll(); len(pending) != 1 || restarted.FindPlugin("shared") != nil {
				t.Fatalf("stale admin approval loaded plugin: pending=%v", pending)
			}
			if change == "corrupt" && errors != 1 {
				t.Fatalf("errors=%d", errors)
			}
			if change == "expanded" {
				reg, err := LoadRegistry(registryPath)
				if err != nil {
					t.Fatal(err)
				}
				reg.UpdatePermissions("shared", PermissionSet{EditorWrite: true})
				if err := reg.Save(); err != nil {
					t.Fatal(err)
				}
				updated := systemManager(t, user, system)
				if pending := updated.LoadAll(); len(pending) != 0 || updated.FindPlugin("shared") == nil || !updated.FindPlugin("shared").Granted.EditorWrite {
					t.Fatal("new admin approval did not cover expanded permissions")
				}
			}
		})
	}
}

func TestSystemDiscoveryErrors(t *testing.T) {
	for _, kind := range []string{"unset", "missing", "file", "relative", "manifest"} {
		t.Run(kind, func(t *testing.T) {
			system := ""
			switch kind {
			case "missing":
				system = filepath.Join(t.TempDir(), "missing")
			case "file":
				system = filepath.Join(t.TempDir(), "file")
				if err := os.WriteFile(system, nil, 0644); err != nil {
					t.Fatal(err)
				}
			case "relative":
				system = "relative"
			case "manifest":
				system = t.TempDir()
				if err := os.Mkdir(filepath.Join(system, "invalid"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			m := systemManager(t, t.TempDir(), system)
			errors := 0
			m.OnDiscoveryError = func(string, error) { errors++ }
			m.LoadAll()
			if (errors > 0) != (kind != "unset") {
				t.Fatalf("errors=%d", errors)
			}
		})
	}
}
