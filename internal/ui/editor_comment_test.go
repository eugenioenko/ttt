package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/highlight"
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
