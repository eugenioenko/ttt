package plugin

import (
	"encoding/json"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestDiffPermissionsEmpty(t *testing.T) {
	a := PermissionSet{PanelSidebar: true, Commands: true}
	b := PermissionSet{PanelSidebar: true, Commands: true}

	diff := DiffPermissions(a, b)
	if !diff.IsEmpty() {
		t.Errorf("expected empty diff, got %d entries", len(diff.Entries))
	}
}

func TestDiffPermissionsNewBool(t *testing.T) {
	granted := PermissionSet{PanelSidebar: true}
	requested := PermissionSet{PanelSidebar: true, Commands: true, FsRead: FsAccess{Paths: []string{FsWorkspace}}}

	diff := DiffPermissions(granted, requested)
	if diff.IsEmpty() {
		t.Fatal("expected non-empty diff")
	}
	if len(diff.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(diff.Entries))
	}

	names := map[string]bool{}
	for _, e := range diff.Entries {
		names[e.Name] = true
	}
	if !names["commands"] {
		t.Error("missing diff entry for commands")
	}
	if !names["fs.read"] {
		t.Error("missing diff entry for fs.read")
	}
}

func TestDiffPermissionsNewExec(t *testing.T) {
	granted := PermissionSet{SystemExec: []string{"docker"}}
	requested := PermissionSet{SystemExec: []string{"docker", "kubectl"}}

	diff := DiffPermissions(granted, requested)
	if diff.IsEmpty() {
		t.Fatal("expected non-empty diff")
	}
	if len(diff.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(diff.Entries))
	}
	if diff.Entries[0].Value != "kubectl" {
		t.Errorf("expected kubectl, got %s", diff.Entries[0].Value)
	}
}

func TestDiffPermissionsSubset(t *testing.T) {
	granted := PermissionSet{PanelSidebar: true, Commands: true, FsRead: FsAccess{Paths: []string{FsWorkspace}}}
	requested := PermissionSet{PanelSidebar: true}

	diff := DiffPermissions(granted, requested)
	if !diff.IsEmpty() {
		t.Errorf("expected empty diff for subset, got %d entries", len(diff.Entries))
	}
}

func TestCheckPermission(t *testing.T) {
	ps := PermissionSet{PanelSidebar: true}

	if err := ps.Check("panel.sidebar"); err != nil {
		t.Errorf("expected allowed, got %v", err)
	}
	if err := ps.Check("commands"); err == nil {
		t.Error("expected denied for commands")
	}
	if err := ps.Check("clipboard.read"); err == nil {
		t.Error("expected denied for clipboard.read")
	}
	ps.ClipboardRead = true
	if err := ps.Check("clipboard.read"); err != nil {
		t.Errorf("expected allowed for clipboard.read, got %v", err)
	}
}

func TestDiffPermissionsClipboardRead(t *testing.T) {
	diff := DiffPermissions(PermissionSet{Commands: true}, PermissionSet{Commands: true, ClipboardRead: true})
	if len(diff.Entries) != 1 || diff.Entries[0].Name != "clipboard.read" {
		t.Fatalf("expected one clipboard.read entry, got %+v", diff.Entries)
	}
}

func TestCheckExec(t *testing.T) {
	ps := PermissionSet{SystemExec: []string{"docker", "git"}}

	if err := ps.CheckExec("docker"); err != nil {
		t.Errorf("expected allowed, got %v", err)
	}
	if err := ps.CheckExec("rm"); err == nil {
		t.Error("expected denied for rm")
	}
}

func TestDisplayEntries(t *testing.T) {
	ps := PermissionSet{
		PanelSidebar: true,
		Commands:     true,
		SystemExec:   []string{"docker"},
	}

	entries := ps.DisplayEntries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 display entries, got %d", len(entries))
	}

	ps.ClipboardRead = true
	if got := len(ps.DisplayEntries()); got != 4 {
		t.Fatalf("expected 4 display entries with clipboard.read, got %d", got)
	}
}

func TestNetworkHTTPUnmarshalBool(t *testing.T) {
	var ps PermissionSet
	if err := json.Unmarshal([]byte(`{"network.http": true}`), &ps); err != nil {
		t.Fatal(err)
	}
	if !ps.NetworkHTTP.All || !ps.NetworkHTTP.Enabled() {
		t.Errorf("expected All+Enabled, got %+v", ps.NetworkHTTP)
	}
	if !ps.NetworkHTTP.AllowsHost("anything.example.com") {
		t.Error("true should allow any host")
	}
}

func TestNetworkHTTPUnmarshalArray(t *testing.T) {
	var ps PermissionSet
	if err := json.Unmarshal([]byte(`{"network.http": ["api.github.com", "cheat.sh"]}`), &ps); err != nil {
		t.Fatal(err)
	}
	if ps.NetworkHTTP.All {
		t.Error("array form must not set All")
	}
	if !ps.NetworkHTTP.Enabled() {
		t.Error("non-empty host list should be Enabled")
	}
	if !ps.NetworkHTTP.AllowsHost("api.github.com") || !ps.NetworkHTTP.AllowsHost("CHEAT.SH") {
		t.Error("listed hosts should be allowed (case-insensitive)")
	}
	if ps.NetworkHTTP.AllowsHost("evil.com") {
		t.Error("unlisted host must be denied")
	}
}

func TestNetworkHTTPUnmarshalAbsentAndFalse(t *testing.T) {
	for _, src := range []string{`{}`, `{"network.http": false}`} {
		var ps PermissionSet
		if err := json.Unmarshal([]byte(src), &ps); err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if ps.NetworkHTTP.Enabled() {
			t.Errorf("%s: expected disabled", src)
		}
		if ps.NetworkHTTP.AllowsHost("x.com") {
			t.Errorf("%s: nothing should be allowed", src)
		}
	}
}

func TestNetworkHTTPMarshalRoundTrip(t *testing.T) {
	cases := map[string]NetworkHTTP{
		`true`:               {All: true},
		`false`:              {},
		`["api.github.com"]`: {Hosts: []string{"api.github.com"}},
		`["a.com","b.com"]`:  {Hosts: []string{"a.com", "b.com"}},
	}
	for want, n := range cases {
		got, err := json.Marshal(n)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("marshal %+v: got %s want %s", n, got, want)
		}
	}
}

func TestCheckURL(t *testing.T) {
	ps := PermissionSet{NetworkHTTP: NetworkHTTP{Hosts: []string{
		"api.github.com", "127.0.0.1:4250", "[::1]:4250", "192.168.1.50", "LAN.example:8080",
	}}}
	for _, tt := range []struct {
		url string
		ok  bool
	}{
		{"https://api.github.com/user", true},
		{"http://API.GITHUB.COM:8443/", true},
		{"http://127.0.0.1:4250/poll", true},
		{"http://[::1]:4250/", true},
		{"http://[0:0:0:0:0:0:0:1]:4250/", true},
		{"http://192.168.1.50:9000/", true},
		{"http://lan.example:8080/", true},
		{"http://127.0.0.1:4251/", false},
		{"http://127.0.0.1/", false},
		{"http://127.0.0.2:4250/", false},
		{"http://lan.example/", false},
		{"https://lan.example:443/", false},
		{"http://evil.com/", false},
		{"http://[fe80::1%25en0]:4250/", false},
	} {
		u, _ := url.Parse(tt.url)
		if err := ps.CheckURL(u); (err == nil) != tt.ok {
			t.Errorf("%s: err = %v, want ok %v", tt.url, err, tt.ok)
		}
	}
	u, _ := url.Parse("http://127.0.0.1:4251/")
	if err := ps.CheckURL(u); err == nil || !strings.Contains(err.Error(), `"127.0.0.1:4251"`) {
		t.Errorf("refusal %v does not name the destination", err)
	}

	// σ and ς fold together but are different domains once IDNA-encoded.
	sigma := PermissionSet{NetworkHTTP: NetworkHTTP{Hosts: []string{"σ.example", "127.0.0.1:080"}}}
	for raw, ok := range map[string]bool{"http://σ.example/": true, "http://ς.example/": false, "http://Σ.example/": false, "http://127.0.0.1/": true} {
		u, _ := url.Parse(raw)
		if err := sigma.CheckURL(u); (err == nil) != ok {
			t.Errorf("%s: err = %v, want ok %v", raw, err, ok)
		}
	}

	all := PermissionSet{NetworkHTTP: NetworkHTTP{All: true}}
	u, _ = url.Parse("http://127.0.0.1:5432/")
	if err := all.CheckURL(u); err != nil {
		t.Errorf("true should permit any destination: %v", err)
	}
}

func TestNetworkHTTPAllowsHostCoversEntries(t *testing.T) {
	granted := NetworkHTTP{Hosts: []string{"127.0.0.1", "api.example.com:443"}}
	for entry, want := range map[string]bool{
		"127.0.0.1:4250":      true,
		"127.0.0.1":           true,
		"api.example.com:443": true,
		"api.example.com":     false,
		"api.example.com:80":  false,
	} {
		if got := granted.AllowsHost(entry); got != want {
			t.Errorf("AllowsHost(%q) = %v, want %v", entry, got, want)
		}
	}
}

func TestIntersectKeepsTheApprovedPortsOfAHostListedWithoutOne(t *testing.T) {
	granted := PermissionSet{NetworkHTTP: NetworkHTTP{Hosts: []string{"api.example.com:443", "other.example:80"}}}
	declared := PermissionSet{NetworkHTTP: NetworkHTTP{Hosts: []string{"api.example.com"}}}
	got := IntersectPermissions(granted, declared).NetworkHTTP.Hosts
	if !slices.Equal(got, []string{"api.example.com:443"}) {
		t.Errorf("hosts = %v, want [api.example.com:443]", got)
	}
}

func TestDisplayEntriesSayAnyHostIncludesLocal(t *testing.T) {
	ps := PermissionSet{NetworkHTTP: NetworkHTTP{All: true}}
	want := []PermissionDiffEntry{{Name: "HTTP requests", Value: "any host, even local"}}
	if got := ps.DisplayEntries(); !slices.Equal(got, want) {
		t.Errorf("entries = %+v, want %+v", got, want)
	}
}

func TestDiffPermissionsNetworkHosts(t *testing.T) {
	granted := PermissionSet{NetworkHTTP: NetworkHTTP{Hosts: []string{"api.github.com"}}}
	requested := PermissionSet{NetworkHTTP: NetworkHTTP{Hosts: []string{"api.github.com", "evil.com"}}}
	diff := DiffPermissions(granted, requested)
	found := false
	for _, e := range diff.Entries {
		if e.Name == "network.http" && e.Value == "evil.com" {
			found = true
		}
		if e.Value == "api.github.com" {
			t.Error("already-granted host should not appear in diff")
		}
	}
	if !found {
		t.Error("newly requested host should require approval")
	}

	// Escalation from host list to all-hosts requires approval.
	esc := DiffPermissions(granted, PermissionSet{NetworkHTTP: NetworkHTTP{All: true}})
	if esc.IsEmpty() {
		t.Error("escalating to all-hosts should require approval")
	}
}

func TestFileHandlersPermission(t *testing.T) {
	var ps PermissionSet
	if err := json.Unmarshal([]byte(`{"editor.file_handlers": true}`), &ps); err != nil {
		t.Fatal(err)
	}
	if err := ps.Check("editor.file_handlers"); err != nil {
		t.Errorf("expected allowed, got %v", err)
	}
	if err := (PermissionSet{}).Check("editor.file_handlers"); err == nil {
		t.Error("expected denied without the permission")
	}

	diff := DiffPermissions(PermissionSet{}, ps)
	if len(diff.Entries) != 1 || diff.Entries[0].Name != "editor.file_handlers" {
		t.Fatalf("diff = %+v, want one editor.file_handlers entry", diff.Entries)
	}

	entries := ps.DisplayEntries()
	if len(entries) != 1 || entries[0].Name != "Handle file types" {
		t.Fatalf("display entries = %+v, want Handle file types", entries)
	}
}

func TestFsAccessUnmarshal(t *testing.T) {
	cases := map[string][]string{
		`{}`:                 nil,
		`{"fs.read": false}`: nil,
		`{"fs.read": true}`:  {FsWorkspace},
		`{"fs.read": ["~/.local/state/omarchy"]}`: {"~/.local/state/omarchy"},
		`{"fs.read": ["*"]}`:                      {FsAnyPath},
	}
	for src, want := range cases {
		var ps PermissionSet
		if err := json.Unmarshal([]byte(src), &ps); err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if !slices.Equal(ps.FsRead.Paths, want) {
			t.Errorf("%s: paths = %v, want %v", src, ps.FsRead.Paths, want)
		}
	}
}

func TestFsAccessRejectsRelativePaths(t *testing.T) {
	for _, src := range []string{`{"fs.write": ["themes"]}`, `{"fs.write": ["{home}/x"]}`, `{"fs.write": ["~x"]}`} {
		var ps PermissionSet
		if err := json.Unmarshal([]byte(src), &ps); err == nil {
			t.Errorf("%s: expected an error", src)
		}
	}
}

func TestFsAccessMarshalRoundTrip(t *testing.T) {
	cases := map[string]FsAccess{
		`true`:                      {Paths: []string{FsWorkspace}},
		`false`:                     {},
		`["{workspace}","~/notes"]`: {Paths: []string{FsWorkspace, "~/notes"}},
	}
	for want, access := range cases {
		data, err := json.Marshal(access)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Errorf("marshal %+v = %s, want %s", access, data, want)
		}
		var back FsAccess
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(back.Paths, access.Paths) {
			t.Errorf("round trip %s = %v, want %v", want, back.Paths, access.Paths)
		}
	}
}

func TestDiffPermissionsFsPaths(t *testing.T) {
	granted := PermissionSet{FsRead: FsAccess{Paths: []string{FsWorkspace}}}
	requested := PermissionSet{
		FsRead:  FsAccess{Paths: []string{FsWorkspace, "~/.local/state/omarchy"}},
		FsWrite: FsAccess{Paths: []string{"{config}/themes"}},
	}
	diff := DiffPermissions(granted, requested)
	want := []PermissionDiffEntry{
		{Name: "fs.read", Value: "~/.local/state/omarchy"},
		{Name: "fs.write", Value: "{config}/themes"},
	}
	if !slices.Equal(diff.Entries, want) {
		t.Errorf("diff = %v, want %v", diff.Entries, want)
	}

	diff = DiffPermissions(PermissionSet{FsRead: FsAccess{Paths: []string{FsAnyPath}}}, requested)
	if !slices.Equal(diff.Entries, want[1:]) {
		t.Errorf("* granted for fs.read: diff = %v, want only the fs.write path", diff.Entries)
	}
}

func TestFsAccessResolve(t *testing.T) {
	vars := FsVars{Workspace: []string{"/ws/a", "/ws/b"}, Config: "/cfg/ttt", Home: "/home/u"}

	dirs, all := FsAccess{Paths: []string{"~/.local/state/omarchy", "{config}/themes", "/opt/data"}}.Resolve(vars)
	if all {
		t.Error("explicit paths must not grant every path")
	}
	want := []string{"/home/u/.local/state/omarchy", "/cfg/ttt/themes", "/opt/data"}
	if !slices.Equal(dirs, want) {
		t.Errorf("dirs = %v, want %v", dirs, want)
	}

	if dirs, _ := (FsAccess{Paths: []string{FsWorkspace}}).Resolve(vars); !slices.Equal(dirs, vars.Workspace) {
		t.Errorf("{workspace} = %v, want %v", dirs, vars.Workspace)
	}
	if _, all := (FsAccess{Paths: []string{FsAnyPath}}).Resolve(vars); !all {
		t.Error("* must grant every path")
	}
}
