package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func noticeCount(a *App) int {
	n := 0
	for _, line := range a.Output.Lines {
		if line.PluginName == "notice" {
			n++
		}
	}
	return n
}

func TestCorruptStateReportedOnceAtStartupAndOverwrittenOnSave(t *testing.T) {
	previous := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = previous })

	path := filepath.Join(config.OverrideConfigDir, "state.json")
	const corrupt = `{"recentFolders": ["/x/y"], "sidebarWidth": 40, "pan`
	if err := os.WriteFile(path, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	cfg := config.Load("")
	borders := BuildBorderSet(cfg.Theme.Borders)
	a := BuildAppFromConfig(&cfg, &borders, workspace.New([]string{projectDir}), nil)
	t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })

	if a.Status.NotifyLevel != view.NotifyWarning {
		t.Errorf("NotifyLevel = %v, want NotifyWarning", a.Status.NotifyLevel)
	}
	if !strings.Contains(a.Status.Notification, path) || !strings.Contains(a.Status.Notification, "reset") {
		t.Errorf("startup notification = %q, want state.json reset notice with path %q", a.Status.Notification, path)
	}
	if got := noticeCount(a); got != 1 {
		t.Fatalf("startup notice count = %d, want 1", got)
	}
	if a.State.SidebarWidth != 0 || len(a.State.RecentFolders) != 0 {
		t.Errorf("corrupt state applied partial fields: %+v", a.State)
	}

	// Subsequent LoadState callers must not repeat the notice while the file is still corrupt.
	_ = a.startupUpdateLastCheck()
	if got := noticeCount(a); got != 1 {
		t.Fatalf("notice repeated after startupUpdateLastCheck: count = %d, want 1", got)
	}

	// Saves must not be blocked and must overwrite the corrupt file without extra notices.
	a.RememberRecentFolders()
	if got := noticeCount(a); got != 1 {
		t.Fatalf("notice count after RememberRecentFolders = %d, want 1", got)
	}
	a.rememberRecentCommand("file.save")
	if got := noticeCount(a); got != 1 {
		t.Fatalf("notice count after rememberRecentCommand = %d, want 1", got)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) == corrupt {
		t.Fatal("corrupt state.json was not overwritten on save")
	}
	reloaded := config.LoadState()
	if !slices.Contains(reloaded.RecentFolders, projectDir) {
		t.Errorf("reloaded RecentFolders = %v, want %q", reloaded.RecentFolders, projectDir)
	}
	if !slices.Contains(reloaded.RecentCommands, "file.save") {
		t.Errorf("reloaded RecentCommands = %v, want file.save", reloaded.RecentCommands)
	}
}

func TestValidAndMissingStateProduceNoStartupNotice(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "missing"},
		{name: "valid", content: `{"sidebarWidth":32}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := config.OverrideConfigDir
			config.OverrideConfigDir = t.TempDir()
			t.Cleanup(func() { config.OverrideConfigDir = previous })
			if tc.content != "" {
				path := filepath.Join(config.OverrideConfigDir, "state.json")
				if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			cfg := config.Load("")
			borders := BuildBorderSet(cfg.Theme.Borders)
			a := BuildAppFromConfig(&cfg, &borders, workspace.New(nil), nil)
			t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })

			if a.Status.Notification != "" {
				t.Errorf("unexpected startup notification: %q", a.Status.Notification)
			}
			if got := noticeCount(a); got != 0 {
				t.Errorf("notice count = %d, want 0", got)
			}
		})
	}
}
