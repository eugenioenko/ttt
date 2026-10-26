package app

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/search"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestSearchReplaceRescanPreservesUpstreamBehavior(t *testing.T) {
	for _, all := range []bool{false, true} {
		mode := map[bool]string{false: "single", true: "all"}[all]
		for _, kind := range []string{"fresh", "empty", "empty line text", "scan error", "empty query"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				if kind != "scan error" && kind != "empty query" {
					if _, err := exec.LookPath("rg"); err != nil {
						t.Skip("ripgrep not installed")
					}
				}
				h := newReplaceTestHarness(t)
				h.a.searchReplace.scan = nil
				original, expected := "foo here\n", "bar here\n"
				switch kind {
				case "empty", "empty line text":
					original, expected = "nothing to see\n", "nothing to see\n"
					if kind == "empty line text" {
						h.a.Search.Groups[0].Matches[0].LineText = ""
					}
				case "scan error":
					t.Setenv("PATH", t.TempDir())
					original, expected = "foo\n", "bar\n"
				case "empty query":
					h.a.Search.Input.Text = ""
					original, expected = "foo\n", "bar\n"
					h.a.searchReplace.scan = func(context.Context, search.Options, []string) (map[string][]search.Match, error) {
						t.Error("empty query attempted a scan instead of using the panel snapshot")
						return nil, nil
					}
				}
				if err := os.WriteFile(h.path, []byte(original), 0600); err != nil {
					t.Fatal(err)
				}
				h.a.Search.OnReplaceRequest = func(req ui.SearchReplaceRequest) { req.AllFiles = all; h.a.startSearchReplace(req) }
				h.a.Search.ReplaceAllFiles()
				h.finish(t)()
				if kind == "empty" || kind == "empty line text" {
					if h.a.Root.HasOverlay() || strings.HasPrefix(h.a.Status.Notification, "Replaced") {
						t.Fatal("successful empty rescan offered or reported a replacement")
					}
				} else if all {
					h.dialog(t).Buttons[1].Handler()
				}
				data, err := os.ReadFile(h.path)
				if err != nil || string(data) != expected {
					t.Fatalf("rescan %s: got %q, %v; want %q", kind, data, err, expected)
				}
			})
		}
	}
}
