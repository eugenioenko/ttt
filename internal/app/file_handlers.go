package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) openWithFileHandler(path string, head []byte) ui.Widget {
	if a.PluginManager == nil {
		return nil
	}
	p, h := a.PluginManager.MatchFileHandler(path, head)
	if h == nil {
		return nil
	}
	panel, err := p.OpenFileHandler(h, path)
	if err != nil {
		a.StatusError(fmt.Sprintf("%s could not open %s: %v", h.Title, path, err))
		return nil
	}
	if a.fileHandlerTabs == nil {
		a.fileHandlerTabs = make(map[string]*plugin.Plugin)
	}
	a.fileHandlerTabs[path] = p
	return panel
}

func (a *App) fileHandlerTabOpened(path string) {
	if a.PluginManager != nil {
		a.PluginManager.DispatchEvent("file.open", path)
	}
}

func (a *App) closeFileHandlerTab(path string) {
	if _, ok := a.fileHandlerTabs[path]; !ok {
		return
	}
	delete(a.fileHandlerTabs, path)
	if a.PluginManager != nil {
		a.PluginManager.DispatchEvent("file.close", path)
	}
}

// Reload, update and disable destroy the plugin a handled tab renders through, so orphaned tabs are found here each frame rather than at every Destroy.
func (a *App) SyncFileHandlerTabs() {
	released := false
	for path, owner := range a.fileHandlerTabs {
		if owner.Loaded() && owner.Enabled {
			continue
		}
		a.EditorGroup.ShowBinaryCard(path)
		a.closeFileHandlerTab(path)
		released = true
	}
	if released {
		a.EditorGroup.RetryFileHandlers()
	}
}
