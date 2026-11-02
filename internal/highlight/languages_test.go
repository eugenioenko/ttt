package highlight

import (
	"strings"
	"testing"
)

func TestLanguagesResolveDisplayedGrammars(t *testing.T) {
	setExternal(t,
		ExternalGrammar{Path: writeGrammar(t, "collision.json", strings.ReplaceAll(fooGrammar, "source.foo", "source.collision")), Language: "source.go"},
		ExternalGrammar{Path: writeGrammar(t, "go.json", commentGo)},
		ExternalGrammar{Path: writeGrammar(t, "foo.json", fooGrammar), Language: "Go"},
		ExternalGrammar{Path: writeGrammar(t, "other.json", strings.ReplaceAll(fooGrammar, "source.foo", "source.other")), Language: "Go"},
	)
	seen := map[string]bool{}
	previous := ""
	for _, language := range Languages() {
		if seen[language.ID] || language.ID == PlainText {
			t.Fatalf("duplicate or reserved ID: %q", language.ID)
		}
		seen[language.ID] = true
		if name := strings.ToLower(language.Name); name < previous {
			t.Fatalf("%q is out of order after %q", name, previous)
		} else {
			previous = name
		}
		h := NewForLanguage(language.ID)
		if h == nil || h.Language() != language.Name {
			t.Fatalf("%+v resolves to %v", language, h)
		}
		h.Close()
	}
	for _, id := range []string{"source.shell", "source.go", "source.foo", "source.other"} {
		if !seen[id] {
			t.Errorf("missing %q", id)
		}
	}
	if h := NewForLanguage("go"); h == nil || h.Language() != "Go (test)" {
		t.Fatalf("external canonical ID lost precedence: %v", h)
	}
	if NewForLanguage("no-such-language") != nil {
		t.Fatal("unknown language resolved")
	}
}
