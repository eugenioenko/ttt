package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config/debugadapter"
	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

func TestDebugSaveFailurePreventsSession(t *testing.T) {
	for _, request := range []string{"launch", "attach"} {
		t.Run(request, func(t *testing.T) {
			h := newTestHarness(t, 120, 40)
			defer h.stop()
			h.app.Settings.Debug.Adapters["go"] = debugadapter.Config{Provider: "delve", Languages: []string{"go"}}
			h.app.EditorGroup.OnError = h.app.StatusError
			path := filepath.Join(h.dir, "alpha.txt")
			h.openFile(path)
			h.pressRune('x')
			requireNoError(t, os.Remove(path))
			requireNoError(t, os.Mkdir(path, 0755))
			requireNoError(t, os.Mkdir(filepath.Join(h.dir, ".ttt"), 0755))
			fields := `"request":"launch","program":"."`
			if request == "attach" {
				fields = `"request":"attach","mode":"remote","port":1`
			}
			requireNoError(t, os.WriteFile(filepath.Join(h.dir, ".ttt", "launch.json"), []byte(`{"configurations":[{"name":"test","type":"go",`+fields+`}]}`), 0600))
			h.exec("debug.start")
			if h.app.Root.HasModalOverlay() {
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
			}
			lines := h.app.Output.Lines
			if len(lines) == 0 || !strings.Contains(lines[len(lines)-1].Message, "Failed to save") {
				t.Fatalf("missing save error in Output: %v", lines)
			}
			if !h.app.EditorGroup.IsDirtyPath(path) || h.app.Sidebar.ActivePanel == "debug" {
				t.Fatal("failed save started debugger or discarded edits")
			}
		})
	}
}

func TestDebugBreakpointSharedAcrossSplits(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	path := filepath.Join(h.dir, "sample.go")
	requireNoError(t, os.WriteFile(path, []byte("package main"), 0600))
	h.app.Settings.Debug.Adapters["go"] = debugadapter.Config{Languages: []string{"go"}}
	h.openFile(path)
	left := h.app.EditorGroup
	h.exec("debug.toggleBreakpoint")
	h.exec("editor.split")
	right := h.app.EditorGroup
	requireNoError(t, right.OpenFile(path))
	l, lok := left.Editor.VisibleBookmark(0)
	r, rok := right.Editor.VisibleBookmark(0)
	if left == right || !lok || !rok || l != r {
		t.Fatal("split did not show the shared breakpoint")
	}
	h.exec("debug.toggleBreakpoint")
	_, lok = left.Editor.VisibleBookmark(0)
	_, rok = right.Editor.VisibleBookmark(0)
	if lok || rok {
		t.Fatal("removal from split left a stale gutter marker")
	}
}

func TestDebugPanelAvailableBeforeConfiguration(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	if !h.app.Sidebar.HasPanel("debug") {
		t.Fatal("Debug missing from sidebar on startup")
	}
	h.exec("sidebar.debug")
	h.assertContains("Configure")
	h.assertContains("Debug File")
	h.assertContains("Breakpoints")
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	path := filepath.Join(h.dir, ".ttt", "launch.json")
	if h.app.EditorGroup.ActiveFilePath() != path || h.app.Root.Focused != h.app.EditorGroup {
		t.Fatal("Configure did not open launch.json for editing")
	}
}

func TestDebugSetupOpensExistingVSCodeConfiguration(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	dir := filepath.Join(h.dir, ".vscode")
	requireNoError(t, os.MkdirAll(dir, 0755))
	path := filepath.Join(dir, "launch.json")
	requireNoError(t, os.WriteFile(path, []byte("{"), 0644))
	h.exec("sidebar.debug")
	h.assertContains("Configure")
	h.assertContains("Debug File")
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if h.app.EditorGroup.ActiveFilePath() != path {
		t.Fatal("did not open existing launch file")
	}
	data, err := os.ReadFile(path)
	requireNoError(t, err)
	if string(data) != "{" {
		t.Fatal("replaced invalid launch file")
	}
	if _, err := os.Stat(filepath.Join(h.dir, ".ttt", "launch.json")); !os.IsNotExist(err) {
		t.Fatal("unexpected .ttt configuration")
	}
}

func TestDebugConfigurationFromPythonFile(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	h.app.Settings.Debug.Adapters["debugpy"] = debugadapter.Config{Provider: "debugpy", Languages: []string{"python"}}
	source := filepath.Join(h.dir, "app.py")
	requireNoError(t, os.WriteFile(source, []byte("print(42)"), 0644))
	h.openFile(source)
	h.exec("sidebar.debug")
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	targets, err := dap.LoadTargets(h.dir)
	requireNoError(t, err)
	if len(targets) != 1 || targets[0].Type != "debugpy" || targets[0].Arguments["program"] != "app.py" {
		t.Fatalf("targets = %+v", targets)
	}
	h.assertContains(" Run & Debug ")
}

func TestDebugSidebarSectionDividers(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	h.openFile(filepath.Join(h.dir, "alpha.txt"))
	h.exec("sidebar.debug")
	row := func(label string) int {
		t.Helper()
		for y := 0; y < 40; y++ {
			if strings.Contains(h.screenRow(y), label) {
				return y
			}
		}
		t.Fatalf("missing %s", label)
		return -1
	}
	controls, variables := row("Configure"), row("Variables")
	for _, label := range []string{"Call Stack", "Breakpoints"} {
		from := row(label) - 1
		for _, event := range []struct {
			y      int
			button tcell.ButtonMask
		}{{from, tcell.Button1}, {from - 2, tcell.Button1}, {from - 2, tcell.ButtonNone}} {
			h.app.Root.HandleEvent(tcell.NewEventMouse(8, event.y, event.button, tcell.ModNone))
			h.redraw()
		}
		h.exec("debug.toggleBreakpoint")
		if row(label) != from-1 || row("Configure") != controls || row("Variables") != variables {
			t.Fatalf("dragging divider above %s moved toolbar or failed to resize section", label)
		}
	}
}

func TestDebugIconsSwitchInOpenSidebar(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	defer icons.Configure("", nil)
	path := filepath.Join(h.dir, "sample.go")
	requireNoError(t, os.WriteFile(path, []byte("package main"), 0600))
	h.app.Settings.Debug.Adapters["go"] = debugadapter.Config{Languages: []string{"go"}}
	h.openFile(path)
	h.exec("sidebar.debug")
	h.exec("debug.toggleBreakpoint")
	for _, dot := range []rune{'⦁', '\uf111'} {
		h.assertContains("Configure")
		h.assertContains("Debug File")
		mark, ok := h.app.EditorGroup.Editor.VisibleBookmark(0)
		if !ok || mark.Icon != dot || mark.Style != term.StyleDebugBreakpoint || strings.Count(h.screenText(), string(dot)) < 2 {
			t.Fatalf("expected a red %c in the gutter and the matching breakpoint list icon", dot)
		}
		h.exec("options.toggleFontIcons")
	}
}

func TestDebugGutterRequiresConfiguredLanguage(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	path := filepath.Join(h.dir, "example.c")
	requireNoError(t, os.WriteFile(path, []byte("int main(void) {}"), 0600))
	h.openFile(path)
	g := h.app.EditorGroup
	if g.OnGutterClick(path, 0) {
		t.Fatal("empty registry claimed the gutter")
	}
	h.app.Settings.Debug.Adapters["python"] = debugadapter.Config{Languages: []string{"python"}}
	if g.OnGutterClick(path, 0) {
		t.Fatal("Python adapter claimed a C source")
	}
	h.app.Settings.Debug.Adapters["native"] = debugadapter.Config{Languages: []string{"c"}}
	if !g.OnGutterClick(path, 0) {
		t.Fatal("C breakpoint click unhandled")
	}
	if _, ok := g.Editor.VisibleBookmark(0); !ok {
		t.Fatal("C breakpoint missing")
	}
}

func TestDebugFileButtonPromptsBeforeLaunchingDirtySource(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	path := filepath.Join(h.dir, "test.js")
	requireNoError(t, os.WriteFile(path, []byte("console.log(1)"), 0600))
	h.app.Settings.Debug.Adapters["node"] = debugadapter.Config{FileLaunch: map[string]any{"program": "${file}"}, Languages: []string{"javascript"}}
	h.openFile(path)
	h.pressRune(' ')
	h.exec("sidebar.debug")
	for y := range 40 {
		row := h.screenRow(y)
		if x := strings.Index(row, "Debug File"); x >= 0 {
			h.click(textwidth.String(row[:x])+2, y)
			h.assertContains("Save test.js before debugging?")
			h.pressKey(tcell.KeyEsc, tcell.ModNone)
			if !h.app.EditorGroup.IsDirtyPath(path) {
				t.Fatal("cancel discarded pending edits")
			}
			return
		}
	}
	t.Fatal("Debug File button missing")
}
