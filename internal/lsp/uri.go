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
		} else if hasDrive(path) {
			path = upperDrive(path)
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
	if drive := strings.TrimPrefix(path, "/"); hasDrive(drive) {
		path = upperDrive(drive)
	} else if u.Host != "" {
		path = "//" + u.Host + path
	}
	return strings.ReplaceAll(path, "/", `\`)
}

func hasDrive(path string) bool {
	return len(path) >= 2 && path[1] == ':' &&
		(('a' <= path[0] && path[0] <= 'z') || ('A' <= path[0] && path[0] <= 'Z'))
}

// Servers may send the drive in lowercase ("file:///c%3A/x") while the app
// compares paths byte for byte, so both directions use the uppercase form.
func upperDrive(path string) string {
	return strings.ToUpper(path[:1]) + path[1:]
}
