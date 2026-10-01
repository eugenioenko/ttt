package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectTerminal(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(k string) string { return vars[k] }
	}
	found := func(name string) (string, error) { return "/usr/bin/" + name, nil }
	missing := func(string) (string, error) { return "", os.ErrNotExist }

	term := detectTerminal(env(map[string]string{"KITTY_WINDOW_ID": "1"}), found)
	if term == nil || strings.Join(term.argv, " ") != "/usr/bin/kitty --class ttt -e" || term.class != "ttt" {
		t.Fatalf("kitty: got %+v", term)
	}
	if term := detectTerminal(env(map[string]string{"TERM": "foot-extra"}), found); term == nil || term.argv[0] != "/usr/bin/foot" {
		t.Fatalf("foot: got %+v", term)
	}
	if term := detectTerminal(env(nil), found); term != nil {
		t.Fatalf("unknown terminal: got %+v, want nil", term)
	}
	if term := detectTerminal(env(map[string]string{"KITTY_WINDOW_ID": "1"}), missing); term != nil {
		t.Fatalf("terminal not on PATH: got %+v, want nil", term)
	}
}

func TestDesktopEntryExec(t *testing.T) {
	entry := desktopEntry("/opt/my apps/ttt", &launchTerminal{argv: []string{"/usr/bin/kitty", "-e"}, class: "ttt"})
	for _, want := range []string{
		`Exec=/usr/bin/kitty -e "/opt/my apps/ttt" --welcome %F`,
		"Terminal=false",
		"StartupWMClass=ttt",
		launcherMarker,
	} {
		if !strings.Contains(entry, want) {
			t.Errorf("entry missing %q:\n%s", want, entry)
		}
	}

	if entry := desktopEntry("/usr/bin/ttt", nil); !strings.Contains(entry, "Exec=/usr/bin/ttt --welcome %F\nTerminal=true\n") {
		t.Errorf("fallback entry:\n%s", entry)
	}
	if got := quoteExecArg(`/a/$b"c`); got != `"/a/\$b\"c"` {
		t.Errorf("quoteExecArg = %s", got)
	}
}

func TestDesktopEntryInstallAndRemove(t *testing.T) {
	dataHome := t.TempDir()
	entryPath, iconPath := desktopEntryPaths(dataHome)

	path, err := installDesktopEntry(dataHome, "/usr/bin/ttt", nil)
	if err != nil || path != entryPath {
		t.Fatalf("install = %q, %v", path, err)
	}
	if _, err := os.Stat(iconPath); err != nil {
		t.Fatalf("icon not written: %v", err)
	}
	if err := removeDesktopEntry(dataHome); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{entryPath, iconPath} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still exists after remove", p)
		}
	}
}

func TestDesktopEntryLeavesHandWrittenLauncher(t *testing.T) {
	dataHome := t.TempDir()
	entryPath, _ := desktopEntryPaths(dataHome)
	mine := "[Desktop Entry]\nExec=kitty -e ttt\n"
	if err := writeFile(entryPath, []byte(mine)); err != nil {
		t.Fatal(err)
	}

	if _, err := installDesktopEntry(dataHome, "/usr/bin/ttt", nil); !errors.Is(err, ErrLauncherUnmanaged) {
		t.Fatalf("install over a hand-written launcher: err = %v", err)
	}
	if err := removeDesktopEntry(dataHome); !errors.Is(err, ErrLauncherUnmanaged) {
		t.Fatalf("remove of a hand-written launcher: err = %v", err)
	}
	if data, _ := os.ReadFile(entryPath); string(data) != mine {
		t.Fatalf("hand-written launcher changed:\n%s", data)
	}
}

func TestTerminalFragment(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ttt")
	path, err := installTerminalFragment(dir, `C:\Tools\ttt.exe`)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fragment struct {
		Profiles []map[string]string `json:"profiles"`
	}
	if err := json.Unmarshal(data, &fragment); err != nil {
		t.Fatal(err)
	}
	if len(fragment.Profiles) != 1 || fragment.Profiles[0]["commandline"] != `"C:\Tools\ttt.exe" --welcome` || fragment.Profiles[0]["icon"] != "ttt.png" {
		t.Fatalf("fragment = %s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "ttt.png")); err != nil {
		t.Fatalf("icon not written: %v", err)
	}
}

// A stowed icon is a symlink into a dotfiles repo: installing must replace the
// link, not overwrite the file it points to.
func TestInstallDesktopEntryReplacesIconSymlink(t *testing.T) {
	dataHome := t.TempDir()
	_, iconPath := desktopEntryPaths(dataHome)
	target := filepath.Join(t.TempDir(), "mine.svg")
	if err := os.WriteFile(target, []byte("<svg>mine</svg>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(iconPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, iconPath); err != nil {
		t.Fatal(err)
	}
	if _, err := installDesktopEntry(dataHome, "/usr/bin/ttt", nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "<svg>mine</svg>" {
		t.Fatalf("symlink target overwritten: %q", got)
	}
	if info, err := os.Lstat(iconPath); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("icon is still a symlink (err %v)", err)
	}
}
