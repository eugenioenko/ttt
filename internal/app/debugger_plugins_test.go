package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
)

func TestDebuggerPluginPackages(t *testing.T) {
	packages := os.Getenv("TTT_TEST_DAP_PLUGINS")
	if packages == "" {
		t.Skip("set TTT_TEST_DAP_PLUGINS to a checkout containing the five debugger packages")
	}
	for name, adapter := range map[string]string{"dap-go": "go", "dap-js-debug": "node", "dap-debugpy": "debugpy", "dap-codelldb": "lldb", "dap-gdb": "gdb"} {
		for _, scenario := range []string{"unchanged", "preexisting", "edited-on-disk", "disabled"} {
			t.Run(name+"/"+scenario, func(t *testing.T) {
				dir := t.TempDir()
				previous := config.OverrideConfigDir
				config.OverrideConfigDir = dir
				t.Cleanup(func() { config.OverrideConfigDir = previous })
				a := buildTestApp(t, config.DefaultSettings())
				a.settingsFile = filepath.Join(dir, "settings.json")
				settings := NewPluginSettingsAPI(a)
				key := "debug.adapters." + adapter
				if scenario == "preexisting" {
					requireNoError(t, settings.Set(key, map[string]any{"command": []string{"custom"}}))
				}
				plugins := filepath.Join(dir, "plugins")
				requireNoError(t, os.CopyFS(filepath.Join(plugins, name), os.DirFS(filepath.Join(packages, name))))
				newManager := func() *plugin.Manager {
					manager := plugin.NewManager(plugins, filepath.Join(dir, "plugins.ttt.json"))
					manager.SetSettingsAPI(settings)
					return manager
				}
				manager := newManager()
				defer func() { manager.Shutdown() }()
				pending := manager.LoadAll()
				if len(pending) != 1 {
					t.Fatalf("found %d plugins requiring approval", len(pending))
				}
				requireNoError(t, manager.ApproveAndLoad(pending[0]))
				if _, ok := settings.Get(key); !ok {
					t.Fatalf("missing installed %s", key)
				}
				switch scenario {
				case "edited-on-disk":
					saved, err := config.ReadSettingsFile(a.settingsFile)
					requireNoError(t, err)
					entry := saved.Debug.Adapters[adapter]
					entry.Command = []string{"custom"}
					saved.Debug.Adapters[adapter] = entry
					requireNoError(t, config.SaveSettingsFile(a.settingsFile, saved))
				case "disabled":
					_, err := manager.SetEnabled(name, false)
					requireNoError(t, err)
					manager.Shutdown()
					manager = newManager()
					manager.LoadAll()
				}
				requireNoError(t, manager.RunUninstallHook(name))
				requireNoError(t, manager.Uninstall(name))
				saved, err := config.ReadSettingsFile(a.settingsFile)
				requireNoError(t, err)
				entry, ok := saved.Debug.Adapters[adapter]
				if scenario == "preexisting" || scenario == "edited-on-disk" {
					if !ok || len(entry.Command) != 1 || entry.Command[0] != "custom" {
						t.Fatalf("lost customized entry %s: %+v", key, entry)
					}
				} else if ok {
					t.Fatalf("left owned entry %s", key)
				}
				if _, ok := saved.Debug.Adapters["pwa-node"]; ok {
					t.Fatal("left untouched alias")
				}
			})
		}
	}
}
