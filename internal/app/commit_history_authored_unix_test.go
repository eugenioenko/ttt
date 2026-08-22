//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCommitDetailPreservesContentWhenAuthoredDateIsUnavailable(t *testing.T) {
	binDir := t.TempDir()
	script := `#!/bin/sh
case " $* " in
  *" --format=%B "*) printf 'subject\n' ;;
  *" --format=%aI "*) printf 'date unavailable\n' >&2; exit 1 ;;
  *" --name-status "*) exit 0 ;;
  *) printf 'unexpected git invocation: %s\n' "$*" >&2; exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	result := readCommitDetail(context.Background(), 1, "/repo", strings.Repeat("a", 40), "aaaaaaa")

	if result.Canceled || result.Err != "" {
		t.Fatalf("detail failed: canceled=%v err=%q", result.Canceled, result.Err)
	}
	if result.Message != "subject" || len(result.Files) != 0 {
		t.Fatalf("content = message %q, files %#v", result.Message, result.Files)
	}
	if !result.AuthoredAt.IsZero() || !result.AuthoredAtUnavailable {
		t.Fatalf("authored state = %v, unavailable=%v", result.AuthoredAt, result.AuthoredAtUnavailable)
	}
}
