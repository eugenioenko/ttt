package e2e

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/gdamore/tcell/v3"
)

func TestPluginRegistryIsFetchedWhenThePluginsPanelFirstOpens(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	fetches := make(chan struct{}, 4)
	h.app.FetchRegistry = func() ([]plugin.RemoteRegistryEntry, error) {
		fetches <- struct{}{}
		return nil, nil
	}
	panel := app.NewPluginsPanel(h.app.PluginManager)
	h.app.Sidebar.AddPanel("plugins", "Plugins", panel.Adapter)
	h.app.WirePluginsPanel(panel)

	h.app.Sidebar.SetActivePanel("explorer")
	select {
	case <-fetches:
		t.Fatal("registry fetched before the plugins panel was opened")
	case <-time.After(50 * time.Millisecond):
	}

	h.app.Sidebar.SetActivePanel("plugins")
	select {
	case <-fetches:
	case <-time.After(5 * time.Second):
		t.Fatal("opening the plugins panel did not fetch the registry")
	}

	h.app.Sidebar.SetActivePanel("explorer")
	h.app.Sidebar.SetActivePanel("plugins")
	select {
	case <-fetches:
		t.Fatal("reopening the plugins panel fetched the registry again")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPluginRegistryFailureReplacesLoadingInBothLists(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	h.app.FetchRegistry = func() ([]plugin.RemoteRegistryEntry, error) {
		return nil, errors.New("offline")
	}
	panel := app.NewPluginsPanel(h.app.PluginManager)
	h.app.Sidebar.AddPanel("plugins", "Plugins", panel.Adapter)
	h.app.WirePluginsPanel(panel)
	h.app.ShowSidebar()

	h.app.Sidebar.SetActivePanel("plugins")
	h.redraw()
	if got := strings.Count(h.screenText(), i18n.Text("plugins.loadingPlugins")); got != 2 {
		t.Fatalf("loading messages while fetching = %d, want 2", got)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-h.screen.EventQ():
			if iev, ok := ev.(*tcell.EventInterrupt); ok {
				if result, ok := iev.Data().(*app.RemoteRegistryResult); ok {
					h.app.HandleRemoteRegistryResult(result)
					h.redraw()
					h.assertNotContains(i18n.Text("plugins.loadingPlugins"))
					if got := strings.Count(h.screenText(), i18n.Text("plugins.couldNotLoadRegistry")); got != 2 {
						t.Fatalf("error messages after a failed fetch = %d, want 2", got)
					}
					return
				}
			}
		case <-deadline:
			t.Fatal("timed out waiting for the registry result")
		}
	}
}
