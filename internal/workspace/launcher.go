package workspace

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed ttt.svg
var launcherSVG []byte

//go:embed ttt.png
var launcherPNG []byte

const launcherMarker = "X-TTT-Managed=true"

var (
	ErrLauncherUnsupported = errors.New("desktop launcher is not supported on this platform")
	ErrLauncherUnmanaged   = errors.New("a launcher not created by ttt already exists")
)

// InstallLauncher adds ttt to the desktop's application launcher and returns the
// file it wrote. An existing launcher is left alone, so that a terminal chosen
// at install time is not rewritten by every later start from another terminal.
func InstallLauncher(exe string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		if os.Getenv("LOCALAPPDATA") == "" {
			return "", ErrLauncherUnsupported
		}
		return installTerminalFragment(windowsFragmentDir(), exe)
	case "darwin":
		return "", ErrLauncherUnsupported
	}
	return installDesktopEntry(xdgDataHome(), exe, detectTerminal(os.Getenv, exec.LookPath))
}

func RemoveLauncher() error {
	switch runtime.GOOS {
	case "windows":
		// Without LOCALAPPDATA the path would be relative to the cwd.
		if os.Getenv("LOCALAPPDATA") == "" {
			return ErrLauncherUnsupported
		}
		return os.RemoveAll(windowsFragmentDir())
	case "darwin":
		return ErrLauncherUnsupported
	}
	return removeDesktopEntry(xdgDataHome())
}

func xdgDataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func windowsFragmentDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows Terminal", "Fragments", "ttt")
}

func desktopEntryPaths(dataHome string) (entry, icon string) {
	return filepath.Join(dataHome, "applications", "ttt.desktop"),
		filepath.Join(dataHome, "icons", "hicolor", "scalable", "apps", "ttt.svg")
}

type launchTerminal struct {
	argv  []string
	class string
}

// nil means Terminal=true: the desktop picks the terminal.
func detectTerminal(getenv func(string) string, lookPath func(string) (string, error)) *launchTerminal {
	candidates := []struct {
		present bool
		argv    []string
		class   string
	}{
		{getenv("KITTY_WINDOW_ID") != "", []string{"kitty", "--class", "ttt", "-e"}, "ttt"},
		{getenv("WEZTERM_PANE") != "", []string{"wezterm", "start", "--class", "ttt", "--"}, "ttt"},
		{getenv("ALACRITTY_WINDOW_ID") != "", []string{"alacritty", "--class", "ttt", "-e"}, "ttt"},
		{getenv("GHOSTTY_RESOURCES_DIR") != "", []string{"ghostty", "-e"}, ""},
		{strings.HasPrefix(getenv("TERM"), "foot"), []string{"foot", "--app-id", "ttt"}, "ttt"},
		{getenv("KONSOLE_VERSION") != "", []string{"konsole", "-e"}, ""},
	}
	for _, c := range candidates {
		if !c.present {
			continue
		}
		// A launcher runs without the shell's PATH, so the terminal needs its
		// absolute path as much as ttt does.
		bin, err := lookPath(c.argv[0])
		if err != nil {
			return nil
		}
		argv := append([]string{bin}, c.argv[1:]...)
		return &launchTerminal{argv: argv, class: c.class}
	}
	return nil
}

func desktopEntry(exe string, term *launchTerminal) string {
	var argv []string
	if term != nil {
		argv = append(argv, term.argv...)
	}
	argv = append(argv, exe, "--welcome")
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = quoteExecArg(arg)
	}

	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Name=TTT Editor\n")
	b.WriteString("GenericName=Text Editor\n")
	b.WriteString("Comment=IDE for the terminal\n")
	b.WriteString("Icon=ttt\n")
	b.WriteString("Exec=" + strings.Join(quoted, " ") + " %F\n")
	if term == nil {
		b.WriteString("Terminal=true\n")
	} else {
		b.WriteString("Terminal=false\n")
		if term.class != "" {
			b.WriteString("StartupWMClass=" + term.class + "\n")
		}
	}
	b.WriteString("Categories=Development;IDE;TextEditor;\n")
	b.WriteString("MimeType=text/plain;inode/directory;\n")
	b.WriteString("Keywords=editor;ide;code;terminal;\n")
	b.WriteString(launcherMarker + "\n")
	return b.String()
}

// quoteExecArg quotes per the Desktop Entry spec's Exec rules: inside double
// quotes, `"`, "`", `$` and `\` must be backslash-escaped.
func quoteExecArg(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n\"'\\><~|&;$*?#()`") {
		return arg
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`)
	return `"` + r.Replace(arg) + `"`
}

func installDesktopEntry(dataHome, exe string, term *launchTerminal) (string, error) {
	entryPath, iconPath := desktopEntryPaths(dataHome)
	if data, err := os.ReadFile(entryPath); err == nil {
		if !strings.Contains(string(data), launcherMarker) {
			return entryPath, ErrLauncherUnmanaged
		}
		return entryPath, nil
	}
	if err := writeFile(iconPath, launcherSVG); err != nil {
		return "", err
	}
	if err := writeFile(entryPath, []byte(desktopEntry(exe, term))); err != nil {
		return "", err
	}
	// Only refreshes the MimeType cache behind "Open With"; launchers read the
	// directory directly, so a missing tool is not an error.
	if tool, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(tool, filepath.Dir(entryPath)).Run()
	}
	return entryPath, nil
}

func removeDesktopEntry(dataHome string) error {
	entryPath, iconPath := desktopEntryPaths(dataHome)
	data, err := os.ReadFile(entryPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.Contains(string(data), launcherMarker) {
		return ErrLauncherUnmanaged
	}
	if err := os.Remove(entryPath); err != nil {
		return err
	}
	if icon, err := os.ReadFile(iconPath); err == nil && string(icon) == string(launcherSVG) {
		_ = os.Remove(iconPath)
	}
	return nil
}

// installTerminalFragment registers a Windows Terminal profile. The icon path is
// relative to the fragment, which Windows Terminal resolves from 1.24 on.
func installTerminalFragment(dir, exe string) (string, error) {
	path := filepath.Join(dir, "ttt.json")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	fragment := map[string]any{
		"profiles": []map[string]any{{
			"name":              "TTT Editor",
			"commandline":       `"` + exe + `" --welcome`,
			"icon":              "ttt.png",
			"startingDirectory": "%USERPROFILE%",
		}},
	}
	data, err := json.MarshalIndent(fragment, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writeFile(filepath.Join(dir, "ttt.png"), launcherPNG); err != nil {
		return "", err
	}
	if err := writeFile(path, data); err != nil {
		return "", err
	}
	return path, nil
}

// writeFile replaces path through a rename, so a symlink there (a stowed
// dotfile, say) is replaced rather than written through to its target.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ttt-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
