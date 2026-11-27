package main

import (
	"os"
	"testing"
)

func TestParseFlagsForwardEligibility(t *testing.T) {
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	for _, tc := range []struct {
		args    []string
		blocked bool
	}{
		{[]string{"file"}, false}, {[]string{"--workspace", "x", "file"}, true},
		{[]string{"--exec", "quit", "file"}, true}, {[]string{"--plugin", "x", "file"}, true},
		{[]string{"--listen", "file"}, true}, {[]string{"-", "file"}, true},
		{[]string{"--new-window", "file"}, true}, {[]string{"--config", "--listen", "file"}, false},
		{[]string{"--config", "custom.json", "file"}, false},
		{[]string{"--debug", "file"}, true}, {[]string{"--size", "120x40", "file"}, true},
		{[]string{"--size", "-", "file"}, true}, {[]string{"--exec-split-on", "--workspace", "file"}, false},
	} {
		os.Args = append([]string{"ttt"}, tc.args...)
		if got := parseFlags().noForward; got != tc.blocked {
			t.Errorf("%v: blocked = %v, want %v", tc.args, got, tc.blocked)
		}
	}
}

func TestMalformedSizeDoesNotMixPartialDimensions(t *testing.T) {
	previous := os.Args
	t.Cleanup(func() { os.Args = previous })
	for _, tc := range []struct {
		size string
		w, h int
	}{
		{"120x40", 120, 40}, {"120x", 0, 0}, {"invalid", 0, 0},
	} {
		os.Args = []string{"ttt", "--size", "80x24", "--size", tc.size}
		got := parseFlags()
		if got.sizeW != tc.w || got.sizeH != tc.h {
			t.Errorf("size %q: %dx%d, want %dx%d", tc.size, got.sizeW, got.sizeH, tc.w, tc.h)
		}
	}
}
