package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestContainingFolder(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := containingFolder(file); got != dir {
		t.Errorf("file: got %q, want %q", got, dir)
	}
	if got := containingFolder(dir); got != dir {
		t.Errorf("dir: got %q, want %q", got, dir)
	}

	// Non-existent path falls back to filepath.Dir.
	missing := filepath.Join(dir, "sub", "gone.txt")
	if got := containingFolder(missing); got != filepath.Dir(missing) {
		t.Errorf("missing: got %q, want %q", got, filepath.Dir(missing))
	}
}

func TestFileDeleteTaskDeliveryAndShutdown(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	for _, shutdown := range []bool{false, true} {
		t.Run(map[bool]string{false: "delivery", true: "shutdown"}[shutdown], func(t *testing.T) {
			a := buildTestApp(t, config.DefaultSettings())
			t.Cleanup(a.CloseTasks)
			baselineTasks := len(a.tasks.tasks)
			posted := make(chan func(), 1)
			a.tasks.post = func(fn func()) error { posted <- fn; return nil }
			contexts := make(chan context.Context, 1)
			reloaded := false
			a.confirmFileDelete("note.txt", func() { reloaded = true }, func(ctx context.Context, _ string) error {
				contexts <- ctx
				return nil
			})
			a.Root.TopOverlayWidget().(*ui.WidgetAdapter).Inner().(*widgets.DialogWidget).Buttons[1].Handler()
			var apply func()
			select {
			case apply = <-posted:
			case <-time.After(time.Second):
				t.Fatal("delete task did not post completion")
			}
			if reloaded || a.Root.HasOverlay() {
				t.Fatal("worker applied UI changes or left the confirmation open")
			}
			ctx := <-contexts
			if shutdown {
				a.CloseTasks()
				if ctx.Err() != context.Canceled {
					t.Fatal("shutdown did not cancel the delete task")
				}
			}
			apply()
			remaining := baselineTasks
			if shutdown {
				remaining = 0
			}
			if reloaded == shutdown || len(a.tasks.tasks) != remaining {
				t.Fatalf("reload=%v, remaining tasks=%d", reloaded, len(a.tasks.tasks))
			}
		})
	}
}

func TestPermanentDeleteFinishesBeforeQuit(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	baselineTasks := len(a.tasks.tasks)
	posted := make(chan func(), 1)
	a.tasks.post = func(fn func()) error { posted <- fn; return nil }
	dir := filepath.Join(t.TempDir(), "folder")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child.txt"), []byte("delete me"), 0o600); err != nil {
		t.Fatal(err)
	}
	finished := false
	a.startFileDelete(dir, false, func() { finished = true }, nil)
	if !finished || len(a.tasks.tasks) != baselineTasks {
		t.Fatal("permanent deletion returned with unfinished work")
	}
	running := true
	a.Running = &running
	a.Quit()
	if running {
		t.Fatal("quit did not complete after permanent deletion")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("quit left directory contents behind: %v", err)
	}
}

func TestFileDeleteConfirmation(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	for _, tc := range []struct {
		name                   string
		useTrash, cancel, fail bool
	}{
		{name: "trash", useTrash: true},
		{name: "permanent"},
		{name: "trash failure", useTrash: true, fail: true},
		{name: "cancel trash", useTrash: true, cancel: true},
		{name: "cancel permanent", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := config.DefaultSettings()
			settings.Explorer.UseTrash = tc.useTrash
			a := buildTestApp(t, settings)
			t.Cleanup(a.CloseTasks)
			path := filepath.Join(t.TempDir(), "note.txt")
			if err := os.WriteFile(path, []byte("recover me"), 0o600); err != nil {
				t.Fatal(err)
			}
			moved := filepath.Join(t.TempDir(), "note.txt")
			called, reloaded := false, false
			a.confirmFileDelete(path, func() { reloaded = true }, func(_ context.Context, got string) error {
				called = true
				if got != path {
					t.Fatalf("trash target = %q, want %q", got, path)
				}
				if tc.fail {
					return os.ErrPermission
				}
				return os.Rename(got, moved)
			})
			dialog := a.Root.TopOverlayWidget().(*ui.WidgetAdapter).Inner().(*widgets.DialogWidget)
			want := "Permanently delete note.txt? This cannot be undone."
			if tc.useTrash {
				want = "Move note.txt to Trash?"
			}
			if got := dialog.Content.(*widgets.ParagraphWidget).Text; got != want {
				t.Fatalf("confirmation = %q, want %q", got, want)
			}
			// The approved operation must keep matching the displayed confirmation.
			a.Settings.Explorer.UseTrash = !tc.useTrash
			button := 1
			if tc.cancel {
				button = 0
			}
			dialog.Buttons[button].Handler()
			if a.Root.HasOverlay() || called != (tc.useTrash && !tc.cancel) || reloaded != (!tc.cancel && !tc.fail) {
				t.Fatalf("overlay=%v, trash called=%v, reloaded=%v", a.Root.HasOverlay(), called, reloaded)
			}
			if tc.cancel || tc.fail {
				if data, err := os.ReadFile(path); err != nil || string(data) != "recover me" {
					t.Fatalf("source changed after cancellation/failure: %q, %v", data, err)
				}
			} else if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("source still exists: %v", err)
			} else if tc.useTrash {
				if data, err := os.ReadFile(moved); err != nil || string(data) != "recover me" {
					t.Fatalf("trash lost file: %q, %v", data, err)
				}
			}
		})
	}
}

func TestNextDuplicatePath(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name     string
		src      string
		existing []string
		want     string
	}{
		{name: "file", want: "note copy.txt"},
		{name: "second", existing: []string{"note copy.txt"}, want: "note copy 2.txt"},
		{name: "third", existing: []string{"note copy.txt", "note copy 2.txt"}, want: "note copy 3.txt"},
		{name: "dotfile", src: ".env", want: ".env copy"},
		{name: "double extension", src: "archive.tar.gz", want: "archive.tar copy.gz"},
		{name: "no extension", src: "README", want: "README copy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srcName := tc.src
			if srcName == "" {
				srcName = "note.txt"
			}
			src := filepath.Join(dir, srcName)
			if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, existing := range tc.existing {
				if err := os.WriteFile(filepath.Join(dir, existing), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := nextDuplicatePath(src)
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(dir, tc.want); got != want {
				t.Fatalf("nextDuplicatePath(%q) = %q, want %q", srcName, got, want)
			}
		})
	}
}

func TestCopyTreeRecursive(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "folder")
	dst := filepath.Join(dir, "folder copy")
	if err := os.MkdirAll(filepath.Join(src, "nested", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "top.txt"), []byte("top"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "child.txt"), []byte("child"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "deep", "leaf.txt"), []byte("leaf"), 0o640); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join("nested", "child.txt")
	if err := os.Symlink(target, filepath.Join(src, "link.txt")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation requires elevated privileges on Windows")
		}
		t.Fatal(err)
	}

	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		"top.txt":              "top",
		"nested/child.txt":     "child",
		"nested/deep/leaf.txt": "leaf",
	} {
		data, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil || string(data) != want {
			t.Errorf("copy %s = %q (%v), want %q", rel, data, err, want)
		}
	}
	srcInfo, err := os.Stat(filepath.Join(src, "nested", "child.txt"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(dst, "nested", "child.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), srcInfo.Mode().Perm(); got != want {
		t.Errorf("nested file mode = %v, want %v (source mode)", got, want)
	}
	if got, err := os.Readlink(filepath.Join(dst, "link.txt")); err != nil || got != target {
		t.Errorf("symlink target = %q (%v), want %q", got, err, target)
	}
}

func TestCopyRegularFilePreservesMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "copy.txt")
	if err := os.WriteFile(src, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(src, 0o664); err != nil {
		t.Fatal(err)
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}

	if err := copyRegularFile(src, dst, srcInfo.Mode()); err != nil {
		t.Fatal(err)
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := dstInfo.Mode().Perm(), srcInfo.Mode().Perm(); got != want {
		t.Errorf("copied mode = %v, want %v (source mode)", got, want)
	}
}

func TestCopyTreeReadOnlySourceDirectory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "locked")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "child.txt"), []byte("child"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(src, 0o555); err != nil {
		t.Fatal(err)
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if srcInfo.Mode().Perm()&0o200 != 0 {
		t.Fatalf("source directory is writable: %v", srcInfo.Mode().Perm())
	}
	dst := filepath.Join(dir, "locked copy")
	t.Cleanup(func() {
		_ = os.Chmod(src, 0o700)
		_ = os.Chmod(dst, 0o700)
	})

	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "child.txt")); err != nil || string(data) != "child" {
		t.Fatalf("copy = %q (%v), want %q", data, err, "child")
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := dstInfo.Mode().Perm(), srcInfo.Mode().Perm(); got != want {
		t.Errorf("directory mode = %v, want %v (source mode)", got, want)
	}
}

func TestFileOpDuplicateDoesNotOpenTab(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)

	dir := t.TempDir()
	src := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(src, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := len(a.EditorGroup.TabBar.Tabs)
	reloaded := false
	a.FileOpDuplicate(src, func() { reloaded = true })
	if got := len(a.EditorGroup.TabBar.Tabs); got != before {
		t.Fatalf("duplicate opened %d tab(s)", got-before)
	}
	if !reloaded {
		t.Fatal("duplicate did not reload the explorer")
	}
	dup := filepath.Join(dir, "note copy.txt")
	if data, err := os.ReadFile(dup); err != nil || string(data) != "original" {
		t.Fatalf("duplicate = %q (%v), want %q", data, err, "original")
	}
}

func TestFileOpDuplicateMessageReportsJoinedCleanupError(t *testing.T) {
	src := filepath.Join("folder", "pipe")
	dst := filepath.Join("tmp", "folder copy")
	unsupported := unsupportedFileTypeError{path: src}
	cleanup := errors.New(i18n.Format("files.duplicateCleanupFailed", i18n.Args{
		"path":  dst,
		"error": "permission denied",
	}))

	got := fileOpDuplicateMessage(unsupported)
	wantUnsupported := i18n.Format("files.unsupportedType", i18n.Args{"path": src})
	if got != wantUnsupported {
		t.Errorf("unsupported only: got %q, want %q", got, wantUnsupported)
	}

	got = fileOpDuplicateMessage(errors.Join(unsupported, cleanup))
	if want := wantUnsupported + "\n" + cleanup.Error(); got != want {
		t.Errorf("joined: got %q, want %q", got, want)
	}
	var as unsupportedFileTypeError
	if !errors.As(errors.Join(unsupported, cleanup), &as) {
		t.Fatal("joined error should still match unsupportedFileTypeError")
	}
}
