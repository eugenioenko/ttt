package ui

import (
	"log/slog"
	"unicode"

	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

type Overlay struct {
	Widget Widget
	Modal  bool
}

type GlobalKeyBinding struct {
	Key     tcell.Key
	Mod     tcell.ModMask
	Rune    rune
	Handler func()
}

type ChordKeyBinding struct {
	Steps   []GlobalKeyBinding
	Handler func()
}

type chordState struct {
	candidates []int
	stepIdx    int
}

type Root struct {
	Main             Widget
	Overlays         []Overlay
	Focused          Widget
	Width            int
	Height           int
	GlobalKeys       []GlobalKeyBinding
	ForceKeys        []GlobalKeyBinding // checked even when focused widget wants raw keys
	ChordKeys        []ChordKeyBinding
	chord            *chordState
	KeyInterceptor   func(ev *tcell.EventKey) bool
	OnRightClick     func(mx, my int)
	EscapeDismissers []func() bool
	EscapeFallback   func()
	capturedWidget   Widget
}

func NewRoot(main Widget) *Root {
	return &Root{Main: main}
}

func (r *Root) SetSize(w, h int) {
	r.Width = w
	r.Height = h
	r.Main.SetRect(Rect{X: 0, Y: 0, W: w, H: h})
}

func (r *Root) AddGlobalKey(key tcell.Key, mod tcell.ModMask, rn rune, handler func()) {
	r.GlobalKeys = append(r.GlobalKeys, GlobalKeyBinding{Key: key, Mod: mod, Rune: rn, Handler: handler})
}

func (r *Root) AddForceKey(key tcell.Key, mod tcell.ModMask, rn rune, handler func()) {
	r.ForceKeys = append(r.ForceKeys, GlobalKeyBinding{Key: key, Mod: mod, Rune: rn, Handler: handler})
}

func (r *Root) AddChordKey(steps []GlobalKeyBinding, handler func()) {
	r.ChordKeys = append(r.ChordKeys, ChordKeyBinding{Steps: steps, Handler: handler})
}

func (r *Root) ClearKeys() {
	r.GlobalKeys = nil
	r.ForceKeys = nil
	r.ChordKeys = nil
	r.chord = nil
}

// normalizeKey folds KeyBackspace2 (DEL, 0x7F) into KeyBackspace (BS, 0x08) so
// bindings registered with either constant match events carrying either byte
// value. tcell normalizes incoming backspace events to KeyBackspace (v2.13+
// and v3), but synthetic events (tests, simulation screens) may still carry
// KeyBackspace2.
func normalizeKey(k tcell.Key) tcell.Key {
	if k == tcell.KeyBackspace2 {
		return tcell.KeyBackspace
	}
	return k
}

// foldCtrlEvent maps ctrl+non-letter printable key events onto canonical
// control-key constants so a single registered form (see comboToTcell in
// internal/app/keys.go) matches every terminal encoding. tcell v3 delivers
// these combos as KeyRune events whose string depends on the encoding:
//
//	combo        legacy bytes    kitty protocol    folded to
//	ctrl+space   " " + ModCtrl   " " + ModCtrl     KeyNUL + ModCtrl
//	ctrl+`       " " + ModCtrl   "`" + ModCtrl     KeyNUL + ModCtrl
//	ctrl+/       "_" + ModCtrl   "/" + ModCtrl     KeyUS  + ModCtrl
//
// (Legacy ctrl+` and ctrl+space both emit NUL — indistinguishable at the
// byte level — and legacy ctrl+/ emits 0x1F, which tcell reports as "_".)
// Ctrl+letter combos are unaffected: tcell folds those to KeyCtrlA..Z itself.
// Returns the canonical key and true when the event was folded.
func foldCtrlEvent(kev *tcell.EventKey) (tcell.Key, bool) {
	if kev.Key() != tcell.KeyRune || kev.Modifiers()&tcell.ModCtrl == 0 {
		return 0, false
	}
	switch kev.Str() {
	case " ", "`":
		return tcell.KeyNUL, true
	case "_", "/":
		return tcell.KeyUS, true
	}
	return 0, false
}

func matchKey(kev *tcell.EventKey, gk GlobalKeyBinding) bool {
	if folded, ok := foldCtrlEvent(kev); ok {
		return folded == normalizeKey(gk.Key) && gk.Key != tcell.KeyRune && kev.Modifiers() == gk.Mod
	}
	if gk.Key != tcell.KeyRune {
		return normalizeKey(kev.Key()) == normalizeKey(gk.Key) && kev.Modifiers() == gk.Mod
	}
	return kev.Key() == tcell.KeyRune && term.KeyRune(kev) == gk.Rune && kev.Modifiers() == gk.Mod
}

// matchKeyChord is like matchKey but compares rune keys case-insensitively.
// This handles caps lock being on: e.g. chord "ctrl+k j" still matches when
// caps lock sends uppercase "J" as the second key.
func matchKeyChord(kev *tcell.EventKey, gk GlobalKeyBinding) bool {
	if folded, ok := foldCtrlEvent(kev); ok {
		return folded == normalizeKey(gk.Key) && gk.Key != tcell.KeyRune && kev.Modifiers() == gk.Mod
	}
	if gk.Key != tcell.KeyRune {
		return normalizeKey(kev.Key()) == normalizeKey(gk.Key) && kev.Modifiers() == gk.Mod
	}
	return kev.Key() == tcell.KeyRune &&
		unicode.ToLower(term.KeyRune(kev)) == unicode.ToLower(gk.Rune) &&
		kev.Modifiers() == gk.Mod
}

func (r *Root) rawKeysFocused() bool {
	rk, ok := r.Focused.(RawKeyConsumer)
	return ok && rk.WantsRawKeys()
}

func (r *Root) HandleEvent(ev tcell.Event) EventResult {
	kev, isKey := ev.(*tcell.EventKey)
	// ForceKeys are the escape hatch out of a raw key consumer, so they only
	// outrank everything while one has focus. Otherwise they are reached below
	// through handleGlobalKeys like any other binding.
	if isKey && r.rawKeysFocused() {
		for _, gk := range r.ForceKeys {
			if matchKey(kev, gk) {
				gk.Handler()
				return EventConsumed
			}
		}
	}

	if !isKey && r.capturedWidget != nil {
		return r.handleMouse(ev)
	}

	if res := r.handleOverlay(ev); res != EventIgnored {
		return EventConsumed
	}

	if !isKey {
		return r.handleMouse(ev)
	}

	// Plugin key interceptors run before Escape handling and chords so modal
	// plugins (e.g. Vim mode) can own the keyboard. Overlays stay above: a
	// plugin must never steal keys from a modal dialog.
	//
	// A chord already in flight also outranks the interceptor: its continuation
	// keys are plain runes (the `s` of `ctrl+k s`), which a modal plugin would
	// otherwise consume, silently breaking every chord binding.
	if r.chord == nil && r.KeyInterceptor != nil && r.KeyInterceptor(kev) {
		return EventConsumed
	}

	if kev.Key() == tcell.KeyEscape {
		for _, dismiss := range r.EscapeDismissers {
			if dismiss() {
				return EventConsumed
			}
		}
		if r.Focused != nil {
			if r.Focused.HandleEvent(ev) == EventConsumed {
				return EventConsumed
			}
		}
		if r.EscapeFallback != nil {
			r.EscapeFallback()
		}
		return EventConsumed
	}

	if res := r.handleChord(kev); res == EventConsumed {
		return EventConsumed
	}

	if r.rawKeysFocused() {
		return r.handleRawKeyConsumer(kev)
	}

	if r.Focused != nil {
		if r.Focused.HandleEvent(ev) == EventConsumed {
			return EventConsumed
		}
	}

	if res := r.handleGlobalKeys(kev); res == EventConsumed {
		return EventConsumed
	}

	return EventIgnored
}

func (r *Root) handleOverlay(ev tcell.Event) EventResult {
	if len(r.Overlays) == 0 {
		return EventIgnored
	}
	top := r.Overlays[len(r.Overlays)-1]
	slog.Debug("root", "action", "overlayIntercept", "modal", top.Modal, "count", len(r.Overlays))
	result := top.Widget.HandleEvent(ev)
	if top.Modal {
		return EventConsumed
	}
	if result == EventConsumed {
		return result
	}
	return EventIgnored
}

func (r *Root) handleMouse(ev tcell.Event) EventResult {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}
	btn := mev.Buttons()

	if r.capturedWidget != nil {
		if btn == tcell.ButtonNone {
			r.capturedWidget.HandleEvent(ev)
			r.capturedWidget = nil
			slog.Debug("root", "action", "mouseCapture", "state", "released")
			return EventConsumed
		}
		r.capturedWidget.HandleEvent(ev)
		slog.Debug("root", "action", "mouseCapture", "state", "active")
		return EventConsumed
	}

	if btn&tcell.Button2 != 0 && r.OnRightClick != nil {
		mx, my := mev.Position()
		slog.Debug("root", "action", "rightClick", "x", mx, "y", my)
		r.OnRightClick(mx, my)
		return EventConsumed
	}

	result := r.Main.HandleEvent(ev)
	if result == EventCaptured {
		r.capturedWidget = r.Main
		slog.Debug("root", "action", "mouseCapture", "state", "set")
		return EventConsumed
	}
	slog.Debug("root", "action", "mouseToMain", "result", result)
	return result
}

func (r *Root) handleChord(kev *tcell.EventKey) EventResult {
	if r.chord != nil {
		var next []int
		for _, ci := range r.chord.candidates {
			chord := r.ChordKeys[ci]
			if r.chord.stepIdx < len(chord.Steps) && matchKeyChord(kev, chord.Steps[r.chord.stepIdx]) {
				if r.chord.stepIdx+1 == len(chord.Steps) {
					r.chord = nil
					chord.Handler()
					return EventConsumed
				}
				next = append(next, ci)
			}
		}
		if len(next) > 0 {
			r.chord.candidates = next
			r.chord.stepIdx++
			return EventConsumed
		}
		r.chord = nil
	}

	var candidates []int
	for i, chord := range r.ChordKeys {
		if len(chord.Steps) > 1 && matchKeyChord(kev, chord.Steps[0]) {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) > 0 {
		r.chord = &chordState{candidates: candidates, stepIdx: 1}
		return EventConsumed
	}

	return EventIgnored
}

func (r *Root) handleRawKeyConsumer(kev *tcell.EventKey) EventResult {
	for _, gk := range r.ForceKeys {
		if matchKey(kev, gk) {
			gk.Handler()
			return EventConsumed
		}
	}
	return r.Focused.HandleEvent(kev)
}

func (r *Root) handleGlobalKeys(kev *tcell.EventKey) EventResult {
	for _, gk := range r.GlobalKeys {
		if matchKey(kev, gk) {
			gk.Handler()
			return EventConsumed
		}
	}
	return EventIgnored
}

func (r *Root) Render(cells [][]term.Cell) {
	surface := NewRenderSurface(cells, Rect{X: 0, Y: 0, W: r.Width, H: r.Height})
	r.Main.Render(surface)

	for _, overlay := range r.Overlays {
		overlay.Widget.SetRect(Rect{X: 0, Y: 0, W: r.Width, H: r.Height})
		overlay.Widget.Render(surface)
	}
}

func (r *Root) HasOverlay() bool {
	return len(r.Overlays) > 0
}

func (r *Root) HasModalOverlay() bool {
	for _, o := range r.Overlays {
		if o.Modal {
			return true
		}
	}
	return false
}

func (r *Root) TopOverlayWidget() Widget {
	if len(r.Overlays) == 0 {
		return nil
	}
	return r.Overlays[len(r.Overlays)-1].Widget
}

func (r *Root) PushOverlay(o Overlay) {
	r.Overlays = append(r.Overlays, o)
	slog.Debug("root", "action", "pushOverlay", "count", len(r.Overlays), "modal", o.Modal)
}

func (r *Root) PopOverlay() {
	if len(r.Overlays) > 0 {
		r.Overlays = r.Overlays[:len(r.Overlays)-1]
		slog.Debug("root", "action", "popOverlay", "count", len(r.Overlays))
	}
}

// RemoveOverlay removes the overlay holding the given widget without disturbing overlays above it.
func (r *Root) RemoveOverlay(w Widget) {
	for i, o := range r.Overlays {
		if o.Widget == w {
			r.Overlays = append(r.Overlays[:i], r.Overlays[i+1:]...)
			slog.Debug("root", "action", "removeOverlay", "count", len(r.Overlays))
			return
		}
	}
}

// Refocusing the already-focused widget is a no-op: a click inside the focused
// pane routes through here, and blurring it first would discard transient state
// such as an open dropdown.
func (r *Root) SetFocus(w Widget) {
	if r.Focused == w {
		return
	}
	if r.Focused != nil {
		if setter, ok := r.Focused.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(false)
		}
	}
	r.Focused = w
	if setter, ok := w.(interface{ SetFocused(bool) }); ok {
		setter.SetFocused(true)
	}
}

func (r *Root) CursorPosition() (x, y int, visible bool) {
	if len(r.Overlays) > 0 {
		top := r.Overlays[len(r.Overlays)-1]
		if cp, ok := top.Widget.(CursorProvider); ok {
			if x, y, vis := cp.CursorPosition(); vis {
				return x, y, true
			}
			// non-modal overlays (e.g. find bar) may lose focus to the editor;
			// fall through so the focused widget can show its cursor instead
			if top.Modal {
				return 0, 0, false
			}
		}
	}
	if r.Focused != nil {
		if cp, ok := r.Focused.(CursorProvider); ok {
			return cp.CursorPosition()
		}
	}
	return 0, 0, false
}
