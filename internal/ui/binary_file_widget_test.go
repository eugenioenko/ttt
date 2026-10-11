package ui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryWidgetMissingFileShowsMessage(t *testing.T) {
	w := NewBinaryFileWidget(filepath.Join(t.TempDir(), "gone.bin"))
	text := renderWidgetText(w, 60, 10)
	if !strings.Contains(text, "File not found.") {
		t.Errorf("missing file should report %q, got:\n%s", "File not found.", text)
	}
}
