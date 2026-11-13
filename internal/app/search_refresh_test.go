package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

type searchRefreshHarness struct {
	app *App
}

func newSearchRefreshHarness(t *testing.T) *searchRefreshHarness {
	t.Helper()
	a := prepareListenTestApp(t)
	t.Cleanup(a.CloseTasks)
	path := filepath.Join(t.TempDir(), "search.txt")
	if err := os.WriteFile(path, []byte("foo bar\nfoo baz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.OpenFile(path)
	a.EditorGroup.Editor.OnChange = a.scheduleFindRefresh
	return &searchRefreshHarness{app: a}
}

func (h *searchRefreshHarness) flushOnChange() {
	h.app.EditorGroup.Editor.FlushOnChange()
}

func (h *searchRefreshHarness) pressKey(key tcell.Key, mod tcell.ModMask) {
	h.app.Root.HandleEvent(tcell.NewEventKey(key, "", mod))
	h.flushOnChange()
}

func (h *searchRefreshHarness) pressRune(r rune) {
	h.app.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	h.flushOnChange()
}

func (h *searchRefreshHarness) exec(id string) {
	h.app.Reg.Execute(id)
	h.flushOnChange()
}

func TestSearchHighlightsFollowEdits(t *testing.T) {
	h := newSearchRefreshHarness(t)

	opts := ui.SearchOptions{CaseSensitive: true}
	matches, _ := ui.FindInLines(h.app.EditorGroup.Editor.Buf.Lines, "foo", opts)
	h.app.EditorGroup.SetSearch("foo", opts, matches)
	h.app.Root.SetFocus(h.app.EditorGroup)

	for range 3 {
		h.pressKey(tcell.KeyDelete, tcell.ModNone)
	}

	got := h.app.EditorGroup.Editor.SearchMatches
	if len(got) != 1 || got[0].Line != 1 || got[0].Col != 0 {
		t.Fatalf("matches after deleting the first foo = %+v, want one match at 1:0", got)
	}
}

func TestReplaceBarMatchesFollowEdits(t *testing.T) {
	h := newSearchRefreshHarness(t)

	h.app.OpenFindReplace()
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatal("replace bar did not open")
	}
	for _, r := range "foo" {
		h.pressRune(r)
	}
	if len(bar.Matches) != 2 {
		t.Fatalf("replace bar matches = %d, want 2", len(bar.Matches))
	}

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 0
	editor.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	h.flushOnChange()

	if len(bar.Matches) != 2 || bar.Matches[0].Line != 1 || bar.Matches[1].Line != 2 {
		t.Fatalf("replace bar matches after inserting a line = %+v, want lines 1 and 2", bar.Matches)
	}
}

func TestReplaceBarAdvancesPastReplacementContainingQuery(t *testing.T) {
	for _, tc := range []struct {
		name, replacement string
		want              []string
		next              ui.FindMatch
	}{
		{"contains query", "foofoo", []string{"foofoo Bar foofoo", "foofoobar FOO"}, ui.FindMatch{Line: 1, Col: 10, Len: 3}},
		{"ends with query", "xfoo", []string{"xfoo Bar xfoo", "xfoobar FOO"}, ui.FindMatch{Line: 1, Col: 8, Len: 3}},
		{"no query", "X", []string{"X Bar X", "Xbar FOO"}, ui.FindMatch{Line: 1, Col: 5, Len: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newSearchRefreshHarness(t)
			editor := h.app.EditorGroup.Editor
			editor.Buf.SetText("foo Bar foo\nfoobar FOO\n")

			h.app.OpenFindReplace()
			bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
			if !ok {
				t.Fatal("replace bar did not open")
			}
			for _, r := range "foo" {
				h.pressRune(r)
			}
			h.pressKey(tcell.KeyTab, tcell.ModNone)
			for _, r := range tc.replacement {
				h.pressRune(r)
			}
			for range 3 {
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
			}

			if !reflect.DeepEqual(editor.Buf.Lines[:2], tc.want) {
				t.Fatalf("buffer = %q, want %q", editor.Buf.Lines[:2], tc.want)
			}
			if got := bar.Matches[bar.Current]; got != tc.next {
				t.Fatalf("current match = %+v, want %+v", got, tc.next)
			}
			if editor.Cursor.Line != tc.next.Line || editor.Cursor.Col != tc.next.Col {
				t.Fatalf("cursor = %d:%d, want %d:%d", editor.Cursor.Line, editor.Cursor.Col, tc.next.Line, tc.next.Col)
			}
		})
	}
}

func TestReplaceBarWrapsToFirstMatchAfterLastReplace(t *testing.T) {
	h := newSearchRefreshHarness(t)
	editor := h.app.EditorGroup.Editor

	h.app.OpenFindReplace()
	bar := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	for _, r := range "foo" {
		h.pressRune(r)
	}
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyTab, tcell.ModNone)
	for _, r := range "foofoo" {
		h.pressRune(r)
	}
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	if editor.Buf.Lines[0] != "foo bar" || editor.Buf.Lines[1] != "foofoo baz" {
		t.Fatalf("buffer = %q, want the second foo replaced", editor.Buf.Lines)
	}
	if want := (ui.FindMatch{Line: 0, Col: 0, Len: 3}); bar.Matches[bar.Current] != want {
		t.Fatalf("current match = %+v, want %+v", bar.Matches[bar.Current], want)
	}
}

func TestSearchRefreshPreservesOptionsAndUndo(t *testing.T) {
	for _, source := range []string{"global literal", "global regex", "plugin literal", "plugin regex"} {
		t.Run(source, func(t *testing.T) {
			h := newSearchRefreshHarness(t)
			switch source {
			case "global literal", "global regex":
				seedInputText(h.app.Search.Input, "FOO")
				if source == "global regex" {
					seedInputText(h.app.Search.Input, "FO+")
					h.app.Search.Options.UseRegex = true
				}
				h.app.NavigateToSearchMatch(h.app.EditorGroup.ActiveFilePath(), 1, 0)
			case "plugin literal", "plugin regex":
				pattern := "foo"
				if source == "plugin regex" {
					pattern = "fo+"
				}
				NewPluginEditorAPI(h.app).SetSearch(pattern, source == "plugin regex")
			}
			editor := h.app.EditorGroup.Editor
			h.app.Root.SetFocus(h.app.EditorGroup)
			want := []ui.FindMatch{{Line: 0, Col: 0, Len: 3}, {Line: 1, Col: 0, Len: 3}}
			check := func(expected []ui.FindMatch) {
				t.Helper()
				if !reflect.DeepEqual(editor.SearchMatches, expected) {
					t.Fatalf("matches = %+v, want %+v", editor.SearchMatches, expected)
				}
			}
			check(want)
			h.pressKey(tcell.KeyEnter, tcell.ModNone)
			shifted := []ui.FindMatch{{Line: 1, Col: 0, Len: 3}, {Line: 2, Col: 0, Len: 3}}
			check(shifted)
			if editor.Cursor.Line != 1 || editor.Cursor.Col != 0 {
				t.Fatalf("refresh moved cursor to %d:%d", editor.Cursor.Line, editor.Cursor.Col)
			}
			h.exec("editor.undo")
			check(want)
			h.exec("editor.redo")
			check(shifted)
			editor.ExecCommand(&undo.DeleteSelectionCommand{StartLine: 1, StartCol: 0, EndLine: 1, EndCol: 3})
			h.flushOnChange()
			check(shifted[1:])
			editor.ExecCommand(&undo.DeleteSelectionCommand{StartLine: 2, StartCol: 0, EndLine: 2, EndCol: 3})
			h.flushOnChange()
			if len(editor.SearchMatches) != 0 {
				t.Fatalf("matches after removing all occurrences = %+v", editor.SearchMatches)
			}
			editor.ExecCommand(&undo.InsertStringCommand{Line: 2, Col: 0, Text: "foo"})
			h.flushOnChange()
			check([]ui.FindMatch{{Line: 2, Col: 0, Len: 3}})
			h.app.EditorGroup.ClearSearch()
			h.pressRune('x')
			if editor.SearchQuery != "" || len(editor.SearchMatches) != 0 {
				t.Fatal("edit revived cleared search")
			}
		})
	}
}

func TestInvalidRegexDoesNotRestorePreviousSearchAfterEdit(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "find"
		if replace {
			name = "replace"
		}
		t.Run(name, func(t *testing.T) {
			h := newSearchRefreshHarness(t)
			var input *widgets.InputWidget
			var matches func() []ui.FindMatch
			if replace {
				h.app.OpenFindReplace()
				bar := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
				input = bar.SearchInput
				matches = func() []ui.FindMatch { return bar.Matches }
				bar.Options.UseRegex = true
			} else {
				h.app.OpenFind()
				bar := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
				input = bar.Input
				matches = func() []ui.FindMatch { return bar.Matches }
				bar.Options.UseRegex = true
			}
			for _, r := range "foo" {
				h.pressRune(r)
			}
			if len(matches()) != 2 {
				t.Fatalf("valid query matches = %+v", matches())
			}
			h.pressRune('[')
			if input.Text() != "foo[" || len(matches()) != 0 {
				t.Fatalf("invalid query %q matches = %+v", input.Text(), matches())
			}
			editor := h.app.EditorGroup.Editor
			editor.Cursor.Line, editor.Cursor.Col = 0, 0
			editor.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
			h.flushOnChange()
			if len(matches()) != 0 || len(editor.SearchMatches) != 0 || editor.SearchQuery != "" {
				t.Fatalf("invalid query retained search: bar=%+v editor=%+v query=%q", matches(), editor.SearchMatches, editor.SearchQuery)
			}
			if replace {
				h.pressKey(tcell.KeyTab, tcell.ModNone)
				h.pressRune('X')
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
				if editor.Buf.Lines[1] != "foo bar" || editor.Buf.Lines[2] != "foo baz" {
					t.Fatalf("invalid regex replaced text: %q", editor.Buf.Lines)
				}
			}
		})
	}
}

func TestSearchRefreshWithEmptyBar(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "find", true: "replace"}[replace], func(t *testing.T) {
			h := newSearchRefreshHarness(t)
			NewPluginEditorAPI(h.app).SetSearch("foo", false)
			h.app.EditorGroup.SetSearchActive(1)
			if replace {
				h.app.OpenFindReplace()
			} else {
				h.app.OpenFind()
			}
			editor := h.app.EditorGroup.Editor
			editor.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
			h.flushOnChange()
			want := []ui.FindMatch{{Line: 1, Col: 0, Len: 3}, {Line: 2, Col: 0, Len: 3}}
			if !reflect.DeepEqual(editor.SearchMatches, want) || editor.SearchActive != 1 {
				t.Fatalf("matches = %+v active = %d, want %+v active = 1", editor.SearchMatches, editor.SearchActive, want)
			}
		})
	}
}

func TestSidebarSearchRefreshAcrossTabChange(t *testing.T) {
	testSearchRefreshAcrossTabChange(t, "sidebar")
}

func TestPluginSearchRefreshAcrossTabChange(t *testing.T) {
	for _, source := range []string{"plugin literal", "plugin regex", "plugin empty sidebar"} {
		t.Run(source, func(t *testing.T) { testSearchRefreshAcrossTabChange(t, source) })
	}
}

func testSearchRefreshAcrossTabChange(t *testing.T, source string) {
	t.Helper()
	h := newSearchRefreshHarness(t)
	a := h.app
	firstPath := a.EditorGroup.ActiveFilePath()
	a.EditorGroup.CommitActiveTab()
	secondPath := filepath.Join(t.TempDir(), "other.txt")
	if err := os.WriteFile(secondPath, []byte("other\nxx foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.OpenFile(secondPath)
	a.EditorGroup.CommitActiveTab()
	a.EditorGroup.SwitchToTabByPath(firstPath)
	if source == "sidebar" {
		a.Sidebar.SetActivePanel("search")
		seedInputText(a.Search.Input, "foo")
		a.applySearchHighlights()
	} else {
		pattern := "foo"
		if source == "plugin regex" {
			pattern = "fo+"
		}
		NewPluginEditorAPI(a).SetSearch(pattern, source == "plugin regex")
		if source == "plugin empty sidebar" {
			a.Sidebar.SetActivePanel("search")
		}
	}
	var work, deliveries []func()
	a.tasks.post = func(fn func()) error { deliveries = append(deliveries, fn); return nil }
	a.findRefresh = NewTask[struct{}](a, asynctask.TaskOptions{Run: func(fn func()) { work = append(work, fn) }})
	editor := a.EditorGroup.Editor
	editor.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	h.flushOnChange()
	if len(work) != 1 {
		t.Fatalf("queued work = %d, want 1", len(work))
	}
	a.EditorGroup.SwitchToTabByPath(secondPath)
	work[0]()
	if len(deliveries) != 1 {
		t.Fatalf("queued deliveries = %d, want 1", len(deliveries))
	}
	deliveries[0]()
	wantSecond := []ui.FindMatch{{Line: 1, Col: 3, Len: 3}}
	if !reflect.DeepEqual(editor.SearchMatches, wantSecond) {
		t.Fatalf("second buffer matches = %+v, want %+v", editor.SearchMatches, wantSecond)
	}
	a.EditorGroup.SwitchToTabByPath(firstPath)
	wantFirst := []ui.FindMatch{{Line: 1, Col: 0, Len: 3}, {Line: 2, Col: 0, Len: 3}}
	if !reflect.DeepEqual(editor.SearchMatches, wantFirst) {
		t.Fatalf("first buffer matches after return = %+v, want %+v", editor.SearchMatches, wantFirst)
	}
}
