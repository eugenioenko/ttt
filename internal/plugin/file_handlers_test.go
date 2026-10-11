package plugin

import (
	"strings"
	"testing"
)

func registerHandlers(t *testing.T, perms PermissionSet, src string) (*Plugin, func(), error) {
	t.Helper()
	p, cleanup := newTestPluginBase(perms)
	p.Enabled = true
	err := p.State.DoString(`local ttt = require("ttt")` + "\n" + src)
	return p, cleanup, err
}

func TestFileHandlerRegistration(t *testing.T) {
	p, cleanup, err := registerHandlers(t, PermissionSet{FileHandlers: true}, `
		ttt.register({ file_handlers = {
			{ id = "pdf", title = "PDF Viewer", extensions = { ".PDF", "pdf" }, magic = "%PDF-", open = function() end },
		} })
	`)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.FileHandlers) != 1 {
		t.Fatalf("handlers = %d, want 1", len(p.FileHandlers))
	}
	h := p.FileHandlers[0]
	if h.ID != "pdf" || h.Title != "PDF Viewer" || string(h.Magic) != "%PDF-" {
		t.Errorf("handler = %+v", h)
	}
	if strings.Join(h.Extensions, ",") != "pdf,pdf" {
		t.Errorf("extensions = %v, want normalized lowercase without dots", h.Extensions)
	}
}

func TestFileHandlerRegistrationErrors(t *testing.T) {
	cases := map[string]struct {
		perms PermissionSet
		src   string
		want  string
	}{
		"no permission": {PermissionSet{}, `ttt.register({ file_handlers = { { id = "pdf", extensions = { "pdf" }, open = function() end } } })`, "editor.file_handlers"},
		"no id":         {PermissionSet{FileHandlers: true}, `ttt.register({ file_handlers = { { extensions = { "pdf" }, open = function() end } } })`, "string id"},
		"no open":       {PermissionSet{FileHandlers: true}, `ttt.register({ file_handlers = { { id = "pdf", extensions = { "pdf" } } } })`, `"pdf" requires an open function`},
		"no matcher":    {PermissionSet{FileHandlers: true}, `ttt.register({ file_handlers = { { id = "pdf", open = function() end } } })`, `"pdf" requires extensions or magic`},
		"long magic":    {PermissionSet{FileHandlers: true}, `ttt.register({ file_handlers = { { id = "pdf", magic = string.rep("x", 513), open = function() end } } })`, "exceeds 512 bytes"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, cleanup, err := registerHandlers(t, tc.perms, tc.src)
			defer cleanup()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
			if len(p.FileHandlers) != 0 {
				t.Errorf("registered %d handlers despite the error", len(p.FileHandlers))
			}
		})
	}
}

func handlerPlugin(name string, handlers ...FileHandler) *Plugin {
	return &Plugin{Name: name, Enabled: true, State: NewSandbox(), FileHandlers: handlers}
}

func TestMatchFileHandlerOrder(t *testing.T) {
	byExt := handlerPlugin("a", FileHandler{ID: "bin", Extensions: []string{"bin", "pdf"}})
	byMagic := handlerPlugin("b", FileHandler{ID: "pdf", Magic: []byte("%PDF-")})
	m := &Manager{plugins: []*Plugin{byExt, byMagic}}
	defer byExt.State.Close()
	defer byMagic.State.Close()

	cases := []struct {
		path, head, wantPlugin, wantID string
	}{
		{"/x/file.bin", "%PDF-1.7", "b", "pdf"},
		{"/x/paper", "%PDF-1.4", "b", "pdf"},
		{"/x/FILE.BIN", "\x00\x01", "a", "bin"},
		{"/x/report.pdf", "\x00\x01", "a", "bin"},
		{"/x/noext", "\x00\x01", "", ""},
	}
	for _, tc := range cases {
		p, h := m.MatchFileHandler(tc.path, []byte(tc.head))
		gotPlugin, gotID := "", ""
		if p != nil {
			gotPlugin, gotID = p.Name, h.ID
		}
		if gotPlugin != tc.wantPlugin || gotID != tc.wantID {
			t.Errorf("%s: got %s/%s, want %s/%s", tc.path, gotPlugin, gotID, tc.wantPlugin, tc.wantID)
		}
	}
}

func TestMatchFileHandlerSkipsDisabledAndFirstWins(t *testing.T) {
	disabled := handlerPlugin("off", FileHandler{ID: "off", Extensions: []string{"pdf"}})
	disabled.Enabled = false
	first := handlerPlugin("first", FileHandler{ID: "one", Extensions: []string{"pdf"}}, FileHandler{ID: "two", Extensions: []string{"pdf"}})
	second := handlerPlugin("second", FileHandler{ID: "three", Extensions: []string{"pdf"}})
	m := &Manager{plugins: []*Plugin{disabled, first, second}}
	for _, p := range m.plugins {
		defer p.State.Close()
	}

	p, h := m.MatchFileHandler("/x/a.pdf", nil)
	if p != first || h.ID != "one" {
		t.Fatalf("got %v/%v, want first/one", p, h)
	}
}

func TestOpenFileHandler(t *testing.T) {
	p, cleanup, err := registerHandlers(t, PermissionSet{FileHandlers: true}, `
		opened = nil
		ttt.register({ file_handlers = {
			{ id = "ok", extensions = { "ok" }, open = function(path, tab)
				opened = path
				tab.render = function(panel) panel:label("hi") end
			end },
			{ id = "norender", extensions = { "nr" }, open = function() end },
			{ id = "boom", extensions = { "boom" }, open = function() error("pdfinfo not found") end },
		} })
	`)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}

	panel, err := p.OpenFileHandler(&p.FileHandlers[0], "/x/a.ok")
	if err != nil || panel == nil {
		t.Fatalf("open ok: panel=%v err=%v", panel, err)
	}
	if got := p.State.GetGlobal("opened").String(); got != "/x/a.ok" {
		t.Errorf("open received path %q", got)
	}

	if _, err := p.OpenFileHandler(&p.FileHandlers[1], "/x/a.nr"); err == nil {
		t.Error("expected an error when open does not set tab.render")
	}
	if _, err := p.OpenFileHandler(&p.FileHandlers[2], "/x/a.boom"); err == nil || !strings.Contains(err.Error(), "pdfinfo not found") {
		t.Errorf("err = %v, want the Lua error", err)
	}
}
