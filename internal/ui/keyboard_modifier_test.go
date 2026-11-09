package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestKeyboardModifierBindings(t *testing.T) {
	for _, allowed := range []tcell.ModMask{0, tcell.ModCtrl, tcell.ModMeta, tcell.ModCtrl | tcell.ModMeta} {
		for _, tc := range []struct {
			key   tcell.Key
			text  string
			extra tcell.ModMask
		}{
			{key: tcell.KeyCtrlA, text: "a"},
			{key: tcell.KeyCtrlZ, text: "Z", extra: tcell.ModShift},
			{key: tcell.KeyCtrlS, text: "s", extra: tcell.ModAlt},
			{key: tcell.KeyLeft},
			{key: tcell.KeyNUL, text: " "},
			{key: tcell.KeyNUL, text: "`"},
			{key: tcell.KeyUS, text: "/"},
		} {
			for _, physical := range []tcell.ModMask{tcell.ModCtrl, tcell.ModMeta, tcell.ModCtrl | tcell.ModMeta} {
				root := NewRoot(&passThroughWidget{})
				root.SetControlModifiers(allowed)
				fired := false
				root.AddGlobalKey(tc.key, tcell.ModCtrl|tc.extra, 0, func() { fired = true })
				key := tc.key
				if physical&tcell.ModMeta != 0 && tc.text != "" {
					key = tcell.KeyRune
				}
				root.HandleEvent(tcell.NewEventKey(key, tc.text, physical|tc.extra))
				effective := allowed
				if effective == 0 {
					effective = tcell.ModCtrl
				}
				want := physical != tcell.ModCtrl|tcell.ModMeta && effective&physical != 0
				if fired != want {
					t.Fatalf("allowed=%v key=%v extra=%v physical=%v: fired=%v, want %v", allowed, tc.key, tc.extra, physical, fired, want)
				}
			}
		}
	}
}

func TestKeyboardModifierInputAndOutput(t *testing.T) {
	clipboard.DisableSystem()
	input := widgets.NewInputWidget(widgets.InputConfig{})
	adapter := NewWidgetAdapter(widgets.NewVStackWidget(input))
	adapter.SetRect(Rect{W: 60, H: 5})
	adapter.Render(NewRenderSurface(makeGrid(60, 5), Rect{W: 60, H: 5}))
	root := NewRoot(adapter)
	root.SetFocus(adapter)
	output := NewOutputWidget()
	output.ControlModifiers = &root.ControlModifiers
	output.AddLine(OutputLine{Time: "12:00", PluginName: "test", Message: "copied output"})
	for _, allowed := range []tcell.ModMask{tcell.ModCtrl, tcell.ModCtrl | tcell.ModMeta, tcell.ModMeta} {
		root.SetControlModifiers(allowed)
		for _, physical := range []tcell.ModMask{tcell.ModCtrl, tcell.ModMeta} {
			root.SetFocus(adapter)
			press := func(ch rune) {
				if physical == tcell.ModCtrl {
					root.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlA+tcell.Key(ch-'a'), "", physical))
				} else {
					root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(ch), physical))
				}
			}
			input.SetText("alpha beta")
			press('a')
			if got, want := input.HasSelection(), allowed&physical != 0; got != want {
				t.Fatalf("allowed=%v physical=%v: input selected=%t, want %t", allowed, physical, got, want)
			}
			if allowed&physical == 0 {
				continue
			}
			press('c')
			if clipboard.Get() != "alpha beta" {
				t.Fatal("copy did not use the input selection")
			}
			press('x')
			press('v')
			press('z')
			if input.Text() != "" {
				t.Fatal("undo did not remove the pasted text")
			}
			press('y')
			if input.Text() != "alpha beta" {
				t.Fatal("redo did not restore the pasted text")
			}
			root.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", physical))
			root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "!", 0))
			if input.Text() != "alpha !beta" {
				t.Fatalf("word navigation left the cursor at the wrong position: %q", input.Text())
			}
			root.SetFocus(output)
			press('c')
			if clipboard.Get() != "12:00 [test] copied output" {
				t.Fatal("output copy did not follow the modifier setting")
			}
		}
	}
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	group.ControlModifiers = &root.ControlModifiers
	tabInput := widgets.NewInputWidget(widgets.InputConfig{})
	tabAdapter := NewWidgetAdapter(widgets.NewVStackWidget(tabInput))
	tabAdapter.SetRect(Rect{W: 60, H: 5})
	tabAdapter.Render(NewRenderSurface(makeGrid(60, 5), Rect{W: 60, H: 5}))
	group.OpenPluginTab("input", "Input", tabAdapter)
	if tabInput.ControlModifiers != &root.ControlModifiers {
		t.Fatal("editor tab did not bind the setting when opened")
	}
	root.SetFocus(group)
	root.SetControlModifiers(tcell.ModMeta)
	tabInput.SetText("editor tab input")
	root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModMeta))
	if !tabInput.HasSelection() {
		t.Fatal("input in an editor tab did not inherit the shortcut modifier")
	}
}

func TestKeyboardModifierChordAndRawKeys(t *testing.T) {
	root := NewRoot(&passThroughWidget{})
	root.SetControlModifiers(tcell.ModMeta)
	fired := 0
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyRune, Rune: 'e'},
	}, func() { fired++ })
	metaK := tcell.NewEventKey(tcell.KeyRune, "k", tcell.ModMeta)
	root.KeyInterceptor = func(ev *tcell.EventKey) bool {
		if ev != metaK {
			t.Fatal("plugin interceptor received a translated key")
		}
		return false
	}
	root.HandleEvent(metaK)
	root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "E", 0))
	if fired != 1 {
		t.Fatal("Super chord did not preserve its plain continuation")
	}
	root.HandleEvent(metaK)
	root.SetControlModifiers(tcell.ModCtrl)
	if root.chord != nil {
		t.Fatal("changing modifier retained an in-flight chord")
	}
	root.SetControlModifiers(tcell.ModMeta)
	terminal := &rawKeysWidget{}
	root.SetFocus(terminal)
	root.HandleEvent(metaK)
	if terminal.lastEvent != metaK {
		t.Fatal("terminal did not receive original event")
	}
	force := false
	root.AddForceKey(tcell.KeyCtrlK, tcell.ModCtrl, 0, func() { force = true })
	root.HandleEvent(metaK)
	if !force || terminal.eventCount != 1 {
		t.Fatal("Super force key did not bypass terminal")
	}
}

func TestKeyboardModifierRecording(t *testing.T) {
	w := NewKeybindingsWidget(nil)
	modifiers := tcell.ModMeta
	w.ControlModifiers = &modifiers
	w.recording = true
	w.handleRecordKey(tcell.NewEventKey(tcell.KeyRune, "s", tcell.ModMeta))
	if w.recordCombo != "ctrl+s" {
		t.Fatalf("recorded %q, want ctrl+s", w.recordCombo)
	}
}
