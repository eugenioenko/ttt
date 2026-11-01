package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
)

// folderPickerRows is how many directory rows the list shows. Tall enough to
// browse without scrolling constantly, short enough that the dialog still reads
// as a dialog on a laptop screen.
const (
	folderPickerRows    = 12
	folderPickerPlacesW = 18
)

// folderPicker is the state behind the Open/Add Folder dialog: an editable path
// on top and a navigable listing under it, kept in sync in both directions.
//
// Typing a path stays the fastest route for anyone who knows where they are
// going, so the input remains the source of truth; the list is a way to discover
// and refine it, not a mode you have to enter.
type folderPicker struct {
	app    *App
	input  *widgets.InputWidget
	places *widgets.TreeWidget
	list   *widgets.TreeWidget
	dir    string
	// filter is the half-typed last segment of the path. Keeping it separate from
	// dir is what lets "~/repos/br" list ~/repos narrowed to what matches, rather
	// than listing nothing because that path does not exist yet.
	filter string
}

// folderPickerEntry is one row. dir is where selecting it takes you; places and
// the parent entry are just rows whose dir happens to be elsewhere.
type folderPickerEntry struct {
	label string
	dir   string
}

// ShowFolderPicker asks for a directory, with the shortcuts the user already
// keeps in their file manager and a listing they can walk.
func (a *App) ShowFolderPicker(title, confirmLabel, initial string, onPick func(string)) {
	fp := &folderPicker{app: a}

	submit := func(text string) {
		path := strings.TrimSpace(text)
		if path == "" {
			return
		}
		a.DismissDialog()
		onPick(path)
	}

	fp.input = widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: "Folder path",
		OnSubmit:    submit,
		OnChange:    func(text string) { fp.syncFromInput(text) },
	})

	// OnCommand("activate"), not OnSelect: OnSelect fires every time the selection
	// moves, so arrowing down a list would walk into each directory it passed over
	// and rebuild the listing under the cursor.
	fp.places = widgets.NewListWidgetFromConfig(widgets.ListConfig{
		EmptyText: "No places",
		OnCommand: func(command string, node *widgets.TreeNode) {
			if command == "activate" {
				fp.enter(node.ID)
			}
		},
	})
	fp.places.SetItems(placeNodes(a.favoriteDirs(), workspace.Places()))

	fp.list = widgets.NewListWidgetFromConfig(widgets.ListConfig{
		EmptyText: "No subdirectories",
		OnCommand: func(command string, node *widgets.TreeNode) {
			if command == "activate" {
				fp.enter(node.ID)
			}
		},
	})

	if initial == "" {
		initial = a.Workspace.Primary()
	}
	fp.setDir(initial)
	fp.input.SetText(fp.dir)

	dialog := widgets.NewDialogWidget(64)
	dialog.Title = title
	dialog.Borders = *a.Borders
	// Places on the left, the listing on the right, the way a file manager puts
	// them: shortcuts are always reachable without scrolling past them to get to
	// the directory you are actually in.
	placesBox := widgets.NewBoxWidget(widgets.BoxModel{PaddingRight: 1})
	placesBox.Child = fp.places
	placesBox.FixedWidth = folderPickerPlacesW

	// Both trees measure as grow children (height 0), so the dialog would collapse
	// them away; the stack is what reserves the rows.
	columns := widgets.NewHStackWidget(placesBox, fp.list)
	columns.FixedHeight = folderPickerRows
	columns.Gap = 1

	dialog.SetContent(widgets.NewVStackWidget(fp.input, columns))
	dialog.Buttons = []widgets.DialogButton{
		{Label: "&Cancel", Handler: func() { a.DismissDialog() }},
		{Label: "&" + confirmLabel, Handler: func() { submit(fp.input.Text()) }},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()
	a.ShowDialog(ui.NewWidgetAdapter(dialog))
}

// enter descends into dir and puts it in the input, so confirming right after
// picking a row does what it looks like it will do.
func (fp *folderPicker) enter(dir string) {
	if dir == "" {
		return
	}
	fp.setDir(dir)
	fp.input.SetText(fp.dir)
}

// syncFromInput follows along as the path is typed: the listing tracks the
// deepest directory the text names, narrowed to whatever is being typed after
// it. Typing "~/repos/br" lists ~/repos showing only the matches, which is the
// difference between a path box and something you can search.
func (fp *folderPicker) syncFromInput(text string) {
	path := expandFolderPath(strings.TrimSpace(text))
	if path == "" {
		return
	}

	dir, filter := path, ""
	// A trailing separator means "inside this one"; otherwise the last segment is
	// either a directory that exists or the prefix of one being typed.
	if !strings.HasSuffix(path, string(os.PathSeparator)) {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			dir, filter = filepath.Dir(path), filepath.Base(path)
		}
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return
	}
	if abs == fp.dir && filter == fp.filter {
		return
	}
	fp.dir, fp.filter = abs, filter
	fp.refreshList()
}

func (fp *folderPicker) setDir(dir string) {
	abs, err := filepath.Abs(expandFolderPath(dir))
	if err != nil {
		return
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return
	}
	fp.dir, fp.filter = abs, ""
	fp.refreshList()
}

func (fp *folderPicker) refreshList() {
	fp.list.SetItems(folderPickerNodes(filterEntries(folderPickerEntries(fp.dir), fp.filter)))
}

// filterEntries narrows a listing to what is being typed. ".." survives every
// filter: the way back out should not disappear because the search matched
// nothing, which is when you most want it.
func filterEntries(entries []folderPickerEntry, filter string) []folderPickerEntry {
	if filter == "" {
		return entries
	}
	needle := strings.ToLower(filter)
	var out []folderPickerEntry
	for _, e := range entries {
		if e.label == ".." || strings.Contains(strings.ToLower(e.label), needle) {
			out = append(out, e)
		}
	}
	return out
}

// folderPickerEntries builds the rows for dir: the way up, then what is inside.
func folderPickerEntries(dir string) []folderPickerEntry {
	var out []folderPickerEntry
	if parent := filepath.Dir(dir); parent != dir {
		out = append(out, folderPickerEntry{label: "..", dir: parent})
	}
	out = append(out, subdirectories(dir)...)
	return out
}

// subdirectories lists dir's child directories, sorted, with dotfiles last:
// they are worth reaching but rarely what someone is looking for.
func subdirectories(dir string) []folderPickerEntry {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []folderPickerEntry
	for _, item := range items {
		path := filepath.Join(dir, item.Name())
		isDir := item.IsDir()
		// DirEntry reports a symlink's own type, never its target's.
		if item.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(path)
			isDir = err == nil && info.IsDir()
		}
		if !isDir {
			continue
		}
		out = append(out, folderPickerEntry{label: item.Name(), dir: path})
	}
	sort.SliceStable(out, func(i, j int) bool {
		hiddenI := strings.HasPrefix(out[i].label, ".")
		hiddenJ := strings.HasPrefix(out[j].label, ".")
		if hiddenI != hiddenJ {
			return hiddenJ
		}
		return strings.ToLower(out[i].label) < strings.ToLower(out[j].label)
	})
	return out
}

func folderPickerNodes(entries []folderPickerEntry) []*widgets.TreeNode {
	nodes := make([]*widgets.TreeNode, len(entries))
	for i, e := range entries {
		nodes[i] = &widgets.TreeNode{ID: e.dir, Label: e.label}
	}
	return nodes
}

// placeNodes lists the favorites under a heading first, then a blank row and
// every place.
func placeNodes(favs []favoriteDir, places []workspace.Place) []*widgets.TreeNode {
	var nodes []*widgets.TreeNode
	if len(favs) > 0 {
		nodes = append(nodes, &widgets.TreeNode{Label: "Favorites", Muted: true, Heading: true})
		for _, f := range favs {
			nodes = append(nodes, &widgets.TreeNode{ID: f.abs, Label: filepath.Base(f.abs)})
		}
		nodes = append(nodes, &widgets.TreeNode{Heading: true})
	}
	for _, p := range places {
		nodes = append(nodes, &widgets.TreeNode{ID: p.Path, Label: p.Name})
	}
	return nodes
}

func expandFolderPath(path string) string {
	return workspace.ExpandPath(path)
}
