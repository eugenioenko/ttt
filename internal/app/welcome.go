package app

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
)

const welcomeTabID = "welcome"

var (
	welcomeFavoritesHint string
	welcomeCommands      []struct{ label, commandID string }
)

func initWelcomeText() {
	welcomeFavoritesHint = i18n.Text("welcome.favoritesHint")

	welcomeCommands = []struct{ label, commandID string }{
		{i18n.Text("welcome.openFolder"), "workspace.openFolder"},
		{i18n.Text("welcome.cloneRepository"), "workspace.clone"},
		{i18n.Text("general.newFile"), "file.new"},
		{i18n.Text("welcome.openWorkspace"), "workspace.open"},
		{i18n.Text("general.settings"), "settings.openUI"},
		{i18n.Text("welcome.commandPalette"), "command.palette"},
	}
}

// welcomeItems lists the start actions with their shortcuts, then the
// favorite folders that exist, each with its path as written in settings,
// then the recently opened folders that exist and are not favorites.
func (a *App) welcomeItems() []welcomeItem {
	var items []welcomeItem
	for _, c := range welcomeCommands {
		shortcut := ""
		if cmd, ok := a.Reg.Get(c.commandID); ok {
			shortcut = cmd.Shortcut
		}
		items = append(items, welcomeItem{label: c.label, detail: shortcut, run: func() { a.Reg.Execute(c.commandID) }})
	}
	for _, fav := range a.favoriteDirs() {
		abs := fav.abs
		item := welcomeItem{
			label:  filepath.Base(abs),
			detail: fav.path,
			tight:  true,
			run:    func() { a.openFolderPath(abs) },
			copy:   func() { a.FileOpCopyAbsolutePath(abs) },
		}
		if len(items) == len(welcomeCommands) {
			item.section = i18n.Text("general.favorites")
			item.tight = false
		}
		items = append(items, item)
	}
	if !a.Settings.Welcome.IsRecentFoldersEnabled() {
		return items
	}
	first := true
	for _, abs := range a.State.RecentFolders {
		if a.favoriteIndex(abs) >= 0 {
			continue
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			continue
		}
		item := welcomeItem{
			label:  filepath.Base(abs),
			detail: tildePath(abs),
			tight:  true,
			run:    func() { a.openFolderPath(abs) },
			copy:   func() { a.FileOpCopyAbsolutePath(abs) },
		}
		if first {
			item.section = i18n.Text("welcome.recent")
			item.tight = false
			first = false
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
	a.EditorGroup.OpenPluginTab(welcomeTabID, i18n.Text("welcome.welcome"), adapter)
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
	v.note = welcomeFavoritesHint
	for _, item := range v.items {
		if item.section == i18n.Text("general.favorites") {
			v.note = ""
		}
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
	// Having no folders is not a choice to hide the sidebar; persisting it
	// would keep the sidebar hidden in the next session that has folders.
	a.hideSidebar()
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

type favoriteDir struct {
	path string
	abs  string
}

// favoriteDirs returns the favorites that exist as directories, each with its
// path as written in settings.
func (a *App) favoriteDirs() []favoriteDir {
	var out []favoriteDir
	for _, fav := range a.Settings.Welcome.Favorites {
		abs, err := filepath.Abs(workspace.ExpandPath(fav))
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			continue
		}
		out = append(out, favoriteDir{path: fav, abs: abs})
	}
	return out
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
		a.StatusError(i18n.Format("status.failedToSaveFavorites", i18n.Args{"error": err.Error()}))
		return
	}
	a.StatusNotify(msg)
}

func (a *App) addFavorite(abs string) {
	if a.favoriteIndex(abs) >= 0 {
		a.StatusNotify(i18n.Format("status.isAlreadyAFavorite", i18n.Args{"filename": filepath.Base(abs)}))
		return
	}
	a.saveFavorites(append(a.Settings.Welcome.Favorites, tildePath(abs)), i18n.Format("welcome.addedToFavorites", i18n.Args{"filename": filepath.Base(abs)}))
}

func (a *App) removeFavorite(i int) {
	favs := a.Settings.Welcome.Favorites
	name := filepath.Base(favs[i])
	a.saveFavorites(append(favs[:i:i], favs[i+1:]...), i18n.Format("welcome.removedFromFavorites", i18n.Args{"name": name}))
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
		a.StatusWarn(i18n.Text("status.noOpenFolderToAddToFavorites"))
	case 1:
		a.addFavorite(items[0].ID)
	default:
		a.ShowSelectDialog(i18n.Text("welcome.addToFavorites"), items, a.addFavorite, nil)
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
		a.StatusWarn(i18n.Text("status.noFavorites"))
		return
	}
	items := make([]widgets.SelectItem, len(favs))
	for i, fav := range favs {
		items[i] = widgets.SelectItem{ID: strconv.Itoa(i), Label: fav}
	}
	a.ShowSelectDialog(i18n.Text("welcome.removeFromFavorites"), items, func(id string) {
		if i, err := strconv.Atoi(id); err == nil && i < len(a.Settings.Welcome.Favorites) {
			a.removeFavorite(i)
		}
	}, nil)
}

const maxRecentFolders = 10

// pushRecent puts paths at the front of recent, most recent first, without
// duplicates.
func pushRecent(recent, paths []string) []string {
	out := slices.Clone(paths)
	for _, p := range recent {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out[:min(len(out), maxRecentFolders)]
}

// RememberRecentFolders records the open folders for the welcome page. It
// merges into the list on disk, so another ttt window's folders are kept.
func (a *App) RememberRecentFolders() {
	if !a.Settings.Welcome.IsRecentFoldersEnabled() {
		a.clearRecentFolders()
		return
	}
	paths := a.Workspace.Paths()
	if len(paths) == 0 {
		return
	}
	disk := config.LoadState()
	recent := pushRecent(disk.RecentFolders, paths)
	a.State.RecentFolders = recent
	if slices.Equal(recent, disk.RecentFolders) {
		return
	}
	disk.RecentFolders = recent
	if err := config.SaveState(disk); err != nil {
		a.StatusError(i18n.Format("status.failedToSaveRecentFolders", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) clearRecentFolders() {
	a.State.RecentFolders = nil
	disk := config.LoadState()
	if len(disk.RecentFolders) == 0 {
		return
	}
	disk.RecentFolders = nil
	if err := config.SaveState(disk); err != nil {
		a.StatusError(i18n.Format("status.failedToClearRecentFolders", i18n.Args{"error": err.Error()}))
	}
}
