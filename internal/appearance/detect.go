package appearance

import (
	"os"
	"runtime"
	"strings"
	"time"
)

// DetectStartup is the full startup chain: real terminal query first (unless
// disabled), then env, then OS. QueryTerminal is skipped under tmux/screen
// until passthrough wrapping exists, mirroring internal/image detection.
// It also reports which leg resolved, for the startup log.
func DetectStartup(query bool) (Appearance, string) {
	if query {
		if a := QueryTerminal(200 * time.Millisecond); a != Unknown {
			return a, "tty"
		}
	}
	return detectFromEnv(os.Getenv)
}

// detectFromEnv resolves without touching the tty: COLORFGBG first, then the
// OS. TTY queries are deliberately not here; they live in query.go and must
// run before tcell.Init.
func detectFromEnv(env func(string) string) (Appearance, string) {
	if env == nil {
		env = os.Getenv
	}
	if a := ParseCOLORFGBG(env("COLORFGBG")); a != Unknown {
		return a, "env"
	}
	if a := osAppearance(); a != Unknown {
		return a, "os"
	}
	return Unknown, "none"
}

// DetectLive resolves the appearance without touching the tty, for moments
// when tcell owns it (focus-gained re-checks, settings apply). Unlike
// DetectFromEnv it prefers the OS signal over COLORFGBG: the env var is
// frozen at spawn time and can never reflect a later appearance toggle,
// while on macOS the system setting is what tracking terminals (e.g. Ghostty
// auto) follow. Off macOS there is no live signal yet, so COLORFGBG stays.
func DetectLive() Appearance {
	if runtime.GOOS == "darwin" {
		return darwinAppearance()
	}
	return ParseCOLORFGBG(os.Getenv("COLORFGBG"))
}

// underMux reports tmux/screen, where raw queries need passthrough wrapping.
func underMux() bool {
	if os.Getenv("TMUX") != "" {
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	return strings.HasPrefix(term, "screen") || strings.HasPrefix(term, "tmux")
}

func osAppearance() Appearance {
	if runtime.GOOS != "darwin" {
		return Unknown
	}
	return darwinAppearance()
}
