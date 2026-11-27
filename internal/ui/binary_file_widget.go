package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/i18n"
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

func (w *BinaryFileWidget) SetFilePath(path string) {
	w.FilePath = path
	w.Refresh()
}

func (w *BinaryFileWidget) Refresh() {
	w.Size = 0
	w.Err = ""
	fi, err := os.Stat(w.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Err = i18n.Text("binaryFile.fileNotFound")
		} else {
			w.Err = i18n.Format("binaryFile.cannotReadFile", i18n.Args{"error": err})
		}
		return
	}
	w.Size = fi.Size()
}

func (w *BinaryFileWidget) Focusable() bool { return true }

func (w *BinaryFileWidget) cardLines() []string {
	name := filepath.Base(w.FilePath)
	if w.Err != "" {
		return []string{name, i18n.Text("binaryFile.binaryFile"), w.Err}
	}
	return []string{
		name,
		i18n.Text("binaryFile.binaryFile"),
		i18n.Format("binaryFile.onDisk", i18n.Args{"size": formatFileSize(w.Size)}),
		i18n.Text("binaryFile.binaryFilesAreShownAsInfoNotText"),
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
		return i18n.Format("files.kilobytes", i18n.Args{"size": fmt.Sprintf("%.1f", float64(n)/1024)})
	default:
		return i18n.Format("files.megabytes", i18n.Args{"size": fmt.Sprintf("%.1f", float64(n)/(1024*1024))})
	}
}
