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

// filePickerRows is how many file rows the list shows. Tall enough to browse a
// directory without constant scrolling, short enough that the dialog still feels
// like a focused chooser rather than a full file manager.
const filePickerRows = 12

// filePicker is the state behind the Open File dialog: an editable path on top,
// plus a navigable listing beneath it. The input remains the source of truth so
// a known path is still easy to type, while the list helps discover nearby files
// and directories without leaving the dialog.
type filePicker struct {
	input  *widgets.InputWidget
	places *widgets.TreeWidget
	list   *widgets.TreeWidget
	dir    string
	filter string
}

// filePickerEntry is one row. It can point at a directory or a file.
type filePickerEntry struct {
	label string
	path  string
	dir   bool
}

// ShowFilePicker opens an existing file and returns its absolute path to onPick.
func (a *App) ShowFilePicker(title, confirmLabel, initial string, onPick func(string)) {
	fp := &filePicker{}

	submit := func(text string) {
		path := strings.TrimSpace(text)
		if path == "" {
			return
		}
		path = workspace.ExpandPath(path)
		abs, err := filepath.Abs(path)
		if err != nil {
			a.StatusError("Error: " + err.Error())
			return
		}
		// OpenFile treats a missing path as a new file, so validate before handing it off.
		info, err := os.Stat(abs)
		if err != nil {
			a.StatusError("Cannot open file " + abs + ": " + err.Error())
			return
		}
		if info.IsDir() {
			a.StatusError("Cannot open file: path is a directory: " + abs)
			return
		}
		if !info.Mode().IsRegular() {
			a.StatusError("Cannot open file: not a regular file: " + abs)
			return
		}
		a.DismissDialog()
		onPick(abs)
	}

	fp.input = widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: "File path",
		OnSubmit:    submit,
		OnChange:    func(text string) { fp.syncFromInput(text) },
	})
	fp.places = widgets.NewListWidgetFromConfig(widgets.ListConfig{
		EmptyText: "No places",
		OnCommand: func(command string, node *widgets.TreeNode) {
			if command == "activate" {
				fp.enter(node.ID)
			}
		},
	})
	fp.places.SetItems(placeNodes(workspace.Places()))
	fp.list = widgets.NewListWidgetFromConfig(widgets.ListConfig{
		EmptyText: "No files or folders",
		OnCommand: func(command string, node *widgets.TreeNode) {
			if command != "activate" {
				return
			}
			if info, err := os.Stat(node.ID); err == nil && info.IsDir() {
				fp.enter(node.ID)
				return
			}
			// Revalidate file rows in case they changed since the listing was loaded.
			submit(node.ID)
		},
	})

	if initial == "" {
		if !a.EditorGroup.IsActiveVirtual() {
			initial = filepath.Dir(a.EditorGroup.ActiveFilePath())
		} else if a.Workspace != nil {
			initial = a.Workspace.Primary()
		}
	}
	if initial == "" {
		initial = "."
	}
	fp.setDir(initial)
	fp.input.SetText(fp.dir)

	dialog := widgets.NewDialogWidget(64)
	dialog.Title = title
	dialog.Borders = *a.Borders
	placesBox := widgets.NewBoxWidget(widgets.BoxModel{PaddingRight: 1})
	placesBox.Child = fp.places
	placesBox.FixedWidth = folderPickerPlacesW
	columns := widgets.NewHStackWidget(placesBox, fp.list)
	columns.FixedHeight = filePickerRows
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
func (fp *filePicker) enter(dir string) {
	if dir == "" {
		return
	}
	fp.setDir(dir)
	fp.input.SetText(fp.dir)
}

// syncFromInput follows along as the path is typed: the listing tracks the
// deepest directory the text names, narrowed to whatever is being typed after
// it. Typing "~/repos/br" lists ~/repos showing only the matches, which keeps
// the path box fast without forcing a separate search mode.
func (fp *filePicker) syncFromInput(text string) {
	path := workspace.ExpandPath(strings.TrimSpace(text))
	if path == "" {
		return
	}
	// Keep the last typed segment as a filter while showing its existing parent directory.
	dir, filter := path, ""
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		dir, filter = filepath.Dir(path), filepath.Base(path)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return
	}
	abs, err := filepath.Abs(dir)
	if err != nil || (abs == fp.dir && filter == fp.filter) {
		return
	}
	fp.dir, fp.filter = abs, filter
	fp.refreshList()
}

func (fp *filePicker) setDir(dir string) {
	abs, err := filepath.Abs(workspace.ExpandPath(dir))
	if err != nil {
		return
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return
	}
	fp.dir, fp.filter = abs, ""
	fp.refreshList()
}

func (fp *filePicker) refreshList() {
	fp.list.SetItems(filePickerNodes(filterFilePickerEntries(filePickerEntries(fp.dir), fp.filter)))
}

// filePickerEntries builds the rows for dir: the way up, then files and
// folders inside it.
func filePickerEntries(dir string) []filePickerEntry {
	var out []filePickerEntry
	if parent := filepath.Dir(dir); parent != dir {
		out = append(out, filePickerEntry{label: "..", path: parent, dir: true})
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, item := range items {
		path := filepath.Join(dir, item.Name())
		isDir := item.IsDir()
		isRegular := item.Type().IsRegular()
		// DirEntry reports a symlink's own type, never its target's, so the target
		// needs one explicit stat. Windows junctions are ModeIrregular, not
		// ModeSymlink, so both need resolving. Ordinary entries use the type bits
		// ReadDir already has, avoiding a metadata call per entry.
		if item.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			isDir = info.IsDir()
			isRegular = info.Mode().IsRegular()
		}
		switch {
		case isDir:
			out = append(out, filePickerEntry{label: item.Name(), path: path, dir: true})
		case isRegular:
			out = append(out, filePickerEntry{label: item.Name(), path: path})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].label == out[j].label {
			return false
		}
		if out[i].label == ".." {
			return true
		}
		if out[j].label == ".." {
			return false
		}
		if out[i].dir != out[j].dir {
			return out[i].dir
		}
		hiddenI := strings.HasPrefix(out[i].label, ".")
		hiddenJ := strings.HasPrefix(out[j].label, ".")
		if hiddenI != hiddenJ {
			return !hiddenI
		}
		return strings.ToLower(out[i].label) < strings.ToLower(out[j].label)
	})
	return out
}

// filterFilePickerEntries narrows a listing to what is being typed. ".." stays
// visible even when the filter matches nothing, because that is the most useful
// escape hatch when the current directory is no longer the one you want.
func filterFilePickerEntries(entries []filePickerEntry, filter string) []filePickerEntry {
	if filter == "" {
		return entries
	}
	needle := strings.ToLower(filter)
	var out []filePickerEntry
	for _, entry := range entries {
		// Keep the parent row available even when no names match the filter.
		if entry.label == ".." || strings.Contains(strings.ToLower(entry.label), needle) {
			out = append(out, entry)
		}
	}
	return out
}

func filePickerNodes(entries []filePickerEntry) []*widgets.TreeNode {
	nodes := make([]*widgets.TreeNode, len(entries))
	for i, entry := range entries {
		nodes[i] = &widgets.TreeNode{ID: entry.path, Label: entry.label}
	}
	return nodes
}
