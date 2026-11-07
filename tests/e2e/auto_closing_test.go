package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func TestAutoClosingSettingsLiveApplyAndReload(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	path := filepath.Join(h.dir, "pairs.go")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.FocusEditor()
	h.redraw()
	h.pressRune('(')
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "(" {
		t.Fatalf("default typing = %q", got)
	}
	h.exec("editor.undo")
	s := *h.app.Settings
	s.Editor.AutoClosing.Brackets = true
	h.app.ApplySettings(s)
	h.pressRune('(')
	h.pressRune(')')
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "()" {
		t.Fatalf("enabled typing = %q", got)
	}
	s.Editor.AutoClosing.SkipClosing = false
	s.Editor.AutoClosing.DeletePairs = false
	s.Editor.AutoClosing.InStrings = true
	s.Editor.AutoClosing.PlainText = true
	s.Editor.AutoClosing.Quotes = true
	if err := config.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	h.exec("settings.reload")
	if got := h.app.EditorGroup.Editor.AutoClosing; got != s.Editor.AutoClosing {
		t.Fatalf("reload = %+v, want %+v", got, s.Editor.AutoClosing)
	}
	h.exec("file.new")
	h.app.FocusEditor()
	h.pressRune('"')
	h.pressKey(tcell.KeyBackspace2, 0)
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "\"" {
		t.Fatalf("plain text quote with paired deletion off = %q", got)
	}
}

func TestAutoClosingSettingsFormRoundTrip(t *testing.T) {
	h := newTestHarness(t, 120, 60)
	h.exec("settings.openUI")
	clickRowControl(t, h, "Editor", "Editor")
	for _, tc := range []struct{ label, state string }{
		{"Auto-close brackets", uncheckedBox},
		{"Auto-close quotes", uncheckedBox},
		{"Skip generated closers", checkedBox},
		{"Delete empty generated pairs", checkedBox},
		{"Auto-close inside strings", uncheckedBox},
		{"Auto-close in plain text", uncheckedBox},
	} {
		if !rowHas(h, tc.label, tc.state) {
			t.Fatalf("missing default %s %s:\n%s", tc.label, tc.state, h.screenText())
		}
		clickRowControl(t, h, tc.label, tc.state)
	}
	h.exec("settings.apply")
	want := config.AutoClosingSettings{Brackets: true, Quotes: true, InStrings: true, PlainText: true}
	if got := h.app.Settings.Editor.AutoClosing; got != want {
		t.Fatalf("applied = %+v, want %+v", got, want)
	}
	if got := config.LoadSettings().Editor.AutoClosing; got != want {
		t.Fatalf("saved = %+v, want %+v", got, want)
	}
	h.exec("settings.openUI")
	clickRowControl(t, h, "Editor", "Editor")
	if !rowHas(h, "Auto-close brackets", checkedBox) || !rowHas(h, "Skip generated closers", uncheckedBox) {
		t.Fatal("settings form did not preserve saved values")
	}
}
