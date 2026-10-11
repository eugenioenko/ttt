package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/term"
)

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
