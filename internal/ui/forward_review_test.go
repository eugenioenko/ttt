package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFileViewerErrors(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "binary")
	image := filepath.Join(dir, "broken.png")
	if err := os.WriteFile(binary, []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(image, []byte("\x89PNG\r\n\x1a\ninvalid"), 0600); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	if err := g.OpenFile(binary); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(binary); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{binary, image} {
		if err := g.OpenFile(path); err == nil {
			t.Errorf("OpenFile(%s) succeeded", path)
		}
	}
}
