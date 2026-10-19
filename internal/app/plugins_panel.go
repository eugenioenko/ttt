package app

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type PluginsPanel struct {
	SearchInput   *widgets.InputWidget
	SearchTree    *widgets.TreeWidget
	FeaturedTree  *widgets.TreeWidget
	InstalledTree *widgets.TreeWidget
	Dropdown      *widgets.DropdownWidget
	Adapter       *ui.WidgetAdapter
	manager       *plugin.Manager

	OnInstall      func(repoURL, repoPath, name string)
	OnUninstall    func(name string)
	OnToggle       func(name string, enabled bool)
	OnUpdate       func(name string)
	OnOpenDetail   func(entry plugin.RemoteRegistryEntry)
	OnRowMenu      func(name string, enabled bool, screenX, screenY int)
	OnDropdownMenu func(entries []widgets.MenuEntry, screenX, screenY int)

	available   []plugin.RemoteRegistryEntry
	searchQuery string
}

func NewPluginsPanel(mgr *plugin.Manager) *PluginsPanel {
	pp := &PluginsPanel{
		manager: mgr,
	}

	pp.SearchInput = widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: "Search plugins",
		OnChange: func(text string) {
			pp.searchQuery = strings.TrimSpace(text)
			pp.refreshAvailable()
		},
	})

	pp.SearchTree = widgets.NewTreeWidget(widgets.TreeConfig{
		EmptyText: "Loading plugins...",
		Indent:    1,
		OnCommand: func(cmd string, node *widgets.TreeNode) {
			if cmd == "activate" {
				for _, entry := range pp.available {
					if node.ID == "available."+entry.Name {
						if pp.OnOpenDetail != nil {
							pp.OnOpenDetail(entry)
						}
						return
					}
				}
			}
		},
	})
	pp.FeaturedTree = widgets.NewTreeWidget(pp.SearchTree.Config)

	pp.InstalledTree = widgets.NewTreeWidget(widgets.TreeConfig{
		EmptyText: "No plugins installed",
		Indent:    1,
		NodeMenu:  []widgets.MenuEntry{{Label: "Actions"}},
		MenuIcon:  string(icons.Rune(icons.More)),
		OnCommand: func(cmd string, node *widgets.TreeNode) {
			pp.handleCommand(cmd, node)
		},
		OnMenu: func(_ []widgets.MenuEntry, node *widgets.TreeNode, screenX, screenY int) {
			pp.showRowMenu(node, screenX, screenY)
		},
	})

	pp.Dropdown = widgets.NewDropdownWidget(widgets.DropdownConfig{
		Entries: []widgets.MenuEntry{
			{Label: "Update All", Command: "updateAll"},
			{Label: "Refresh", Command: "refresh"},
			{Separator: true},
			{Label: "Help", Command: "help"},
		},
		OnMenu: func(entries []widgets.MenuEntry, screenX, screenY int) {
			if pp.OnDropdownMenu != nil {
				pp.OnDropdownMenu(entries, screenX, screenY)
			}
		},
	})

	titleLabel := widgets.NewLabelWidget(widgets.LabelConfig{
		Text:  "Installed",
		Style: term.StyleHoverBold,
	})

	titleLabel.Box.PaddingLeft = 1

	divider := widgets.NewDividerWidget(widgets.DividerConfig{})

	titleRow := widgets.NewHStackWidget(titleLabel, pp.Dropdown)
	titleRow.FixedHeight = 1

	divSearch := widgets.NewDividerWidget(widgets.DividerConfig{})
	divInstalled := widgets.NewDividerWidget(widgets.DividerConfig{})
	featuredHeading := widgets.NewLabelWidget(widgets.LabelConfig{Text: "Featured", Style: term.StyleHoverBold})
	featuredHeading.Box.PaddingLeft = 1

	vstack := widgets.NewVStackWidget(
		pp.SearchInput,
		divSearch,
		pp.SearchTree,
		widgets.NewDividerWidget(widgets.DividerConfig{}),
		featuredHeading,
		widgets.NewDividerWidget(widgets.DividerConfig{}),
		pp.FeaturedTree,
		divInstalled,
		titleRow,
		divider,
		pp.InstalledTree,
	)

	pp.Adapter = ui.NewWidgetAdapter(vstack)
	pp.Refresh()
	return pp
}

// handleCommand opens the plugin's README on activate (click/Enter/Space), like
// the search list. Per-plugin actions live on the ⋮ button.
func (pp *PluginsPanel) handleCommand(cmd string, node *widgets.TreeNode) {
	if node == nil || cmd != "activate" {
		return
	}
	pp.openInstalledDetail(node.ID)
}

// openInstalledDetail reuses the search detail flow, rebuilding a registry entry
// from the local registry (repo/path) and loaded manifest (description).
func (pp *PluginsPanel) openInstalledDetail(name string) {
	if pp.OnOpenDetail == nil {
		return
	}
	entry := plugin.RemoteRegistryEntry{Name: name}
	for _, remote := range pp.available {
		if remote.Name == name {
			entry.Featured = remote.Featured
			break
		}
	}
	if reg := pp.manager.Registry(); reg != nil {
		if e := reg.Find(name); e != nil {
			entry.DisplayName = e.DisplayName
			entry.Repo = e.Repo
			entry.Path = e.Path
			entry.Version = e.Version
		}
	}
	if p := pp.manager.FindPlugin(name); p != nil {
		entry.Description = p.Manifest.Description
		entry.Author = p.Manifest.Author
		if entry.DisplayName == "" {
			entry.DisplayName = p.Manifest.DisplayName
		}
		if entry.Version == "" {
			entry.Version = p.Manifest.Version
		}
	}
	pp.OnOpenDetail(entry)
}

// showRowMenu opens the per-plugin actions menu for the given row.
func (pp *PluginsPanel) showRowMenu(node *widgets.TreeNode, x, y int) {
	if node == nil || pp.OnRowMenu == nil {
		return
	}
	enabled := false
	if p := pp.manager.FindPlugin(node.ID); p != nil {
		enabled = p.Enabled
	}
	pp.OnRowMenu(node.ID, enabled, x, y)
}

func (pp *PluginsPanel) Refresh() {
	var installed []*widgets.TreeNode

	reg := pp.manager.Registry()
	names := pp.manager.InstalledPluginNames()

	for _, name := range names {
		var badge string
		var icon string
		var iconStyle term.Style

		label := name
		p := pp.manager.FindPlugin(name)
		if p != nil && p.Enabled {
			badge = "v" + p.Manifest.Version
			icon = "●"
			iconStyle = term.StyleSuccess
			label = p.Manifest.Title()
		} else {
			if reg != nil {
				if entry := reg.Find(name); entry != nil {
					badge = "v" + entry.Version
					label = entry.Title()
				}
			}
			icon = "○"
			iconStyle = term.StyleMuted
		}

		node := &widgets.TreeNode{
			ID:        name,
			Label:     label,
			Badge:     badge,
			Icon:      icon,
			IconStyle: iconStyle,
		}
		installed = append(installed, node)
	}

	pp.InstalledTree.SetItems(installed)
	pp.refreshAvailable()
	pp.refreshFeatured()
}

func (pp *PluginsPanel) refreshAvailable() {
	if len(pp.available) == 0 {
		pp.SearchTree.SetItems(nil)
		return
	}

	installedSet := make(map[string]bool)
	for _, name := range pp.manager.InstalledPluginNames() {
		installedSet[name] = true
	}

	var items []*widgets.TreeNode
	for _, entry := range pp.available {
		if installedSet[entry.Name] {
			continue
		}
		if pp.searchQuery != "" && !matchesSearch(entry, pp.searchQuery) {
			continue
		}
		items = append(items, &widgets.TreeNode{
			ID:    "available." + entry.Name,
			Label: entry.Title(),
			Badge: entry.Description,
		})
	}

	pp.SearchTree.SetItems(items)
}

func (pp *PluginsPanel) refreshFeatured() {
	installedSet := make(map[string]bool)
	for _, name := range pp.manager.InstalledPluginNames() {
		installedSet[name] = true
	}
	var items []*widgets.TreeNode
	for _, entry := range pp.available {
		if !entry.Featured || installedSet[entry.Name] {
			continue
		}
		items = append(items, &widgets.TreeNode{
			ID:    "available." + entry.Name,
			Label: entry.Title(),
			Badge: entry.Description,
		})
	}
	pp.FeaturedTree.SetItems(items)
}

func matchesSearch(entry plugin.RemoteRegistryEntry, query string) bool {
	query = strings.ToLower(query)
	terms := strings.Fields(query)
	for _, t := range terms {
		if !termMatches(entry, t) {
			return false
		}
	}
	return true
}

func termMatches(entry plugin.RemoteRegistryEntry, t string) bool {
	if strings.Contains(strings.ToLower(entry.Name), t) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Description), t) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Author), t) {
		return true
	}
	for _, tag := range entry.Tags {
		if strings.Contains(strings.ToLower(tag), t) {
			return true
		}
	}
	return false
}

func (pp *PluginsPanel) SetAvailable(entries []plugin.RemoteRegistryEntry) {
	pp.available = entries
	pp.SearchTree.Config.EmptyText = "Type to search plugins"
	pp.FeaturedTree.Config.EmptyText = "No featured plugins"
	pp.Refresh()
}

func (a *App) WirePluginsPanel(pp *PluginsPanel) {
	a.PluginsPanel = pp
	pp.OnInstall = a.PluginInstallFromURL
	pp.OnUninstall = a.PluginUninstallByName
	pp.OnToggle = a.PluginToggleByName
	pp.OnUpdate = a.PluginUpdateByName
	pp.OnOpenDetail = a.OpenPluginDetail
	pp.OnRowMenu = a.ShowPluginRowMenu
	pp.OnDropdownMenu = a.ShowPluginDropdownMenu
}
