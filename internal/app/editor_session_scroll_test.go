package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestEditorSessionRestoresVisibleContentAfterFileShrinks(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	dir := t.TempDir()
	path := filepath.Join(dir, "shortened.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("original\n", 200)), 0o600); err != nil {
		t.Fatal(err)
	}
	saved := sessionTestApp(t, dir, true)
	if err := saved.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	saved.EditorGroup.GoToLineCol(150, 1)
	saved.EditorGroup.Editor.Viewport.TopLine = 140
	saved.forceQuit()

	if err := os.WriteFile(path, []byte("first\nsecond\nthird\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	restored := sessionTestApp(t, dir, true)
	if got := restored.EditorGroup.Editor.Viewport.TopLine; got != 0 {
		t.Errorf("restored top line = %d, want 0 to show all lines of the shortened file", got)
	}
	if line, _ := restored.EditorGroup.ActiveCursor(); line != 3 {
		t.Errorf("restored cursor line = %d, want the last line at 3", line)
	}
}
