package ui

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tttimage "github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/term"
)

func writeTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func gridText(grid [][]term.Cell) string {
	var sb strings.Builder
	for y, row := range grid {
		if y > 0 {
			sb.WriteByte('\n')
		}
		for _, c := range row {
			if c.Ch == 0 {
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(c.Ch)
			}
		}
	}
	return sb.String()
}

func renderWidgetText(w Widget, width, height int) string {
	grid := makeGrid(width, height)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: width, H: height})
	w.Render(surface)
	return gridText(grid)
}

func TestOpenImageFileProducesContentTab(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeTestPNG(t, path, 8, 6)

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(path)

	tab := g.activeTab()
	if tab == nil || tab.Content == nil {
		t.Fatal("opening a PNG should produce a content tab, got a text buffer")
	}
	iv, ok := tab.Content.(*ImageViewWidget)
	if !ok {
		t.Fatalf("content tab holds %T, want *ImageViewWidget", tab.Content)
	}
	if tab.Buf != nil {
		t.Fatal("image tab must not carry a text buffer")
	}
	if g.ActiveBuffer() != nil {
		t.Fatal("ActiveBuffer must be nil for an image tab")
	}
	if activeImage(g) != iv {
		t.Fatal("ActiveImageWidget should return the open viewer")
	}
	if iv.WidthPx != 8 || iv.HeightPx != 6 {
		t.Fatalf("dimensions = %dx%d, want 8x6", iv.WidthPx, iv.HeightPx)
	}
	if iv.Format != "PNG" {
		t.Fatalf("format = %q, want PNG", iv.Format)
	}
}

func activeImage(g *EditorGroupWidget) *ImageViewWidget {
	if t := g.activeTab(); t != nil {
		iv, _ := t.Content.(*ImageViewWidget)
		return iv
	}
	return nil
}

func TestImageFallbackCardReportsDimensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeTestPNG(t, path, 8, 6)

	w := NewImageViewWidget(path)
	text := renderWidgetText(w, 60, 10)
	for _, want := range []string{"photo.png", "PNG", "8x6"} {
		if !strings.Contains(text, want) {
			t.Errorf("fallback card should contain %q, got:\n%s", want, text)
		}
	}
}

func TestOpenBinaryFileProducesBinaryTab(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02, 'a', '\n', 0xff}, 0644); err != nil {
		t.Fatal(err)
	}

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(path)

	tab := g.activeTab()
	if tab == nil || tab.Content == nil {
		t.Fatal("opening a binary file should produce a content tab, got a text buffer")
	}
	if _, ok := tab.Content.(*BinaryFileWidget); !ok {
		t.Fatalf("content tab holds %T, want *BinaryFileWidget", tab.Content)
	}
	if tab.Buf != nil {
		t.Fatal("binary tab must not carry a text buffer")
	}
	text := renderWidgetText(tab.Content, 60, 10)
	if !strings.Contains(text, "Binary file") {
		t.Errorf("binary card should say %q, got:\n%s", "Binary file", text)
	}
}

func TestOpenTextFileStillProducesTextBuffer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(path)

	tab := g.activeTab()
	if tab == nil || tab.Content != nil {
		t.Fatal("a text file should still open as a text buffer")
	}
	if tab.Buf == nil {
		t.Fatal("text tab must carry a buffer")
	}
}

func TestImageWidgetMissingFileShowsMessage(t *testing.T) {
	w := NewImageViewWidget(filepath.Join(t.TempDir(), "gone.png"))
	text := renderWidgetText(w, 60, 10)
	if !strings.Contains(text, "File not found.") {
		t.Errorf("missing file should report %q, got:\n%s", "File not found.", text)
	}
}

func TestImageWidgetUndecodableFileShowsMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bogus.png")
	if err := os.WriteFile(path, []byte("this is not image data\n"), 0644); err != nil {
		t.Fatal(err)
	}
	w := NewImageViewWidget(path)
	if w.Err == "" {
		t.Fatal("undecodable file should set Err")
	}
	text := renderWidgetText(w, 60, 10)
	if !strings.Contains(text, "Not a readable image") {
		t.Errorf("undecodable file should report a readable message, got:\n%s", text)
	}
}

func TestBinaryWidgetMissingFileShowsMessage(t *testing.T) {
	w := NewBinaryFileWidget(filepath.Join(t.TempDir(), "gone.bin"))
	text := renderWidgetText(w, 60, 10)
	if !strings.Contains(text, "File not found.") {
		t.Errorf("missing file should report %q, got:\n%s", "File not found.", text)
	}
}

func renderWidgetOnLayer(t *testing.T, w *ImageViewWidget, width, height int) (string, string) {
	t.Helper()
	layer := tttimage.NewLayer()
	grid := makeGrid(width, height)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: width, H: height})
	surface.SetImageLayer(layer)
	layer.Begin()
	w.Render(surface)
	var buf bytes.Buffer
	if err := layer.Commit(&buf); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	layer.Begin()
	w.Render(surface)
	var second bytes.Buffer
	if err := layer.Commit(&second); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if len(second.String()) != 0 {
		t.Errorf("settled frame wrote %d bytes: %q", len(second.String()), second.String())
	}
	return gridText(grid), buf.String()
}

func TestImageWidgetKittyPathPlacesImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeTestPNG(t, path, 16, 12)

	w := NewImageViewWidget(path)
	w.Protocol = "kitty"
	w.CellW, w.CellH = 10, 20
	card, out := renderWidgetOnLayer(t, w, 60, 20)
	if !strings.Contains(out, "a=t") {
		t.Errorf("kitty path should transmit image data, got %q", out)
	}
	if !strings.Contains(out, "a=p") {
		t.Errorf("kitty path should place the image, got %q", out)
	}
	// Transmit must never display: a=T draws at the parked cursor and scrolls the screen.
	if strings.Contains(out, "a=T") {
		t.Errorf("transmit must use a=t, got %q", out)
	}
	// 16x12 px at 10x20 px cells is a 2x1 box, centered in 60x20: col 29, row 9.
	if !strings.Contains(out, "\x1b[10;30H") {
		t.Errorf("image should be centered at row 10 col 30, got %q", out)
	}
	if strings.Contains(card, "photo.png") {
		t.Errorf("card must not render alongside the image, got:\n%s", card)
	}
}

func TestImageWidgetFitsWithoutUpscaling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.png")
	writeTestPNG(t, path, 4, 4)

	w := NewImageViewWidget(path)
	w.Protocol = "kitty"
	w.CellW, w.CellH = 10, 20
	_, out := renderWidgetOnLayer(t, w, 60, 20)
	// 4x4 px at 1:1 needs a single cell, never an upscaled box.
	if !strings.Contains(out, "c=1,r=1") {
		t.Errorf("tiny image should place 1x1 cells, got %q", out)
	}
}

func TestImageWidgetProtocolNoneKeepsFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeTestPNG(t, path, 16, 12)

	w := NewImageViewWidget(path)
	w.Protocol = "none"
	w.CellW, w.CellH = 10, 20
	card, out := renderWidgetOnLayer(t, w, 60, 20)
	if len(out) != 0 {
		t.Errorf("protocol none wrote %d bytes: %q", len(out), out)
	}
	if !strings.Contains(card, "16x12") {
		t.Errorf("fallback card should report dimensions, got:\n%s", card)
	}
}

func TestImageWidgetWithoutCellSizeKeepsFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeTestPNG(t, path, 16, 12)

	w := NewImageViewWidget(path)
	w.Protocol = "kitty"
	card, out := renderWidgetOnLayer(t, w, 60, 20)
	if len(out) != 0 {
		t.Errorf("unknown cell size wrote %d bytes: %q", len(out), out)
	}
	if !strings.Contains(card, "16x12") {
		t.Errorf("fallback card should report dimensions, got:\n%s", card)
	}
}

func TestSetImageProtocolPropagatesToTabs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeTestPNG(t, path, 8, 6)

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.SetImageProtocol("kitty")
	g.SetImageCellSize(10, 20)
	g.OpenFile(path)
	iv := activeImage(g)
	if iv == nil {
		t.Fatal("expected an image tab")
	}
	if iv.Protocol != "kitty" || iv.CellW != 10 || iv.CellH != 20 {
		t.Fatalf("new tab should inherit group image prefs, got %+v", iv)
	}
	g.SetImageProtocol("none")
	if iv.Protocol != "none" {
		t.Fatalf("open tab should follow protocol change, got %q", iv.Protocol)
	}
}

func TestTextOpenReplacesImagePreview(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "photo.png")
	writeTestPNG(t, imgPath, 8, 6)
	txtPath := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(txtPath, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(imgPath)
	if activeImage(g) == nil {
		t.Fatal("expected an image tab")
	}
	tabsBefore := g.TabCount()
	g.OpenFile(txtPath)
	if activeImage(g) != nil {
		t.Error("opening a text file should replace the image preview tab")
	}
	if g.TabCount() != tabsBefore {
		t.Errorf("expected the preview tab to be replaced, tabs %d -> %d", tabsBefore, g.TabCount())
	}
	if g.ActiveBuffer() == nil {
		t.Error("expected a text buffer after opening a text file")
	}
}

func TestReopenImageTabAppliesCurrentPrefs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeTestPNG(t, path, 8, 6)

	g := NewEditorGroupWidget(nil, 4, true, "relative")
	g.OpenFile(path)
	g.SetImageProtocol("none")
	g.SetImageCellSize(11, 22)
	g.OpenFile(path)
	iv := activeImage(g)
	if iv == nil {
		t.Fatal("expected an image tab")
	}
	if iv.Protocol != "none" || iv.CellW != 11 || iv.CellH != 22 {
		t.Errorf("reopened tab should carry current prefs, got %+v", iv)
	}
}

func TestImageWidgetCloseFreesTerminalImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.png")
	writeTestPNG(t, path, 16, 12)
	w := NewImageViewWidget(path)
	w.Protocol = "kitty"
	w.CellW, w.CellH = 10, 20

	layer := tttimage.NewLayer()
	grid := makeGrid(60, 20)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 60, H: 20})
	surface.SetImageLayer(layer)
	layer.Begin()
	w.Render(surface)
	var buf bytes.Buffer
	if err := layer.Commit(&buf); err != nil {
		t.Fatal(err)
	}

	w.Close()
	layer.Begin()
	buf.Reset()
	if err := layer.Commit(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "d=I") {
		t.Errorf("closing the tab should free the image, got %q", buf.String())
	}
}
