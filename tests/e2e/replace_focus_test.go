package e2e

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestReplaceBarEditorFocus(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	h.pressCtrl(tcell.KeyCtrlR)
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatalf("expected ReplaceBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	h.pressRune('a')
	h.pressKey(tcell.KeyTab, tcell.ModNone)
	h.pressRune('b')

	h.click(6, 4)
	if bar.FocusedInput() != nil {
		t.Fatal("replace bar kept input focus after clicking the editor")
	}
	if _, _, visible := bar.CursorPosition(); visible {
		t.Fatal("replace bar kept its cursor after clicking the editor")
	}
	h.pressRune('x')
	pasteText(h, pasteKeys("yz"))
	if got := editorText(h); got != "xyz" {
		t.Fatalf("editor text = %q, want xyz", got)
	}
	if bar.SearchInput.Text != "a" || bar.ReplaceInput.Text != "b" {
		t.Fatal("editor input changed the search or replacement")
	}
	h.pressCtrl(tcell.KeyCtrlZ)
	if got := editorText(h); got != "x" {
		t.Fatalf("editor text after undo = %q, want x", got)
	}
	if h.app.Root.TopOverlayWidget() != bar {
		t.Fatal("editing closed the replace bar")
	}

	h.click(84, 5)
	if bar.FocusedInput() != bar.SearchInput {
		t.Fatal("clicking search did not restore input focus")
	}
	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	h.pressRune('c')
	if bar.SearchInput.Text != "ac" {
		t.Fatalf("search input = %q, want ac", bar.SearchInput.Text)
	}
	h.pressCtrl(tcell.KeyCtrlZ)
	if got := editorText(h); got != "x" {
		t.Fatalf("undo in search input changed editor text to %q", got)
	}
	h.click(84, 6)
	if bar.FocusedInput() != bar.ReplaceInput {
		t.Fatal("clicking replacement did not restore input focus")
	}
	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	pasteText(h, pasteKeys("d"))
	if bar.ReplaceInput.Text != "bd" {
		t.Fatalf("replacement = %q, want bd", bar.ReplaceInput.Text)
	}
	h.pressCtrl(tcell.KeyCtrlZ)
	if got := editorText(h); got != "x" {
		t.Fatalf("undo in replacement input changed editor text to %q", got)
	}

	h.click(6, 4)
	h.pressCtrl(tcell.KeyCtrlR)
	if h.app.Root.TopOverlayWidget() != bar || len(h.app.Root.Overlays) != 1 || bar.FocusedInput() != bar.ReplaceInput {
		t.Fatal("Ctrl+R did not refocus the existing replace bar")
	}
	h.pressRune('e')
	if bar.ReplaceInput.Text != "bde" {
		t.Fatalf("replacement after Ctrl+R = %q, want bde", bar.ReplaceInput.Text)
	}
	h.click(6, 4)
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if h.app.Root.HasOverlay() {
		t.Fatal("Escape in the editor did not dismiss the replace bar")
	}
}

func TestReplaceBarRefreshesBeforeReplacement(t *testing.T) {
	for _, action := range []string{"enter", "button"} {
		for _, tc := range []struct {
			name, initial, edited, want string
			current, wantCurrent        int
			noMatches                   bool
		}{
			{name: "moved", initial: "foo tail", edited: "Xfoo tail", want: "Xbar tail"},
			{name: "moved_line", initial: "foo tail", edited: "header\nfoo tail", want: "header\nbar tail"},
			{name: "removed", initial: "foo tail", edited: "plain tail", want: "plain tail", noMatches: true},
			{name: "preserves_current", initial: "foo foo foo", edited: "Xfoo foo foo", want: "Xfoo bar foo", current: 1, wantCurrent: 1},
			{name: "clamps_current", initial: "foo foo foo", edited: "Xfoo foo", want: "Xfoo bar", current: 2},
		} {
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				h := newTestHarness(t, 120, 30)
				defer h.stop()
				h.exec("file.new")
				pasteText(h, pasteKeys(tc.initial))
				h.pressCtrl(tcell.KeyCtrlR)
				bar := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
				pasteText(h, pasteKeys("foo"))
				for range tc.current {
					h.pressKey(tcell.KeyEnter, tcell.ModNone)
				}
				h.pressKey(tcell.KeyTab, tcell.ModNone)
				pasteText(h, pasteKeys("bar"))
				stale := slices.Clone(bar.Matches)
				h.click(6, 4)
				h.pressCtrl(tcell.KeyCtrlA)
				pasteText(h, pasteKeys(tc.edited))
				if !slices.Equal(bar.Matches, stale) || bar.Current != tc.current {
					t.Fatalf("expected stale matches %v at index %d, got %v at index %d", stale, tc.current, bar.Matches, bar.Current)
				}
				editor := h.app.EditorGroup.Editor
				line, col := editor.Cursor.Line, editor.Cursor.Col
				replacements, navigations := 0, 0
				onReplace, onNavigate := bar.OnReplace, bar.OnNavigate
				bar.OnReplace = func(match ui.FindMatch, replacement string) {
					if editor.Cursor.Line != line || editor.Cursor.Col != col {
						t.Fatal("pre-replacement refresh moved the editor cursor")
					}
					replacements++
					onReplace(match, replacement)
				}
				bar.OnNavigate = func(match ui.FindMatch) {
					if replacements == 0 {
						t.Fatal("pre-replacement refresh navigated to a match")
					}
					navigations++
					onNavigate(match)
				}

				if action == "button" {
					hit := bar.ReplaceInput.ActionHits[0]
					h.click(hit.X, hit.Y)
				} else {
					h.click(84, 6)
					h.pressKey(tcell.KeyEnter, tcell.ModNone)
				}
				if got := editorText(h); got != tc.want {
					t.Fatalf("replacement after edit = %q, want %q", got, tc.want)
				}
				wantReplacements := 1
				if tc.noMatches {
					wantReplacements = 0
					if editor.Cursor.Line != line || editor.Cursor.Col != col {
						t.Fatal("replacement without matches moved the editor cursor")
					}
				}
				if replacements != wantReplacements {
					t.Fatalf("replacement calls = %d, want %d", replacements, wantReplacements)
				}
				wantMatches, err := ui.FindInLines(editor.Buf.Lines, "foo", ui.SearchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(bar.Matches, wantMatches) || !slices.Equal(editor.SearchMatches, wantMatches) {
					t.Fatalf("matches after replacement: bar=%v editor=%v, want %v", bar.Matches, editor.SearchMatches, wantMatches)
				}
				if bar.Current != tc.wantCurrent {
					t.Fatalf("current match after replacement = %d, want %d", bar.Current, tc.wantCurrent)
				}
				wantNavigations := 0
				if len(wantMatches) > 0 {
					wantNavigations = 1
					if editor.SearchActive != tc.wantCurrent {
						t.Fatalf("active editor match = %d, want %d", editor.SearchActive, tc.wantCurrent)
					}
				}
				if navigations != wantNavigations {
					t.Fatalf("post-replacement navigations = %d, want %d", navigations, wantNavigations)
				}
			})
		}
	}
}
