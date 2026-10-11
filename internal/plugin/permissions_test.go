package plugin

import (
	"encoding/json"
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
	requested := PermissionSet{PanelSidebar: true, Commands: true, FsRead: true}

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
	granted := PermissionSet{PanelSidebar: true, Commands: true, FsRead: true}
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

func TestCheckHost(t *testing.T) {
	ps := PermissionSet{NetworkHTTP: NetworkHTTP{Hosts: []string{"api.github.com"}}}
	if err := ps.CheckHost("api.github.com"); err != nil {
		t.Errorf("allowed host rejected: %v", err)
	}
	if err := ps.CheckHost("evil.com"); err == nil {
		t.Error("expected denial for unlisted host")
	}

	all := PermissionSet{NetworkHTTP: NetworkHTTP{All: true}}
	if err := all.CheckHost("evil.com"); err != nil {
		t.Errorf("All should permit any host: %v", err)
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
