package e2e

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestKeyInterceptorBlocksRune(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	intercepted := false
	h.app.Root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		if ev.Key() == tcell.KeyRune && ev.Str() == "j" {
			intercepted = true
			return true
		}
		return false
	}

	h.pressRune('j')

	if !intercepted {
		t.Fatal("expected interceptor to be called")
	}
	if h.containsText("j") {
		t.Fatal("expected 'j' to be suppressed by interceptor")
	}
}

func TestKeyInterceptorPassthrough(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	h.app.Root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		return false
	}

	h.pressRune('x')

	h.assertContains("x")
}

// Modal plugins (Vim mode) need Esc to leave insert mode, so the interceptor
// runs before EscapeDismissers.
func TestKeyInterceptorConsumesEscape(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	dismissed := false
	h.app.Root.EscapeDismissers = append(h.app.Root.EscapeDismissers, func() bool {
		dismissed = true
		return true
	})

	intercepted := false
	h.app.Root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		if ev.Key() == tcell.KeyEscape {
			intercepted = true
			return true
		}
		return false
	}

	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if !intercepted {
		t.Fatal("expected interceptor to receive Escape")
	}
	if dismissed {
		t.Fatal("expected interceptor to preempt EscapeDismissers")
	}
}

func TestKeyInterceptorEscapePassthrough(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	dismissed := false
	h.app.Root.EscapeDismissers = append(h.app.Root.EscapeDismissers, func() bool {
		dismissed = true
		return true
	})

	h.app.Root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		return false
	}

	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if !dismissed {
		t.Fatal("expected EscapeDismissers to run when the interceptor declines")
	}
}

// A chord in flight outranks the interceptor: its continuation keys are plain
// runes that a modal plugin would otherwise swallow.
func TestKeyInterceptorDoesNotBreakChords(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	// Stand in for a modal plugin that consumes every printable key.
	h.app.Root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		return ev.Key() == tcell.KeyRune
	}

	fired := false
	h.app.Root.AddChordKey([]ui.GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyRune, Rune: 'z'},
	}, func() { fired = true })

	h.pressCtrl(tcell.KeyCtrlK)
	h.pressRune('z')

	if !fired {
		t.Fatal("expected chord to fire despite an interceptor that consumes runes")
	}
}

// rawKeyWidget stands in for the integrated terminal.
type rawKeyWidget struct {
	widgets.BaseWidget
	wants bool
}

func (w *rawKeyWidget) Height() int              { return 1 }
func (w *rawKeyWidget) Width() int               { return 1 }
func (w *rawKeyWidget) Render(_ widgets.Surface) {}
func (w *rawKeyWidget) WantsRawKeys() bool       { return w.wants }
func (w *rawKeyWidget) HandleEvent(_ tcell.Event) widgets.EventResult {
	return widgets.EventConsumed
}

// registerForceKey mirrors BindKeys: globally *and* as a force key.
func registerForceKey(h *testHarness, key tcell.Key, fired *bool) {
	handler := func() { *fired = true }
	h.app.Root.AddGlobalKey(key, tcell.ModCtrl, 0, handler)
	h.app.Root.AddForceKey(key, tcell.ModCtrl, 0, handler)
}

// With the editor focused there is no raw consumer to escape from, so a
// keybinding plugin gets first refusal on ctrl+t.
func TestForceKeyReachesInterceptorWhenEditorFocused(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	forceFired := false
	registerForceKey(h, tcell.KeyCtrlT, &forceFired)

	intercepted := false
	h.app.Root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		if ev.Key() == tcell.KeyCtrlT {
			intercepted = true
			return true
		}
		return false
	}

	h.pressCtrl(tcell.KeyCtrlT)

	if !intercepted {
		t.Fatal("expected interceptor to receive ctrl+t when the editor has focus")
	}
	if forceFired {
		t.Fatal("expected the interceptor to preempt the force key")
	}
}

// The invariant: a plugin can never swallow the toggle out of the terminal.
func TestForceKeyBypassesInterceptorWhenRawConsumerFocused(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	forceFired := false
	registerForceKey(h, tcell.KeyCtrlE, &forceFired)

	intercepted := false
	h.app.Root.KeyInterceptor = func(_ *tcell.EventKey) bool {
		intercepted = true
		return true
	}

	h.app.Root.Focused = &rawKeyWidget{wants: true}
	h.pressCtrl(tcell.KeyCtrlE)

	if !forceFired {
		t.Fatal("expected the force key to fire while a raw key consumer has focus")
	}
	if intercepted {
		t.Fatal("expected the force key to preempt the interceptor")
	}
}

// Without an interceptor the binding still fires, via handleGlobalKeys.
func TestForceKeyStillFiresWithoutInterceptor(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	forceFired := false
	registerForceKey(h, tcell.KeyCtrlE, &forceFired)

	h.pressCtrl(tcell.KeyCtrlE)

	if !forceFired {
		t.Fatal("expected ctrl+e to fire via the global binding with no interceptor")
	}
}

// An interceptor that declines must not stop the binding from firing.
func TestForceKeyFiresWhenInterceptorDeclines(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	forceFired := false
	registerForceKey(h, tcell.KeyCtrlE, &forceFired)

	h.app.Root.KeyInterceptor = func(_ *tcell.EventKey) bool { return false }

	h.pressCtrl(tcell.KeyCtrlE)

	if !forceFired {
		t.Fatal("expected ctrl+e to fire when the interceptor declines it")
	}
}

func TestKeyInterceptorNotCalledForOverlays(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("file.new")

	called := false
	h.app.Root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		called = true
		return true
	}

	h.exec("command.palette")
	h.pressRune('a')

	if called {
		t.Fatal("interceptor should not be called when an overlay is active")
	}
}
