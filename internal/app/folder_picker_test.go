package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func labels(entries []folderPickerEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.label
	}
	return out
}

func TestFolderPickerEntriesOrdersTheListing(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zeta", ".hidden", "Alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a-file.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	got := labels(folderPickerEntries(dir))
	want := []string{"..", "Alpha", "beta", "zeta", ".hidden"}
	if !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
}

func TestFolderPickerEntriesFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{"to-dir": target, "to-file": filepath.Join(dir, "file"), "broken": filepath.Join(dir, "missing")}
	for name, dest := range links {
		if err := os.Symlink(dest, filepath.Join(dir, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	got := labels(folderPickerEntries(dir))
	want := []string{"..", "to-dir"}
	if !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
}

// The root has no parent, and offering ".." there would just point at itself.
func TestFolderPickerEntriesOmitsParentAtRoot(t *testing.T) {
	got := labels(folderPickerEntries(string(os.PathSeparator)))
	if slices.Contains(got, "..") {
		t.Fatalf("entries at root = %v, want no \"..\"", got)
	}
}

func TestFolderPickerEntriesOnUnreadableDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nope")
	// A directory that cannot be listed still offers the way back out.
	if got := labels(folderPickerEntries(dir)); !slices.Equal(got, []string{".."}) {
		t.Fatalf("entries = %v, want just [..]", got)
	}
}

func TestFilterEntriesKeepsTheWayOut(t *testing.T) {
	entries := []folderPickerEntry{
		{label: ".."}, {label: "vault"}, {label: "scripts"}, {label: "APP"},
	}

	// Case-insensitive, and a substring is enough: the point is finding a folder
	// you half remember, not matching a prefix exactly.
	got := labels(filterEntries(entries, "ap"))
	if !slices.Equal(got, []string{"..", "APP"}) {
		t.Errorf("filter %q = %v, want [.. APP]", "ap", got)
	}

	// ".." survives a filter that matches nothing else — losing the way back out
	// exactly when the search fails is the worst possible time for it.
	got = labels(filterEntries(entries, "zzz"))
	if !slices.Equal(got, []string{".."}) {
		t.Errorf("filter %q = %v, want [..]", "zzz", got)
	}

	if got := labels(filterEntries(entries, "")); len(got) != 4 {
		t.Errorf("empty filter = %v, want everything", got)
	}
}

func TestPlaceNodesKeepsPlacesAndFavorites(t *testing.T) {
	places := []workspace.Place{{Name: "Home", Path: "/home/u"}, {Name: "Documents", Path: "/home/u/Documents"}}
	nodeFields := func(nodes []*widgets.TreeNode) (labels, ids []string) {
		for _, n := range nodes {
			if n.Label == "Places" && (!n.Heading || !n.Muted) {
				t.Errorf("Places must be a muted heading, got %+v", n)
			}
			labels = append(labels, n.Label)
			ids = append(ids, n.ID)
		}
		return labels, ids
	}

	gotLabels, gotIDs := nodeFields(placeNodes(nil, places))
	if !slices.Equal(gotLabels, []string{"Places", "Home", "Documents"}) {
		t.Errorf("without favorites = %v, want Places heading and every place", gotLabels)
	}
	if !slices.Equal(gotIDs, []string{"", "/home/u", "/home/u/Documents"}) {
		t.Errorf("ids without favorites = %v, heading must have no ID", gotIDs)
	}

	gotLabels, gotIDs = nodeFields(placeNodes([]favoriteDir{{path: "~/repos/ttt", abs: "/home/u/repos/ttt"}}, places))
	if !slices.Equal(gotLabels, []string{"Favorites", "ttt", "", "Places", "Home", "Documents"}) {
		t.Errorf("labels = %v", gotLabels)
	}
	if !slices.Equal(gotIDs, []string{"", "/home/u/repos/ttt", "", "", "/home/u", "/home/u/Documents"}) {
		t.Errorf("ids = %v, headings must have no ID", gotIDs)
	}
}

func TestPickerInputTextEndsWithSeparator(t *testing.T) {
	sep := string(os.PathSeparator)
	dir := filepath.Join(t.TempDir(), "repos")
	for in, want := range map[string]string{dir: dir + sep, dir + sep: dir + sep, sep: sep, "": ""} {
		if got := pickerInputText(in); got != want {
			t.Errorf("pickerInputText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFolderPickerPathDropsTrailingSeparator(t *testing.T) {
	sep := string(os.PathSeparator)
	dir := filepath.Join(t.TempDir(), "repos")
	for in, want := range map[string]string{dir + sep: dir, " " + dir + sep + " ": dir, dir: dir, sep: sep, "~" + sep: "~", "  ": ""} {
		if got := folderPickerPath(in); got != want {
			t.Errorf("folderPickerPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFolderPickerEnterLeavesTheInputReadyForAName(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"repos", filepath.Join("repos", "ttt"), filepath.Join("repos", "tools")} {
		if err := os.Mkdir(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	repos := filepath.Join(dir, "repos")
	fp := &folderPicker{
		input: widgets.NewInputWidget(widgets.InputConfig{}),
		list:  widgets.NewListWidgetFromConfig(widgets.ListConfig{}),
	}
	fp.enter(repos)
	if got, want := fp.input.Text(), repos+string(os.PathSeparator); got != want {
		t.Fatalf("input = %q, want %q", got, want)
	}

	fp.syncFromInput(fp.input.Text() + "tt")
	if fp.dir != repos || fp.filter != "tt" {
		t.Fatalf("directory, filter = %q, %q, want %q, %q", fp.dir, fp.filter, repos, "tt")
	}
	if got := folderPickerPath(fp.input.Text()); got != repos {
		t.Fatalf("confirming right away picks %q, want %q", got, repos)
	}
}
