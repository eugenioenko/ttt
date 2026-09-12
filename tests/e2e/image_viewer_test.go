package e2e

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeE2EPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestOpenImageShowsFallbackCard(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	path := filepath.Join(h.dir, "photo.png")
	writeE2EPNG(t, path, 16, 12)
	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	if h.app.EditorGroup.ActiveBuffer() != nil {
		t.Fatal("opening an image should produce a content tab, not a text buffer")
	}
	for _, want := range []string{"photo.png", "PNG", "16x12"} {
		h.assertContains(want)
	}
}

func TestOpenBinaryShowsBinaryTab(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	path := filepath.Join(h.dir, "blob.bin")
	if err := os.WriteFile(path, []byte{0x89, 'P', 'N', 'G', 0x00, 0xff, '\n', 0x00}, 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	if h.app.EditorGroup.ActiveBuffer() != nil {
		t.Fatal("opening a binary file should produce a content tab, not a text buffer")
	}
	h.assertContains("Binary file")
}

func TestSaveOnImageTabIsNoop(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	path := filepath.Join(h.dir, "photo.png")
	writeE2EPNG(t, path, 16, 12)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	h.exec("file.save")
	h.redraw()

	if h.app.Root.HasModalOverlay() {
		t.Error("saving an image tab must not open a dialog")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("saving an image tab must not touch the file")
	}
}
