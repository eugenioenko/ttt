//go:build chaos

package chaos

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/workspace"

	"github.com/gdamore/tcell/v3"
)

type EventRecord struct {
	Type string `json:"type"`
	Desc string `json:"desc"`
}

type CrashReport struct {
	Seed       int64         `json:"seed"`
	Iteration  int           `json:"iteration"`
	EventCount int           `json:"event_count"`
	Events     []EventRecord `json:"events"`
	Panic      string        `json:"panic"`
	Stack      string        `json:"stack"`
}

type chaosHarness struct {
	app         *app.App
	screen      *term.SimScreen
	reg         *command.Registry
	renderer    *render.Renderer
	dir         string
	events      []EventRecord
	rng         *rand.Rand
	commandPool []command.Command
}

// destructiveCommands are excluded from the random command pool: they end
// the run mid-iteration, masking whatever else the chaos test was probing.
var destructiveCommands = map[string]bool{
	"editor.quit": true,
}

func newChaosHarness(seed int64) *chaosHarness {
	// Prevent OSC 52 escape sequences from leaking into test output
	clipboard.DisableSystem()
	dir, _ := os.MkdirTemp("", "chaos-*")

	// Isolate config writes — random commands persist settings and keybindings.
	config.OverrideConfigDir = filepath.Join(dir, "config")

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Test\n\nSome content here.\n\n- item 1\n- item 2\n"), 0644)
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte(strings.Repeat("The quick brown fox jumps over the lazy dog.\n", 20)), 0644)
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "lib.go"), []byte("package src\n\nfunc Add(a, b int) int { return a + b }\n"), 0644)

	w, h := 80, 24
	sim := term.NewSimScreen()
	sim.Init()
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

	pluginsDir := filepath.Join(dir, "plugins")
	registryPath := filepath.Join(dir, "registry.json")
	editor.PluginManager = plugin.NewManager(pluginsDir, registryPath)

	reg := command.NewRegistry()
	editor.Reg = reg
	running := true
	editor.Running = &running
	app.RegisterCommands(editor)
	app.BindKeys(editor.Root, reg, cfg.Keybindings)
	editor.Root.SetSize(w, h)

	cells := make([][]term.Cell, h)
	for y := range cells {
		cells[y] = make([]term.Cell, w)
	}
	editor.Root.Render(cells)
	editor.Renderer.SetCurrent(cells)
	editor.Renderer.Render(screen)

	var commandPool []command.Command
	for _, cmd := range reg.List() {
		if !destructiveCommands[cmd.ID] {
			commandPool = append(commandPool, cmd)
		}
	}

	return &chaosHarness{
		app:         editor,
		screen:      sim,
		reg:         reg,
		renderer:    editor.Renderer,
		dir:         dir,
		events:      nil,
		rng:         rand.New(rand.NewSource(seed)),
		commandPool: commandPool,
	}
}

func (h *chaosHarness) cleanup() {
	// Close terminals before removing the temp dir to avoid PTY fd leaks across iterations
	h.app.CloseAllTerminals()
	h.screen.Fini()
	os.RemoveAll(h.dir)
}

func (h *chaosHarness) redraw() {
	cells := make([][]term.Cell, h.app.Root.Height)
	for y := range cells {
		cells[y] = make([]term.Cell, h.app.Root.Width)
	}
	h.app.Root.Render(cells)
	h.renderer.SetCurrent(cells)
	h.renderer.Render(h.app.Screen)
}

func (h *chaosHarness) flushOnChange() {
	if h.app.EditorGroup.Editor != nil {
		h.app.EditorGroup.Editor.FlushOnChange()
	}
}

func (h *chaosHarness) dispatch(ev tcell.Event) {
	h.app.Root.HandleEvent(ev)
	h.flushOnChange()
	h.redraw()
}

func (h *chaosHarness) record(typ, desc string) {
	h.events = append(h.events, EventRecord{Type: typ, Desc: desc})
}

var printableRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 \t!@#$%^&*()_+-=[]{}|;':\",./<>?`~")

var specialKeys = []tcell.Key{
	tcell.KeyEscape, tcell.KeyEnter, tcell.KeyTab, tcell.KeyBacktab,
	tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete,
	tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight,
	tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn,
	tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF5,
}

var ctrlKeys = []tcell.Key{
	tcell.KeyCtrlA, tcell.KeyCtrlB, tcell.KeyCtrlC, tcell.KeyCtrlD,
	tcell.KeyCtrlE, tcell.KeyCtrlF, tcell.KeyCtrlG, tcell.KeyCtrlH,
	tcell.KeyCtrlK, tcell.KeyCtrlL, tcell.KeyCtrlN, tcell.KeyCtrlO,
	tcell.KeyCtrlP, tcell.KeyCtrlQ, tcell.KeyCtrlR, tcell.KeyCtrlS,
	tcell.KeyCtrlT, tcell.KeyCtrlU, tcell.KeyCtrlV, tcell.KeyCtrlW,
	tcell.KeyCtrlX, tcell.KeyCtrlY, tcell.KeyCtrlZ,
	tcell.KeyNUL,
}

var chordFollowRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

var pasteSnippets = []string{
	"hello world",
	"line one\nline two\nline three",
	"func main() {\n\tfmt.Println(\"hi\")\n}",
	"\t\tindented\ttext",
	"emoji 🎉 and unicode ünïcödé",
	"",
}

func (h *chaosHarness) randomEvent() {
	w, hh := h.app.Root.Width, h.app.Root.Height

	n := h.rng.Intn(100)
	switch {
	case n >= 0 && n < 25:
		// 25%: printable rune
		r := printableRunes[h.rng.Intn(len(printableRunes))]
		h.record("rune", string(r))
		h.dispatch(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))

	case n >= 25 && n < 37:
		// 12%: special key
		k := specialKeys[h.rng.Intn(len(specialKeys))]
		h.record("special", fmt.Sprintf("key=%d", k))
		h.dispatch(tcell.NewEventKey(k, "", tcell.ModNone))

	case n >= 37 && n < 45:
		// 8%: ctrl+key
		k := ctrlKeys[h.rng.Intn(len(ctrlKeys))]
		h.record("ctrl", fmt.Sprintf("key=%d", k))
		h.dispatch(tcell.NewEventKey(k, "", tcell.ModCtrl))

	case n >= 45 && n < 53:
		// 8%: chord (ctrl+k followed by a rune)
		h.record("chord", "ctrl+k")
		h.dispatch(tcell.NewEventKey(tcell.KeyCtrlK, "", tcell.ModCtrl))
		r := chordFollowRunes[h.rng.Intn(len(chordFollowRunes))]
		h.record("chord-follow", string(r))
		h.dispatch(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))

	case n >= 53 && n < 58:
		// 5%: alt+rune (e.g. alt+t terminal fullscreen)
		r := chordFollowRunes[h.rng.Intn(len(chordFollowRunes))]
		h.record("alt", string(r))
		h.dispatch(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModAlt))

	case n >= 58 && n < 66:
		// 8%: mouse click (left)
		mx := h.rng.Intn(w)
		my := h.rng.Intn(hh)
		h.record("click", fmt.Sprintf("x=%d,y=%d", mx, my))
		h.dispatch(tcell.NewEventMouse(mx, my, tcell.Button1, tcell.ModNone))
		h.dispatch(tcell.NewEventMouse(mx, my, tcell.ButtonNone, tcell.ModNone))

	case n >= 66 && n < 70:
		// 4%: mouse right-click (context menus)
		mx := h.rng.Intn(w)
		my := h.rng.Intn(hh)
		h.record("rclick", fmt.Sprintf("x=%d,y=%d", mx, my))
		h.dispatch(tcell.NewEventMouse(mx, my, tcell.Button2, tcell.ModNone))
		h.dispatch(tcell.NewEventMouse(mx, my, tcell.ButtonNone, tcell.ModNone))

	case n >= 70 && n < 74:
		// 4%: mouse drag
		x1 := h.rng.Intn(w)
		y1 := h.rng.Intn(hh)
		x2 := h.rng.Intn(w)
		y2 := h.rng.Intn(hh)
		steps := 3 + h.rng.Intn(4)
		h.record("drag", fmt.Sprintf("(%d,%d)->(%d,%d) steps=%d", x1, y1, x2, y2, steps))
		h.dispatch(tcell.NewEventMouse(x1, y1, tcell.Button1, tcell.ModNone))
		for s := 1; s <= steps; s++ {
			ix := x1 + (x2-x1)*s/steps
			iy := y1 + (y2-y1)*s/steps
			h.dispatch(tcell.NewEventMouse(ix, iy, tcell.Button1, tcell.ModNone))
		}
		h.dispatch(tcell.NewEventMouse(x2, y2, tcell.ButtonNone, tcell.ModNone))

	case n >= 74 && n < 78:
		// 4%: mouse scroll (vertical)
		mx := h.rng.Intn(w)
		my := h.rng.Intn(hh)
		btn := tcell.WheelUp
		dir := "up"
		if h.rng.Intn(2) == 0 {
			btn = tcell.WheelDown
			dir = "down"
		}
		h.record("scroll", fmt.Sprintf("x=%d,y=%d,dir=%s", mx, my, dir))
		h.dispatch(tcell.NewEventMouse(mx, my, btn, tcell.ModNone))

	case n >= 78 && n < 81:
		// 3%: mouse scroll (horizontal)
		mx := h.rng.Intn(w)
		my := h.rng.Intn(hh)
		btn := tcell.WheelLeft
		dir := "left"
		if h.rng.Intn(2) == 0 {
			btn = tcell.WheelRight
			dir = "right"
		}
		h.record("hscroll", fmt.Sprintf("x=%d,y=%d,dir=%s", mx, my, dir))
		h.dispatch(tcell.NewEventMouse(mx, my, btn, tcell.ModNone))

	case n >= 81 && n < 85:
		// 4%: bracketed paste
		text := pasteSnippets[h.rng.Intn(len(pasteSnippets))]
		h.record("paste", text)
		h.app.PasteText(text)
		h.flushOnChange()
		h.redraw()

	case n >= 85 && n < 89:
		// 4%: resize
		nw := 40 + h.rng.Intn(120)
		nh := 10 + h.rng.Intn(50)
		h.record("resize", fmt.Sprintf("w=%d,h=%d", nw, nh))
		h.screen.SetSize(nw, nh)
		h.app.Root.SetSize(nw, nh)
		h.redraw()

	case n >= 89 && n < 94:
		// 5%: execute random command (destructive commands excluded)
		if len(h.commandPool) > 0 {
			cmd := h.commandPool[h.rng.Intn(len(h.commandPool))]
			h.record("command", cmd.ID)
			h.reg.Execute(cmd.ID)
			h.flushOnChange()
			h.redraw()
		}

	case n >= 94 && n < 98:
		// 4%: shift+special key
		k := specialKeys[h.rng.Intn(len(specialKeys))]
		h.record("shift-special", fmt.Sprintf("key=%d", k))
		h.dispatch(tcell.NewEventKey(k, "", tcell.ModShift))

	default:
		// 2%: printable rune (leftover share)
		r := printableRunes[h.rng.Intn(len(printableRunes))]
		h.record("rune", string(r))
		h.dispatch(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
}

func writeCrashReport(report CrashReport) string {
	outputDir := os.Getenv("CHAOS_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "."
	}
	os.MkdirAll(outputDir, 0755)
	filename := filepath.Join(outputDir, fmt.Sprintf("crash-%d-%d.json", report.Seed, report.Iteration))
	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(filename, data, 0644)
	return filename
}

func runIteration(seed int64, eventsPerRun int) *CrashReport {
	h := newChaosHarness(seed)
	defer h.cleanup()

	var report *CrashReport
	func() {
		defer func() {
			if r := recover(); r != nil {
				report = &CrashReport{
					Seed:       seed,
					Iteration:  0,
					EventCount: len(h.events),
					Events:     h.events,
					Panic:      fmt.Sprintf("%v", r),
					Stack:      string(debug.Stack()),
				}
			}
		}()

		for i := 0; i < eventsPerRun; i++ {
			h.randomEvent()
		}
	}()

	return report
}

// requireSandbox skips chaos tests outside a container — random commands can
// write, delete, and execute anywhere the host user can.
func requireSandbox(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return
	}
	if os.Getenv("CHAOS_SANDBOXED") == "1" || os.Getenv("CHAOS_ALLOW_HOST") == "1" {
		return
	}
	t.Skip("chaos tests execute arbitrary random commands and must run in Docker " +
		"(make chaos, make chaos-docker, make chaos-replay); set CHAOS_ALLOW_HOST=1 to force a host run")
}

func TestChaosMonkey(t *testing.T) {
	requireSandbox(t)
	iterations := 50
	eventsPerRun := 500

	if v := os.Getenv("CHAOS_ITERATIONS"); v != "" {
		fmt.Sscanf(v, "%d", &iterations)
	}
	if v := os.Getenv("CHAOS_EVENTS"); v != "" {
		fmt.Sscanf(v, "%d", &eventsPerRun)
	}

	baseSeed := time.Now().UnixNano()
	if v := os.Getenv("CHAOS_SEED"); v != "" {
		fmt.Sscanf(v, "%d", &baseSeed)
	}

	var crashes []CrashReport
	start := time.Now()
	progressEvery := iterations / 20
	if progressEvery < 1 {
		progressEvery = 1
	}

	for i := 0; i < iterations; i++ {
		seed := baseSeed + int64(i)
		report := runIteration(seed, eventsPerRun)
		if report != nil {
			report.Iteration = i
			file := writeCrashReport(*report)
			t.Errorf("CRASH at iteration %d (seed=%d): %s\n  saved to %s", i, seed, report.Panic, file)
			crashes = append(crashes, *report)
		}

		done := i + 1
		if done%progressEvery == 0 || done == iterations {
			elapsed := time.Since(start)
			perIter := elapsed / time.Duration(done)
			eta := perIter * time.Duration(iterations-done)
			fmt.Fprintf(os.Stderr, "CHAOS PROGRESS: %d/%d iterations, %d crashes, elapsed=%s, eta=%s\n",
				done, iterations, len(crashes), elapsed.Round(time.Second), eta.Round(time.Second))
		}
	}

	if len(crashes) == 0 {
		t.Logf("OK: %d iterations x %d events = %d total events, no panics (base seed: %d)",
			iterations, eventsPerRun, iterations*eventsPerRun, baseSeed)
	} else {
		t.Errorf("FAILED: %d/%d iterations crashed", len(crashes), iterations)
	}
}

func TestChaosReplay(t *testing.T) {
	requireSandbox(t)
	replayFile := os.Getenv("CHAOS_REPLAY")
	if replayFile == "" {
		t.Skip("set CHAOS_REPLAY=<crash-file.json> to replay")
	}

	data, err := os.ReadFile(replayFile)
	if err != nil {
		t.Fatal(err)
	}

	var report CrashReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}

	h := newChaosHarness(report.Seed)
	defer h.cleanup()

	defer func() {
		if r := recover(); r != nil {
			t.Logf("REPRODUCED panic: %v", r)
			t.Logf("Stack:\n%s", debug.Stack())
			t.FailNow()
		}
	}()

	for i := 0; i < report.EventCount; i++ {
		h.randomEvent()
	}

	t.Log("Replay completed without panic — may be non-deterministic or already fixed")
}

// TestChaosLoop runs continuously until stopped — designed for Docker.
func TestChaosLoop(t *testing.T) {
	requireSandbox(t)
	if os.Getenv("CHAOS_LOOP") == "" {
		t.Skip("set CHAOS_LOOP=1 to run continuous chaos loop")
	}

	eventsPerRun := 500
	if v := os.Getenv("CHAOS_EVENTS"); v != "" {
		fmt.Sscanf(v, "%d", &eventsPerRun)
	}

	outputDir := os.Getenv("CHAOS_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "/output"
	}
	os.Setenv("CHAOS_OUTPUT_DIR", outputDir)

	maxLoops := 0
	if v := os.Getenv("CHAOS_MAX_LOOPS"); v != "" {
		fmt.Sscanf(v, "%d", &maxLoops)
	}

	start := time.Now()
	iteration := 0
	totalCrashes := 0

	for maxLoops <= 0 || iteration < maxLoops {
		seed := time.Now().UnixNano()
		report := runIteration(seed, eventsPerRun)
		if report != nil {
			report.Iteration = iteration
			file := writeCrashReport(*report)
			totalCrashes++
			fmt.Fprintf(os.Stderr, "CRASH #%d at iteration %d (seed=%d): %s\n  → %s\n",
				totalCrashes, iteration, seed, report.Panic, file)
		}

		iteration++
		if iteration%100 == 0 {
			pending := "unbounded"
			if maxLoops > 0 {
				pending = fmt.Sprintf("%d", maxLoops-iteration)
			}
			fmt.Fprintf(os.Stderr, "CHAOS PROGRESS: %d iterations (pending=%s), %d crashes, elapsed=%s\n",
				iteration, pending, totalCrashes, time.Since(start).Round(time.Second))
		}
	}
}
