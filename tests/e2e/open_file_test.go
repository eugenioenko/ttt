package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestOpenFilePickerPreservesWhitespace(t *testing.T) {
	for _, tc := range []struct {
		name        string
		filename    string
		fromList    bool
		trimmedFile bool
	}{
		{name: "typed leading space", filename: " example.txt", trimmedFile: true},
		{name: "typed trailing space", filename: "example.txt ", trimmedFile: true},
		{name: "typed leading and trailing spaces", filename: " example.txt ", trimmedFile: true},
		{name: "typed whitespace only", filename: " "},
		{name: "typed trailing space without trimmed file", filename: "example.txt "},
		{name: "listed trailing space", filename: "example.txt ", fromList: true, trimmedFile: true},
		{name: "listed trailing space without trimmed file", filename: "example.txt ", fromList: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && strings.HasSuffix(tc.filename, " ") {
				t.Skip("Windows ordinary paths cannot distinguish filenames with trailing spaces")
			}
			t.Chdir(t.TempDir())
			dir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, tc.filename)
			const content = "file with whitespace in its name\n"
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			if tc.trimmedFile {
				trimmed := filepath.Join(dir, strings.TrimSpace(tc.filename))
				if err := os.WriteFile(trimmed, []byte("wrong file\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			h := newTestHarness(t, 100, 30)
			defer h.stop()
			h.exec("file.open")
			h.pressKey(tcell.KeyCtrlA, tcell.ModCtrl)
			if tc.fromList {
				h.typeText(dir + string(os.PathSeparator))
				h.pressKey(tcell.KeyTab, tcell.ModNone)
				h.pressKey(tcell.KeyTab, tcell.ModNone)
				h.pressKey(tcell.KeyEnd, tcell.ModNone)
			} else {
				h.typeText(tc.filename)
			}
			h.pressKey(tcell.KeyEnter, tcell.ModNone)

			if got := h.app.EditorGroup.ActiveFilePath(); got != path {
				t.Fatalf("active file = %q, want %q", got, path)
			}
			if got := strings.Join(h.app.EditorGroup.Editor.Buf.Lines, "\n"); got != content {
				t.Fatalf("opened contents = %q, want %q", got, content)
			}
			if h.app.Root.HasOverlay() {
				t.Fatal("file picker should close after opening the exact file")
			}
		})
	}
}

func TestOpenFileFromCommandPaletteOutsideWorkspace(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	outside := t.TempDir()
	path := filepath.Join(outside, "outside.txt")
	if err := os.WriteFile(path, []byte("outside workspace content"), 0644); err != nil {
		t.Fatal(err)
	}
	workspaceBefore := h.app.Workspace.Paths()

	h.exec("command.palette")
	h.typeText("Open File")
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.assertContains("Open File")

	h.pressKey(tcell.KeyCtrlA, tcell.ModCtrl)
	h.typeText(path)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	if got := h.app.EditorGroup.ActiveFilePath(); got != path {
		t.Fatalf("active file = %q, want %q", got, path)
	}
	if got := strings.Join(h.app.EditorGroup.Editor.Buf.Lines, "\n"); got != "outside workspace content\n" {
		t.Fatalf("opened contents = %q", got)
	}
	if got := h.app.Workspace.Paths(); !slices.Equal(got, workspaceBefore) {
		t.Fatalf("workspace paths = %v, want unchanged %v", got, workspaceBefore)
	}
	if h.app.Root.HasOverlay() {
		t.Fatal("file picker should close after opening a file")
	}
}

func TestOpenFilePickerCancelLeavesEditorAndWorkspaceUnchanged(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	activeBefore := h.app.EditorGroup.ActiveFilePath()
	workspaceBefore := h.app.Workspace.Paths()
	h.exec("file.open")
	if !h.app.Root.HasOverlay() {
		t.Fatal("Open File should show the file picker")
	}
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if got := h.app.EditorGroup.ActiveFilePath(); got != activeBefore {
		t.Fatalf("active file after cancel = %q, want %q", got, activeBefore)
	}
	if got := h.app.Workspace.Paths(); !slices.Equal(got, workspaceBefore) {
		t.Fatalf("workspace paths after cancel = %v, want unchanged %v", got, workspaceBefore)
	}
	if h.app.Root.HasOverlay() {
		t.Fatal("file picker should close after cancellation")
	}
}

func TestOpenFilePickerRejectsMissingFile(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	missing := filepath.Join(t.TempDir(), "missing.txt")
	h.exec("file.open")
	h.pressKey(tcell.KeyCtrlA, tcell.ModCtrl)
	h.typeText(missing)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	h.assertContains("Cannot open file")
	if !h.app.Root.HasOverlay() {
		t.Fatal("picker should remain open when the requested file does not exist")
	}
	if h.app.EditorGroup.ActiveFilePath() == missing {
		t.Fatal("missing file should not be opened as an empty new file")
	}
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
}

func TestOpenFilePickerExplainsDirectorySelection(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	dir := t.TempDir()
	h.exec("file.open")
	h.pressKey(tcell.KeyCtrlA, tcell.ModCtrl)
	h.typeText(dir)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	h.assertContains("path is a directory")
	if !h.app.Root.HasOverlay() {
		t.Fatal("picker should remain open when the selected path is a directory")
	}
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
}

func TestOpenFilePickerRejectsNonRegularFile(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	activeBefore := h.app.EditorGroup.ActiveFilePath()
	h.exec("file.open")
	h.pressKey(tcell.KeyCtrlA, tcell.ModCtrl)
	h.typeText(os.DevNull)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	h.assertContains("not a regular file")
	if !h.app.Root.HasOverlay() || h.app.EditorGroup.ActiveFilePath() != activeBefore {
		t.Fatal("picker should remain open without opening the non-regular file")
	}
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
}
