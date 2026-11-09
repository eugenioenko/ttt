package highlight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

const fooGrammar = `{
	"scopeName": "source.foo",
	"name": "Foo",
	"fileTypes": ["foo"],
	"patterns": [
		{ "match": "\\bfoo\\b", "name": "keyword.control.foo" },
		{ "include": "source.go" }
	]
}`

// commentGo replaces the embedded Go grammar: every line is a comment.
const commentGo = `{
	"scopeName": "source.go",
	"name": "Go (test)",
	"patterns": [ { "match": ".+", "name": "comment.line.test" } ]
}`

func writeGrammar(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func setExternal(t *testing.T, list ...ExternalGrammar) []error {
	t.Helper()
	t.Cleanup(func() { SetExternalGrammars(nil) })
	return SetExternalGrammars(list)
}

func TestExternalGrammarAddsFileType(t *testing.T) {
	if New("main.foo") != nil {
		t.Fatal("main.foo highlighted before any external grammar")
	}
	if errs := setExternal(t, ExternalGrammar{Path: writeGrammar(t, "foo.json", fooGrammar)}); len(errs) > 0 {
		t.Fatal(errs)
	}

	h := New("main.foo")
	if h == nil {
		t.Fatal("no highlighter for main.foo")
	}
	if h.Language() != "Foo" {
		t.Errorf("language = %q, want Foo", h.Language())
	}
	if got := styleAt(h.HighlightLine("foo x"), 0); got == term.StyleDefault {
		t.Error("foo keyword left unstyled")
	}
	if NewForLanguage("foo") == nil {
		t.Error("NewForLanguage(foo) = nil")
	}
}

func TestExternalGrammarOverridesFromSettings(t *testing.T) {
	setExternal(t, ExternalGrammar{
		Path:      writeGrammar(t, "foo.json", fooGrammar),
		Language:  "Fooish",
		FileTypes: []string{".fooish"},
	})
	if New("a.foo") != nil {
		t.Error("grammar's own fileTypes used despite override")
	}
	h := New("a.fooish")
	if h == nil || h.Language() != "Fooish" {
		t.Fatalf("a.fooish highlighter = %v, want language Fooish", h)
	}
}

func TestExternalGrammarReplacesEmbedded(t *testing.T) {
	setExternal(t, ExternalGrammar{Path: writeGrammar(t, "go.json", commentGo), FileTypes: []string{"go"}})

	h := New("main.go")
	if h == nil || h.Language() != "Go (test)" {
		t.Fatalf("main.go highlighter = %v, want the external grammar", h)
	}
	if got := styleAt(h.HighlightLine("package main"), 0); got != term.StyleSyntaxComment {
		t.Errorf("style = %v, want comment from external grammar", got)
	}
}

func TestExternalGrammarPrecedence(t *testing.T) {
	first := strings.Replace(fooGrammar, `"name": "Foo"`, `"name": "First"`, 1)
	second := strings.Replace(fooGrammar, `"name": "Foo"`, `"name": "Second"`, 1)
	setExternal(t,
		ExternalGrammar{Path: writeGrammar(t, "first.json", first)},
		ExternalGrammar{Path: writeGrammar(t, "second.json", second)},
	)
	if h := New("a.foo"); h == nil || h.Language() != "First" {
		t.Fatalf("a.foo highlighter = %v, want First", h)
	}
}

func TestIncludesResolveAcrossSources(t *testing.T) {
	setExternal(t,
		ExternalGrammar{Path: writeGrammar(t, "go.json", commentGo)},
		ExternalGrammar{Path: writeGrammar(t, "foo.json", fooGrammar)},
	)

	t.Run("external includes external", func(t *testing.T) {
		h := New("a.foo")
		if got := styleAt(h.HighlightLine("x := 1"), 0); got != term.StyleSyntaxComment {
			t.Errorf("style = %v, want comment via included source.go", got)
		}
	})

	t.Run("embedded includes external", func(t *testing.T) {
		h := New("README.md")
		lines := []string{"```go", "package main", "```"}
		if got := styleAt(h.HighlightLineAt(lines, 1), 0); got != term.StyleSyntaxComment {
			t.Errorf("fenced Go style = %v, want comment from external source.go", got)
		}
	})
}

func TestInvalidExternalGrammarIsSkipped(t *testing.T) {
	broken := writeGrammar(t, "broken.json", `{ "patterns": [`)
	noScope := writeGrammar(t, "noscope.json", `{ "patterns": [] }`)
	errs := setExternal(t,
		ExternalGrammar{Path: broken},
		ExternalGrammar{Path: noScope},
		ExternalGrammar{Path: filepath.Join(t.TempDir(), "missing.json")},
		ExternalGrammar{Path: writeGrammar(t, "foo.json", fooGrammar)},
	)
	if len(errs) != 3 {
		t.Fatalf("errors = %v, want 3", errs)
	}
	if New("a.foo") == nil {
		t.Error("valid grammar after invalid ones was not loaded")
	}
	if New("main.go") == nil {
		t.Error("embedded grammars lost after an invalid external grammar")
	}
}

func TestClearingExternalGrammarsRestoresEmbedded(t *testing.T) {
	setExternal(t, ExternalGrammar{Path: writeGrammar(t, "go.json", commentGo), FileTypes: []string{"go"}})
	SetExternalGrammars(nil)
	if h := New("main.go"); h == nil || h.Language() == "Go (test)" {
		t.Fatalf("main.go highlighter = %v, want the embedded Go grammar", h)
	}
	if New("a.foo") != nil {
		t.Error("a.foo still highlighted after clearing")
	}
}

func TestExternalGrammarScopeSuffixKeepsEmbeddedLanguageNames(t *testing.T) {
	elisp := `{"scopeName": "source.emacs.lisp", "name": "Emacs Lisp", "patterns": []}`
	if errs := setExternal(t, ExternalGrammar{
		Path:      writeGrammar(t, "emacs-lisp.json", elisp),
		Language:  "Emacs Lisp",
		FileTypes: []string{"el", "elisp", "emacs-lisp"},
	}); len(errs) > 0 {
		t.Fatal(errs)
	}
	for lang, want := range map[string]string{
		"lisp":       "Common Lisp",
		"elisp":      "Emacs Lisp",
		"emacs-lisp": "Emacs Lisp",
		"Emacs Lisp": "Emacs Lisp",
	} {
		h := NewForLanguage(lang)
		if h == nil {
			t.Errorf("NewForLanguage(%q) = nil, want %s", lang, want)
		} else if h.Language() != want {
			t.Errorf("NewForLanguage(%q) language = %q, want %q", lang, h.Language(), want)
		}
	}
	if h := New("init.el"); h == nil || h.Language() != "Emacs Lisp" {
		t.Errorf("init.el did not use the external grammar")
	}
}

func TestNamelessExternalGrammarKeepsEmbeddedLanguageNames(t *testing.T) {
	nameless := `{"scopeName": "source.emacs.lisp", "patterns": []}`
	if errs := setExternal(t, ExternalGrammar{
		Path:      writeGrammar(t, "emacs-lisp.json", nameless),
		FileTypes: []string{"el"},
	}); len(errs) > 0 {
		t.Fatal(errs)
	}
	if h := NewForLanguage("lisp"); h == nil || h.Language() != "Common Lisp" {
		t.Errorf("NewForLanguage(lisp) did not keep Common Lisp")
	}
	h := New("init.el")
	if h == nil {
		t.Fatal("init.el did not use the external grammar")
	}
	if h.Language() != "lisp" {
		t.Errorf("language = %q, want the scope suffix as its display name", h.Language())
	}
}
