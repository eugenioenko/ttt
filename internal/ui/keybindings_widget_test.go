package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func kbTestCommands() []command.Command {
	return []command.Command{
		{ID: "file.save", Title: "File: Save", Keywords: []string{"save", "write"}},
		{ID: "file.open", Title: "File: Open", Keywords: []string{"open", "browse"}},
		{ID: "editor.undo", Title: "Undo", Keywords: []string{"undo"}},
		{ID: "editor.redo", Title: "Redo", Keywords: []string{"redo"}},
		{ID: "sidebar.toggle", Title: "View: Toggle Sidebar"},
	}
}

func TestKeybindingsWidgetCreation(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	if w == nil {
		t.Fatal("expected widget")
	}
	if len(w.allItems) != 5 {
		t.Fatalf("expected 5 items, got %d", len(w.allItems))
	}
	if len(w.items) != 5 {
		t.Fatalf("expected 5 filtered items, got %d", len(w.items))
	}
	// items should be sorted alphabetically
	if w.items[0].Title != "File: Open" {
		t.Errorf("expected first item 'File: Open', got %q", w.items[0].Title)
	}
}

func TestKeybindingsWidgetFilter(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	w.input.SetText("undo")
	if len(w.items) != 1 {
		t.Fatalf("expected 1 item for 'undo', got %d", len(w.items))
	}
	if w.items[0].CmdID != "editor.undo" {
		t.Errorf("expected editor.undo, got %q", w.items[0].CmdID)
	}
}

func TestKeybindingsWidgetFilterByShortcut(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	w.GetShortcut = func(cmdID string) string {
		if cmdID == "file.save" {
			return "Ctrl+S"
		}
		return ""
	}
	w.input.SetText("ctrl+s")
	if len(w.items) == 0 {
		t.Fatal("expected at least 1 match for shortcut search")
	}
	found := false
	for _, item := range w.items {
		if item.CmdID == "file.save" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected file.save in results when searching by shortcut")
	}
}

func TestKeybindingsWidgetNavigation(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	if w.selected != 0 {
		t.Fatalf("expected selected=0, got %d", w.selected)
	}

	// Down arrow
	w.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if w.selected != 1 {
		t.Errorf("expected selected=1 after down, got %d", w.selected)
	}

	// Up arrow
	w.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	if w.selected != 0 {
		t.Errorf("expected selected=0 after up, got %d", w.selected)
	}

	// Up wraps to last
	w.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	if w.selected != len(w.items)-1 {
		t.Errorf("expected selected=%d after wrap up, got %d", len(w.items)-1, w.selected)
	}

	// Down wraps to first
	w.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if w.selected != 0 {
		t.Errorf("expected selected=0 after wrap down, got %d", w.selected)
	}
}

func TestKeybindingsWidgetRecording(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	var editedCmd, editedKey string
	w.OnEdit = func(cmdID, newKey string) {
		editedCmd = cmdID
		editedKey = newKey
	}

	// Enter starts recording
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !w.recording {
		t.Fatal("expected recording mode after Enter")
	}

	// Press Ctrl+S, then Enter to confirm single key
	w.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlS, "", tcell.ModCtrl))
	if w.recordCombo != "ctrl+s" {
		t.Errorf("expected recordCombo='ctrl+s', got %q", w.recordCombo)
	}
	// Still recording (waiting for chord or Enter)
	if !w.recording {
		t.Fatal("expected still recording after first key")
	}

	// Enter confirms
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if w.recording {
		t.Fatal("expected recording to stop after Enter confirm")
	}
	if editedKey != "ctrl+s" {
		t.Errorf("expected editedKey='ctrl+s', got %q", editedKey)
	}
	if editedCmd != w.items[0].CmdID {
		t.Errorf("expected editedCmd=%q, got %q", w.items[0].CmdID, editedCmd)
	}
}

func TestKeybindingsWidgetRecordChord(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	var editedKey string
	w.OnEdit = func(cmdID, newKey string) {
		editedKey = newKey
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	// First key: Ctrl+K
	w.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlK, "", tcell.ModCtrl))
	// Second key: e (chord auto-saves)
	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "e", tcell.ModNone))

	if w.recording {
		t.Fatal("expected recording to stop after chord")
	}
	if editedKey != "ctrl+k e" {
		t.Errorf("expected chord 'ctrl+k e', got %q", editedKey)
	}
}

func TestKeybindingsWidgetRecordEscape(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	called := false
	w.OnEdit = func(cmdID, newKey string) {
		called = true
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !w.recording {
		t.Fatal("expected recording")
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone))
	if w.recording {
		t.Fatal("expected recording cancelled")
	}
	if called {
		t.Fatal("OnEdit should not be called on cancel")
	}
}

func TestKeybindingsWidgetClear(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	var clearedCmd string
	w.OnClear = func(cmdID string) {
		clearedCmd = cmdID
	}

	// Delete key clears binding
	w.HandleEvent(tcell.NewEventKey(tcell.KeyDelete, "", tcell.ModNone))
	if clearedCmd != w.items[0].CmdID {
		t.Errorf("expected cleared %q, got %q", w.items[0].CmdID, clearedCmd)
	}
}

func TestKeybindingsWidgetReset(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	var resetCmd string
	w.OnReset = func(cmdID string) {
		resetCmd = cmdID
	}

	// Backspace on empty input resets
	w.HandleEvent(tcell.NewEventKey(tcell.KeyBackspace2, "", tcell.ModNone))
	if resetCmd != w.items[0].CmdID {
		t.Errorf("expected reset %q, got %q", w.items[0].CmdID, resetCmd)
	}
}

func TestKeybindingsWidgetDismiss(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	dismissed := false
	w.OnDismiss = func() {
		dismissed = true
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone))
	if !dismissed {
		t.Fatal("expected dismiss on Escape")
	}
}

func TestKeybindingsWidgetTabNavigation(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	if w.focusedAction != -1 {
		t.Fatalf("expected focusedAction=-1, got %d", w.focusedAction)
	}

	// Tab cycles through footer buttons: 0=Cancel, 1=Edit, 2=Reset, 3=Clear, 4=Help
	w.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	if w.focusedAction != 0 {
		t.Errorf("expected focusedAction=0 after first Tab, got %d", w.focusedAction)
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	if w.focusedAction != 1 {
		t.Errorf("expected focusedAction=1 after second Tab, got %d", w.focusedAction)
	}

	// Tab wraps around
	w.focusedAction = 4
	w.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	if w.focusedAction != 0 {
		t.Errorf("expected focusedAction=0 after wrap, got %d", w.focusedAction)
	}
}

func TestKeybindingsWidgetShiftTab(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())

	// Shift+Tab wraps to last
	w.HandleEvent(tcell.NewEventKey(tcell.KeyBacktab, "", tcell.ModShift))
	if w.focusedAction != 4 {
		t.Errorf("expected focusedAction=4 after Shift+Tab from -1, got %d", w.focusedAction)
	}

	// Shift+Tab goes back
	w.HandleEvent(tcell.NewEventKey(tcell.KeyBacktab, "", tcell.ModShift))
	if w.focusedAction != 3 {
		t.Errorf("expected focusedAction=3 after Shift+Tab, got %d", w.focusedAction)
	}
}

func TestKeybindingsWidgetTabEnterActivatesEdit(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	var editedCmd string
	w.OnEdit = func(cmdID, newKey string) {
		editedCmd = cmdID
	}

	// Tab to Edit (index 1), then Enter
	w.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	w.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	if w.focusedAction != 1 {
		t.Fatalf("expected focusedAction=1, got %d", w.focusedAction)
	}
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !w.recording {
		t.Fatal("expected recording mode after Enter on Edit button")
	}
	_ = editedCmd
}

func TestKeybindingsWidgetTabEnterActivatesCancel(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	dismissed := false
	w.OnDismiss = func() {
		dismissed = true
	}

	// Tab to Cancel (index 0), then Enter
	w.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	if w.focusedAction != 0 {
		t.Fatalf("expected focusedAction=0, got %d", w.focusedAction)
	}
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !dismissed {
		t.Fatal("expected dismiss after Enter on Cancel button")
	}
}

func TestKeybindingsWidgetEscapeFromButton(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())
	dismissed := false
	w.OnDismiss = func() {
		dismissed = true
	}

	// Tab to a button, then Escape returns to input
	w.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone))
	if w.focusedAction != -1 {
		t.Errorf("expected focusedAction=-1 after Escape, got %d", w.focusedAction)
	}
	if dismissed {
		t.Fatal("Escape from button should return to input, not dismiss")
	}

	// Second Escape dismisses
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone))
	if !dismissed {
		t.Fatal("second Escape should dismiss")
	}
}

func TestKeybindingsWidgetTypingReturnsFocus(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())

	// Tab to button, then type a character
	w.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	if w.focusedAction != 0 {
		t.Fatalf("expected focusedAction=0, got %d", w.focusedAction)
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModNone))
	if w.focusedAction != -1 {
		t.Errorf("expected focusedAction=-1 after typing, got %d", w.focusedAction)
	}
}

func TestKeybindingsWidgetArrowReturnsFocus(t *testing.T) {
	w := NewKeybindingsWidget(kbTestCommands())

	w.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	w.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if w.focusedAction != -1 {
		t.Errorf("expected focusedAction=-1 after Down, got %d", w.focusedAction)
	}
}

func TestDescribeKeyCombo(t *testing.T) {
	tests := []struct {
		key  tcell.Key
		str  string
		mod  tcell.ModMask
		want string
	}{
		{tcell.KeyCtrlS, "", tcell.ModCtrl, "ctrl+s"},
		{tcell.KeyRune, "a", tcell.ModNone, "a"},
		{tcell.KeyRune, "a", tcell.ModAlt, "alt+a"},
		{tcell.KeyRune, "a", tcell.ModMeta, "super+a"},
		{tcell.KeyRune, "a", tcell.ModCtrl | tcell.ModMeta, "ctrl+super+a"},
		{tcell.KeyLeft, "", tcell.ModMeta, "super+left"},
		{tcell.KeyF1, "", tcell.ModNone, "f1"},
		{tcell.KeyEnter, "", tcell.ModNone, "enter"},
		{tcell.KeyTab, "", tcell.ModNone, "tab"},
		{tcell.KeyUp, "", tcell.ModNone, "up"},
		{tcell.KeyRune, " ", tcell.ModNone, "space"},
	}

	for _, tt := range tests {
		ev := tcell.NewEventKey(tt.key, tt.str, tt.mod)
		got := describeKeyCombo(ev)
		if got != tt.want {
			t.Errorf("describeKeyCombo(%v) = %q, want %q", tt, got, tt.want)
		}
	}
}

func FuzzSuperKeyRecording(f *testing.F) {
	f.Add(uint8(0), uint8(0))
	f.Add(uint8(25), uint8(7))
	f.Fuzz(func(t *testing.T, letter, extra uint8) {
		ch := 'a' + rune(letter%26)
		mod := tcell.ModMeta
		if extra&1 != 0 {
			mod |= tcell.ModCtrl
		}
		if extra&2 != 0 {
			mod |= tcell.ModAlt
		}
		if extra&4 != 0 {
			mod |= tcell.ModShift
		}
		ev := tcell.NewEventKey(tcell.KeyRune, string(ch), mod)
		tester := NewKeyTesterWidget()
		tester.HandleEvent(ev)
		for _, recorded := range []string{describeKeyCombo(ev), tester.lines[0]} {
			steps, err := config.ParseKeyString(recorded)
			if err != nil {
				t.Fatal(err)
			}
			if len(steps) != 1 || steps[0].Rune != ch || !steps[0].Super ||
				steps[0].Ctrl != (mod&tcell.ModCtrl != 0) ||
				steps[0].Alt != (mod&tcell.ModAlt != 0) ||
				steps[0].Shift != (mod&tcell.ModShift != 0) {
				t.Fatalf("recording %q lost key or modifiers from %v", recorded, ev)
			}
		}
	})
}
