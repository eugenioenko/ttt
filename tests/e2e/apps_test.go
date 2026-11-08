package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
)

func applyApps(h *testHarness, apps ...config.Application) {
	h.t.Helper()
	s := *h.app.Settings
	s.Applications = apps
	h.app.ApplySettings(s)
	h.redraw()
}

func waitForFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return string(data)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s was never written", path)
	return ""
}

func TestAppsTabFollowsSettings(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	if h.app.Sidebar.HasPanel("apps") {
		t.Fatal("the Apps tab must stay hidden with no apps configured")
	}

	applyApps(h,
		config.Application{Name: "viewer", Command: "true"},
		config.Application{Name: "", Command: "true"},
		config.Application{Name: "viewer", Command: "false"},
	)
	if !h.app.Sidebar.HasPanel("apps") {
		t.Fatal("configuring an app must add the Apps tab")
	}
	if _, ok := h.reg.Get("apps.run:viewer"); !ok {
		t.Fatal("each app must get a run command")
	}
	if n := len(h.app.Apps.Tree.Config.Items); n != 1 {
		t.Fatalf("apps listed = %d, want 1 (invalid and duplicate entries dropped)", n)
	}

	applyApps(h)
	if h.app.Sidebar.HasPanel("apps") {
		t.Fatal("removing every app must remove the Apps tab")
	}
	if _, ok := h.reg.Get("apps.run:viewer"); ok {
		t.Fatal("a removed app's command must be unregistered")
	}
}

func TestLaunchAppQuotesPlaceholdersAndReusesItsTab(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	file := filepath.Join(h.dir, "it's here.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(h.dir, "out.txt")
	applyApps(h, config.Application{
		Name:     "echo",
		Command:  "printf '%s|%s' {file} {dir} > " + out + "; sleep 30",
		KeepOpen: true,
	})

	h.exec("apps.run:echo")
	if len(h.app.EditorTerminals) != 0 {
		t.Fatal("an app using {file} must not launch without an open file")
	}

	if err := h.app.EditorGroup.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	h.exec("apps.run:echo")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatalf("editor terminals = %d, want 1", len(h.app.EditorTerminals))
	}
	defer h.app.EditorTerminals[0].Term.Close()
	if want := file + "|" + h.dir; strings.TrimSpace(waitForFile(t, out)) != want {
		t.Fatalf("command saw %q, want %q", waitForFile(t, out), want)
	}

	h.exec("apps.run:echo")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatal("running an app again must focus its tab, not open a second one")
	}
	if h.app.EditorGroup.ActiveTerminalTab() != h.app.EditorTerminals[0].Tab {
		t.Fatal("running an app again must focus its tab")
	}
}

func TestLaunchAppFromAnotherAppTabUsesTheLastFile(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	file := filepath.Join(h.dir, "alpha.txt")
	out := filepath.Join(h.dir, "out.txt")
	applyApps(h,
		config.Application{Name: "first", Command: "sleep 30"},
		config.Application{Name: "second", Command: "printf '%s' {file} > " + out + "; sleep 30"},
	)
	if err := h.app.EditorGroup.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	h.exec("apps.run:first")
	h.exec("apps.run:second")
	for _, tt := range h.app.EditorTerminals {
		defer tt.Term.Close()
	}
	if len(h.app.EditorTerminals) != 2 {
		t.Fatalf("editor terminals = %d, want 2", len(h.app.EditorTerminals))
	}
	if got := waitForFile(t, out); got != file {
		t.Fatalf("{file} = %q, want the file active before the first app, %q", got, file)
	}
}

func TestDirFallsBackToWorkspaceWithoutAFile(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	out := filepath.Join(h.dir, "out.txt")
	applyApps(h, config.Application{Name: "browse", Command: "printf '%s' {dir} > " + out + "; sleep 30"})
	h.app.EditorGroup.CloseAllTabs()
	h.exec("apps.run:browse")
	if len(h.app.EditorTerminals) != 1 {
		t.Fatal("an app using only {dir} must launch without an open file")
	}
	defer h.app.EditorTerminals[0].Term.Close()
	if got, want := waitForFile(t, out), h.app.Workspace.Primary(); got != want {
		t.Fatalf("{dir} = %q, want the workspace %q", got, want)
	}
}
