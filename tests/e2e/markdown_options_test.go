package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func TestMarkdownOptionsApplyAndRestoreInheritance(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	path := filepath.Join(h.dir, "readme.md")
	if err := os.WriteFile(path, []byte("Markdown text"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.exec("options.markdownWordWrap")
	h.assertContains("Use Editor Default")
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if !h.app.EditorGroup.Editor.WordWrap || h.app.Settings.Editor.WordWrap {
		t.Fatal("Markdown wrapping did not apply independently of global wrapping")
	}
	h.exec("options.markdownWrapWidth")
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.assertContains("Markdown Wrap Column")
	h.pressCtrl(tcell.KeyCtrlA)
	h.pressRune('0')
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.assertContains("Enter a column limit of 1 or greater.")
	if h.app.Settings.Editor.LanguageOverrides["markdown"].WordWrapColumn != nil {
		t.Fatal("invalid width was saved")
	}
	h.pressCtrl(tcell.KeyCtrlA)
	h.pressRune('6')
	h.pressRune('5')
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if h.app.EditorGroup.Editor.WordWrapColumn != 65 {
		t.Fatal("custom width did not apply to the open Markdown file")
	}
	loaded := config.LoadSettings()
	if enabled, column := loaded.Editor.ResolveWordWrap("markdown"); !enabled || column != 65 {
		t.Fatalf("saved Markdown wrapping = %v/%d", enabled, column)
	}
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	if h.app.EditorGroup.Editor.WordWrap {
		t.Fatal("Markdown override changed wrapping in another language")
	}

	h.exec("options.markdownWordWrap")
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if o := h.app.Settings.Editor.LanguageOverrides["markdown"]; o.WordWrap == nil || *o.WordWrap {
		t.Fatal("picker did not start on current On value or save explicit Off")
	}
	h.exec("options.markdownWordWrap")
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.exec("options.markdownWrapWidth")
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if o := h.app.Settings.Editor.LanguageOverrides["markdown"]; o.WordWrap != nil || o.WordWrapColumn == nil || *o.WordWrapColumn != 0 {
		t.Fatal("inherit wrap / explicit pane width not saved")
	}
	h.exec("options.markdownWrapWidth")
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if _, ok := config.LoadSettings().Editor.LanguageOverrides["markdown"]; ok {
		t.Fatal("restoring both defaults left a Markdown override")
	}
}

func TestMarkdownSettingsDraftAndCustomWidth(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	enabled := true
	h.app.Settings.Editor.LanguageOverrides = map[string]config.EditorLanguageOverride{
		"markdown": {WordWrap: &enabled, WordWrapBreak: config.WordWrapBreakCharacter},
		"go":       {WordWrap: &enabled},
	}
	h.exec("settings.openUI")
	clickRowControl(t, h, "Markdown", "Markdown")
	clickRowControl(t, h, "Word wrap", "On")
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if !*h.app.Settings.Editor.LanguageOverrides["markdown"].WordWrap {
		t.Fatal("unapplied draft mutated the live override")
	}
	h.exec("settings.cancel")
	h.exec("settings.openUI")
	clickRowControl(t, h, "Markdown", "Markdown")
	if !rowHas(h, "Word wrap", "On") {
		t.Fatal("Cancel failed to discard the override edit")
	}
	clickRowControl(t, h, "Wrap width", "Use Editor Default")
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.pressKey(tcell.KeyTab, tcell.ModNone)
	h.pressCtrl(tcell.KeyCtrlA)
	h.pressRune('0')
	h.exec("settings.apply")
	h.assertContains("Invalid value")
	if h.app.Settings.Editor.LanguageOverrides["markdown"].WordWrapColumn != nil {
		t.Fatal("invalid custom width applied")
	}
	h.pressCtrl(tcell.KeyCtrlA)
	h.pressRune('7')
	h.pressRune('2')
	h.exec("settings.apply")
	loaded := config.LoadSettings()
	o := loaded.Editor.LanguageOverrides["markdown"]
	if o.WordWrapColumn == nil || *o.WordWrapColumn != 72 || o.WordWrap == nil || !*o.WordWrap || o.WordWrapBreak != config.WordWrapBreakCharacter || loaded.Editor.LanguageOverrides["go"].WordWrap == nil {
		t.Fatalf("custom width lost existing overrides: %+v", loaded.Editor.LanguageOverrides)
	}
	clickRowControl(t, h, "Wrap width", "Custom")
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	clickRowControl(t, h, "Word wrap", "On")
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.exec("settings.apply")
	o = config.LoadSettings().Editor.LanguageOverrides["markdown"]
	if o.WordWrap != nil || o.WordWrapColumn != nil || o.WordWrapBreak != config.WordWrapBreakCharacter {
		t.Fatalf("restoring defaults changed the break override: %+v", o)
	}
}
