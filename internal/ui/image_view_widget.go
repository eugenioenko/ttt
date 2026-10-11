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

	tttimage "github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

const (
	imageCardHint    = "Graphics preview is not supported in this terminal yet."
	imageLoadingHint = "Loading preview..."
)

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
	// Redraw is called from another goroutine when a decode finishes; nil decodes on the UI goroutine.
	Redraw     func()
	loading    bool
	image      *widgets.ImageWidget
	data       string
	dataRead   bool
	readFailed bool
	dataSize   int64
	dataMod    time.Time
}

func NewImageViewWidget(path string) *ImageViewWidget {
	w := &ImageViewWidget{FilePath: path, image: widgets.NewImageWidget(widgets.ImageConfig{})}
	w.Refresh()
	return w
}

// A changed file on disk drops the cached bytes, which releases the old image in the terminal.
func (w *ImageViewWidget) Refresh() {
	w.Format = ""
	w.WidthPx, w.HeightPx = 0, 0
	w.Size = 0
	w.Err = ""
	fi, err := os.Stat(w.FilePath)
	if err != nil {
		w.dropData()
		if os.IsNotExist(err) {
			w.Err = "File not found."
		} else {
			w.Err = fmt.Sprintf("Cannot read file: %v", err)
		}
		return
	}
	w.Size = fi.Size()
	if w.readFailed || (w.dataRead && (fi.Size() != w.dataSize || !fi.ModTime().Equal(w.dataMod))) {
		w.dropData()
	}
	w.dataSize, w.dataMod = fi.Size(), fi.ModTime()
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

// Close runs when the tab closes and frees the image in the terminal.
func (w *ImageViewWidget) Close() { w.image.Close() }

func (w *ImageViewWidget) dropData() {
	w.data, w.dataRead, w.readFailed = "", false, false
	w.image.SetConfig(widgets.ImageConfig{})
}

func (w *ImageViewWidget) cardLines() []string {
	name := filepath.Base(w.FilePath)
	if w.Err != "" {
		return []string{name, w.Err}
	}
	hint := imageCardHint
	if w.loading {
		hint = imageLoadingHint
	}
	return []string{
		name,
		fmt.Sprintf("%s image %dx%d", w.Format, w.WidthPx, w.HeightPx),
		fmt.Sprintf("%s on disk", formatFileSize(w.Size)),
		hint,
	}
}

// A failed read is not retried until Refresh, so a broken image never costs per-frame disk I/O.
func (w *ImageViewWidget) loadData() bool {
	if !w.dataRead {
		w.dataRead = true
		data, err := tttimage.ReadFile(w.FilePath)
		if err != nil {
			w.readFailed = true
			w.Err = fmt.Sprintf("Not a readable image: %v", err)
			return false
		}
		w.data = string(data)
	}
	return !w.readFailed
}

func (w *ImageViewWidget) placeImage(surface Surface) bool {
	w.loading = false
	protocol := tttimage.ResolveProtocol(w.Protocol, os.Getenv, w.CellW, w.CellH)
	if w.Err != "" || protocol != tttimage.Kitty || w.CellW <= 0 || w.CellH <= 0 {
		return false
	}
	if !w.loadData() {
		return false
	}
	w.image.SetConfig(widgets.ImageConfig{
		Data:     w.data,
		Fit:      widgets.ImageFitContain,
		Protocol: protocol,
		CellW:    w.CellW,
		CellH:    w.CellH,
		Redraw:   w.Redraw,
	})
	if w.image.Place(surface) {
		return true
	}
	if err := w.image.Err(); err != "" {
		w.Err = "Not a readable image: " + err
	} else {
		w.loading = w.image.Decoding()
	}
	return false
}

func (w *ImageViewWidget) Render(surface Surface) {
	sw, sh := surface.Size()
	if sw <= 0 || sh <= 0 {
		return
	}
	if w.placeImage(surface) {
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
