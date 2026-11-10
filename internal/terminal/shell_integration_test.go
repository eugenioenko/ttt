package terminal

import (
	"os"
	"slices"
	"testing"
)

func TestIntegrateShellAddsBashRcfile(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	shell, args := IntegrateShell("/usr/bin/bash", nil)
	if shell != "/usr/bin/bash" || len(args) != 2 || args[0] != "--rcfile" {
		t.Fatalf("IntegrateShell(bash) = %q %q, want --rcfile", shell, args)
	}
	script, err := os.ReadFile(args[1])
	if err != nil || string(script) != string(bashIntegration) {
		t.Fatalf("rcfile %s does not hold the integration script: %v", args[1], err)
	}
	if _, again := IntegrateShell("bash", nil); !slices.Equal(again, args) {
		t.Fatalf("second call used %q, want %q", again, args)
	}
}

func TestIntegrateShellLeavesOtherLaunchesAlone(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	for _, tc := range []struct {
		shell string
		args  []string
	}{
		{"/usr/bin/fish", nil},
		{"/bin/zsh", nil},
		{"/bin/bash", []string{"-l"}},
	} {
		shell, args := IntegrateShell(tc.shell, tc.args)
		if shell != tc.shell || !slices.Equal(args, tc.args) {
			t.Errorf("IntegrateShell(%q, %q) = %q %q, want it unchanged", tc.shell, tc.args, shell, args)
		}
	}
}
