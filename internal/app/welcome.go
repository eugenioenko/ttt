package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
)

const (
	welcomeTabID         = "welcome"
	welcomeFavoritesHint = "Right-click a folder in the Explorer to add it to favorites"
)

var welcomeCommands = []struct{ label, commandID string }{
	{"Open Folder…", "workspace.openFolder"},
	{"New File", "file.new"},
	{"Open Workspace…", "workspace.open"},
	{"Settings", "settings.openUI"},
	{"Command Palette", "command.palette"},
}

// welcomeItems lists the start actions with their shortcuts, then the
// favorite folders that exist, each with its path as written in settings.
func (a *App) welcomeItems() []welcomeItem {
	var items []welcomeItem
	for _, c := range welcomeCommands {
		shortcut := ""
		if cmd, ok := a.Reg.Get(c.commandID); ok {
			shortcut = cmd.Shortcut
		}
		items = append(items, welcomeItem{label: c.label, detail: shortcut, run: func() { a.Reg.Execute(c.commandID) }})
	}
	for _, fav := range a.Settings.Welcome.Favorites {
		abs, err := filepath.Abs(workspace.ExpandPath(fav))
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			continue
		}
		item := welcomeItem{
			label:  filepath.Base(abs),
			detail: fav,
			tight:  true,
			run:    func() { a.openFolderPath(abs) },
			copy:   func() { a.FileOpCopyAbsolutePath(abs) },
		}
		if len(items) == len(welcomeCommands) {
			item.section = "Favorites"
			item.tight = false
		}
		items = append(items, item)
	}
	return items
}

func (a *App) ShowWelcome() {
	if a.EditorGroup.SwitchToTabByPath(welcomeTabID) {
		a.FocusEditor()
		return
	}
	view := &welcomeView{}
	a.welcomeView = view
	a.refreshWelcome()
	adapter := ui.NewWidgetAdapter(view)
	onlyBlankUntitled := a.editorIsBlank()
	a.EditorGroup.OpenPluginTab(welcomeTabID, "Welcome", adapter)
	if onlyBlankUntitled {
		a.EditorGroup.CloseOtherTabs()
	}
	a.FocusEditor()
	adapter.SetFocused(true)
}

// refreshWelcome rebuilds the list, so favorites changed while the page is
// open show up on it.
func (a *App) refreshWelcome() {
	v := a.welcomeView
	if v == nil {
		return
	}
	v.items = a.welcomeItems()
	v.note = ""
	if len(v.items) == len(welcomeCommands) {
		v.note = welcomeFavoritesHint
	}
	v.selected = min(v.selected, len(v.items)-1)
}

// editorIsBlank reports whether the only tab is an untouched untitled buffer.
func (a *App) editorIsBlank() bool {
	return a.EditorGroup.TabCount() == 1 && a.EditorGroup.IsActiveVirtual() && isBlank(a.EditorGroup.ActiveBuffer())
}

func isBlank(b *buffer.Buffer) bool {
	return b != nil && !b.Dirty && len(b.Lines) <= 1 && (len(b.Lines) == 0 || b.Lines[0] == "")
}

// ShowEmptyState makes the welcome page the editor's empty state: it stays
// while nothing is open, and the sidebar has nothing to add to it.
func (a *App) ShowEmptyState() {
	a.ShowWelcome()
	a.EditorGroup.EmptyStateID = welcomeTabID
	a.welcomeIsEmptyState = true
	a.welcomeWhenEmpty = true
	a.HideSidebar()
}

// SyncEmptyState steps the welcome page aside once something else opens.
func (a *App) SyncEmptyState() {
	if a.welcomeIsEmptyState && a.EditorGroup.TabCount() > 1 {
		a.closeWelcome()
	}
}

func (a *App) closeWelcome() {
	a.welcomeIsEmptyState = false
	a.EditorGroup.EmptyStateID = ""
	a.EditorGroup.ClosePluginTab(welcomeTabID)
}

// favoriteIndex finds abs in welcome.favorites, comparing expanded paths.
func (a *App) favoriteIndex(abs string) int {
	for i, fav := range a.Settings.Welcome.Favorites {
		if p, err := filepath.Abs(workspace.ExpandPath(fav)); err == nil && p == abs {
			return i
		}
	}
	return -1
}

// Favorites under $HOME are stored as ~/…, so a settings file shared across
// machines keeps working.
func tildePath(abs string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return abs
	}
	if rel, err := filepath.Rel(home, abs); err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "~/" + filepath.ToSlash(rel)
	}
	return abs
}

// saveFavorites writes only the favorites onto the settings on disk: saving
// a.Settings whole would also write this window's layout over whatever another
// ttt window saved.
func (a *App) saveFavorites(favs []string, msg string) {
	a.Settings.Welcome.Favorites = favs
	a.refreshWelcome()
	disk := config.LoadSettings()
	disk.Welcome.Favorites = favs
	if err := config.SaveSettings(disk); err != nil {
		a.StatusError("Failed to save favorites: " + err.Error())
		return
	}
	a.StatusNotify(msg)
}

func (a *App) addFavorite(abs string) {
	if a.favoriteIndex(abs) >= 0 {
		a.StatusNotify(filepath.Base(abs) + " is already a favorite")
		return
	}
	a.saveFavorites(append(a.Settings.Welcome.Favorites, tildePath(abs)), "Added "+filepath.Base(abs)+" to favorites")
}

func (a *App) removeFavorite(i int) {
	favs := a.Settings.Welcome.Favorites
	name := filepath.Base(favs[i])
	a.saveFavorites(append(favs[:i:i], favs[i+1:]...), "Removed "+name+" from favorites")
}

// AddFavorite adds the right-clicked Explorer root, or a workspace folder.
func (a *App) AddFavorite() {
	if node := a.ExplorerContextNode; node != nil {
		a.ExplorerContextNode = nil
		a.addFavorite(node.ID)
		return
	}
	var items []widgets.SelectItem
	for _, p := range a.Workspace.Paths() {
		if a.favoriteIndex(p) < 0 {
			items = append(items, widgets.SelectItem{ID: p, Label: filepath.Base(p)})
		}
	}
	switch len(items) {
	case 0:
		a.StatusWarn("No open folder to add to favorites")
	case 1:
		a.addFavorite(items[0].ID)
	default:
		a.ShowSelectDialog("Add to Favorites", items, a.addFavorite, nil)
	}
}

// RemoveFavorite removes the right-clicked Explorer root, or asks which
// favorite to drop; missing folders are listed too so they can be cleaned up.
func (a *App) RemoveFavorite() {
	if node := a.ExplorerContextNode; node != nil {
		a.ExplorerContextNode = nil
		if i := a.favoriteIndex(node.ID); i >= 0 {
			a.removeFavorite(i)
		}
		return
	}
	favs := a.Settings.Welcome.Favorites
	if len(favs) == 0 {
		a.StatusWarn("No favorites")
		return
	}
	items := make([]widgets.SelectItem, len(favs))
	for i, fav := range favs {
		items[i] = widgets.SelectItem{ID: strconv.Itoa(i), Label: fav}
	}
	a.ShowSelectDialog("Remove from Favorites", items, func(id string) {
		if i, err := strconv.Atoi(id); err == nil && i < len(a.Settings.Welcome.Favorites) {
			a.removeFavorite(i)
		}
	}, nil)
}
