package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestEditorGroupHomeEditPreservesUTF8BOM(t *testing.T) {
	for _, name := range []string{"save", "save as"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bom.txt")
			if err := os.WriteFile(path, []byte("\ufeffbom\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			g := NewEditorGroupWidget(nil, 4, false, "extended")
			g.ShowTrailingNewline = true
			g.InsertFinalNewline = true
			t.Cleanup(g.CloseAllTabs)
			if err := g.OpenFile(path); err != nil {
				t.Fatal(err)
			}
			g.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModNone))
			g.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyHome, "", tcell.ModNone))
			g.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "Q", tcell.ModNone))
			if name == "save as" {
				path = filepath.Join(filepath.Dir(path), "saved-as.txt")
				if !g.SaveAs(path) {
					t.Fatal("SaveAs failed")
				}
			} else if !g.Save() {
				t.Fatal("Save failed")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if want := "\ufeffQbom\n"; string(data) != want {
				t.Fatalf("saved bytes = %x, want %x", data, want)
			}
		})
	}
}
