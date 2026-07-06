package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/spell"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v2"
)

func setupSpellTest(t *testing.T, name, content string) *testHarness {
	t.Helper()
	if !spell.Available() {
		t.Skip("aspell not installed")
	}
	h := newTestHarness(t, 100, 24)
	enabled := true
	h.app.Settings.Spell.Enabled = &enabled
	h.app.Settings.Spell.Lang = "en_US"
	f := filepath.Join(h.dir, name)
	os.WriteFile(f, []byte(content), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()
	return h
}

// runSpellCheck triggers an async spell check and pumps the posted result
// through the same handler the event loop uses.
func runSpellCheck(h *testHarness) {
	h.t.Helper()
	h.app.RequestSpellCheck()
	watchdog := time.AfterFunc(10*time.Second, func() {
		h.screen.PostEvent(tcell.NewEventInterrupt(nil))
	})
	defer watchdog.Stop()
	for {
		ev := h.screen.PollEvent()
		iev, ok := ev.(*tcell.EventInterrupt)
		if !ok {
			continue
		}
		res, ok := iev.Data().(*app.SpellResult)
		if !ok {
			h.t.Fatal("timed out waiting for SpellResult")
		}
		if res.Err != "" {
			h.t.Fatalf("spell check failed: %s", res.Err)
		}
		h.app.HandleSpellResult(res)
		h.redraw()
		return
	}
}

func TestSpellCheckMarksMisspellings(t *testing.T) {
	h := setupSpellTest(t, "typo.md", "# My Documnet\n\nThis is a smiple test.\n")
	defer h.stop()

	runSpellCheck(h)

	editor := h.app.EditorGroup.Editor
	if len(editor.Misspellings) != 2 {
		t.Fatalf("expected 2 misspellings, got %+v", editor.Misspellings)
	}
	m := editor.MisspellingAt(0, 7)
	if m == nil || m.Word != "Documnet" || m.Col != 5 {
		t.Fatalf("MisspellingAt(0,7) = %+v, want Documnet at col 5", m)
	}
	if editor.MisspellingAt(0, 3) != nil {
		t.Error("MisspellingAt(0,3) matched outside the word")
	}
	if len(m.Suggestions) == 0 {
		t.Error("expected suggestions for Documnet")
	}

	// The misspelled cells carry the spell underline; correct cells do not.
	cells := make([][]term.Cell, h.app.Root.Height)
	for y := range cells {
		cells[y] = make([]term.Cell, h.app.Root.Width)
	}
	h.app.Root.Render(cells)
	r := editor.GetRect()
	gw := editor.GutterWidth()
	row := cells[r.Y]
	if got := row[r.X+gw+5].UlStyle; got != term.StyleSpellError {
		t.Errorf("cell in Documnet: UlStyle = %v, want StyleSpellError", got)
	}
	if got := row[r.X+gw+2].UlStyle; got != 0 {
		t.Errorf("cell in 'My': UlStyle = %v, want 0", got)
	}
}

func TestSpellSuggestAppliesCorrection(t *testing.T) {
	h := setupSpellTest(t, "typo.md", "a smiple test\n")
	defer h.stop()

	runSpellCheck(h)

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 3
	h.exec("spell.suggest")
	if h.app.EditorGroup.Autocomplete == nil {
		t.Fatal("expected suggestion popup")
	}

	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if h.app.EditorGroup.Autocomplete != nil {
		t.Error("expected popup dismissed after accept")
	}
	line := editor.Buf.Lines[0]
	if strings.Contains(line, "smiple") {
		t.Errorf("expected smiple replaced, got %q", line)
	}
	if !strings.HasPrefix(line, "a ") || !strings.HasSuffix(line, " test") {
		t.Errorf("replacement damaged surrounding text: %q", line)
	}

	h.exec("editor.undo")
	if got := editor.Buf.Lines[0]; got != "a smiple test" {
		t.Errorf("undo did not restore original line: %q", got)
	}
}

func TestSpellCheckSkipsCodeFiles(t *testing.T) {
	h := setupSpellTest(t, "main.go", "package main // a smiple typo\n")
	defer h.stop()

	// Code filetypes resolve to no aspell mode: the request completes
	// synchronously with no goroutine and clears any stale spans.
	h.app.EditorGroup.Editor.Misspellings = []ui.SpellSpan{{Line: 0, Col: 0, Len: 1, Word: "x"}}
	h.app.RequestSpellCheck()
	if got := h.app.EditorGroup.Editor.Misspellings; len(got) != 0 {
		t.Errorf("expected no misspellings for Go file, got %+v", got)
	}
}

func TestToggleSpellCheckClearsSpans(t *testing.T) {
	h := setupSpellTest(t, "typo.md", "a smiple test\n")
	defer h.stop()

	runSpellCheck(h)
	if len(h.app.EditorGroup.Editor.Misspellings) == 0 {
		t.Fatal("expected misspellings before toggle")
	}

	h.exec("options.toggleSpellCheck")
	if h.app.Settings.Spell.IsEnabled() {
		t.Error("expected spell check disabled after toggle")
	}
	if got := h.app.EditorGroup.Editor.Misspellings; len(got) != 0 {
		t.Errorf("expected spans cleared after disable, got %+v", got)
	}
}
