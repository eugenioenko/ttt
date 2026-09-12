package ui

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	tttimage "github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

const imageCardHint = "Graphics preview is not supported in this terminal yet."

type ImageViewWidget struct {
	BaseWidget
	FilePath string
	Format   string
	WidthPx  int
	HeightPx int
	Size     int64
	Err      string
	// Protocol is "auto", "kitty" or "none"; CellW/CellH are pixels per cell and zero places nothing.
	Protocol     string
	CellW, CellH int
	src          *tttimage.Source
	srcSize      int64
	srcMod       time.Time
}

func NewImageViewWidget(path string) *ImageViewWidget {
	w := &ImageViewWidget{FilePath: path}
	w.Refresh()
	return w
}

// A changed file on disk drops the cached decode.
func (w *ImageViewWidget) Refresh() {
	w.Format = ""
	w.WidthPx, w.HeightPx = 0, 0
	w.Size = 0
	w.Err = ""
	fi, err := os.Stat(w.FilePath)
	if err != nil {
		w.src = nil
		if os.IsNotExist(err) {
			w.Err = "File not found."
		} else {
			w.Err = fmt.Sprintf("Cannot read file: %v", err)
		}
		return
	}
	w.Size = fi.Size()
	if w.src != nil && (fi.Size() != w.srcSize || !fi.ModTime().Equal(w.srcMod)) {
		w.src = nil
	}
	w.srcSize, w.srcMod = fi.Size(), fi.ModTime()
	f, err := os.Open(w.FilePath)
	if err != nil {
		w.Err = fmt.Sprintf("Cannot read file: %v", err)
		return
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		w.Err = fmt.Sprintf("Not a readable image: %v", err)
		return
	}
	w.Format = strings.ToUpper(format)
	w.WidthPx, w.HeightPx = cfg.Width, cfg.Height
}

func (w *ImageViewWidget) Focusable() bool { return true }

// The cell box the graphics path drew into on the last Render.

func (w *ImageViewWidget) cardLines() []string {
	name := filepath.Base(w.FilePath)
	if w.Err != "" {
		return []string{name, w.Err}
	}
	return []string{
		name,
		fmt.Sprintf("%s image %dx%d", w.Format, w.WidthPx, w.HeightPx),
		fmt.Sprintf("%s on disk", formatFileSize(w.Size)),
		imageCardHint,
	}
}

func (w *ImageViewWidget) resolveProtocol() tttimage.Protocol {
	switch w.Protocol {
	case config.ImageProtocolKitty:
		return tttimage.Kitty
	case config.ImageProtocolNone:
		return tttimage.None
	default:
		return tttimage.Detect(os.Getenv, w.CellW, w.CellH)
	}
}

// A rejected file is not retried, so a broken image never costs per-frame disk I/O.
func (w *ImageViewWidget) decoded() *tttimage.Source {
	if w.src != nil {
		return w.src
	}
	if w.Err != "" {
		return nil
	}
	src, err := tttimage.Load(w.FilePath)
	if err != nil {
		w.Err = fmt.Sprintf("Not a readable image: %v", err)
		return nil
	}
	w.src = src
	return src
}

// Result is centered and in surface-local cells, never upscaled.
func (w *ImageViewWidget) fitImageBox(box Rect, src *tttimage.Source) (Rect, bool) {
	if box.W <= 0 || box.H <= 0 || src.W <= 0 || src.H <= 0 {
		return Rect{}, false
	}
	maxW := float64(box.W * w.CellW)
	maxH := float64(box.H * w.CellH)
	if maxW <= 0 || maxH <= 0 {
		return Rect{}, false
	}
	scale := min(maxW/float64(src.W), maxH/float64(src.H))
	if scale > 1 {
		scale = 1
	}
	cw := min(max(int(float64(src.W)*scale/float64(w.CellW)+0.5), 1), box.W)
	ch := min(max(int(float64(src.H)*scale/float64(w.CellH)+0.5), 1), box.H)
	return Rect{X: box.X + (box.W-cw)/2, Y: box.Y + (box.H-ch)/2, W: cw, H: ch}, true
}

func (w *ImageViewWidget) placeGraphic(surface Surface, box Rect) bool {
	placer, ok := surface.(widgets.ImagePlacer)
	if !ok {
		return false
	}
	if w.resolveProtocol() != tttimage.Kitty {
		return false
	}
	if w.CellW <= 0 || w.CellH <= 0 {
		return false
	}
	src := w.decoded()
	if src == nil {
		return false
	}
	fit, ok := w.fitImageBox(box, src)
	if !ok {
		return false
	}
	placer.PlaceImage(fit.X, fit.Y, fit.W, fit.H, src)
	return true
}

func (w *ImageViewWidget) Render(surface Surface) {
	sw, sh := surface.Size()
	if sw <= 0 || sh <= 0 {
		return
	}
	if w.placeGraphic(surface, Rect{X: 0, Y: 0, W: sw, H: sh}) {
		return
	}
	lines := w.cardLines()
	surface.DrawText(2, 0, lines[0], 0, term.StyleDefault)
	for i, line := range lines[1:] {
		if i+1 >= sh {
			break
		}
		surface.DrawText(2, i+1, line, 0, term.StyleMuted)
	}
}

// The editor group never attaches a buffer to a BinaryFileWidget.
type BinaryFileWidget struct {
	BaseWidget
	FilePath string
	Size     int64
	Err      string
}

func NewBinaryFileWidget(path string) *BinaryFileWidget {
	w := &BinaryFileWidget{FilePath: path}
	w.Refresh()
	return w
}

func (w *BinaryFileWidget) Refresh() {
	w.Size = 0
	w.Err = ""
	fi, err := os.Stat(w.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Err = "File not found."
		} else {
			w.Err = fmt.Sprintf("Cannot read file: %v", err)
		}
		return
	}
	w.Size = fi.Size()
}

func (w *BinaryFileWidget) Focusable() bool { return true }

func (w *BinaryFileWidget) cardLines() []string {
	name := filepath.Base(w.FilePath)
	if w.Err != "" {
		return []string{name, "Binary file", w.Err}
	}
	return []string{
		name,
		"Binary file",
		fmt.Sprintf("%s on disk", formatFileSize(w.Size)),
		"Binary files are shown as info, not text.",
	}
}

func (w *BinaryFileWidget) Render(surface Surface) {
	sw, sh := surface.Size()
	if sw <= 0 || sh <= 0 {
		return
	}
	lines := w.cardLines()
	surface.DrawText(2, 0, lines[0], 0, term.StyleDefault)
	for i, line := range lines[1:] {
		if i+1 >= sh {
			break
		}
		surface.DrawText(2, i+1, line, 0, term.StyleMuted)
	}
}

func formatFileSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}
