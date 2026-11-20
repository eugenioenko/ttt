package app

import (
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
