package clipboard

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	content   string
	useSystem = true
	oscWriter io.Writer
)

func DisableSystem() {
	useSystem = false
}

// SetOSCWriter sets the destination for OSC 52 clipboard escape sequences.
// Without this, OSC 52 writes to raw stderr which leaks escape sequences in
// headless, piped, and unsupported terminal contexts. Set to the tcell tty.
func SetOSCWriter(w io.Writer) {
	oscWriter = w
}

func Set(s string) {
	content = s
	if useSystem {
		writeSystemClipboard(s)
	}
}

func Get() string {
	if useSystem {
		if sys, ok := readSystemClipboard(); ok {
			return sys
		}
	}
	return content
}

func writeSystemClipboard(s string) {
	// Only emit OSC 52 when a writer is configured — avoids leaking escape sequences
	if oscWriter != nil {
		encoded := base64.StdEncoding.EncodeToString([]byte(s))
		fmt.Fprintf(oscWriter, "\033]52;c;%s\a", encoded)
	}

	if name, args := findCopyCmd(); name != "" {
		c := exec.Command(name, args...)
		c.Stdin = strings.NewReader(s)
		c.Run()
	}
}

func readSystemClipboard() (string, bool) {
	name, args := findPasteCmd()
	if name == "" {
		return "", false
	}
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		// A clipboard holding only an image has no text, but it was read:
		// falling back to the process-local copy would paste stale text.
		return "", holdsNonText(name)
	}
	if len(out) == 0 {
		return "", false
	}
	return string(out), true
}

func holdsNonText(pasteCmd string) bool {
	if filepath.Base(pasteCmd) != "wl-paste" {
		return false
	}
	types, err := exec.Command(pasteCmd, "--list-types").Output()
	return err == nil && len(bytes.TrimSpace(types)) > 0
}

func findCopyCmd() (string, []string) {
	if runtime.GOOS == "darwin" {
		return "pbcopy", nil
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if p, err := exec.LookPath("wl-copy"); err == nil {
			return p, nil
		}
	}
	if os.Getenv("DISPLAY") != "" {
		if p, err := exec.LookPath("xclip"); err == nil {
			return p, []string{"-selection", "clipboard"}
		}
		if p, err := exec.LookPath("xsel"); err == nil {
			return p, []string{"--clipboard", "--input"}
		}
	}
	return "", nil
}

func findPasteCmd() (string, []string) {
	if runtime.GOOS == "darwin" {
		return "pbpaste", nil
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if p, err := exec.LookPath("wl-paste"); err == nil {
			return p, []string{"--no-newline", "--type", "text"}
		}
	}
	if os.Getenv("DISPLAY") != "" {
		if p, err := exec.LookPath("xclip"); err == nil {
			return p, []string{"-selection", "clipboard", "-o"}
		}
		if p, err := exec.LookPath("xsel"); err == nil {
			return p, []string{"--clipboard", "--output"}
		}
	}
	return "", nil
}
