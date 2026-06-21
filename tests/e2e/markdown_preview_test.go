package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkdownPreview_OpenViaCommand(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "readme.md")
	os.WriteFile(f, []byte("# Hello World\n\nSome text here.\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.exec("editor.openPreview")
	h.redraw()

	// Tab name should be "Preview: readme.md"
	h.assertContains("Preview: readme.md")
}

func TestMarkdownPreview_TabDedup(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "readme.md")
	os.WriteFile(f, []byte("# Hello\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.exec("editor.openPreview")
	h.redraw()

	// Open preview again — should switch to existing tab, not create a new one
	h.app.EditorGroup.OpenFile(f)
	h.redraw()
	h.exec("editor.openPreview")
	h.redraw()

	// Count tabs with "Preview:" prefix — there should be exactly one
	count := 0
	for _, name := range h.app.EditorGroup.TabNames() {
		if name == "Preview: readme.md" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 preview tab, got %d", count)
	}
}

func TestMarkdownPreview_HeadingRendered(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "test.md")
	os.WriteFile(f, []byte("# My Title\n\nSome content.\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.exec("editor.openPreview")
	h.redraw()

	// The rendered preview should show the heading text
	h.assertContains("# My Title")
}

func TestMarkdownPreview_NonMdFile(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "code.go")
	os.WriteFile(f, []byte("package main\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.exec("editor.openPreview")
	h.redraw()

	// Should NOT open a preview tab
	h.assertNotContains("Preview:")
}

func TestMarkdownPreview_ListRendered(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "list.md")
	os.WriteFile(f, []byte("- item one\n- item two\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.exec("editor.openPreview")
	h.redraw()

	h.assertContains("item one")
	h.assertContains("item two")
}
