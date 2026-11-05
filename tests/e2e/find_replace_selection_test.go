package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestFindPrefilledWithSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("hello world hello"))

	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 6)
	editor.Cursor.Line = 0
	editor.Cursor.Col = 11
	editor.Selection.Active = true

	h.exec("search.find")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatalf("expected FindBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.Input.Text != "world" {
		t.Fatalf("find input = %q, want %q", bar.Input.Text, "world")
	}
	if len(bar.Matches) != 1 || bar.Matches[0].Col != 6 {
		t.Fatalf("unexpected matches: %+v", bar.Matches)
	}
}

func TestReplacePrefilledWithSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("apple banana apple"))

	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 0)
	editor.Cursor.Line = 0
	editor.Cursor.Col = 5
	editor.Selection.Active = true

	h.exec("search.replace")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatalf("expected ReplaceBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.SearchInput.Text != "apple" {
		t.Fatalf("replace search input = %q, want %q", bar.SearchInput.Text, "apple")
	}
	if len(bar.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(bar.Matches))
	}
}

func TestFindRefocusUpdatesSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("alpha beta gamma"))

	// Open Find initially without selection
	h.exec("search.find")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatalf("expected FindBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.Input.Text != "" {
		t.Fatalf("initial find input = %q, want empty", bar.Input.Text)
	}

	// Click editor, select "beta", then re-run search.find
	h.click(6, 4)
	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 6)
	editor.Cursor.Line = 0
	editor.Cursor.Col = 10
	editor.Selection.Active = true

	h.exec("search.find")
	if bar.Input.Text != "beta" {
		t.Fatalf("find input after refocus = %q, want %q", bar.Input.Text, "beta")
	}
	if len(bar.Matches) != 1 || bar.Matches[0].Col != 6 {
		t.Fatalf("unexpected matches after refocus: %+v", bar.Matches)
	}
}

func TestReplaceRefocusUpdatesSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("cat dog cat"))

	// Open Replace initially without selection
	h.exec("search.replace")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatalf("expected ReplaceBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.SearchInput.Text != "" {
		t.Fatalf("initial replace search input = %q, want empty", bar.SearchInput.Text)
	}

	// Click editor, select "cat", then re-run search.replace
	h.click(6, 4)
	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 0)
	editor.Cursor.Line = 0
	editor.Cursor.Col = 3
	editor.Selection.Active = true

	h.exec("search.replace")
	if bar.SearchInput.Text != "cat" {
		t.Fatalf("replace search input after refocus = %q, want %q", bar.SearchInput.Text, "cat")
	}
	if len(bar.Matches) != 2 {
		t.Fatalf("expected 2 matches after refocus, got %d", len(bar.Matches))
	}
}

func TestFindMultilineSelectionIgnored(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("first line\nsecond line"))

	// Multi-line selection spanning line 0 to line 1
	editor := h.app.EditorGroup.Editor
	editor.Selection.Start(0, 0)
	editor.Cursor.Line = 1
	editor.Cursor.Col = 5
	editor.Selection.Active = true

	h.exec("search.find")
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatalf("expected FindBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.Input.Text != "" {
		t.Fatalf("multi-line selection should not populate single-line input, got %q", bar.Input.Text)
	}
}

func TestDiffFindPrefilledWithSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.app.EditorGroup.OpenDiff("code.txt", diff.FileDiff{}, []string{"keep", "old needle"}, []string{"keep", "new needle"}, true)
	h.redraw()
	dv := h.app.EditorGroup.ActiveDiffWidget()
	if dv == nil {
		t.Fatal("expected ActiveDiffWidget")
	}

	p := dv.KeyPane()
	p.Selection.Start(1, 4)
	p.Cursor.Line = 1
	p.Cursor.Col = 10
	p.Selection.Active = true

	h.app.OpenFind()
	h.redraw()

	bar, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatalf("expected FindBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	if bar.Input.Text != "needle" {
		t.Fatalf("diff find input = %q, want %q", bar.Input.Text, "needle")
	}
	if len(dv.SearchMatchesLeft) != 1 || len(dv.SearchMatchesRight) != 1 {
		t.Fatalf("find matches left %v right %v, want one per side", dv.SearchMatchesLeft, dv.SearchMatchesRight)
	}
	if !p.Selection.Active {
		t.Fatal("expected diff selection to remain active after prefilling find bar")
	}
}

func TestSearchPrefillSurvivesRepeatedShortcut(t *testing.T) {
	for _, command := range []string{"search.find", "search.replace"} {
		t.Run(command, func(t *testing.T) {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			h.exec("file.new")
			pasteText(h, pasteKeys("beta beta"))
			editor := h.app.EditorGroup.Editor
			editor.Selection.Start(0, 5)
			editor.Cursor.Col = 9

			for range 3 {
				h.exec(command)
				var query string
				switch bar := h.app.Root.TopOverlayWidget().(type) {
				case *ui.FindBarWidget:
					query = bar.Input.Text
				case *ui.ReplaceBarWidget:
					query = bar.SearchInput.Text
				default:
					t.Fatalf("unexpected overlay: %T", bar)
				}
				if query != "beta" {
					t.Fatalf("query after shortcut = %q, want beta", query)
				}
			}
		})
	}
}

func TestReplacePrefillRequiresEditableSelection(t *testing.T) {
	for _, kind := range []string{"diff", "read-only", "diff-base", "diff-base-after-find"} {
		t.Run(kind, func(t *testing.T) {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			h.exec("file.new")
			pasteText(h, pasteKeys("needle retained"))
			editor := h.app.EditorGroup.Editor
			cursor := *editor.Cursor

			var pane *ui.EditorPaneWidget
			switch kind {
			case "diff":
				h.app.EditorGroup.OpenDiff("code.txt", diff.FileDiff{}, []string{"old needle"}, []string{"new needle"}, true)
				pane = h.app.EditorGroup.ActiveDiffWidget().KeyPane()
			case "read-only":
				h.app.EditorGroup.OpenBufferReadOnly("Read only", "readonly.txt", []string{"new needle"})
				pane = editor
			case "diff-base", "diff-base-after-find":
				path := h.app.EditorGroup.ActiveFilePath()
				if !h.app.EditorGroup.EnableInlineDiff(path, []string{"old needle"}) {
					t.Fatal("could not open editable diff")
				}
				h.redraw()
				dv := h.app.EditorGroup.ActiveInlineDiff()
				dv.SetMode(ui.DiffModeSplit)
				h.redraw()
				r := dv.GetRect()
				h.click(r.X+5, r.Y)
				pane = dv.KeyPane()
				if !pane.ReadOnly {
					t.Fatal("expected read-only base pane")
				}
			}
			h.redraw()
			pane.Selection.Start(0, 4)
			pane.Cursor.Line, pane.Cursor.Col = 0, 10
			if kind == "read-only" {
				cursor = *editor.Cursor
			}
			if kind == "diff-base-after-find" {
				h.exec("search.find")
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
				h.pressKey(tcell.KeyEscape, tcell.ModNone)
				if h.app.EditorGroup.ActiveInlineDiff().KeyPane().ReadOnly || !pane.Selection.Active {
					t.Fatal("expected writable pane focus with the base selection retained")
				}
				cursor = *editor.Cursor
			}

			for range 2 {
				h.exec("search.replace")
				bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
				if !ok {
					t.Fatalf("unexpected overlay: %T", h.app.Root.TopOverlayWidget())
				}
				if bar.SearchInput.Text != "" || len(bar.Matches) != 0 {
					t.Fatalf("read-only selection populated Replace: query=%q matches=%v", bar.SearchInput.Text, bar.Matches)
				}
				if *editor.Cursor != cursor {
					t.Fatalf("Replace moved the retained editor cursor: got %+v, want %+v", *editor.Cursor, cursor)
				}
				if !pane.Selection.Active {
					t.Fatal("Replace cleared the read-only selection")
				}
			}
		})
	}
}

func TestGlobalSearchPrefillsSelection(t *testing.T) {
	for _, command := range []string{"sidebar.search", "sidebar.searchReplace"} {
		t.Run(command, func(t *testing.T) {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			h.exec("file.new")
			pasteText(h, pasteKeys("alpha beta\ngamma"))
			editor := h.app.EditorGroup.Editor
			search := h.app.Search
			search.Debounce.DelayMs = 60_000
			t.Cleanup(search.Debounce.Stop)
			search.Input.Text = "previous query"
			search.ReplaceInput.Text = "replacement"
			search.Include.Text = "*.go"
			search.Exclude.Text = "vendor"
			search.Options.CaseSensitive = true
			editor.Selection.Start(0, 0)
			editor.Cursor.Line, editor.Cursor.Col = 0, 5
			cursor := *editor.Cursor

			h.exec(command)
			if search.Input.Text != "alpha" || !search.Searching || h.app.Root.Focused != search {
				t.Fatalf("global prefill: query=%q searching=%v focus=%T", search.Input.Text, search.Searching, h.app.Root.Focused)
			}
			if *editor.Cursor != cursor || !editor.Selection.Active {
				t.Fatal("global prefill changed the editor selection")
			}
			if search.IsReplaceMode() != (command == "sidebar.searchReplace") {
				t.Fatal("global search opened in the wrong mode")
			}

			editor.Selection.Start(0, 6)
			editor.Cursor.Col = 10
			h.exec(command)
			if search.Input.Text != "beta" {
				t.Fatalf("query after new selection = %q, want beta", search.Input.Text)
			}
			editor.Selection.Clear()
			h.exec(command)
			if search.Input.Text != "beta" {
				t.Fatal("empty selection overwrote the query")
			}
			editor.Selection.Start(0, 0)
			editor.Cursor.Line, editor.Cursor.Col = 1, 5
			h.exec(command)
			if search.Input.Text != "beta" {
				t.Fatal("multiline selection overwrote the query")
			}
			editor.Selection.Start(0, 0)
			editor.Cursor.Line, editor.Cursor.Col = 0, 5
			h.app.ShowPanel("explorer", h.app.Explorer.Adapter)
			h.app.ShowPanel("search", search)
			if search.Input.Text != "beta" {
				t.Fatal("switching sidebar panels overwrote the query")
			}
			if search.ReplaceInput.Text != "replacement" || search.Include.Text != "*.go" || search.Exclude.Text != "vendor" || !search.Options.CaseSensitive {
				t.Fatal("selection prefill changed replacement text, filters, or options")
			}
		})
	}
}

func TestInlineDiffFindPreservesBaseSelection(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	pasteText(h, pasteKeys("new needle"))
	group := h.app.EditorGroup
	if !group.EnableInlineDiff(group.ActiveFilePath(), []string{"old needle"}) {
		t.Fatal("could not open editable diff")
	}
	dv := group.ActiveInlineDiff()
	dv.SetMode(ui.DiffModeSplit)
	h.redraw()
	r := dv.GetRect()
	h.click(r.X+5, r.Y)
	pane := dv.KeyPane()
	pane.Selection.Start(0, 4)
	pane.Cursor.Line, pane.Cursor.Col = 0, 10
	h.exec("search.find")
	bar := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if bar.Input.Text != "needle" || !pane.Selection.Active {
		t.Fatalf("diff prefill: query=%q selection=%v", bar.Input.Text, pane.Selection.Active)
	}
	if text := dv.CopySelection(); text != "needle" || pane.Selection.Active {
		t.Fatalf("copy after prefill: text=%q selection=%v", text, pane.Selection.Active)
	}
}
