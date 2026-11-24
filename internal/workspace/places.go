package workspace

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Place is a named shortcut to a directory, as file managers present them in
// their sidebar.
type Place struct {
	Name string
	Path string
}

// Places returns the directories worth offering as shortcuts, in the order a
// file manager would show them: home first, then the XDG user directories, then
// whatever the user has bookmarked themselves.
//
// Both sources are plain text files that every GTK-based file manager already
// writes, so this reuses the shortcuts the user has curated elsewhere instead of
// asking them to curate a second list here. Nonexistent entries are skipped: the
// bookmark file happily outlives the directory, and offering a dead shortcut is
// worse than offering none. Duplicates are dropped, keeping the first name seen.
func Places() []Place {
	var out []Place
	seen := map[string]bool{}

	add := func(name, path string) {
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil || seen[abs] {
			return
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return
		}
		seen[abs] = true
		out = append(out, Place{Name: name, Path: abs})
	}

	home, _ := os.UserHomeDir()
	add("Home", home)
	for _, d := range xdgUserDirs(home) {
		add(d.Name, d.Path)
	}
	for _, b := range gtkBookmarks() {
		add(b.Name, b.Path)
	}
	return out
}

// xdgUserDirsOrder keeps the well-known directories in a stable, useful order
// rather than the map order or the order the file happens to list them in.
var xdgUserDirsOrder = []struct{ key, name string }{
	{"XDG_DOCUMENTS_DIR", "Documents"},
	{"XDG_DOWNLOAD_DIR", "Downloads"},
	{"XDG_DESKTOP_DIR", "Desktop"},
	{"XDG_PICTURES_DIR", "Pictures"},
	{"XDG_MUSIC_DIR", "Music"},
	{"XDG_VIDEOS_DIR", "Videos"},
}

// xdgUserDirs parses ~/.config/user-dirs.dirs, whose lines look like
// `XDG_DOWNLOAD_DIR="$HOME/Downloads"`. The names are localized on many systems,
// which is exactly why the paths cannot be guessed from English defaults. The
// result follows xdgUserDirsOrder, not the order of the file.
func xdgUserDirs(home string) []Place {
	path := filepath.Join(configHome(home), "user-dirs.dirs")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	raw := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"`)
		if value == "" || value == "$HOME" {
			continue // the file marks "unset" by pointing the entry at $HOME
		}
		raw[strings.TrimSpace(key)] = strings.Replace(value, "$HOME", home, 1)
	}

	var out []Place
	for _, d := range xdgUserDirsOrder {
		if p, ok := raw[d.key]; ok {
			out = append(out, Place{Name: d.name, Path: p})
		}
	}
	return out
}

// gtkBookmarks parses the bookmark file GTK file managers write. Lines are
// `file:///path` with an optional display name after a space; paths are
// URL-escaped, and remote bookmarks use other schemes, which are skipped.
func gtkBookmarks() []Place {
	home, _ := os.UserHomeDir()
	f, err := os.Open(filepath.Join(configHome(home), "gtk-3.0", "bookmarks"))
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var out []Place
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		uri, name, _ := strings.Cut(strings.TrimSpace(scanner.Text()), " ")
		if !strings.HasPrefix(uri, "file://") {
			continue
		}
		path := unescapeURIPath(strings.TrimPrefix(uri, "file://"))
		if path == "" {
			continue
		}
		if name == "" {
			name = filepath.Base(path)
		}
		out = append(out, Place{Name: name, Path: path})
	}
	return out
}

func configHome(home string) string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".config")
}

// unescapeURIPath decodes the %XX escapes GTK writes for spaces and other
// characters. net/url is avoided on purpose: it rejects the whole bookmark on a
// stray percent sign, and a malformed escape should cost one character, not the
// entry.
func unescapeURIPath(s string) string {
	if !strings.ContainsRune(s, '%') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if hi, ok1 := hexVal(s[i+1]); ok1 {
				if lo, ok2 := hexVal(s[i+2]); ok2 {
					b.WriteByte(hi<<4 | lo)
					i += 2
					continue
				}
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
