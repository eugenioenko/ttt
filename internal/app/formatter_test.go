package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestUnchangedFormatterDoesNotEmitAnotherEdit(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("test formatter requires cat")
	}
	for _, trailing := range []bool{false, true} {
		t.Run(fmt.Sprintf("trailing=%v", trailing), func(t *testing.T) {
			settings := config.DefaultSettings()
			settings.Editor.ShowTrailingNewline = &trailing
			a := buildTestApp(t, settings)
			t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
			path := filepath.Join(t.TempDir(), "README.md")
			if err := os.WriteFile(path, []byte("# README\n\nUnchanged paragraph.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			a.EditorGroup.OpenFile(path)
			buf := a.EditorGroup.ActiveBuffer()
			version := buf.Version()
			changes := 0
			a.EditorGroup.Editor.OnChange = func() { changes++ }
			if err := a.applyExternalFormatter("cat", path); err != nil {
				t.Fatal(err)
			}
			if buf.Version() != version || buf.Dirty || changes != 0 {
				t.Fatalf("unchanged formatter emitted an edit: version %d -> %d, dirty=%v, events=%d", version, buf.Version(), buf.Dirty, changes)
			}
		})
	}
}
