package app

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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

func TestPluginFilesystemAPI_PathRestriction(t *testing.T) {
	tmpDir := t.TempDir()
	allowed := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(allowed, 0755)
	os.WriteFile(filepath.Join(allowed, "test.txt"), []byte("hello"), 0644)

	forbidden := filepath.Join(tmpDir, "secrets")
	os.MkdirAll(forbidden, 0755)
	os.WriteFile(filepath.Join(forbidden, "key.pem"), []byte("secret"), 0644)

	api := NewPluginFilesystemAPI(allowed)

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

func TestPluginFilesystemAPI_MultipleRoots(t *testing.T) {
	tmpDir := t.TempDir()
	root1 := filepath.Join(tmpDir, "project")
	root2 := filepath.Join(tmpDir, "plugin")
	outside := filepath.Join(tmpDir, "outside")

	for _, d := range []string{root1, root2, outside} {
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, "file.txt"), []byte("data"), 0644)
	}

	api := NewPluginFilesystemAPI(root1, root2)

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

	api := NewPluginFilesystemAPI(allowed)

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
	dangling := filepath.Join(allowed, "new.pem")
	planted := filepath.Join(secrets, "planted.pem")
	if err := os.Symlink(planted, dangling); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(allowed, "notes-link.txt")
	if err := os.Symlink(filepath.Join(allowed, "notes.txt"), inside); err != nil {
		t.Fatal(err)
	}

	api := NewPluginFilesystemAPI(allowed)

	if _, err := api.ReadFile(link); err == nil {
		t.Error("read through a final symlink to an outside file was allowed")
	}
	if err := api.WriteFile(link, "overwritten"); err == nil {
		t.Error("write through a final symlink to an outside file was allowed")
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
		{"general =! injection", "/usr/bin/some-tool", []string{"--config=!evil"}, true},
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

	stdout, _, code, err := api.Exec("cat", nil, "hello from stdin\n")
	if err != nil || code != 0 {
		t.Fatalf("exec cat: err=%v code=%d", err, code)
	}
	if stdout != "hello from stdin\n" {
		t.Errorf("stdin not piped to child, got %q", stdout)
	}

	// With no stdin the child must see EOF immediately. If Stdin were left
	// attached to the terminal instead, this would block forever.
	stdout, _, code, err = api.Exec("cat", nil, "")
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
		_, _, _, err := api.Exec("true", nil, "")
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

	fs := NewPluginFilesystemAPI(link)
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
	fs := NewPluginFilesystemAPIFor(p, []string{workspace})

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
	fs := NewPluginFilesystemAPIFor(p, []string{workspace})

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
