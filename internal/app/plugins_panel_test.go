package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestFeaturedPluginDiscovery(t *testing.T) {
	dir := t.TempDir()
	m := plugin.NewManager(filepath.Join(dir, "plugins"), filepath.Join(dir, "registry.json"))
	m.LoadAll()
	m.Registry().AddOrUpdate("installed", "Installed", "", "", "1", plugin.PermissionSet{})
	pp := NewPluginsPanel(m)
	pp.SetAvailable([]plugin.RemoteRegistryEntry{
		{Name: "json", DisplayName: "JSON Viewer", Description: "Browse data", Tags: []string{"data"}},
		{Name: "pdf", DisplayName: "PDF Viewer", Description: "Read documents", Featured: true},
		{Name: "installed", Featured: true},
		{Name: "docker", DisplayName: "Docker Manager", Description: "Manage containers", Featured: true},
	})
	ids := func(tree *widgets.TreeWidget) []string {
		var result []string
		for _, node := range tree.Config.Items {
			result = append(result, node.ID)
		}
		return result
	}
	if got := ids(pp.FeaturedTree); !slices.Equal(got, []string{"available.pdf", "available.docker"}) {
		t.Fatalf("featured = %v", got)
	}
	if got := ids(pp.SearchTree); !slices.Equal(got, []string{"available.json", "available.pdf", "available.docker"}) {
		t.Fatalf("available plugins without a query = %v", got)
	}
	pp.SearchInput.Config.OnChange("data")
	var opened []string
	pp.OnOpenDetail = func(entry plugin.RemoteRegistryEntry) { opened = append(opened, entry.Name) }
	for _, tree := range []*widgets.TreeWidget{pp.FeaturedTree, pp.SearchTree} {
		tree.Config.OnCommand("activate", tree.Config.Items[0])
	}
	if !slices.Equal(opened, []string{"pdf", "json"}) {
		t.Fatalf("opened = %v", opened)
	}
	if !slices.Equal(ids(pp.FeaturedTree), []string{"available.pdf", "available.docker"}) || !slices.Equal(ids(pp.SearchTree), []string{"available.json"}) {
		t.Fatal("search changed the featured list or did not match registry metadata")
	}
	pp.SearchInput.Config.OnChange("containers")
	if !slices.Equal(ids(pp.FeaturedTree), []string{"available.pdf", "available.docker"}) || !slices.Equal(ids(pp.SearchTree), []string{"available.docker"}) {
		t.Fatal("featured plugins were not independently searchable")
	}
	pp.SearchInput.Config.OnChange("missing")
	if len(pp.FeaturedTree.Config.Items) != 2 || len(pp.SearchTree.Config.Items) != 0 {
		t.Fatal("empty search results changed the featured list")
	}
	pp.SearchInput.Config.OnChange("")
	if !slices.Equal(ids(pp.SearchTree), []string{"available.json", "available.pdf", "available.docker"}) {
		t.Fatal("clearing search did not restore all available plugins")
	}
}

func TestPluginPanelThreeSections(t *testing.T) {
	for _, height := range []int{20, 30} {
		m := plugin.NewManager(t.TempDir(), filepath.Join(t.TempDir(), "registry.json"))
		m.LoadAll()
		pp := NewPluginsPanel(m)
		pp.SetAvailable([]plugin.RemoteRegistryEntry{
			{Name: "other", DisplayName: "Other Plugin"},
			{Name: "one", DisplayName: "Featured One", Featured: true},
			{Name: "two", DisplayName: "Featured Two", Featured: true},
			{Name: "three", DisplayName: "Featured Three", Featured: true},
			{Name: "four", DisplayName: "Featured Four", Featured: true},
		})
		pp.SearchInput.Config.OnChange("other")
		grid := make([][]term.Cell, height)
		for y := range grid {
			grid[y] = make([]term.Cell, 40)
		}
		r := ui.Rect{X: 5, Y: 3, W: 40, H: height}
		pp.Adapter.SetRect(r)
		pp.Adapter.Render(ui.NewRenderSurface(grid, ui.Rect{W: 40, H: height}))
		var rows []string
		for _, row := range grid {
			var text strings.Builder
			for _, cell := range row {
				if cell.Ch == 0 {
					text.WriteRune(' ')
				} else {
					text.WriteRune(cell.Ch)
				}
			}
			rows = append(rows, strings.TrimSpace(text.String()))
		}
		last := slices.Index(rows, "Featured Four")
		featuredHeading := slices.Index(rows, "Featured")
		installedHeading := slices.IndexFunc(rows, func(row string) bool { return strings.HasPrefix(row, "Installed") })
		if !strings.Contains(rows[0], "Search plugins") || featuredHeading < 0 || last < 0 || installedHeading <= last {
			t.Fatalf("height %d: section headings are missing or unordered: %q", height, rows)
		}
		if result := slices.Index(rows, "Other Plugin"); result < 0 || result >= featuredHeading {
			t.Fatalf("height %d: search results are not in the top section: %q", height, rows)
		}
		for _, tree := range []*widgets.TreeWidget{pp.SearchTree, pp.FeaturedTree, pp.InstalledTree} {
			if tree.GetRect().H <= 0 {
				t.Fatalf("height %d: one of the three lists has no space", height)
			}
		}
		var opened string
		pp.OnOpenDetail = func(entry plugin.RemoteRegistryEntry) { opened = entry.Name }
		y := slices.Index(rows, "Featured One")
		pp.Adapter.HandleEvent(tcell.NewEventMouse(r.X+2, r.Y+y, tcell.Button1, tcell.ModNone))
		pp.Adapter.HandleEvent(tcell.NewEventMouse(r.X+2, r.Y+y, tcell.ButtonNone, tcell.ModNone))
		if opened != "one" {
			t.Fatalf("height %d: click opened %q", height, opened)
		}
	}
}
