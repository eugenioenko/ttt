package lsp

import (
	"net/url"
	"path/filepath"
	"strings"
)

// FileURI builds the file:// URI a language server is handed for a path.
//
// Concatenating the path onto the scheme is not enough. A relative path turns
// its first segment into the authority — "docs/a.md" becomes "file://docs/a.md",
// where "docs" is the hostname — and a path containing a space is not a valid
// URI at all. Servers that parse URIs strictly reject both; marksman crashes
// outright on the first, taking the server down for the rest of the session.
func FileURI(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	} else if !strings.HasPrefix(path, "/") {
		// Abs only fails when the working directory is gone. Leading slash or not
		// is what decides whether the first segment is read as a hostname, so it
		// is the one thing worth forcing even then.
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

// URIToPath is the inverse, undoing the escaping FileURI applies.
func URIToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return strings.TrimPrefix(uri, "file://")
	}
	return u.Path
}
