package image

import "testing"

func envFrom(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestDetect(t *testing.T) {
	for _, tc := range []struct {
		name  string
		env   map[string]string
		cellW int
		cellH int
		want  Protocol
	}{
		{"zero-cell", map[string]string{"KITTY_WINDOW_ID": "1"}, 0, 10, None},
		{"zero-cell-height", map[string]string{"KITTY_WINDOW_ID": "1"}, 10, 0, None},
		{"tmux-env", map[string]string{"TMUX": "/tmp/tmux-1", "KITTY_WINDOW_ID": "1"}, 10, 20, None},
		{"screen-term", map[string]string{"TERM": "screen-256color"}, 10, 20, None},
		{"tmux-term", map[string]string{"TERM": "tmux-256color"}, 10, 20, None},
		{"ghostty-program", map[string]string{"TERM_PROGRAM": "ghostty"}, 10, 20, Kitty},
		{"ghostty-dir", map[string]string{"GHOSTTY_BIN_DIR": "/x"}, 10, 20, Kitty},
		{"kitty-id", map[string]string{"KITTY_WINDOW_ID": "1"}, 10, 20, Kitty},
		{"kitty-term", map[string]string{"TERM": "xterm-kitty"}, 10, 20, Kitty},
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, 10, 20, Kitty},
		{"konsole", map[string]string{"KONSOLE_VERSION": "24.0"}, 10, 20, Kitty},
		{"plain", map[string]string{"TERM": "xterm-256color"}, 10, 20, None},
		{"empty", map[string]string{}, 10, 20, None},
	} {
		if got := Detect(envFrom(tc.env), tc.cellW, tc.cellH); got != tc.want {
			t.Errorf("%s: Detect = %v, want %v", tc.name, got, tc.want)
		}
	}
}
