package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/gdamore/tcell/v3"
)

func TestTerminalShortcutPrefix(t *testing.T) {
	clipboard.DisableSystem()
	dev, err := terminal.New("/bin/sh", 80, 24, 0, nil, "", "-c", "stty raw -echo; printf shortcut-ready; exec cat")
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	updated := newUpdateChan(dev)
	dev.Run()
	waitFor(t, updated, func() bool { return strings.Contains(string(dev.RawTail()), "shortcut-ready") })
	tw := NewTerminalWidget(dev, nil)
	root := NewRoot(&passThroughWidget{})
	root.SetFocus(tw)
	chords, forced := 0, 0
	root.AddTerminalForceKey(tcell.KeyCtrlK, tcell.ModCtrl, 0, func() { root.SendNextTerminalKey(tw) })
	root.AddForceKey(tcell.KeyCtrlP, tcell.ModCtrl, 0, func() { forced++ })
	root.AddForceKey(tcell.KeyCtrlQ, tcell.ModCtrl, 0, func() { forced++ })
	root.AddForceKey(tcell.KeyRune, tcell.ModAlt, 't', func() { forced++ })
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyRune, Rune: 'p'},
	}, func() { chords++ })
	root.AddChordKey([]GlobalKeyBinding{
		{Key: tcell.KeyCtrlK, Mod: tcell.ModCtrl},
		{Key: tcell.KeyRune, Rune: 'x', Mod: tcell.ModShift},
	}, func() { chords++ })

	for _, mode := range []tcell.ModMask{tcell.ModCtrl, tcell.ModMeta, tcell.ModCtrl | tcell.ModMeta} {
		root.SetControlModifiers(mode)
		prefix := tcell.NewEventKey(tcell.KeyCtrlK, "", tcell.ModCtrl)
		if mode&tcell.ModMeta != 0 {
			prefix = tcell.NewEventKey(tcell.KeyRune, "k", tcell.ModMeta)
		}
		for _, tc := range []struct {
			name string
			key  *tcell.EventKey
			want string
		}{
			{"plain chord", tcell.NewEventKey(tcell.KeyRune, "p", 0), ""},
			{"shift chord", tcell.NewEventKeyEx(tcell.KeyRune, "P", tcell.ModShift, true, tcell.Key('p'), 1), ""},
			{"explicit shift chord", tcell.NewEventKeyEx(tcell.KeyRune, "X", tcell.ModShift, true, tcell.Key('x'), 1), ""},
			{"force key", tcell.NewEventKey(tcell.KeyCtrlP, "", tcell.ModCtrl), "\x10"},
			{"quit key", tcell.NewEventKey(tcell.KeyCtrlQ, "", tcell.ModCtrl), "\x11"},
			{"prefix itself", tcell.NewEventKey(tcell.KeyCtrlK, "", tcell.ModCtrl), "\x0b"},
			{"Alt force key", tcell.NewEventKey(tcell.KeyRune, "t", tcell.ModAlt), "\x1bt"},
			{"clipboard key", tcell.NewEventKey(tcell.KeyCtrlV, "", tcell.ModCtrl), "\x16"},
		} {
			t.Run(tc.name+"/"+prefix.Name(), func(t *testing.T) {
				before, beforeChords := len(dev.RawTail()), chords
				clipboard.Set("must-not-paste")
				root.HandleEvent(prefix)
				if !root.AwaitingTerminalKey() {
					t.Fatal("prefix must arm terminal forwarding")
				}
				root.HandleEvent(tc.key)
				if root.AwaitingTerminalKey() {
					t.Fatal("continuation must disarm forwarding")
				}
				if (chords > beforeChords) != (tc.want == "") || forced != 0 {
					t.Fatalf("chords=%d forced=%d", chords-beforeChords, forced)
				}
				tw.SendKey(tcell.NewEventKey(tcell.KeyRune, ".", 0))
				waitFor(t, updated, func() bool { return len(dev.RawTail()) >= before+len(tc.want)+1 })
				if got := string(dev.RawTail()[before:]); got != tc.want+"." {
					t.Fatalf("terminal got %q, want %q", got, tc.want+".")
				}
			})
		}
	}

	root.SetControlModifiers(tcell.ModCtrl)
	root.SetForwardShortcuts(true)
	before := len(dev.RawTail())
	root.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlK, "", tcell.ModCtrl))
	root.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlP, "", tcell.ModCtrl))
	root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "t", tcell.ModAlt))
	tw.SendKey(tcell.NewEventKey(tcell.KeyRune, ".", 0))
	want := "\x0b\x10\x1bt."
	waitFor(t, updated, func() bool { return len(dev.RawTail()) >= before+len(want) })
	if got := string(dev.RawTail()[before:]); got != want || forced != 0 || root.AwaitingTerminalKey() {
		t.Fatalf("direct forwarding got %q, forced=%d, awaiting=%t", got, forced, root.AwaitingTerminalKey())
	}
}

func TestTerminalPrefixDoesNotCaptureOverlayKeys(t *testing.T) {
	root := NewRoot(&passThroughWidget{})
	root.SetFocus(&rawKeysWidget{})
	overlay := &mockWidget{}
	root.PushOverlay(Overlay{Widget: overlay, Modal: true})
	root.AddTerminalForceKey(tcell.KeyCtrlK, tcell.ModCtrl, 0, func() { t.Fatal("terminal prefix captured an overlay key") })
	key := tcell.NewEventKey(tcell.KeyCtrlK, "", tcell.ModCtrl)
	root.HandleEvent(key)
	if overlay.lastEvent != key {
		t.Fatal("overlay must receive Ctrl+K so shortcut recording remains usable")
	}
}

func TestTerminalForwardingCancellation(t *testing.T) {
	dev, err := terminal.New("/bin/cat", 80, 24, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	dev.Run()
	tw := NewTerminalWidget(dev, nil)
	root := NewRoot(&passThroughWidget{})
	root.SetSize(80, 24)
	root.SetFocus(tw)
	for _, cancel := range []struct {
		name string
		run  func()
	}{
		{"focus change", func() { root.SetFocus(&passThroughWidget{}) }},
		{"internal tab switch", func() { tw.SetFocused(false); tw.SetFocused(true) }},
		{"click", func() { root.HandleEvent(tcell.NewEventMouse(1, 1, tcell.Button1, 0)) }},
		{"overlay", func() { root.PushOverlay(Overlay{Widget: &passThroughWidget{}, Modal: true}); root.PopOverlay() }},
		{"setting change", func() { root.SetForwardShortcuts(!root.ForwardShortcuts) }},
	} {
		t.Run(cancel.name, func(t *testing.T) {
			root.SetFocus(tw)
			root.SendNextTerminalKey(tw)
			if !root.AwaitingTerminalKey() {
				t.Fatal("not armed")
			}
			cancel.run()
			if root.AwaitingTerminalKey() {
				t.Fatal("forwarding was not canceled")
			}
		})
	}
	root.SetFocus(tw)
	root.SendNextTerminalKey(tw)
	dev.Close()
	if root.AwaitingTerminalKey() {
		t.Fatal("closing the terminal must cancel forwarding")
	}
}
