package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func isolatedSettingsExecutable(t *testing.T) bool {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TTT_TEST_SETTINGS_EXECUTABLE") == exe {
		return true
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	isolated := filepath.Join(t.TempDir(), filepath.Base(exe))
	if err := os.WriteFile(isolated, data, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, isolated, "-test.run", "^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), "TTT_TEST_SETTINGS_EXECUTABLE="+isolated)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("isolated settings test: %v\n%s", err, output)
	}
	return false
}
