package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestTabMoreMenuReopenClosedLeadsAndFollowsHistory(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())

	menu := a.tabMoreMenu()
	if menu[0].Command != "tab.reopenClosed" || !menu[1].IsSep || menu[2].Command != "tab.closeAll" || menu[3].Command != "tab.closeAllSaved" {
		t.Fatalf("more menu = %+v, want Reopen Closed Tab, separator, Close All, Close All Saved", menu)
	}
	if !menu[0].Disabled {
		t.Fatal("Reopen Closed Tab is enabled with no closed tabs")
	}

	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.CloseTab()
	if a.tabMoreMenu()[0].Disabled {
		t.Fatal("Reopen Closed Tab is disabled after closing a file tab")
	}

	a.EditorGroup.ReopenClosedTab()
	if !a.tabMoreMenu()[0].Disabled {
		t.Fatal("Reopen Closed Tab is enabled after the history was used up")
	}
}
