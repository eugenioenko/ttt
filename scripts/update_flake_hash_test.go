package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateFlakeHash(t *testing.T) {
	script, err := filepath.Abs("update-flake-hash.sh")
	if err != nil {
		t.Fatal(err)
	}
	const originalHash = "sha256-h2f9czqMrdqif0EbBejGMge+Ra4jpkzIJ7ZiYN41V20="
	const replacement = "sha256-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBA="
	const placeholder = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	original := "{\n  vendorHash  = \"" + originalHash + "\";  # keep formatting\n}\n"
	mismatch := "error: hash mismatch in fixed-output derivation '/nix/store/abc-ttt-dev-go-modules.drv':\n  specified: " + placeholder + "\n  got: " + replacement + "\n"
	for _, tt := range []struct {
		name       string
		probe      string
		probeOK    bool
		verifyFail bool
		changed    bool
		wantError  bool
		calls      string
	}{
		{name: "unchanged", probe: strings.ReplaceAll(mismatch, replacement, originalHash), calls: "1"},
		{name: "replacement with whitespace", probe: strings.ReplaceAll(strings.ReplaceAll(mismatch, "specified: ", "specified:\t   "), "got: ", "got:     "), changed: true, calls: "2"},
		{name: "verification failure", probe: mismatch, verifyFail: true, wantError: true, calls: "2"},
		{name: "missing hash", probe: strings.ReplaceAll(mismatch, "  got: "+replacement+"\n", ""), wantError: true, calls: "1"},
		{name: "multiple hashes", probe: mismatch + "  got: " + originalHash + "\n", wantError: true, calls: "1"},
		{name: "unrelated derivation", probe: strings.ReplaceAll(mismatch, "ttt-dev-go-modules", "unrelated"), wantError: true, calls: "1"},
		{name: "wrong specified hash", probe: strings.ReplaceAll(mismatch, placeholder, originalHash), wantError: true, calls: "1"},
		{name: "unexpected success", probe: "unexpected probe success\n", probeOK: true, wantError: true, calls: "1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			output := filepath.Join(dir, "output")
			probeExit := 1
			if tt.probeOK {
				probeExit = 0
			}
			verifyExit := 0
			if tt.verifyFail {
				verifyExit = 1
			}
			fakeNix := fmt.Sprintf(`#!/bin/bash
set -eu
[[ "$*" == 'build .#default --no-link' ]]
if [[ ! -f calls ]]; then
  echo -n 1 > calls
  grep -F '"%s"' flake.nix > /dev/null
  cat >&2 <<'DIAGNOSTICS'
%sDIAGNOSTICS
  exit %d
fi
echo -n 2 > calls
grep -F '"%s"' flake.nix > /dev/null
if [[ -e "$GITHUB_OUTPUT" ]] && grep -q 'changed=true' "$GITHUB_OUTPUT"; then
  echo 'changed emitted before verification' >&2
  exit 1
fi
echo 'verification diagnostic' >&2
exit %d
`, placeholder, tt.probe, probeExit, replacement, verifyExit)
			for name, content := range map[string]string{"flake.nix": original, "nix": fakeNix} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", script)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GITHUB_OUTPUT="+output)
			diagnostics, err := cmd.CombinedOutput()
			if (err != nil) != tt.wantError {
				t.Fatalf("exit error = %v, want error %v; output:\n%s", err, tt.wantError, diagnostics)
			}
			if tt.wantError {
				wantDiagnostic := tt.probe
				if tt.verifyFail {
					wantDiagnostic = "verification diagnostic"
				}
				if !strings.Contains(string(diagnostics), wantDiagnostic) {
					t.Fatalf("missing diagnostics %q in %s", wantDiagnostic, diagnostics)
				}
			}
			wantFlake := original
			wantOutput := "changed=false\n"
			if tt.changed {
				wantFlake = strings.ReplaceAll(original, originalHash, replacement)
				wantOutput = "changed=true\n"
			}
			if tt.wantError {
				wantOutput = ""
			}
			for name, want := range map[string]string{"flake.nix": wantFlake, "output": wantOutput, "calls": tt.calls} {
				got, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil && !(name == "output" && os.IsNotExist(err)) {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}
