package e2e

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func (h *testHarness) typeText(s string) {
	h.t.Helper()
	for _, r := range s {
		h.pressRune(r)
	}
}

func TestCommandLineShowTypeSubmit(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	submitted := ""
	h.app.ShowCommandLine(":", nil, func(text string) { submitted = text }, nil)
	h.redraw()

	if !h.app.CommandLineActive() {
		t.Fatal("expected the command line to be active")
	}

	h.typeText("wq")
	h.assertContains(":wq")

	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	if submitted != "wq" {
		t.Fatalf("expected submit %q, got %q", "wq", submitted)
	}
	if h.app.CommandLineActive() {
		t.Fatal("expected the command line to close on submit")
	}
	if h.app.Root.Focused != h.app.EditorGroup {
		t.Fatalf("expected focus restored to EditorGroup, got %T", h.app.Root.Focused)
	}
	h.assertNotContains(":wq")
}

func TestCommandLineEscapeCancelsAndRestoresFocus(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	cancelled := false
	h.app.ShowCommandLine(":", nil, func(string) { t.Fatal("submit must not fire") }, func() { cancelled = true })
	h.redraw()

	h.typeText("q")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if !cancelled {
		t.Fatal("expected OnCancel to fire")
	}
	if h.app.CommandLineActive() {
		t.Fatal("expected the command line to close on Escape")
	}
	if h.app.Root.Focused != h.app.EditorGroup {
		t.Fatalf("expected focus restored to EditorGroup, got %T", h.app.Root.Focused)
	}

	// Focus really is back on the editor: typing edits the buffer.
	h.pressRune('z')
	h.assertContains("z")
}

func TestCommandLineOnChangeFires(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	var changes []string
	h.app.ShowCommandLine("/", func(text string) { changes = append(changes, text) }, nil, nil)
	h.redraw()

	h.typeText("ab")

	if len(changes) != 2 || changes[0] != "a" || changes[1] != "ab" {
		t.Fatalf("expected [a ab], got %v", changes)
	}
	h.app.HideCommandLine()
}

// The whole design rests on this: because the command line is a modal overlay,
// Root.handleOverlay runs above the plugin KeyInterceptor, so a modal plugin
// (Vim mode) goes silent while the command line is open without any focus
// stashing or mode flags.
func TestCommandLineSilencesKeyInterceptor(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	intercepted := 0
	h.app.Root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		intercepted++
		return true
	}

	// Sanity check: the interceptor is wired and does receive keys normally.
	h.pressRune('j')
	if intercepted != 1 {
		t.Fatalf("expected the interceptor to receive keys before opening, got %d", intercepted)
	}

	h.app.ShowCommandLine(":", nil, nil, nil)
	h.redraw()

	h.typeText("abc")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if intercepted != 1 {
		t.Fatalf("expected the interceptor to receive no keys while the command line is open, got %d extra", intercepted-1)
	}

	// ...and it starts receiving keys again once the command line closes.
	h.pressRune('j')
	if intercepted != 2 {
		t.Fatalf("expected the interceptor to resume after close, got %d", intercepted)
	}
}

func TestCommandLineShowReplacesInsteadOfStacking(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	h.app.ShowCommandLine(":", nil, nil, nil)
	h.app.SetCommandLineText("first")
	h.app.ShowCommandLine("/", nil, nil, nil)
	h.redraw()

	if h.app.CommandLineText() != "" {
		t.Fatalf("expected the second command line to start empty, got %q", h.app.CommandLineText())
	}
	h.assertNotContains(":first")

	h.app.HideCommandLine()
	if h.app.Root.HasOverlay() {
		t.Fatal("expected no overlays left after a single hide")
	}
	if h.app.Root.Focused != h.app.EditorGroup {
		t.Fatalf("expected focus restored to EditorGroup, got %T", h.app.Root.Focused)
	}
}

func TestCommandLineOpensOverFindAndReplace(t *testing.T) {
	for _, search := range []string{"search.find", "search.replace"} {
		t.Run(search, func(t *testing.T) {
			h := newTestHarness(t, 100, 30)
			defer h.stop()
			h.exec("file.new")
			h.exec(search)
			searchBar := h.app.Root.TopOverlayWidget()
			if w := h.app.ShowCommandLine(":", nil, nil, nil); w == nil {
				t.Fatal("search bar blocked the command line")
			}
			if len(h.app.Root.Overlays) != 2 || !h.app.Root.HasModalOverlay() {
				t.Fatal("command line should open above the search bar")
			}
			h.pressKey(tcell.KeyEscape, tcell.ModNone)
			if len(h.app.Root.Overlays) != 1 || h.app.Root.TopOverlayWidget() != searchBar || h.app.Root.HasModalOverlay() {
				t.Fatal("dismissing the command line should preserve the search bar")
			}
		})
	}
}

func TestCommandLineNotShownOverAnotherModalOverlay(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")
	h.exec("command.palette")

	if w := h.app.ShowCommandLine(":", nil, nil, nil); w != nil {
		t.Fatal("expected ShowCommandLine to decline while another modal overlay is up")
	}
	if h.app.CommandLineActive() {
		t.Fatal("expected no command line")
	}
}

func TestHideCommandLineWithoutShowIsNoop(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")
	focus := h.app.Root.Focused
	h.app.HideCommandLine()
	h.app.SetCommandLineText("ignored")

	if h.app.Root.Focused != focus {
		t.Fatal("expected focus to be untouched")
	}
}
