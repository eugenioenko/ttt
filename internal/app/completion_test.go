package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

func completionTestApp(t *testing.T, name, content string) *App {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	a.EditorGroup.OpenFile(path)
	t.Cleanup(func() {
		a.CloseTasks()
		a.EditorGroup.CloseAllTabs()
	})
	return a
}

func moveCursor(a *App, line, col int) {
	a.EditorGroup.Editor.Cursor.Line = line
	a.EditorGroup.Editor.Cursor.Col = col
}

func TestCompletionReplyNeedsCursorInRequestedWord(t *testing.T) {
	a := completionTestApp(t, "a.go", "\tfieldName str\n\tother\n")
	reply := completionReply{items: []ui.CompletionItem{{Label: "string", InsertText: "string"}}}
	moveCursor(a, 0, 14)
	target := a.currentCompletionTarget()

	moveCursor(a, 1, 0)
	a.applyCompletionReply(target, reply)
	if a.IsAutocompleteActive() {
		t.Fatal("reply for another line opened the popup")
	}

	moveCursor(a, 0, 14)
	a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 14, Rune: 'i'})
	moveCursor(a, 0, 15)
	a.applyCompletionReply(target, reply)
	if !a.IsAutocompleteActive() {
		t.Fatal("reply for the word still being typed did not open the popup")
	}
}

func manualAutoSuggest(t *testing.T, a *App) (fire func()) {
	t.Helper()
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"go": {Command: []string{"gopls"}, Languages: map[string]string{".go": "go"}},
	}})
	post := func(fn func()) error { fn(); return nil }
	var runs []func()
	a.completion.debounce = asynctask.New[struct{}](asynctask.TaskOptions{Run: func(fn func()) { runs = append(runs, fn) }}, post)
	a.completion.delay = time.Duration(a.Settings.Autocomplete.Debounce) * time.Millisecond
	a.completion.read = asynctask.New[completionReply](asynctask.TaskOptions{Run: func(func()) {}}, post)
	return func() {
		if len(runs) == 0 {
			t.Fatal("auto-suggest was not scheduled")
		}
		runs[len(runs)-1]()
	}
}

func TestAutoSuggestNeedsCursorInTypedWord(t *testing.T) {
	other := filepath.Join(t.TempDir(), "b.go")
	if err := os.WriteFile(other, []byte("\tfmt.Println()\n\tfmt.Pr\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		leave func(a *App)
		want  bool
	}{
		{"cursor stays in the word", func(a *App) { moveCursor(a, 1, 7) }, true},
		{"cursor moves to the line above", func(a *App) { moveCursor(a, 0, 6) }, false},
		{"active buffer changes", func(a *App) {
			a.EditorGroup.OpenFile(other)
			moveCursor(a, 1, 7)
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := completionTestApp(t, "a.go", "\tfmt.Println()\n\tfmt.P\n")
			fire := manualAutoSuggest(t, a)
			moveCursor(a, 1, 6)
			a.ScheduleAutocomplete()
			a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 1, Col: 6, Rune: 'r'})
			moveCursor(a, 1, 7)
			a.ScheduleAutocomplete()

			tt.leave(a)
			fire()
			if got := a.completion.read.Pending(); got != tt.want {
				t.Fatalf("completion requested = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSignatureHelpReplyNeedsCursorInCall(t *testing.T) {
	a := completionTestApp(t, "a.go", "\tfn(\n\tother\n")
	buf := a.EditorGroup.ActiveBuffer()
	result := &SignatureHelpResult{Label: "fn(value string)"}

	for _, pos := range [][2]int{{0, 2}, {1, 4}} {
		moveCursor(a, pos[0], pos[1])
		a.applySignatureHelp(buf, 0, 4, result)
		if a.EditorGroup.SignatureHelp != nil {
			t.Fatalf("signature help shown with the cursor at %v, outside the call", pos)
		}
	}
	moveCursor(a, 0, 4)
	a.applySignatureHelp(buf, 0, 4, result)
	if a.EditorGroup.SignatureHelp == nil {
		t.Fatal("signature help not shown inside the call")
	}
}

func TestCompletionTriggersAreTheServersOwn(t *testing.T) {
	a := completionTestApp(t, "a.go", "x\n")
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"go": {Command: []string{"gopls"}, Languages: map[string]string{".go": "go"}},
	}})
	// A server that advertises no trigger characters, or is not running,
	// has none: a space must not count as one.
	for _, ch := range []string{" ", ".", "a"} {
		if a.isCompletionTrigger(ch) {
			t.Fatalf("%q counted as a trigger with no advertised trigger characters", ch)
		}
	}
}

func TestCompletionEditCoversCharactersTypedAfterTheRequest(t *testing.T) {
	tests := []struct {
		name, line string
		cursor     int
		start, end int
		want       string
		wantCol    int
	}{
		{"typed past the requested word", "x := str", 8, 5, 7, "x := string", 11},
		{"range already reaches the cursor", "x := str", 8, 5, 8, "x := string", 11},
		{"replace range past the cursor", "x := strz", 8, 5, 9, "x := string", 11},
		{"not an identifier after the range", "x := st)", 8, 5, 7, "x := string)", 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := completionTestApp(t, "a.go", tt.line+"\n")
			moveCursor(a, 0, tt.cursor)
			a.insertCompletion(ui.CompletionItem{Label: "string"}, &lsp.TextEdit{
				Range: lsp.Range{
					Start: lsp.Position{Line: 0, Character: tt.start},
					End:   lsp.Position{Line: 0, Character: tt.end},
				},
				NewText: "string",
			})
			if got := a.EditorGroup.ActiveBuffer().Lines[0]; got != tt.want {
				t.Fatalf("line = %q, want %q", got, tt.want)
			}
			if _, col := a.EditorGroup.ActiveCursor(); col != tt.wantCol {
				t.Fatalf("cursor col = %d, want %d", col, tt.wantCol)
			}
		})
	}
}
