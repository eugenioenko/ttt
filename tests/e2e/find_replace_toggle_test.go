package e2e

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestFindReplaceTogglePreservesInput(t *testing.T) {
	for _, start := range []string{"search.find", "search.replace"} {
		t.Run(start, func(t *testing.T) {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			h.exec("file.new")
			pasteText(h, pasteKeys("日foo 日foo 日FOO"))
			h.exec(start)
			if start == "search.replace" {
				h.click(81, 5)
			}
			find := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
			find.Input.SetText("日foo")
			h.app.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "c", tcell.ModAlt))
			h.app.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "r", tcell.ModAlt))
			h.pressKey(tcell.KeyEnter, tcell.ModNone)
			h.pressKey(tcell.KeyHome, tcell.ModNone)
			h.pressKey(tcell.KeyRight, tcell.ModShift)
			cursor, _, _ := find.Input.CursorPosition()
			current := find.Current
			options := find.Options
			h.click(81, 5)
			replace, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
			if !ok {
				t.Fatalf("toggle opened %T", h.app.Root.TopOverlayWidget())
			}
			replaceCursor, _, _ := replace.SearchInput.CursorPosition()
			if replace.SearchInput.Text() != "日foo" || replaceCursor != cursor || replace.Options != options || replace.Current != current || h.app.EditorGroup.Editor.SearchActive != current {
				t.Fatalf("find state lost: query=%q cursor=%d options=%+v current=%d", replace.SearchInput.Text(), replaceCursor, replace.Options, replace.Current)
			}
			h.pressRune('界')
			if replace.SearchInput.Text() != "界foo" || len(replace.Matches) != 0 {
				t.Fatalf("search selection or callback lost: %q %+v", replace.SearchInput.Text(), replace.Matches)
			}
			replace.SearchInput.SetText("日foo")
			h.pressKey(tcell.KeyTab, tcell.ModNone)
			pasteText(h, pasteKeys("replacement"))
			h.click(81, 5)
			find = h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
			if find.Input.Text() != "日foo" || find.Options != options {
				t.Fatal("collapse lost query or options")
			}
			find.Input.SetText("foo")
			if len(find.Matches) != 2 {
				t.Fatalf("find callback after collapse: %+v", find.Matches)
			}
			h.click(81, 5)
			replace = h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
			if replace.SearchInput.Text() != "foo" || replace.ReplaceInput.Text() != "replacement" || replace.Options != options {
				t.Fatal("expand lost search or replacement text or options")
			}
			hit := replace.ReplaceInput.ActionHits[1]
			h.click(hit.X, hit.Y)
			if got := editorText(h); got != "日replacement 日replacement 日FOO" {
				t.Fatalf("replace all after toggling = %q", got)
			}

		})
	}
}

func TestFindReplaceShortcutSwitchesMode(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	h.exec("search.find")
	find := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	find.Input.SetText("query")
	h.exec("search.replace")
	replace := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	replace.ReplaceInput.SetText("replacement")
	h.exec("search.find")
	h.exec("search.replace")
	replace = h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if replace.SearchInput.Text() != "query" || replace.ReplaceInput.Text() != "replacement" {
		t.Fatal("shortcuts lost input")
	}
}

func TestFindReplaceConfiguredIcons(t *testing.T) {
	for _, mode := range []string{config.IconsNone, config.IconsNerdFont} {
		t.Run(mode, func(t *testing.T) {
			h := newTestHarness(t, 60, 25)
			defer h.stop()
			icons.Configure(mode, map[string]string{"chevron-right": ">", "chevron-down": "v", "replace-all": "@"})
			defer icons.Configure("", nil)
			h.exec("file.new")
			h.exec("search.find")
			if got := h.screenText(); !strings.Contains(got, " > Search") {
				t.Fatalf("configured collapsed chevron missing:\n%s", got)
			}
			h.click(21, 5)
			replace, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
			if !ok {
				t.Fatalf("toggle opened %T", h.app.Root.TopOverlayWidget())
			}
			if got := h.screenText(); !strings.Contains(got, " v Search") || strings.Contains(got, "❯") {
				t.Fatalf("wrong expanded chevron:\n%s", got)
			}
			want := "All"
			if mode == config.IconsNerdFont {
				want = "@"
			}
			if replace.ReplaceInput.Actions[1].Label != want {
				t.Fatalf("Replace All = %q, want %q", replace.ReplaceInput.Actions[1].Label, want)
			}
			h.click(21, 5)
			if _, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget); !ok {
				t.Fatal("configured expanded chevron did not collapse")
			}
		})
	}
}
