package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/view"
)

// An external grammar configured with the "Shell" alias must comment with "#"
// even though its language ID is not the canonical "shellscript".
func TestCommentSyntaxResolvesShellAliasToHash(t *testing.T) {
	grammar := filepath.Join(t.TempDir(), "custom-shell.json")
	if err := os.WriteFile(grammar, []byte(`{"scopeName": "source.custom-shell", "name": "Custom Shell", "patterns": []}`), 0600); err != nil {
		t.Fatal(err)
	}
	highlight.SetExternalGrammars([]highlight.ExternalGrammar{
		{Path: grammar, Language: "Shell", FileTypes: []string{"csh"}},
	})
	t.Cleanup(func() { highlight.SetExternalGrammars(nil) })

	h := highlight.New("a.csh")
	if h == nil || h.Language() != "Shell" {
		t.Fatalf("a.csh highlighter = %v, want the configured display name Shell", h)
	}
	e := &EditorPaneWidget{Highlighter: h}
	if prefix, postfix := e.commentSyntax(); prefix != "#" || postfix != "" {
		t.Fatalf("comment syntax = %q/%q, want # with no postfix", prefix, postfix)
	}
}

func newHTMLCommentEditor(t *testing.T, lines []string, line, col int) *EditorPaneWidget {
	t.Helper()
	h := highlight.New("a.html")
	if h == nil {
		t.Fatal("no highlighter for a.html")
	}
	e := NewEditorPaneWidget(
		&buffer.Buffer{Lines: lines},
		&cursor.Cursor{Line: line, Col: col},
		&view.Viewport{Width: 80, Height: 10},
	)
	e.Highlighter = h
	return e
}

// An inner "-->" would end the new HTML comment early, so wrapping must escape
// it and toggling back off must restore the line exactly.
func TestToggleLineCommentEscapesInlineHTMLComments(t *testing.T) {
	const original = "<!-- a -->hello<!-- b -->"
	e := newHTMLCommentEditor(t, []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<!-- &lt;!-- a --&gt;hello&lt;!-- b --&gt; -->"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("unwrapped line = %q, want %q", got, original)
	}
}

// A mixed selection comments every line, including lines that already hold a
// comment pair, and the second toggle restores both original lines.
func TestToggleLineCommentEscapesMixedSelection(t *testing.T) {
	original := []string{"<!-- a -->", "<div>hello</div>"}
	e := newHTMLCommentEditor(t, append([]string(nil), original...), 1, len([]rune(original[1])))
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"<!-- &lt;!-- a --&gt; -->", "<!-- <div>hello</div> -->"}
	for i := range want {
		if e.Buf.Lines[i] != want[i] {
			t.Fatalf("wrapped line %d = %q, want %q", i, e.Buf.Lines[i], want[i])
		}
	}

	e.ToggleLineComment()
	for i := range original {
		if e.Buf.Lines[i] != original[i] {
			t.Fatalf("unwrapped line %d = %q, want %q", i, e.Buf.Lines[i], original[i])
		}
	}
}
