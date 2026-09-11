package highlight

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
)

const miniLexer = `<lexer>
  <config>
    <name>Mini</name>
    <alias>mini</alias>
    <filename>*.mini</filename>
    <dot_all>true</dot_all>
    <ensure_nl>true</ensure_nl>
  </config>
  <rules>
    <state name="root">
      <rule pattern="/\*.*?\*/"><token type="CommentMultiline" /></rule>
      <rule pattern="\bwhen\b"><token type="Keyword" /></rule>
      <rule pattern="."><token type="Text" /></rule>
    </state>
  </rules>
</lexer>`

// lexerNamed rewrites the minimal lexer's identity, so two files can collide
// on a name or claim a builtin's name.
func lexerNamed(name, alias, glob string) string {
	body := strings.Replace(miniLexer, "<name>Mini</name>", "<name>"+name+"</name>", 1)
	body = strings.Replace(body, "<alias>mini</alias>", "<alias>"+alias+"</alias>", 1)
	return strings.Replace(body, "<filename>*.mini</filename>", "<filename>"+glob+"</filename>", 1)
}

// writeLexerDir points config lookup at a fresh dir holding the given
// lexers/<name> files.
func writeLexerDir(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lexers"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, "lexers", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prev := config.OverrideConfigDir
	config.OverrideConfigDir = dir
	t.Cleanup(func() { config.OverrideConfigDir = prev })
}

// loadIsolated scans into a private registry, so a lexer registered by one
// test cannot reach another: chroma offers no way to unregister.
func loadIsolated(t *testing.T, files map[string]string) []error {
	t.Helper()
	writeLexerDir(t, files)
	return loadExternalLexersInto(chroma.NewLexerRegistry())
}

// loadGlobal exercises the production path, for the cases that must prove
// lexers.Match sees the result. It rearms the one-shot scan; the registration
// itself leaks into chroma's global registry for the rest of the binary, which
// is why only lexers with names and globs of their own are loaded this way.
func loadGlobal(t *testing.T, files map[string]string) {
	t.Helper()
	writeLexerDir(t, files)
	externalOnce = sync.Once{}
	externalErrors = nil
	t.Cleanup(func() {
		externalOnce = sync.Once{}
		externalErrors = nil
	})
}

func TestExternalLexerHighlightsUnknownExtension(t *testing.T) {
	loadGlobal(t, map[string]string{"mini.xml": miniLexer})

	h := New("example.mini")
	if h == nil {
		t.Fatal("no highlighter for .mini after installing an external lexer")
	}
	if got := h.Language(); got != "Mini" {
		t.Fatalf("language = %q, want Mini", got)
	}

	spans := h.HighlightLine("when x")
	if len(spans) == 0 || spans[0].Style != term.StyleSyntaxKeyword {
		t.Fatalf("spans = %+v, want a keyword span at the start", spans)
	}
	if errs := ExternalLexerErrors(); len(errs) != 0 {
		t.Fatalf("valid lexer reported errors: %v", errs)
	}
}

// A file dropped in by hand can be malformed. The editor must keep running,
// report why, and keep scanning the rest of the directory.
func TestExternalLexerRejectsMalformedFile(t *testing.T) {
	errs := loadIsolated(t, map[string]string{
		"broken.xml": "<lexer><config><name>Broken</name>",
		"ok.xml":     lexerNamed("Sound", "sound", "*.sound"),
	})

	if len(errs) != 1 {
		t.Fatalf("errors = %v, want exactly one", errs)
	}
	if !strings.Contains(errs[0].Error(), "broken.xml") {
		t.Fatalf("error %v does not name the rejected file", errs[0])
	}
}

// chroma's Register replaces by name, so an unguarded file called Go would
// take highlighting away from every .go file in the editor.
func TestExternalLexerRejectsBuiltinName(t *testing.T) {
	writeLexerDir(t, map[string]string{"go.xml": lexerNamed("Go", "golang", "*.notgo")})

	errs := loadExternalLexersInto(lexers.GlobalLexerRegistry)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), `"Go"`) {
		t.Fatalf("errors = %v, want one naming the Go collision", errs)
	}
	lexer := lexers.Match("main.go")
	if lexer == nil || lexer.Config().Name != "Go" {
		t.Fatalf("builtin Go lexer was replaced: %v", lexer)
	}
	if lexers.Match("x.notgo") != nil {
		t.Fatal("rejected lexer was registered anyway")
	}
}

func TestExternalLexerRejectsDuplicateName(t *testing.T) {
	errs := loadIsolated(t, map[string]string{
		"a-twin.xml": lexerNamed("Twin", "twin", "*.twin"),
		"b-twin.xml": lexerNamed("Twin", "twin2", "*.twin2"),
	})

	if len(errs) != 1 {
		t.Fatalf("errors = %v, want exactly one", errs)
	}
	msg := errs[0].Error()
	if !strings.Contains(msg, "b-twin.xml") || !strings.Contains(msg, "a-twin.xml") {
		t.Fatalf("error %q should reject the second file and name the first", msg)
	}
}

func TestExternalLexerRejectsOversizedFile(t *testing.T) {
	padding := strings.Repeat(" ", maxLexerFileBytes)
	errs := loadIsolated(t, map[string]string{"huge.xml": miniLexer + "<!--" + padding + "-->"})

	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "limit") {
		t.Fatalf("errors = %v, want one size rejection", errs)
	}
}

// A push to a state the lexer never defines panics chroma mid-tokenise, on
// whatever line first matches that rule, which the root-state probe misses.
func TestExternalLexerRejectsUnknownPushState(t *testing.T) {
	body := strings.Replace(miniLexer,
		`<rule pattern="\bwhen\b"><token type="Keyword" /></rule>`,
		`<rule pattern="\bwhen\b"><token type="Keyword" /><push state="nosuchstate" /></rule>`, 1)
	errs := loadIsolated(t, map[string]string{"bad-push.xml": lexerNamed("Pushy", "pushy", "*.pushy")})
	if len(errs) != 0 {
		t.Fatalf("sanity: valid lexer rejected: %v", errs)
	}

	errs = loadIsolated(t, map[string]string{"bad-push.xml": body})
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "nosuchstate") {
		t.Fatalf("errors = %v, want one naming the missing state", errs)
	}
}
