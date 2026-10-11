package app

import (
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
	lua "github.com/yuin/gopher-lua"
)

func TestCloseFileHandlerTabDispatchesFileClose(t *testing.T) {
	dir := t.TempDir()
	m := plugin.NewManager(filepath.Join(dir, "plugins"), filepath.Join(dir, "plugins.ttt.json"))
	p := &plugin.Plugin{Name: "listener", Enabled: true, Granted: plugin.PermissionSet{EventsFile: true}}
	if err := p.InitFromSource(`
		local events = require("ttt.events")
		closed = {}
		events.on("file.close", function(path) table.insert(closed, path) end)
	`); err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	m.RegisterDebugPlugin(p)

	a := &App{PluginManager: m, fileHandlerTabs: map[string]*plugin.Plugin{"/x/paper.pdf": p}}
	a.closeFileHandlerTab("Settings")
	a.closeFileHandlerTab("/x/paper.pdf")
	a.closeFileHandlerTab("/x/paper.pdf")

	closed := p.State.GetGlobal("closed")
	if n := p.State.ObjLen(closed); n != 1 {
		t.Fatalf("file.close fired %d times, want once for the handled tab", n)
	}
	if got := p.State.GetTable(closed, lua.LNumber(1)).String(); got != "/x/paper.pdf" {
		t.Errorf("file.close path = %q", got)
	}
}
