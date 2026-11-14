package plugin

import (
	"errors"
	"log/slog"
	"path/filepath"
	"slices"

	"github.com/eugenioenko/ttt/internal/i18n"
)

func (m *Manager) SetSystemDir(dir string) {
	m.systemDir = dir
}

func (m *Manager) pluginDirs() []string {
	dirs := slices.Concat([]string{m.pluginsDir}, m.extraDirs)
	if m.systemDir != "" && filepath.IsAbs(m.systemDir) {
		dirs = append(dirs, filepath.Clean(m.systemDir))
	}
	return dirs
}

func (m *Manager) isSystemPlugin(dir string) bool {
	return m.systemDir != "" && filepath.Dir(dir) == filepath.Clean(m.systemDir)
}

func (m *Manager) discoveryError(path string, err error) {
	slog.Error("discover plugins", "path", path, "error", err)
	if m.OnDiscoveryError != nil {
		m.OnDiscoveryError(path, err)
	}
}

func (m *Manager) loadSystemRegistry() {
	m.systemRegistry = nil
	if m.systemDir == "" {
		return
	}
	if !filepath.IsAbs(m.systemDir) {
		m.discoveryError(m.systemDir, errors.New(i18n.Text("plugins.systemDirAbsolute")))
		return
	}
	m.systemDir = filepath.Clean(m.systemDir)
	path := filepath.Join(m.systemDir, "plugins.ttt.json")
	reg, err := LoadRegistry(path)
	if err != nil {
		m.discoveryError(path, err)
		return
	}
	m.systemRegistry = reg
}

func (m *Manager) approvalFor(dir string, manifest Manifest) (*RegistryEntry, error) {
	local := m.registry.Find(manifest.Name)
	var personal *RegistryEntry
	if local != nil && !local.SystemApproval {
		personal = local
		if DiffPermissions(local.Permissions, manifest.Permissions).IsEmpty() {
			return local, nil
		}
	}
	if !m.isSystemPlugin(dir) || m.systemRegistry == nil {
		return personal, nil
	}
	approved := m.systemRegistry.Find(manifest.Name)
	if approved == nil || !approved.Enabled || approved.SystemApproval || !DiffPermissions(approved.Permissions, manifest.Permissions).IsEmpty() {
		return personal, nil
	}
	if local == nil {
		// Persist only user state, never the inherited grant: copying permissions
		// would authorize a same-named user plugin or survive admin revocation.
		m.registry.AddOrUpdate(manifest.Name, manifest.DisplayName, "", "", manifest.Version, PermissionSet{})
		local = m.registry.Find(manifest.Name)
		local.SystemApproval = true
		if err := m.registry.Save(); err != nil {
			m.registry.Remove(manifest.Name)
			return nil, err
		}
	}
	entry := *approved
	entry.SystemApproval = true
	entry.ExecPaths = slices.Concat(approved.ExecPaths, local.ExecPaths)
	return &entry, nil
}

func (m *Manager) IsSystemPlugin(name string) bool {
	if p := m.FindPlugin(name); p != nil {
		return m.isSystemPlugin(p.Dir)
	}
	for _, root := range m.pluginDirs() {
		dir := filepath.Join(root, name)
		if _, err := LoadManifest(dir); err == nil {
			return m.isSystemPlugin(dir)
		}
	}
	return false
}

func systemManagedError() error {
	return errors.New(i18n.Text("plugins.systemManaged"))
}
