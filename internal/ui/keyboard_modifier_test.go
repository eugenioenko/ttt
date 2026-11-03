package ui

import (
	"testing"

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
	w.ControlModifiers = tcell.ModMeta
	w.recording = true
	w.handleRecordKey(tcell.NewEventKey(tcell.KeyRune, "s", tcell.ModMeta))
	if w.recordCombo != "ctrl+s" {
		t.Fatalf("recorded %q, want ctrl+s", w.recordCombo)
	}
}
