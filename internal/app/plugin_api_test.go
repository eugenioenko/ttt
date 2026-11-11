package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestContextMenuItemFromWidgetEntryPreservesMenuContract(t *testing.T) {
	checked, unchecked := true, false
	tests := []struct {
		name      string
		checked   *bool
		separator bool
		want      int
	}{
		{name: "omitted", want: 0},
		{name: "unchecked", checked: &unchecked, want: ui.MenuUnchecked},
		{name: "checked", checked: &checked, want: ui.MenuChecked},
		{name: "separator", separator: true, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := widgets.MenuEntry{
				Label:     "Mode",
				Command:   "mode",
				Separator: test.separator,
				Checked:   test.checked,
			}
			item := contextMenuItemFromWidgetEntry(entry)

			if item.Label != entry.Label || item.Command != entry.Command || item.IsSep != entry.Separator {
				t.Fatalf("menu fields changed during conversion: got %+v, entry %+v", item, entry)
			}
			if item.Checked != test.want {
				t.Fatalf("checked indicator = %d, want %d", item.Checked, test.want)
			}
		})
	}
}

func TestContextMenuItemsFromWidgetEntriesPreservesMixedRows(t *testing.T) {
	checked, unchecked := true, false
	entries := []widgets.MenuEntry{
		{Label: "Omitted", Command: "omitted"},
		{Label: "Unchecked", Command: "unchecked", Checked: &unchecked},
		{Separator: true, Checked: &checked},
		{Label: "Checked", Command: "checked", Checked: &checked},
	}

	items := contextMenuItemsFromWidgetEntries(entries)
	if len(items) != len(entries) {
		t.Fatalf("items = %d, want %d", len(items), len(entries))
	}
	wantChecked := []int{0, ui.MenuUnchecked, 0, ui.MenuChecked}
	for i := range items {
		if items[i].Label != entries[i].Label || items[i].Command != entries[i].Command || items[i].IsSep != entries[i].Separator {
			t.Errorf("row %d changed during conversion: got %+v, entry %+v", i, items[i], entries[i])
		}
		if items[i].Checked != wantChecked[i] {
			t.Errorf("row %d checked = %d, want %d", i, items[i].Checked, wantChecked[i])
		}
	}
}

func TestSetPathNilDeletesKey(t *testing.T) {
	m := map[string]any{
		"lsp": map[string]any{
			"servers": map[string]any{
				"go":   map[string]any{"command": []any{"gopls"}},
				"rust": map[string]any{"command": []any{"rust-analyzer"}},
			},
		},
	}

	setPath(m, []string{"lsp", "servers", "go"}, nil)

	servers := m["lsp"].(map[string]any)["servers"].(map[string]any)
	if _, exists := servers["go"]; exists {
		t.Error("expected 'go' key to be deleted")
	}
	if _, exists := servers["rust"]; !exists {
		t.Error("expected 'rust' key to remain")
	}
}

func TestGetPathNestedValue(t *testing.T) {
	m := map[string]any{
		"lsp": map[string]any{
			"servers": map[string]any{
				"go": map[string]any{"command": []any{"gopls"}},
			},
		},
	}

	val, ok := getPath(m, []string{"lsp", "servers", "go"})
	if !ok {
		t.Fatal("expected to find lsp.servers.go")
	}
	srv, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	cmd := srv["command"].([]any)
	if len(cmd) != 1 || cmd[0] != "gopls" {
		t.Errorf("expected [gopls], got %v", cmd)
	}

	_, ok = getPath(m, []string{"lsp", "servers", "nonexistent"})
	if ok {
		t.Error("expected false for nonexistent key")
	}
}

func trackPluginFilesystemAPI(t *testing.T, api *PluginFilesystemAPI) *PluginFilesystemAPI {
	t.Helper()
	t.Cleanup(func() {
		if err := api.Close(); err != nil {
			t.Errorf("close plugin filesystem API: %v", err)
		}
	})
	return api
}

func exactFileGrantAPI(t *testing.T, path string) *PluginFilesystemAPI {
	t.Helper()
	p := &plugin.Plugin{Granted: plugin.PermissionSet{
		FsRead:  plugin.FsAccess{Paths: []string{path}},
		FsWrite: plugin.FsAccess{Paths: []string{path}},
	}}
	return trackPluginFilesystemAPI(t, NewPluginFilesystemAPIFor(p, nil))
}

func TestPluginFilesystemAPIClose(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	api := trackPluginFilesystemAPI(t, NewPluginFilesystemAPI(root))

	if err := api.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := api.ReadFile(path); err == nil {
		t.Error("read succeeded after closing filesystem API")
	}
	if err := api.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}

	p := &plugin.Plugin{Granted: plugin.PermissionSet{
		FsRead: plugin.FsAccess{Paths: []string{plugin.FsAnyPath}},
	}}
	allAPI := trackPluginFilesystemAPI(t, NewPluginFilesystemAPIFor(p, nil))
	if err := allAPI.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := allAPI.ReadFile(path); !errors.Is(err, os.ErrClosed) {
		t.Errorf("unrestricted read after close error = %v; want %v", err, os.ErrClosed)
	}
}

func TestPluginFilesystemAPI_PathRestriction(t *testing.T) {
	tmpDir := t.TempDir()
	allowed := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(allowed, 0755)
	os.WriteFile(filepath.Join(allowed, "test.txt"), []byte("hello"), 0644)

	forbidden := filepath.Join(tmpDir, "secrets")
	os.MkdirAll(forbidden, 0755)
	os.WriteFile(filepath.Join(forbidden, "key.pem"), []byte("secret"), 0644)

	api := trackPluginFilesystemAPI(t, NewPluginFilesystemAPI(allowed))

	t.Run("read allowed", func(t *testing.T) {
		content, err := api.ReadFile(filepath.Join(allowed, "test.txt"))
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if content != "hello" {
			t.Errorf("expected 'hello', got %q", content)
		}
	})

	t.Run("read forbidden", func(t *testing.T) {
		_, err := api.ReadFile(filepath.Join(forbidden, "key.pem"))
		if err == nil {
			t.Fatal("expected access denied error")
		}
	})

	t.Run("write allowed", func(t *testing.T) {
		err := api.WriteFile(filepath.Join(allowed, "out.txt"), "data")
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
	})

	t.Run("write forbidden", func(t *testing.T) {
		err := api.WriteFile(filepath.Join(forbidden, "evil.txt"), "data")
		if err == nil {
			t.Fatal("expected access denied error")
		}
	})

	t.Run("exists allowed", func(t *testing.T) {
		if !api.FileExists(filepath.Join(allowed, "test.txt")) {
			t.Error("expected true for existing allowed file")
		}
	})

	t.Run("exists forbidden returns false", func(t *testing.T) {
		if api.FileExists(filepath.Join(forbidden, "key.pem")) {
			t.Error("expected false for forbidden path")
		}
	})

	t.Run("list allowed", func(t *testing.T) {
		entries, err := api.ListDir(allowed)
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if len(entries) == 0 {
			t.Error("expected entries in allowed dir")
		}
	})

	t.Run("list forbidden", func(t *testing.T) {
		_, err := api.ListDir(forbidden)
		if err == nil {
			t.Fatal("expected access denied error")
		}
	})

	t.Run("path traversal blocked", func(t *testing.T) {
		_, err := api.ReadFile(filepath.Join(allowed, "..", "secrets", "key.pem"))
		if err == nil {
			t.Fatal("expected path traversal to be blocked")
		}
	})

	t.Run("absolute path outside roots blocked", func(t *testing.T) {
		_, err := api.ReadFile("/etc/passwd")
		if err == nil {
			t.Fatal("expected absolute path outside roots to be blocked")
		}
	})
}

func TestPluginFilesystemAPI_ExactFileGrantsShareParentRoot(t *testing.T) {
	base := t.TempDir()
	paths := []string{filepath.Join(base, "first.txt"), filepath.Join(base, "second.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	p := &plugin.Plugin{Granted: plugin.PermissionSet{
		FsRead:  plugin.FsAccess{Paths: paths},
		FsWrite: plugin.FsAccess{Paths: paths},
	}}
	api := trackPluginFilesystemAPI(t, NewPluginFilesystemAPIFor(p, nil))
	if api.read.roots[0].root != api.read.roots[1].root {
		t.Fatal("sibling exact-file grants opened separate parent roots")
	}
}

func TestPluginFilesystemAPI_ListDirSortsEntries(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"z.txt", "a.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	api := trackPluginFilesystemAPI(t, NewPluginFilesystemAPI(root))
	entries, err := api.ListDir(root)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(entries))
	for i, entry := range entries {
		got[i] = entry.Name
	}
	if !slices.Equal(got, []string{"a.txt", "z.txt"}) {
		t.Errorf("ListDir entries = %v, want [a.txt z.txt]", got)
	}
}

func TestPluginFilesystemAPI_ExactFileGrant(t *testing.T) {
	base := t.TempDir()
	granted := filepath.Join(base, "granted.txt")
	sibling := filepath.Join(base, "sibling.txt")
	if err := os.WriteFile(granted, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := exactFileGrantAPI(t, granted)
	if got, err := api.ReadFile(granted); err != nil || got != "original" {
		t.Fatalf("read exact grant = %q, %v; want %q", got, err, "original")
	}
	if !api.FileExists(granted) {
		t.Fatal("exact file grant does not exist")
	}
	if err := api.WriteFile(granted, "updated"); err != nil {
		t.Fatalf("write exact grant: %v", err)
	}
	if got, err := os.ReadFile(granted); err != nil || string(got) != "updated" {
		t.Errorf("exact grant after write = %q, %v; want %q", got, err, "updated")
	}
	if _, err := api.ReadFile(sibling); err == nil {
		t.Error("sibling file was accessible through exact file grant")
	}
	if _, err := api.ReadFile(filepath.Join(granted, "child.txt")); err == nil {
		t.Error("descendant path was accessible through exact file grant")
	}
}

func TestPluginFilesystemAPI_MissingExactFileGrantCanCreateOnlyGrantedPath(t *testing.T) {
	base := t.TempDir()
	granted := filepath.Join(base, "new.txt")
	sibling := filepath.Join(base, "sibling.txt")
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := exactFileGrantAPI(t, granted)
	if api.FileExists(granted) {
		t.Fatal("missing exact grant exists before write")
	}
	resolved, err := api.write.openRoot(granted)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.temporary {
		t.Fatal("exact missing-file operation marked its cached parent root temporary")
	}
	if err := api.WriteFile(granted, "created"); err != nil {
		t.Fatalf("create exact grant: %v", err)
	}
	if got, err := api.ReadFile(granted); err != nil || got != "created" {
		t.Fatalf("read created exact grant = %q, %v; want %q", got, err, "created")
	}
	if !api.FileExists(granted) {
		t.Error("created exact file grant does not exist")
	}
	if err := api.WriteFile(sibling, "changed"); err == nil {
		t.Error("sibling file was writable through missing exact file grant")
	}
	if got, err := os.ReadFile(sibling); err != nil || string(got) != "sibling" {
		t.Errorf("sibling after denied write = %q, %v; want %q", got, err, "sibling")
	}
}

func TestPluginFilesystemAPI_MissingExactFileGrantCreatesAfterParentAppears(t *testing.T) {
	base := t.TempDir()
	parent := filepath.Join(base, "future-parent")
	granted := filepath.Join(parent, "new.txt")
	api := exactFileGrantAPI(t, granted)

	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(parent, "sibling.txt")
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := api.WriteFile(granted, "created"); err != nil {
		t.Fatalf("create exact grant after parent appears: %v", err)
	}
	if got, err := api.ReadFile(granted); err != nil || got != "created" {
		t.Errorf("read created exact grant = %q, %v; want %q", got, err, "created")
	}
	if err := api.WriteFile(sibling, "changed"); err == nil {
		t.Error("sibling was writable through missing exact-file grant")
	}
	if got, err := os.ReadFile(sibling); err != nil || string(got) != "sibling" {
		t.Errorf("sibling after denied write = %q, %v; want %q", got, err, "sibling")
	}
}

func TestPluginFilesystemAPI_MissingGrantThatBecomesDirectoryAllowsDescendants(t *testing.T) {
	base := t.TempDir()
	granted := filepath.Join(base, "future")
	sibling := filepath.Join(base, "sibling.txt")
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := exactFileGrantAPI(t, granted)
	if err := os.Mkdir(granted, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(granted, "child.txt")
	if err := os.WriteFile(child, []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, err := api.ReadFile(child); err != nil || got != "inside" {
		t.Errorf("read descendant of materialized grant = %q, %v; want %q", got, err, "inside")
	}
	if !api.FileExists(child) || !api.FileExists(granted) {
		t.Error("FileExists did not recognize the materialized grant or its child")
	}
	if entries, err := api.ListDir(granted); err != nil || len(entries) != 1 || entries[0].Name != "child.txt" {
		t.Errorf("ListDir materialized grant = %v, %v; want [child.txt]", entries, err)
	}
	written := filepath.Join(granted, "written.txt")
	if err := api.WriteFile(written, "written"); err != nil {
		t.Errorf("write descendant of materialized grant: %v", err)
	} else if got, err := api.ReadFile(written); err != nil || got != "written" {
		t.Errorf("read written descendant = %q, %v; want %q", got, err, "written")
	}
	if _, err := api.ReadFile(sibling); err == nil {
		t.Error("sibling outside materialized grant was accessible")
	}
}

func TestPluginFilesystemAPI_MissingGrantCanChangeFromFileToDirectory(t *testing.T) {
	base := t.TempDir()
	granted := filepath.Join(base, "future")
	sibling := filepath.Join(base, "sibling.txt")
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := exactFileGrantAPI(t, granted)
	if err := api.WriteFile(granted, "first file"); err != nil {
		t.Fatalf("create missing grant as file: %v", err)
	}
	if err := os.Remove(granted); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(granted, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(granted, "child.txt")
	if err := os.WriteFile(child, []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, err := api.ReadFile(child); err != nil || got != "inside" {
		t.Errorf("read descendant after missing grant changes from file to directory = %q, %v; want %q", got, err, "inside")
	}
	if _, err := api.ReadFile(sibling); err == nil {
		t.Error("sibling outside materialized grant was accessible")
	}
}

func TestPluginFilesystemAPI_MissingParentGrantThatBecomesDirectoryAllowsDescendants(t *testing.T) {
	base := t.TempDir()
	granted := filepath.Join(base, "future-parent", "future")
	api := exactFileGrantAPI(t, granted)

	child := filepath.Join(granted, "child.txt")
	sibling := filepath.Join(base, "future-parent", "sibling.txt")
	if err := os.MkdirAll(granted, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, err := api.ReadFile(child); err != nil || got != "inside" {
		t.Errorf("read descendant after missing parents materialize = %q, %v; want %q", got, err, "inside")
	}
	if _, err := api.ReadFile(sibling); err == nil {
		t.Error("sibling outside materialized grant was accessible")
	}
}

func TestPluginFilesystemAPI_MissingGrantUnderSymlinkedParentDoesNotGrantDescendants(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(base, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	granted := filepath.Join(link, "future")
	api := exactFileGrantAPI(t, granted)

	if err := os.Mkdir(granted, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(granted, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, err := api.ReadFile(secret); err == nil {
		t.Errorf("read through newly materialized grant with symlinked parent = %q, want denial", got)
	}
}

func TestPluginFilesystemAPI_MissingGrantRejectsInsertedIntermediateSymlink(t *testing.T) {
	base := t.TempDir()
	sibling := filepath.Join(base, "sibling")
	granted := filepath.Join(base, "future-parent", "future")
	api := exactFileGrantAPI(t, granted)
	if err := os.MkdirAll(filepath.Join(sibling, "future"), 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(sibling, "future", "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sibling, filepath.Join(base, "future-parent")); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}

	if got, err := api.ReadFile(filepath.Join(granted, "secret.txt")); err == nil {
		t.Errorf("read through inserted intermediate symlink to sibling = %q, want denial", got)
	}
}

func TestPluginFilesystemAPI_MissingGrantRejectsInsertedLeafSymlink(t *testing.T) {
	base := t.TempDir()
	sibling := filepath.Join(base, "sibling")
	granted := filepath.Join(base, "future")
	api := exactFileGrantAPI(t, granted)
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(sibling, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sibling, granted); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}

	if got, err := api.ReadFile(filepath.Join(granted, "secret.txt")); err == nil {
		t.Errorf("read through inserted leaf symlink to sibling = %q, want denial", got)
	}
}

func TestPluginFilesystemAPI_ExactFileGrantRejectsLeafSymlinkReplacement(t *testing.T) {
	base := t.TempDir()
	granted := filepath.Join(base, "granted.txt")
	sibling := filepath.Join(base, "sibling.txt")
	if err := os.WriteFile(granted, []byte("authorized"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sibling, []byte("sibling secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := exactFileGrantAPI(t, granted)
	if got, err := api.ReadFile(granted); err != nil || got != "authorized" {
		t.Fatalf("read exact grant before replacement = %q, %v; want %q", got, err, "authorized")
	}
	if err := os.Remove(granted); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sibling, granted); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if got, err := api.ReadFile(granted); err == nil {
		t.Errorf("read through replaced exact grant = %q, want an error", got)
	}
	if api.FileExists(granted) {
		t.Error("FileExists followed replaced exact grant symlink")
	}
	if err := api.WriteFile(granted, "overwritten"); err == nil {
		t.Error("write through replaced exact grant succeeded")
	}
	if got, err := os.ReadFile(sibling); err != nil || string(got) != "sibling secret" {
		t.Errorf("sibling after symlink replacement = %q, %v; want %q", got, err, "sibling secret")
	}
}

func TestPluginFilesystemAPI_ExactFileGrantRejectsDirectoryReplacement(t *testing.T) {
	base := t.TempDir()
	granted := filepath.Join(base, "granted.txt")
	if err := os.WriteFile(granted, []byte("authorized"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := exactFileGrantAPI(t, granted)
	if err := os.Remove(granted); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(granted, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(granted, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	if entries, err := api.ListDir(granted); err == nil {
		t.Errorf("ListDir through replaced exact-file grant returned %v, want an error", entries)
	}
}

func TestPluginFilesystemAPI_MultipleRoots(t *testing.T) {
	tmpDir := t.TempDir()
	root1 := filepath.Join(tmpDir, "project")
	root2 := filepath.Join(tmpDir, "plugin")
	outside := filepath.Join(tmpDir, "outside")

	for _, d := range []string{root1, root2, outside} {
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, "file.txt"), []byte("data"), 0644)
	}

	api := trackPluginFilesystemAPI(t, NewPluginFilesystemAPI(root1, root2))

	if _, err := api.ReadFile(filepath.Join(root1, "file.txt")); err != nil {
		t.Errorf("root1 should be accessible: %v", err)
	}
	if _, err := api.ReadFile(filepath.Join(root2, "file.txt")); err != nil {
		t.Errorf("root2 should be accessible: %v", err)
	}
	if _, err := api.ReadFile(filepath.Join(outside, "file.txt")); err == nil {
		t.Error("outside should not be accessible")
	}
}

func TestPluginFilesystemAPI_SymlinkEscape(t *testing.T) {
	tmpDir := t.TempDir()
	allowed := filepath.Join(tmpDir, "workspace")
	secrets := filepath.Join(tmpDir, "secrets")
	os.MkdirAll(allowed, 0755)
	os.MkdirAll(secrets, 0755)
	os.WriteFile(filepath.Join(secrets, "key.pem"), []byte("secret"), 0644)

	symlink := filepath.Join(allowed, "escape")
	if err := os.Symlink(secrets, symlink); err != nil {
		t.Skip("symlinks not supported")
	}

	api := trackPluginFilesystemAPI(t, NewPluginFilesystemAPI(allowed))

	_, err := api.ReadFile(filepath.Join(symlink, "key.pem"))
	if err == nil {
		t.Fatal("expected symlink escape to be blocked")
	}
}

func TestPluginFilesystemAPI_FinalSymlinkEscape(t *testing.T) {
	tmpDir := t.TempDir()
	allowed := filepath.Join(tmpDir, "workspace")
	secrets := filepath.Join(tmpDir, "secrets")
	os.MkdirAll(allowed, 0755)
	os.MkdirAll(secrets, 0755)
	secret := filepath.Join(secrets, "key.pem")
	os.WriteFile(secret, []byte("secret"), 0644)
	os.WriteFile(filepath.Join(allowed, "notes.txt"), []byte("notes"), 0644)

	link := filepath.Join(allowed, "key.pem")
	if err := os.Symlink(secret, link); err != nil {
		t.Skip("symlinks not supported")
	}
	dirLink := filepath.Join(allowed, "secrets-link")
	if err := os.Symlink(secrets, dirLink); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(allowed, "new.pem")
	planted := filepath.Join(secrets, "planted.pem")
	if err := os.Symlink(planted, dangling); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(allowed, "notes-link.txt")
	if err := os.Symlink(filepath.Join(allowed, "notes.txt"), inside); err != nil {
		t.Fatal(err)
	}

	api := trackPluginFilesystemAPI(t, NewPluginFilesystemAPI(allowed))

	if _, err := api.ReadFile(link); err == nil {
		t.Error("read through a final symlink to an outside file was allowed")
	}
	if err := api.WriteFile(link, "overwritten"); err == nil {
		t.Error("write through a final symlink to an outside file was allowed")
	}
	if api.FileExists(link) {
		t.Error("exists accepted a final symlink to an outside file")
	}
	if _, err := api.ListDir(dirLink); err == nil {
		t.Error("list through a final symlink to an outside directory was allowed")
	}
	if data, _ := os.ReadFile(secret); string(data) != "secret" {
		t.Errorf("outside file was modified: %q", data)
	}
	if err := api.WriteFile(dangling, "planted"); err == nil {
		t.Error("write through a dangling final symlink was allowed")
	}
	if _, err := os.Stat(planted); err == nil {
		t.Error("write through a dangling symlink created a file outside the grant")
	}
	if got, err := api.ReadFile(inside); err != nil || got != "notes" {
		t.Errorf("read through a symlink inside the grant = %q, %v", got, err)
	}
	if err := api.WriteFile(filepath.Join(allowed, "fresh.txt"), "new"); err != nil {
		t.Errorf("creating a new file inside the grant failed: %v", err)
	}
}

func TestPluginNetworkAPI_OnlyHTTPAndHTTPS(t *testing.T) {
	api := NewPluginNetworkAPI()
	for _, tt := range []struct {
		url     string
		blocked bool
	}{
		{"https://example.com/api", false},
		{"http://example.com/api", false},
		{"http://127.0.0.1:4250/", false},
		{"http://localhost/", false},
		{"http://192.168.1.50:8080/", false},
		{"file:///etc/passwd", true},
		{"ftp://example.com/file", true},
	} {
		if err := api.validateURL(tt.url); (err != nil) != tt.blocked {
			t.Errorf("%s: err = %v, want blocked %v", tt.url, err, tt.blocked)
		}
	}
}

func TestPluginSystemAPI_ArgumentInjection(t *testing.T) {
	api := NewPluginSystemAPI()

	tests := []struct {
		name    string
		binary  string
		args    []string
		blocked bool
	}{
		{"safe git args", "git", []string{"status"}, false},
		{"safe git log", "git", []string{"log", "--oneline", "-5"}, false},
		{"git fsmonitor injection", "git", []string{"-c", "core.fsmonitor=!malicious"}, true},
		{"git sshCommand injection", "git", []string{"-c", "core.sshCommand=evil"}, true},
		{"git pager injection", "git", []string{"-c", "core.pager=!cmd"}, true},
		{"git upload-pack", "git", []string{"clone", "--upload-pack=evil"}, true},
		{"git receive-pack", "git", []string{"push", "--receive-pack=evil"}, true},
		{"git =! in any argument", "git", []string{"-calias.x=!evil", "x"}, true},
		{"git config-env", "git", []string{"--config-env=alias.x=FOO", "x"}, true},
		{"git by absolute path", "/usr/bin/git", []string{"-c", "alias.x=!evil"}, true},
		{"git with exe suffix", "git.exe", []string{"-c", "alias.x=!evil"}, true},
		{"git in another case", "/usr/bin/Git", []string{"-c", "core.pager=!cmd"}, true},
		{"=! for a non-git binary", "/usr/bin/some-tool", []string{"--config=!evil"}, false},
		{"psql connection string", "psql", []string{"postgresql://u:pa=!ss@host/db"}, false},
		{"sqlite3 with =! in sql", "sqlite3", []string{"db.sqlite", "select 'a=!b'"}, false},
		{"git-like name is not git", "legit", []string{"-c", "alias.x=!evil"}, false},
		{"safe non-git args", "docker", []string{"ps", "-a"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := api.validateArgs(tt.binary, tt.args)
			if tt.blocked && err == nil {
				t.Error("expected argument to be blocked")
			}
			if !tt.blocked && err != nil {
				t.Errorf("expected argument to be allowed, got: %v", err)
			}
		})
	}
}

// TestPluginSystemAPI_ExecStdin exercises the real subprocess path. The
// Lua-level tests in internal/plugin use a mock SystemAPI, so this is the only
// coverage that stdin actually reaches the child process.
func TestPluginSystemAPI_ExecStdin(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat not available")
	}
	api := NewPluginSystemAPI()
	execCount := 0
	api.onExec = func() { execCount++ }

	stdout, _, code, err := api.Exec(context.Background(), "cat", nil, plugin.ExecOptions{Stdin: "hello from stdin\n"})
	if err != nil || code != 0 {
		t.Fatalf("exec cat: err=%v code=%d", err, code)
	}
	if stdout != "hello from stdin\n" {
		t.Errorf("stdin not piped to child, got %q", stdout)
	}

	// With no stdin the child must see EOF immediately. If Stdin were left
	// attached to the terminal instead, this would block forever.
	stdout, _, code, err = api.Exec(context.Background(), "cat", nil, plugin.ExecOptions{})
	if err != nil || code != 0 {
		t.Fatalf("exec cat with no stdin: err=%v code=%d", err, code)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if execCount != 2 {
		t.Fatalf("system command completions invalidated repository %d times, want 2", execCount)
	}
}

func TestPluginSystemAPI_ExecEnv(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	t.Setenv("TTT_EXEC_INHERITED", "inherited")
	api := NewPluginSystemAPI()
	stdout, _, code, err := api.Exec(context.Background(), "sh", []string{"-c", `printf '%s %s' "$TTT_EXEC_INHERITED" "$TTT_EXEC_EXTRA"`},
		plugin.ExecOptions{Env: []string{"TTT_EXEC_EXTRA=extra"}})
	if err != nil || code != 0 {
		t.Fatalf("exec sh: err=%v code=%d", err, code)
	}
	if stdout != "inherited extra" {
		t.Errorf("child env = %q, want editor env plus extra", stdout)
	}
	if os.Getenv("TTT_EXEC_EXTRA") != "" {
		t.Error("extra env leaked into the editor's environment")
	}
}

func TestPluginSystemAPI_ExecKilledOnContextDone(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
	api := NewPluginSystemAPI()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, code, _ := api.Exec(ctx, "sleep", []string{"10"}, plugin.ExecOptions{})
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("child not killed when context ended, took %s", elapsed)
	}
	if code == 0 {
		t.Error("killed child reported exit code 0")
	}
}

// TestPluginSystemAPI_Spawn exercises the real subprocess-stdio path: writes
// reach the child's stdin, close_stdin gives it EOF, and output is readable
// until Wait reaps it.
func TestPluginSystemAPI_Spawn(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat not available")
	}
	api := NewPluginSystemAPI()

	proc, err := api.Spawn("cat", nil, plugin.SpawnOptions{})
	if err != nil {
		t.Fatalf("spawn cat: %v", err)
	}
	if proc.PID() <= 0 {
		t.Fatal("expected a positive pid")
	}

	if _, err := proc.Write([]byte("hello spawn\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := proc.CloseStdin(); err != nil {
		t.Fatalf("close stdin: %v", err)
	}

	out, err := io.ReadAll(proc.Stdout())
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if string(out) != "hello spawn\n" {
		t.Errorf("expected echo, got %q", out)
	}
	if _, err := io.ReadAll(proc.Stderr()); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	code, err := proc.Wait()
	if err != nil || code != 0 {
		t.Fatalf("wait: code=%d err=%v", code, err)
	}
}

func TestPluginSystemAPI_SpawnBlockedArgs(t *testing.T) {
	api := NewPluginSystemAPI()
	if _, err := api.Spawn("git", []string{"-c", "core.fsmonitor=!sh"}, plugin.SpawnOptions{}); err == nil {
		t.Fatal("expected dangerous git args to be rejected")
	}
}

func TestPluginSystemExecCompletionPostsMainThreadRepositoryRequest(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	screen := term.NewTcellScreenFrom(sim)
	repository := NewRepositoryState(nil, []string{"/repo"})
	repository.poster = newTestEventPoster()
	a := &App{Screen: screen, Repository: repository}
	api := NewPluginSystemAPI()
	api.onExec = func() {
		a.postRepositoryInvalidation("", RepositoryWorktree|RepositoryHistory)
	}

	done := make(chan error, 1)
	go func() {
		_, _, _, err := api.Exec(context.Background(), "true", nil, plugin.ExecOptions{})
		done <- err
	}()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if repository.dirty != 0 {
		t.Fatal("system.exec completion mutated coordinator state off the event loop")
	}
	event := screen.PollEvent()
	interrupt, ok := event.(*tcell.EventInterrupt)
	if !ok {
		t.Fatalf("completion event = %T, want interrupt", event)
	}
	request, ok := interrupt.Data().(*RepositoryInvalidationRequest)
	if !ok {
		t.Fatalf("interrupt data = %T, want repository invalidation", interrupt.Data())
	}
	a.handleRepositoryInvalidation(request)
	if repository.dirty != RepositoryWorktree|RepositoryHistory {
		t.Fatalf("main-thread invalidation resources = %b", repository.dirty)
	}
}

func TestPluginFilesystemAPI_SymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(real, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(real, "file.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(real, "escape")); err != nil {
		t.Fatal(err)
	}

	fs := trackPluginFilesystemAPI(t, NewPluginFilesystemAPI(link))
	if got, err := fs.ReadFile(filepath.Join(link, "file.txt")); err != nil || got != "hi" {
		t.Errorf("read through the symlinked root = %q, %v", got, err)
	}
	if err := fs.WriteFile(filepath.Join(link, "sub", "new.txt"), "x"); err != nil {
		t.Errorf("write through the symlinked root: %v", err)
	}
	if err := fs.WriteFile(filepath.Join(link, "escape", "x.txt"), "x"); err == nil {
		t.Error("a symlink inside the root that points outside it was allowed")
	}
}

func TestPluginFilesystemAPIFor_ScopedPaths(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	cfg := filepath.Join(base, "cfg")
	workspace := filepath.Join(base, "ws")
	pluginDir := filepath.Join(base, "plugin")
	palette := filepath.Join(home, ".local", "state", "omarchy")
	for _, d := range []string{palette, filepath.Join(cfg, "themes"), workspace, pluginDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(palette, "colors.toml"), []byte(`background = "#000000"`), 0o644)
	os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main"), 0o644)
	t.Setenv("HOME", home)
	config.OverrideConfigDir = cfg
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	p := &plugin.Plugin{Dir: pluginDir, Granted: plugin.PermissionSet{
		FsRead:  plugin.FsAccess{Paths: []string{"~/.local/state/omarchy"}},
		FsWrite: plugin.FsAccess{Paths: []string{"{config}/themes"}},
	}}
	fs := trackPluginFilesystemAPI(t, NewPluginFilesystemAPIFor(p, []string{workspace}))

	if got, err := fs.ReadFile("~/.local/state/omarchy/colors.toml"); err != nil || got != `background = "#000000"` {
		t.Errorf("read the granted path = %q, %v", got, err)
	}
	if _, err := fs.ReadFile(filepath.Join(workspace, "main.go")); err == nil {
		t.Error("the workspace was readable without {workspace} in fs.read")
	}
	if err := fs.WriteFile("{config}/themes/omarchy.json", "{}"); err != nil {
		t.Errorf("write the granted path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg, "themes", "omarchy.json")); err != nil {
		t.Errorf("{config} did not expand to the config dir: %v", err)
	}
	if err := fs.WriteFile(filepath.Join(palette, "colors.toml"), "x"); err == nil {
		t.Error("a read-only path was writable")
	}
	if _, err := fs.ReadFile(filepath.Join(cfg, "themes", "omarchy.json")); err == nil {
		t.Error("a write-only path was readable")
	}
	if err := fs.WriteFile(filepath.Join(pluginDir, "cache.json"), "{}"); err != nil {
		t.Errorf("the plugin dir must stay writable: %v", err)
	}
}

func TestPluginFilesystemAPIFor_AnyPathAndWorkspace(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "ws")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{workspace, outside} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "f.txt"), []byte("x"), 0o644)
	}

	p := &plugin.Plugin{Granted: plugin.PermissionSet{
		FsRead:  plugin.FsAccess{Paths: []string{plugin.FsAnyPath}},
		FsWrite: plugin.FsAccess{Paths: []string{plugin.FsWorkspace}},
	}}
	fs := trackPluginFilesystemAPI(t, NewPluginFilesystemAPIFor(p, []string{workspace}))

	if _, err := fs.ReadFile(filepath.Join(outside, "f.txt")); err != nil {
		t.Errorf("* must allow reading any path: %v", err)
	}
	if err := fs.WriteFile(filepath.Join(workspace, "f.txt"), "y"); err != nil {
		t.Errorf("{workspace} must allow writing the workspace: %v", err)
	}
	if err := fs.WriteFile(filepath.Join(outside, "f.txt"), "y"); err == nil {
		t.Error("writing outside the workspace was allowed")
	}
}

// redirectingNetworkAPI serves every name from start, so a redirect test
// needs no real network; 127.0.0.1 addresses are dialled as they are.
func redirectingNetworkAPI(t *testing.T, start *httptest.Server, grant plugin.NetworkHTTP) *PluginNetworkAPI {
	t.Helper()
	ps := plugin.PermissionSet{NetworkHTTP: grant}
	api := NewPluginNetworkAPI().ForPlugin(ps.CheckURL).(*PluginNetworkAPI)
	startAddr := strings.TrimPrefix(start.URL, "http://")
	api.client.Transport = &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		if h, _, _ := net.SplitHostPort(addr); h != "127.0.0.1" {
			addr = startAddr
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}}
	return api
}

// localRedirect returns a local server and a public.example server that
// redirects every request to it; reached reports whether the local one was hit.
func localRedirect(t *testing.T) (local, public *httptest.Server, reached *bool) {
	t.Helper()
	reached = new(bool)
	local = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.Write([]byte("local"))
	}))
	t.Cleanup(local.Close)
	public = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, local.URL+"/admin", http.StatusFound)
	}))
	t.Cleanup(public.Close)
	return local, public, reached
}

func TestPluginNetworkAPI_RedirectToALocalServerItDoesNotListIsBlocked(t *testing.T) {
	_, public, reached := localRedirect(t)
	// 127.0.0.1 is listed, but not on the local server's port.
	api := redirectingNetworkAPI(t, public, plugin.NetworkHTTP{Hosts: []string{"public.example", "127.0.0.1:1"}})

	if _, _, _, err := api.Get("http://public.example/", nil); err == nil {
		t.Error("Get followed a redirect to an unlisted local server")
	}
	if _, _, _, err := api.Post("http://public.example/", nil, "x"); err == nil {
		t.Error("Post followed a redirect to an unlisted local server")
	}
	if *reached {
		t.Error("the local server received a request")
	}
}

func TestPluginNetworkAPI_RedirectToAListedLocalServerIsFollowed(t *testing.T) {
	local, public, reached := localRedirect(t)
	api := redirectingNetworkAPI(t, public, plugin.NetworkHTTP{Hosts: []string{"public.example", strings.TrimPrefix(local.URL, "http://")}})

	status, body, _, err := api.Get("http://public.example/", nil)
	if err != nil || status != 200 || body != "local" || !*reached {
		t.Errorf("status %d, body %q, err %v, reached %v", status, body, err, *reached)
	}
}

func TestPluginNetworkAPI_AnyDestinationGrantReachesLocalServers(t *testing.T) {
	_, public, reached := localRedirect(t)
	api := redirectingNetworkAPI(t, public, plugin.NetworkHTTP{All: true})

	if _, _, _, err := api.Get("http://public.example/", nil); err != nil || !*reached {
		t.Errorf("err %v, reached %v", err, *reached)
	}
}

func TestPluginNetworkAPI_RedirectsStayOnTheAllowedHosts(t *testing.T) {
	var paths []string
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Host+r.URL.Path)
		switch r.URL.Path {
		case "/elsewhere":
			http.Redirect(w, r, "http://other.example/steal", http.StatusFound)
		case "/moved":
			http.Redirect(w, r, "http://mirror.example/new", http.StatusFound)
		default:
			w.Write([]byte("ok"))
		}
	}))
	defer public.Close()
	api := redirectingNetworkAPI(t, public, plugin.NetworkHTTP{Hosts: []string{"public.example", "mirror.example"}})

	if _, _, _, err := api.Get("http://public.example/elsewhere", nil); err == nil {
		t.Error("followed a redirect to a host the plugin does not list")
	}
	status, body, _, err := api.Get("http://public.example/moved", nil)
	if err != nil || status != 200 || body != "ok" {
		t.Errorf("redirect to a listed host: status %d, body %q, err %v", status, body, err)
	}
	want := []string{"public.example/elsewhere", "public.example/moved", "mirror.example/new"}
	if !slices.Equal(paths, want) {
		t.Errorf("requests = %v, want %v", paths, want)
	}
}

func TestPluginFilesystemAPIFor_AnyPathWritesThroughDanglingSymlink(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "created.txt")
	link := filepath.Join(workspace, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	p := &plugin.Plugin{Granted: plugin.PermissionSet{
		FsRead:  plugin.FsAccess{Paths: []string{plugin.FsAnyPath}},
		FsWrite: plugin.FsAccess{Paths: []string{plugin.FsAnyPath}},
	}}
	fs := trackPluginFilesystemAPI(t, NewPluginFilesystemAPIFor(p, []string{workspace}))
	if _, err := fs.ReadFile(link); err == nil {
		t.Error("read through wildcard dangling symlink succeeded before its target existed")
	}
	if err := fs.WriteFile(link, "created"); err != nil {
		t.Fatalf("write through wildcard dangling symlink: %v", err)
	}
	if got, err := fs.ReadFile(link); err != nil || got != "created" {
		t.Errorf("read through wildcard symlink = %q, %v; want %q", got, err, "created")
	}
}

func TestOpenGrantedRootRejectsReplacementAfterValidation(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	moved := filepath.Join(base, "moved-root")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	canonical, original, err := canonicalGrantedRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, root); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}

	handle, err := openGrantedRoot(root, canonical, original)
	if handle != nil {
		_ = handle.Close()
	}
	if err == nil {
		t.Fatal("opened replaced grant root")
	}
}

func TestOpenGrantedRootRejectsReplacedComponent(t *testing.T) {
	base := t.TempDir()
	parent, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	if err := os.Mkdir(filepath.Join(base, "component"), 0o755); err != nil {
		t.Fatal(err)
	}
	observed, err := parent.Lstat("component")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(base, "component"), filepath.Join(base, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(base, "component"), 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := openGrantedChildRoot(parent, "component", observed)
	if root != nil {
		_ = root.Close()
	}
	if err == nil {
		t.Fatal("opened replaced pending-grant component")
	}
}

func TestPluginFilesystemAPI_GrantedRootReplacement(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	moved := filepath.Join(base, "moved-root")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "value.txt"), []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "value.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := trackPluginFilesystemAPI(t, NewPluginFilesystemAPI(root))
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, root); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}

	got, err := api.ReadFile(filepath.Join(root, "value.txt"))
	if err != nil || got != "inside" {
		t.Fatalf("read through replaced grant root = %q, %v; want original contents", got, err)
	}
}

func TestPluginFilesystemAPI_SymlinkReplacementAfterResolution(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	inside := filepath.Join(root, "inside")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{inside, outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(inside, "value.txt"):         "inside",
		filepath.Join(inside, "inside-only.txt"):   "",
		filepath.Join(outside, "value.txt"):        "outside",
		filepath.Join(outside, "outside-only.txt"): "",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	link := filepath.Join(root, "switch")
	if err := os.Symlink("inside", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	expectedInfo, err := os.Stat(filepath.Join(inside, "value.txt"))
	if err != nil {
		t.Fatal(err)
	}
	api := trackPluginFilesystemAPI(t, NewPluginFilesystemAPI(root))
	path := filepath.Join(link, "value.txt")
	readPath, err := api.read.openRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	writePath, err := api.write.openRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	directoryPath, err := api.read.openRoot(link)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside", link); err != nil {
		t.Fatal(err)
	}
	if target, err := filepath.EvalSymlinks(path); err != nil || target != filepath.Join(outside, "value.txt") {
		t.Fatalf("replacement target = %q, %v; want outside sentinel", target, err)
	}

	data, err := readPath.root.ReadFile(readPath.relative)
	if err != nil || string(data) != "inside" {
		t.Fatalf("read after symlink replacement = %q, %v; want %q", data, err, "inside")
	}
	info, err := readPath.root.Stat(readPath.relative)
	if err != nil || !os.SameFile(info, expectedInfo) {
		t.Errorf("stat after symlink replacement = %v, %v; want original in-root file", info, err)
	}
	directory, err := directoryPath.root.Open(directoryPath.relative)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := directory.ReadDir(-1)
	closeErr := directory.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	listed := make(map[string]bool, len(entries))
	for _, entry := range entries {
		listed[entry.Name()] = true
	}
	if !listed["inside-only.txt"] || listed["outside-only.txt"] {
		t.Errorf("listing after symlink replacement = %v; want only in-root entries", listed)
	}
	if err := writePath.root.WriteFile(writePath.relative, []byte("updated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(inside, "value.txt")); err != nil || string(got) != "updated" {
		t.Errorf("write after symlink replacement = %q, %v; want in-root update", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(outside, "value.txt")); err != nil || string(got) != "outside" {
		t.Errorf("outside sentinel after replacement = %q, %v; want unchanged", got, err)
	}
}
