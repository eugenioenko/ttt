package plugin

import (
	"fmt"
	"io/fs"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
)

func (m *Manager) pluginManifests(pluginDir string) iter.Seq2[string, Manifest] {
	return func(yield func(string, Manifest) bool) {
		entries, err := os.ReadDir(pluginDir)
		if err != nil {
			if pluginDir == m.systemDir {
				m.discoveryError(pluginDir, err)
			}
			return
		}
		for _, entry := range entries {
			dir := filepath.Join(pluginDir, entry.Name())
			if !entry.IsDir() {
				// entry.IsDir reports the link's own type, so a symlinked
				// plugin dir needs its target checked instead.
				if entry.Type()&fs.ModeSymlink == 0 || !isLinkedDir(dir, pluginDir) {
					continue
				}
			}
			manifest, err := LoadManifest(dir)
			if err != nil {
				slog.Warn("skip plugin", "dir", entry.Name(), "error", err)
				if pluginDir == m.systemDir {
					m.discoveryError(dir, err)
				}
				continue
			}
			if !yield(dir, manifest) {
				return
			}
		}
	}
}

func (m *Manager) findManifest(name string) (string, Manifest, error) {
	for _, root := range m.pluginDirs() {
		for dir, manifest := range m.pluginManifests(root) {
			if manifest.Name == name {
				return dir, manifest, nil
			}
		}
	}
	return "", Manifest{}, fmt.Errorf("plugin %q not found", name)
}
