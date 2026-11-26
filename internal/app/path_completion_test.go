package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestPathCompletionContinuesFromWorkspaceAndUndoes(t *testing.T) {
	const original = "é [link](./ima)"
	a := completionTestApp(t, "setup.md", original)
	root := t.TempDir()
	a.Workspace = workspace.New([]string{root})
	dir := filepath.Join(root, "images")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cover.png"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var run func()
	a.completion.pathRead = asynctask.New[[]ui.CompletionItem](asynctask.TaskOptions{
		Run: func(fn func()) { run = fn },
	}, func(fn func()) error { fn(); return nil })
	moveCursor(a, 0, len([]rune(original))-1)
	a.TriggerAutocomplete()
	if run == nil {
		t.Fatal("manual trigger did not schedule a path lookup")
	}
	run()
	popup := a.EditorGroup.Autocomplete
	if popup == nil || len(popup.Items) != 1 || popup.Items[0].Label != "images/" {
		t.Fatal("expected the workspace's images directory")
	}
	popup.OnSelect(popup.Items[0])
	if got := a.EditorGroup.Editor.Buf.Lines[0]; got != "é [link](./images/)" {
		t.Fatalf("after accepting directory: %q", got)
	}
	run()
	popup = a.EditorGroup.Autocomplete
	if popup == nil || len(popup.Items) != 1 || popup.Items[0].Label != "cover.png" {
		t.Fatal("expected files within the accepted directory")
	}
	popup.OnSelect(popup.Items[0])
	if got := a.EditorGroup.Editor.Buf.Lines[0]; got != "é [link](./images/cover.png)" {
		t.Fatalf("after accepting file: %q", got)
	}
	editor := a.EditorGroup.Editor
	editor.Undo.Undo(editor.Buf)
	editor.Undo.Undo(editor.Buf)
	if got := editor.Buf.Lines[0]; got != original {
		t.Fatalf("after undo: %q", got)
	}
}

func TestPathCompletionDiscardsMovedCursorAndRespectsToggle(t *testing.T) {
	a := completionTestApp(t, "notes.txt", "./\n")
	a.Workspace = workspace.New([]string{t.TempDir()})
	if err := os.Mkdir(filepath.Join(a.Workspace.Primary(), "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	var run func()
	a.completion.pathRead = asynctask.New[[]ui.CompletionItem](asynctask.TaskOptions{
		Run: func(fn func()) { run = fn },
	}, func(fn func()) error { fn(); return nil })
	moveCursor(a, 0, 2)
	a.TriggerAutocomplete()
	if run == nil {
		t.Fatal("expected a pending path lookup")
	}
	moveCursor(a, 0, 1)
	run()
	if a.IsAutocompleteActive() {
		t.Fatal("completion opened after the cursor moved")
	}
	moveCursor(a, 0, 2)
	run = nil
	a.Settings.Autocomplete.PathCompletion = false
	a.TriggerAutocomplete()
	if run != nil {
		t.Fatal("completion ran while disabled")
	}
}

func TestSplitQuotedPathCompletion(t *testing.T) {
	dir, prefix, separator, quote := splitPathCompletion([]rune(`open("./with Bob's files/fi`))
	if dir != "./with Bob's files/" || prefix != "fi" || separator != '/' || quote != '"' {
		t.Fatalf("unexpected path context: %q %q %q %q", dir, prefix, separator, quote)
	}
	if _, _, separator, _ := splitPathCompletion([]rune("https://example.com/page")); separator != 0 {
		t.Fatal("URL triggered filesystem completion")
	}
}

func TestAbsolutePathCompletionRequiresOptIn(t *testing.T) {
	root := t.TempDir()
	absoluteRoot := filepath.VolumeName(root) + string(filepath.Separator)
	for _, tc := range []struct {
		name, text string
		absolute   bool
	}{
		{"root", absoluteRoot, true},
		{"absolute", filepath.Join(root, "ima"), true},
		{"quoted absolute", `"` + filepath.Join(root, "ima"), true},
		{"division", "1 " + absoluteRoot, true},
		{"relative", "./ima", false},
		{"parent", "../ima", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, trigger := range []struct {
				name string
				run  func(*App)
			}{
				{"automatic", (*App).HandleAutocompleteTrigger},
				{"manual", (*App).TriggerAutocomplete},
			} {
				t.Run(trigger.name, func(t *testing.T) {
					a := completionTestApp(t, "notes.txt", tc.text)
					a.Workspace = workspace.New([]string{root})
					moveCursor(a, 0, len([]rune(tc.text)))
					scheduled := false
					a.completion.pathRead = asynctask.New[[]ui.CompletionItem](asynctask.TaskOptions{
						Run: func(func()) { scheduled = true },
					}, func(fn func()) error { fn(); return nil })

					trigger.run(a)
					if scheduled != !tc.absolute {
						t.Fatalf("default settings scheduled lookup = %v, want %v", scheduled, !tc.absolute)
					}
					a.CloseAutocomplete()
					scheduled = false
					a.Settings.Autocomplete.AbsolutePathCompletion = true
					trigger.run(a)
					if !scheduled {
						t.Fatal("opted-in path lookup was not scheduled")
					}
					a.CloseAutocomplete()
					scheduled = false
					a.Settings.Autocomplete.PathCompletion = false
					trigger.run(a)
					if scheduled {
						t.Fatal("master path-completion toggle was ignored")
					}
				})
			}
		})
	}
}

func writePathCompletionFiles(t *testing.T, dir string, names []string) {
	t.Helper()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func numberedNames(prefix string, count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = fmt.Sprintf("%s%03d.txt", prefix, i)
	}
	return names
}

func TestReadPathCompletionsLimitsMatches(t *testing.T) {
	dir := t.TempDir()
	writePathCompletionFiles(t, dir, numberedNames("file", 250))

	items, err := readPathCompletions(context.Background(), pathCompletionTarget{
		dir: dir + string(filepath.Separator), separator: filepath.Separator,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != pathCompletionLimit {
		t.Fatalf("items = %d, want %d", len(items), pathCompletionLimit)
	}
	if items[0].Label != "file000.txt" || items[len(items)-1].Label != "file099.txt" {
		t.Fatalf("kept %q..%q, want the first %d matches", items[0].Label, items[len(items)-1].Label, pathCompletionLimit)
	}
}

func TestReadPathCompletionsFindsEntryBeyondTheLimit(t *testing.T) {
	dir := t.TempDir()
	names := append(numberedNames("log", 250), "zz-late.txt")
	writePathCompletionFiles(t, dir, names)

	narrow, err := readPathCompletions(context.Background(), pathCompletionTarget{
		dir: dir + string(filepath.Separator), prefix: "zz-", separator: filepath.Separator,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(narrow) != 1 || narrow[0].Label != "zz-late.txt" {
		t.Fatalf("narrow prefix returned %v, want the entry past the first page", narrow)
	}

	all, err := readPathCompletions(context.Background(), pathCompletionTarget{
		dir: dir + string(filepath.Separator), separator: filepath.Separator,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range all {
		if item.Label == "zz-late.txt" {
			t.Fatal("a broader prefix should stay capped")
		}
	}
}

func TestReadPathCompletionsKeepsDirectoriesAndStopsOnCancel(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePathCompletionFiles(t, dir, numberedNames("p", 40))

	items, err := readPathCompletions(context.Background(), pathCompletionTarget{
		dir: dir + string(filepath.Separator), prefix: "pkg", separator: filepath.Separator,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Label != "pkg"+string(filepath.Separator) || items[0].Kind != ui.CompletionModule {
		t.Fatalf("directory completion = %v", items)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readPathCompletions(ctx, pathCompletionTarget{
		dir: dir + string(filepath.Separator), separator: filepath.Separator,
	}); err == nil {
		t.Fatal("a cancelled lookup returned results")
	}
}

// cancelAfterEntries reports no error for its first budget Err calls, then reports
// context.Canceled. It pins cancellation to a point mid-enumeration without timing.
type cancelAfterEntries struct {
	context.Context
	budget int
	calls  int
}

func (c *cancelAfterEntries) Err() error {
	c.calls++
	if c.budget > 0 {
		c.budget--
		return nil
	}
	return context.Canceled
}

func TestReadPathCompletionsStopsWhenCancelledMidEnumeration(t *testing.T) {
	dir := t.TempDir()
	writePathCompletionFiles(t, dir, numberedNames("file", 3*pathCompletionBatch))

	ctx := &cancelAfterEntries{Context: context.Background(), budget: pathCompletionBatch}
	_, err := readPathCompletions(ctx, pathCompletionTarget{
		dir: dir + string(filepath.Separator), prefix: "file", separator: filepath.Separator,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if ctx.calls > 2*pathCompletionBatch {
		t.Fatalf("lookup checked ctx %d times, want cancellation after the first batch", ctx.calls)
	}
}

func TestInsertPathCompletionDropsTheLargestLabel(t *testing.T) {
	items := make([]ui.CompletionItem, 0, pathCompletionLimit)
	for i := 0; i < pathCompletionLimit; i++ {
		items = insertPathCompletion(items, ui.CompletionItem{Label: fmt.Sprintf("m%03d", i)})
	}
	items = insertPathCompletion(items, ui.CompletionItem{Label: "a-new"})
	items = insertPathCompletion(items, ui.CompletionItem{Label: "zzz-late"})

	if len(items) != pathCompletionLimit {
		t.Fatalf("items grew to %d", len(items))
	}
	if items[0].Label != "a-new" {
		t.Fatalf("smallest label = %q", items[0].Label)
	}
	for _, item := range items {
		if item.Label == "m099" || item.Label == "zzz-late" {
			t.Fatalf("%q should have been dropped", item.Label)
		}
	}
}
