package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
)

func TestFindShowsNoMatchesOnContentTab(t *testing.T) {
	for _, tc := range []struct {
		name  string
		file  string
		write func(t *testing.T, path string)
		want  string
	}{
		{
			name:  "image",
			file:  "photo.png",
			write: func(t *testing.T, path string) { writeE2EPNG(t, path, 16, 12) },
			want:  "PNG",
		},
		{
			name: "binary",
			file: "blob.bin",
			write: func(t *testing.T, path string) {
				if err := os.WriteFile(path, []byte{0x89, 'P', 'N', 'G', 0x00, 0xff, '\n', 0x00}, 0644); err != nil {
					t.Fatal(err)
				}
			},
			want: "Binary file",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t, 100, 30)
			defer h.stop()

			textPath := filepath.Join(h.dir, "notes.txt")
			if err := os.WriteFile(textPath, []byte("cat\ncat\ncat\n"), 0644); err != nil {
				t.Fatal(err)
			}
			h.app.EditorGroup.OpenFile(textPath)
			h.app.EditorGroup.CommitActiveTab()
			h.redraw()

			h.exec("search.find")
			for _, r := range "cat" {
				h.pressRune(r)
			}
			h.assertContains(" 1/3")

			contentPath := filepath.Join(h.dir, tc.file)
			tc.write(t, contentPath)
			h.app.EditorGroup.OpenFile(contentPath)
			h.redraw()

			if h.app.EditorGroup.IsEditorActive() {
				t.Fatal("content tab should not leave the text editor active")
			}
			h.assertContains(tc.want)
			fb := findBarWidget(t, h)
			if fb.Input.Text != "cat" {
				t.Fatalf("query = %q, want it kept", fb.Input.Text)
			}
			if len(fb.Matches) != 0 {
				t.Fatalf("content tab matches = %d, want 0", len(fb.Matches))
			}
			h.assertContains(" 0/0")
			h.assertNotContains(" 1/3")

			h.app.EditorGroup.SwitchTab(0)
			h.redraw()
			if fb.Input.Text != "cat" {
				t.Fatalf("query after return = %q, want cat", fb.Input.Text)
			}
			if len(fb.Matches) != 3 {
				t.Fatalf("text tab matches = %d, want 3", len(fb.Matches))
			}
			h.assertContains(" 1/3")
		})
	}
}

func findBarWidget(t *testing.T, h *testHarness) *ui.FindBarWidget {
	t.Helper()
	fb, ok := h.app.Root.TopOverlayWidget().(*ui.FindBarWidget)
	if !ok {
		t.Fatalf("expected FindBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	return fb
}
