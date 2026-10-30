package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
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

func TestLocationResultDismissesSignatureHelpAndCancelsInFlightRequest(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.go")
	second := filepath.Join(dir, "b.go")
	if err := os.WriteFile(first, []byte("package p\n\tcall(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("package p\nfunc call() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := prepareListenTestApp(t)
	a.EditorGroup.OpenFile(first)
	moveCursor(a, 1, 6)
	startListenTestEventLoop(t, a)

	var lateRun func()
	if err := runExecMain(a, func() error {
		a.ShowSignatureHelp(&SignatureHelpResult{Label: "call(value string)"})
		buf := a.EditorGroup.ActiveBuffer()
		a.completion.signature = asynctask.New[*SignatureHelpResult](
			asynctask.TaskOptions{Run: func(fn func()) { lateRun = fn }},
			func(fn func()) error { fn(); return nil },
		)
		a.completion.signature.Schedule(func(context.Context) (*SignatureHelpResult, error) {
			return &SignatureHelpResult{Label: "staleCall(value string)"}, nil
		}, func(res *SignatureHelpResult, err error) {
			if err == nil {
				a.applySignatureHelp(buf, 1, 6, res)
			}
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	a.Screen.PostEvent(tcell.NewEventInterrupt(&LocationResult{
		Locations: []lsp.Location{{
			URI:   FileURI(second),
			Range: lsp.Range{Start: lsp.Position{Line: 1, Character: 0}},
		}},
	}))

	if err := runExecMain(a, func() error {
		if got := a.EditorGroup.ActiveFilePath(); got != second {
			t.Fatalf("active file = %q, want %q", got, second)
		}
		if a.EditorGroup.SignatureHelp != nil {
			t.Fatal("signature help remained visible after LocationResult opened another file")
		}
		if a.completion.signature.Pending() {
			t.Fatal("in-flight signature help request was not cancelled after LocationResult navigation")
		}
		// Even if the user returns to the first file before a late worker finishes,
		// the cancelled request cannot reopen the popup.
		a.EditorGroup.OpenFile(first)
		moveCursor(a, 1, 6)
		if lateRun != nil {
			lateRun()
		}
		if a.EditorGroup.SignatureHelp != nil {
			t.Fatal("late signature help response from previous file reopened popup after navigation")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
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

// Acceptance of completion with additional edits (such as imports) undoes in one step (#876).
func TestInsertCompletionSingleUndoStep(t *testing.T) {
	a := completionTestApp(t, "a.go", "package main\n\nfunc main() {\n\tToUpper\n}\n")
	moveCursor(a, 3, 8)
	item := ui.CompletionItem{
		Label:      "ToUpper",
		InsertText: "strings.ToUpper",
		AdditionalEdits: []ui.AdditionalEdit{
			{
				StartLine: 0, StartCol: 12,
				EndLine: 0, EndCol: 12,
				NewText: "\n\nimport \"strings\"",
			},
		},
	}
	a.insertCompletion(item, nil)

	buf := a.EditorGroup.ActiveBuffer()
	if !strings.Contains(buf.Lines[2], "import \"strings\"") {
		t.Fatalf("import not added: %v", buf.Lines)
	}
	if !strings.Contains(buf.Lines[5], "strings.ToUpper") {
		t.Fatalf("completion text not inserted: %v", buf.Lines)
	}

	a.EditorGroup.Editor.Undo.Undo(buf)
	expected := "package main\n\nfunc main() {\n\tToUpper\n}\n"
	if got := strings.Join(buf.Lines, "\n"); got != expected {
		t.Fatalf("after single undo: got %q, want %q", got, expected)
	}
}
