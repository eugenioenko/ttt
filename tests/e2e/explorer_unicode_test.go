package e2e

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestExplorerCanonicallyEquivalentFilesOpenSeparately(t *testing.T) {
	h := newTestHarness(t, 120, 32)
	defer h.stop()
	paths := []string{filepath.Join(h.dir, "café.txt"), filepath.Join(h.dir, "cafe\u0301.txt")}
	for _, path := range paths {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			t.Skip("filesystem treats canonically equivalent names as one entry")
		}
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := file.WriteString(filepath.Base(path))
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			t.Fatal(err)
		}
	}
	h.exec("explorer.refresh")
	h.exec("sidebar.explorer")
	for _, path := range paths {
		h.app.Explorer.Tree.SelectByID(path)
		h.pressKey(tcell.KeyEnter, tcell.ModNone)
		if got := h.app.EditorGroup.ActiveFilePath(); got != path {
			t.Fatalf("opened %q, want %q", got, path)
		}
		if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != filepath.Base(path) {
			t.Fatalf("opened wrong file: %q", got)
		}
		h.exec("sidebar.explorer")
	}
}

func TestExplorerDecomposedFilenameNavigation(t *testing.T) {
	h := newTestHarness(t, 120, 32)
	defer h.stop()
	dir := filepath.Join(h.dir, "re\u0301pertoire")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "e\u0301lastique.txt")
	if err := os.WriteFile(path, []byte("original file contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.exec("explorer.refresh")
	h.exec("sidebar.explorer")
	h.app.Explorer.Tree.SelectByID(dir)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if node := h.app.Explorer.Tree.Selected(); node.ID != dir || !node.Expanded {
		t.Fatalf("directory navigation changed path: %+v", node)
	}
	if !strings.Contains(h.screenText(), "re\u0301pertoire") || !strings.Contains(h.screenText(), "e\u0301lastique.txt") {
		t.Fatalf("decomposed names missing from rendered Explorer:\n%s", h.screenText())
	}
	h.app.Explorer.Tree.SelectByID(path)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if got := h.app.EditorGroup.ActiveFilePath(); got != path {
		t.Fatalf("opened %q, want original path %q", got, path)
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "original file contents" {
		t.Fatalf("opened wrong content: %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "e\u0301lastique.txt" {
		t.Fatalf("filesystem name changed: %v, %v", entries, err)
	}
}
