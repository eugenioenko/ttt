package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalFormatterGofmt(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available")
	}

	h := newTestHarness(t, 80, 24)

	unformatted := "package main\n\nimport (\n\"fmt\"\n)\n\nfunc    main()    {\nfmt.Println(   \"hello\"   )\n}"
	requireNoError(t, os.WriteFile(filepath.Join(h.dir, "test.go"), []byte(unformatted), 0644))
	h.app.Settings.Formatters = map[string]string{"go": "gofmt"}
	h.openFile(filepath.Join(h.dir, "test.go"))
	h.redraw()

	h.exec("editor.formatExternal")

	lines := h.app.EditorGroup.Editor.Buf.Lines
	content := strings.Join(lines, "\n")

	if strings.Contains(content, "func    main()") {
		t.Error("expected gofmt to fix spacing in func declaration")
	}
	if !strings.Contains(content, "func main() {") {
		t.Errorf("expected formatted func signature, got: %s", content)
	}
	if !strings.Contains(content, "\t\"fmt\"") {
		t.Errorf("expected gofmt to add tab indentation to import, got: %s", content)
	}
	if !strings.Contains(content, "\tfmt.Println(\"hello\")") {
		t.Errorf("expected gofmt to fix Println spacing, got: %s", content)
	}
}

func TestExternalFormatterNoConfig(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	requireNoError(t, os.WriteFile(filepath.Join(h.dir, "test.txt"), []byte("hello world"), 0644))
	h.openFile(filepath.Join(h.dir, "test.txt"))
	h.redraw()

	h.exec("editor.formatExternal")

	screen := h.screenText()
	if !strings.Contains(screen, "No formatter configured") {
		t.Error("expected 'No formatter configured' status message")
	}
}

func TestExternalFormatterOnSavePriority(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available")
	}

	h := newTestHarness(t, 80, 24)

	unformatted := "package main\n\nfunc    main()    {\n}"
	requireNoError(t, os.WriteFile(filepath.Join(h.dir, "test.go"), []byte(unformatted), 0644))
	h.app.Settings.Formatters = map[string]string{"go": "gofmt"}
	h.app.Settings.Editor.FormatOnSave = true
	h.openFile(filepath.Join(h.dir, "test.go"))
	h.redraw()

	h.exec("file.save")

	lines := h.app.EditorGroup.Editor.Buf.Lines
	content := strings.Join(lines, "\n")
	if !strings.Contains(content, "func main() {") {
		t.Errorf("expected format on save to run gofmt, got: %s", content)
	}
}
