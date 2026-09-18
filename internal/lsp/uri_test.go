package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

// A relative path used to turn its first segment into the URI authority, which
// is what crashed marksman: "Invalid URI: The hostname could not be parsed",
// taking the language server down for the rest of the session.
func TestFileURIRejectsAnAuthority(t *testing.T) {
	for _, path := range []string{"docs/note.md", "note.md", "./a/b.md"} {
		uri := FileURI(path)
		if !strings.HasPrefix(uri, "file:///") {
			t.Errorf("FileURI(%q) = %q, want a file:/// URI with no authority", path, uri)
		}
	}
}

// Spaces are common in real directories and are not valid in a URI unescaped.
func TestFileURIEscapesSpaces(t *testing.T) {
	uri := FileURI("/home/u/Ingenieria en Sistemas/note.md")
	if strings.Contains(uri, " ") {
		t.Errorf("FileURI kept a raw space: %q", uri)
	}
	if !strings.Contains(uri, "%20") {
		t.Errorf("FileURI did not escape the spaces: %q", uri)
	}
}

func TestFileURIRoundTrip(t *testing.T) {
	for _, path := range []string{
		"/home/u/plain.md",
		"/home/u/with space/note.md",
		"/home/u/acentuado/ñoño.md",
		"/home/u/100% real/note.md",
	} {
		if got := URIToPath(FileURI(path)); got != path {
			t.Errorf("round trip of %q gave %q", path, got)
		}
	}
}

// Absolute paths are what the rest of the app works in; a relative one reaching
// the server would make every position it reports refer to another file.
func TestFileURIIsAbsolute(t *testing.T) {
	got := URIToPath(FileURI("relative/note.md"))
	if !filepath.IsAbs(got) {
		t.Errorf("FileURI(%q) resolved to %q, which is not absolute", "relative/note.md", got)
	}
}
