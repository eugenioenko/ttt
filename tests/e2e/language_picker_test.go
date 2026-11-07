package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestLanguagePickerPaletteAndStatusBar(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	path := filepath.Join(h.dir, ".bash_aliases")
	if err := os.WriteFile(path, []byte("alias ll='ls -la'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := h.app.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.CommitActiveTab()
	h.app.FocusEditor()
	h.pressCtrl(tcell.KeyCtrlP)
	for _, r := range "Change Language Mode" {
		h.pressRune(r)
	}
	h.pressKey(tcell.KeyEnter, 0)
	h.assertContains("Auto Detect (current)")
	for _, r := range "Shell" {
		h.pressRune(r)
	}
	if h.app.EditorGroup.Editor.Highlighter != nil {
		t.Fatal("filtering previewed a language before confirmation")
	}
	h.pressKey(tcell.KeyDown, 0)
	h.pressKey(tcell.KeyDown, 0)
	h.pressKey(tcell.KeyEnter, 0)
	h.settle()
	if got := h.app.EditorGroup.Editor.Highlighter; got == nil || got.Language() != "Shell" {
		t.Fatalf("palette selection = %v", got)
	}
	status := h.screenRow(29)
	x := displayColumnOf(status, "Shell")
	if x < 0 {
		t.Fatalf("language is missing from status bar: %s", status)
	}
	h.click(x+1, 29)
	for _, r := range "Shell" {
		h.pressRune(r)
	}
	h.assertContains("Shell (current)")
	h.pressKey(tcell.KeyEscape, 0)
	if h.app.EditorGroup.Editor.Highlighter.Language() != "Shell" {
		t.Fatal("cancelling the picker changed the language")
	}
	h.click(x+1, 29)
	for _, r := range "Plain Text" {
		h.pressRune(r)
	}
	h.pressKey(tcell.KeyEnter, 0)
	h.settle()
	if h.app.EditorGroup.Editor.Highlighter != nil || !strings.Contains(h.screenRow(29), "Plain Text") {
		t.Fatal("status-bar selection did not apply Plain Text")
	}
	h.app.EditorGroup.OpenBufferReadOnly("Go source", "main.go", []string{"package main"})
	h.app.SyncLanguageSegment()
	h.settle()
	if !strings.Contains(h.screenRow(29), "Go") {
		t.Fatal("second tab did not show its own language")
	}
	h.app.EditorGroup.SwitchTab(0)
	h.app.SyncLanguageSegment()
	h.settle()
	if !strings.Contains(h.screenRow(29), "Plain Text") {
		t.Fatal("tab switch lost the selected language")
	}
	h.exec("editor.changeLanguage")
	for _, r := range "Auto Detect" {
		h.pressRune(r)
	}
	h.pressKey(tcell.KeyEnter, 0)
	if h.app.EditorGroup.ActiveLanguageSelection() != "" || h.app.EditorGroup.ActiveBuffer().Dirty {
		t.Fatal("Auto Detect changed document contents or retained an override")
	}
}
