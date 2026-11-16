package ui

import (
	"os"
	"path/filepath"
	"strings"
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

func newCommentEditor(t *testing.T, filename string, lines []string, line, col int) *EditorPaneWidget {
	t.Helper()
	h := highlight.New(filename)
	if h == nil {
		t.Fatalf("no highlighter for %s", filename)
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
	e := newCommentEditor(t, "a.html", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<!-- <!&#45;&#45; a &#45;&#45;>hello<!&#45;&#45; b &#45;&#45;> -->"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}
	if strings.Contains(e.Buf.Lines[0][len("<!-- "):len(e.Buf.Lines[0])-len(" -->")], "--") {
		t.Fatalf("wrapped content = %q, contains a raw -- run", e.Buf.Lines[0])
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
	e := newCommentEditor(t, "a.html", append([]string(nil), original...), 1, len([]rune(original[1])))
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"<!-- <!&#45;&#45; a &#45;&#45;> -->", "<!-- <div>hello</div> -->"}
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

// OCaml comments nest, so inner closers must not stop an outer wrapper from
// being recognized as commented.
func TestToggleLineCommentRecognizesNestedComments(t *testing.T) {
	e := newCommentEditor(t, "a.ml", []string{"(* outer (* inner *) *)"}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "outer (* inner *)"; got != want {
		t.Fatalf("uncommented line = %q, want %q", got, want)
	}
}

// Pre-existing entity text must survive a wrap/unwrap round trip: unescaping
// has to reverse only what the wrapper itself encoded.
func TestToggleLineCommentPreservesEntityText(t *testing.T) {
	const original = "&amp;lt;!-- &nbsp;"
	e := newCommentEditor(t, "a.html", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<!-- &amp;amp;lt;!&#45;&#45; &amp;nbsp; -->"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("round-tripped line = %q, want %q", got, original)
	}
}

// An unpaired "--" run is invalid inside an XML comment but is not a
// delimiter, so it must be encoded too and restored verbatim on unwrap.
func TestToggleLineCommentEscapesDoubleHyphenRuns(t *testing.T) {
	const original = "a--b --- c - d"
	e := newCommentEditor(t, "a.xml", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<!-- a&#45;&#45;b &#45;&#45;&#45; c - d -->"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("unwrapped line = %q, want %q", got, original)
	}
}

// Batch's REM is a keyword, so an identifier that merely starts with it is code
// and must get wrapped instead of being uncommented.
func TestToggleLineCommentKeepsWordPrefixIdentifiers(t *testing.T) {
	const original = "REMOVE_ME=1"
	e := newCommentEditor(t, "a.bat", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "REM REMOVE_ME=1"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("unwrapped line = %q, want %q", got, original)
	}
}

func TestToggleLineCommentMatchesWordPrefixCaseInsensitively(t *testing.T) {
	tests := []struct {
		name, line, want string
	}{
		{"lowercase prefix", "rem note", "note"},
		{"prefix alone", "REM", ""},
		{"prefix inside identifier", "REMOTE", "REM REMOTE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newCommentEditor(t, "a.bat", []string{tt.line}, 0, 0)
			e.ToggleLineComment()
			if got := e.Buf.Lines[0]; got != tt.want {
				t.Fatalf("toggled line = %q, want %q", got, tt.want)
			}
		})
	}
}
