package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func TestAutoSurroundLanguageSettingsAndReload(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	s := *h.app.Settings
	off := false
	s.Editor.SyntaxHighlight = &off
	s.Editor.AutoSurround.ExtraPairs = []config.SurroundingPair{{"!", "!"}}
	s.Editor.LanguageOverrides = map[string]config.EditorLanguageOverride{
		"markdown": {AutoSurround: &config.AutoSurroundSettings{ExtraPairs: []config.SurroundingPair{{"*", "*"}}}},
		"bat":      {AutoSurround: &config.AutoSurroundSettings{ExtraPairs: []config.SurroundingPair{{"%", "%"}}}},
	}
	h.app.ApplySettings(s)
	for _, tc := range []struct {
		file string
		key  rune
		want string
	}{
		{"a.md", '*', "*word*"}, {"a.bat", '%', "%word%"}, {"a.txt", '!', "!word!"},
	} {
		path := filepath.Join(h.dir, tc.file)
		if err := os.WriteFile(path, []byte("word"), 0o644); err != nil {
			t.Fatal(err)
		}
		h.app.EditorGroup.OpenFile(path)
		h.app.FocusEditor()
		h.redraw()
		h.pressKey(tcell.KeyHome, tcell.ModCtrl)
		h.pressKey(tcell.KeyEnd, tcell.ModShift)
		h.pressRune(tc.key)
		if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != tc.want {
			t.Fatalf("%s: %q, want %q", tc.file, got, tc.want)
		}
	}
	h.app.EditorGroup.SwitchToTabByPath(filepath.Join(h.dir, "a.md"))
	h.exec("editor.undo")
	h.pressKey(tcell.KeyHome, tcell.ModCtrl)
	h.pressKey(tcell.KeyEnd, tcell.ModShift)
	h.pressRune('*')
	if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != "*word*" {
		t.Fatalf("tab switch: %q", got)
	}
	h.app.EditorGroup.SetActiveLanguage("bat")
	h.pressRune('%')
	if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != "*%word%*" {
		t.Fatalf("manual language: %q", got)
	}
	h.exec("editor.split")
	s.Editor.AutoSurround.Enabled = &off
	if err := config.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	h.exec("settings.reload")
	for _, group := range h.app.EditorSplit.Groups() {
		if group.Editor.AutoSurround.IsEnabled() {
			t.Fatal("reload did not disable wrapping in every split")
		}
	}
	h.app.FocusEditor()
	h.typeText("word")
	h.pressKey(tcell.KeyHome, tcell.ModCtrl)
	h.pressKey(tcell.KeyEnd, tcell.ModShift)
	h.pressRune('(')
	if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != "(" {
		t.Fatalf("disabled: %q", got)
	}
}

func TestAutoSurroundSettingsForm(t *testing.T) {
	h := newTestHarness(t, 120, 60)
	h.exec("settings.openUI")
	clickRowControl(t, h, "Editor", "Editor")
	clickRowControl(t, h, "Surround selected text", checkedBox)
	h.exec("settings.apply")
	if h.app.Settings.Editor.AutoSurround.IsEnabled() || config.LoadSettings().Editor.AutoSurround.IsEnabled() {
		t.Fatal("form did not apply and save opt-out")
	}
}
