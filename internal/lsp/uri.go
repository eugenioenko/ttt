package lsp

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// FileURI builds the file:// URI a language server is handed for a path.
//
// Concatenating the path onto the scheme is not enough. A relative path turns
// its first segment into the authority — "docs/a.md" becomes "file://docs/a.md",
// where "docs" is the hostname — and a path containing a space is not a valid
// URI at all. Servers that parse URIs strictly reject both; marksman crashes
// outright on the first, taking the server down for the rest of the session.
// On Windows a drive path has the same problem: "C:\x" must become
// "file:///C:/x", not "file://C:%5Cx", which phpantom rejects at initialize.
func FileURI(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return fileURI(path, runtime.GOOS == "windows")
}

func fileURI(path string, windows bool) string {
	u := &url.URL{Scheme: "file"}
	if windows {
		path = strings.ReplaceAll(path, `\`, "/")
		if unc, ok := strings.CutPrefix(path, "//"); ok {
			u.Host, path, _ = strings.Cut(unc, "/")
		}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u.Path = path
	return u.String()
}

// URIToPath is the inverse, undoing the escaping FileURI applies.
func URIToPath(uri string) string {
	return uriToPath(uri, runtime.GOOS == "windows")
}

func uriToPath(uri string, windows bool) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return strings.TrimPrefix(uri, "file://")
	}
	if !windows {
		return u.Path
	}
	path := u.Path
	if u.Host != "" && u.Host != "localhost" {
		path = "//" + u.Host + path
	} else if len(path) >= 3 && path[0] == '/' && isDriveLetter(path[1]) && path[2] == ':' {
		// Servers may send the drive in lowercase ("file:///c%3A/x"); the app
		// compares paths byte for byte against what filepath.Abs returned.
		path = strings.ToUpper(path[1:2]) + path[2:]
	}
	return strings.ReplaceAll(path, "/", `\`)
}

func isDriveLetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
