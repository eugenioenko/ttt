package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/workspace"

	"github.com/gdamore/tcell/v3"
)

type testHarness struct {
	t        *testing.T
	app      *app.App
	screen   *term.SimScreen
	reg      *command.Registry
	renderer *render.Renderer
	running  bool
	dir      string
}

func newTestHarness(t *testing.T, w, h int) *testHarness {
	t.Helper()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "beta.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, "gamma.txt"), []byte("c"), 0644)
	os.WriteFile(filepath.Join(dir, "delta.txt"), []byte("d"), 0644)
	os.WriteFile(filepath.Join(dir, "epsilon.txt"), []byte("e"), 0644)
	os.WriteFile(filepath.Join(dir, "zeta.txt"), []byte("f"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "nested.txt"), []byte("nested"), 0644)

	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0755)
	config.OverrideConfigDir = configDir
	t.Cleanup(func() { config.OverrideConfigDir = "" })

	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(w, h)

	cfg := config.AppConfig{
		Keybindings: config.DefaultKeybindings(),
		Settings:    config.DefaultSettings(),
		Theme:       config.DefaultTheme(),
	}
	config.ParseKeyBindings(cfg.Keybindings)

	screen := term.NewTcellScreenFrom(sim)
	screen.SetStyleMap(app.BuildStyleMap(cfg.Theme))

	borders := app.BuildBorderSet(cfg.Theme.Borders)

	ws := workspace.New([]string{dir})
	editor := app.BuildAppFromConfig(&cfg, &borders, ws, nil)
	editor.Screen = screen
	editor.Renderer = &render.Renderer{}

	reg := command.NewRegistry()
	editor.Reg = reg
	running := true
	editor.Running = &running
	app.RegisterCommands(editor)
	app.BindKeys(editor.Root, reg, cfg.Keybindings)

	editor.Root.SetSize(w, h)

	h2 := &testHarness{
		t:        t,
		app:      editor,
		screen:   sim,
		reg:      reg,
		renderer: editor.Renderer,
		running:  running,
		dir:      dir,
	}
	h2.redraw()
	return h2
}

func (h *testHarness) redraw() {
	h.t.Helper()
	cells := make([][]term.Cell, h.app.Root.Height)
	for y := range cells {
		cells[y] = make([]term.Cell, h.app.Root.Width)
	}
	h.app.Root.Render(cells)
	h.renderer.SetCurrent(cells)
	h.renderer.Render(h.app.Screen)
}

func (h *testHarness) flushOnChange() {
	if h.app.EditorGroup.Editor != nil {
		h.app.EditorGroup.Editor.FlushOnChange()
	}
}

func (h *testHarness) pressKey(key tcell.Key, mod tcell.ModMask) {
	h.t.Helper()
	ev := tcell.NewEventKey(key, "", mod)
	h.app.Root.HandleEvent(ev)
	h.flushOnChange()
	h.redraw()
}

func (h *testHarness) pressRune(r rune) {
	h.t.Helper()
	ev := tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone)
	h.app.Root.HandleEvent(ev)
	h.flushOnChange()
	h.redraw()
}

func (h *testHarness) pressCtrl(key tcell.Key) {
	h.t.Helper()
	h.pressKey(key, tcell.ModCtrl)
}

func (h *testHarness) click(x, y int) {
	h.t.Helper()
	down := tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone)
	h.app.Root.HandleEvent(down)
	up := tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone)
	h.app.Root.HandleEvent(up)
	h.flushOnChange()
	h.redraw()
}

func (h *testHarness) rightClick(x, y int) {
	h.t.Helper()
	down := tcell.NewEventMouse(x, y, tcell.Button2, tcell.ModNone)
	h.app.Root.HandleEvent(down)
	up := tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone)
	h.app.Root.HandleEvent(up)
	h.flushOnChange()
	h.redraw()
}

func (h *testHarness) exec(cmdID string) {
	h.t.Helper()
	h.reg.Execute(cmdID)
	h.flushOnChange()
	h.redraw()
}

func (h *testHarness) screenText() string {
	cells, w, ht := h.screen.GetContents()
	var lines []string
	for y := 0; y < ht; y++ {
		lines = append(lines, rowString(cells[y*w:(y+1)*w]))
	}
	return strings.Join(lines, "\n")
}

func (h *testHarness) screenRow(y int) string {
	cells, w, _ := h.screen.GetContents()
	return rowString(cells[y*w : (y+1)*w])
}

// rowString reassembles a row the way the terminal displays it. A wide rune
// already accounts for the column it covers, so the continuation cell that
// follows it contributes nothing; every other empty cell is a blank column.
func rowString(cells []term.SimCell) string {
	var line strings.Builder
	for x := 0; x < len(cells); x++ {
		sc := cells[x]
		if sc.Str == "" {
			line.WriteRune(' ')
			continue
		}
		line.WriteString(sc.Str)
		if textwidth.String(sc.Str) > 1 {
			x++
		}
	}
	return line.String()
}

func (h *testHarness) containsText(text string) bool {
	return strings.Contains(h.screenText(), text)
}

func (h *testHarness) assertContains(text string) {
	h.t.Helper()
	if !h.containsText(text) {
		h.t.Errorf("expected screen to contain %q, got:\n%s", text, h.screenText())
	}
}

func (h *testHarness) assertNotContains(text string) {
	h.t.Helper()
	if h.containsText(text) {
		h.t.Errorf("expected screen NOT to contain %q, got:\n%s", text, h.screenText())
	}
}

func (h *testHarness) stop() {
	h.screen.Fini()
}

type emptyWidget struct{ ui.BaseWidget }

func newEmptyWidget() *emptyWidget                               { return &emptyWidget{} }
func (e *emptyWidget) Focusable() bool                           { return false }
func (e *emptyWidget) Render(surface ui.Surface)                 {}
func (e *emptyWidget) HandleEvent(ev tcell.Event) ui.EventResult { return ui.EventIgnored }
