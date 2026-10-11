package app

import (
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	tttimage "github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
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

func TestPluginGraphicsNeedsACellSize(t *testing.T) {
	a := &App{EditorGroup: ui.NewEditorGroupWidget(nil, 4, true, "relative")}
	a.EditorGroup.ImageProtocol = config.ImageProtocolKitty
	if g := a.pluginGraphics(); g.Protocol != tttimage.None {
		t.Errorf("protocol without a cell size = %v, want none", g.Protocol)
	}
	a.imageCellW, a.imageCellH = 10, 20
	if g := a.pluginGraphics(); g.Protocol != tttimage.Kitty || g.CellW != 10 || g.CellH != 20 {
		t.Errorf("graphics = %+v, want kitty with the cell size", g)
	}
}
