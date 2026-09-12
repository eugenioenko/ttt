package image

import "strings"

type Protocol int

const (
	None Protocol = iota
	Kitty
)

func (p Protocol) String() string {
	switch p {
	case Kitty:
		return "kitty"
	default:
		return "none"
	}
}

// Query-and-await-reply detection is impossible through tcell, which swallows APC replies, so environment sniffing is the mechanism.
func Detect(env func(string) string, cellW, cellH int) Protocol {
	if cellW <= 0 || cellH <= 0 {
		return None
	}
	term := strings.ToLower(env("TERM"))
	if env("TMUX") != "" {
		return None
	}
	if strings.HasPrefix(term, "screen") || strings.HasPrefix(term, "tmux") {
		return None
	}
	termProgram := env("TERM_PROGRAM")
	if strings.EqualFold(termProgram, "ghostty") ||
		env("GHOSTTY_BIN_DIR") != "" ||
		env("GHOSTTY_RESOURCES_DIR") != "" {
		return Kitty
	}
	if env("KITTY_WINDOW_ID") != "" || strings.Contains(term, "kitty") {
		return Kitty
	}
	if strings.EqualFold(termProgram, "wezterm") || env("WEZTERM_PANE") != "" {
		return Kitty
	}
	if env("KONSOLE_VERSION") != "" || strings.Contains(term, "konsole") {
		return Kitty
	}
	return None
}
