package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/workspace"
	"github.com/gdamore/tcell/v3"
)

func TestQuickOpenDefersFilesAndDropsDismissedDelivery(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	t.Cleanup(a.CloseTasks)
	dir := t.TempDir()
	for _, name := range []string{"first.txt", "second.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	a.Workspace = workspace.New([]string{dir})
	taskCount := len(a.tasks.tasks)
	var work, delivery []func()
	a.tasks.post = func(fn func()) error { delivery = append(delivery, fn); return nil }
	open := func() *ui.SelectDialogWidget {
		a.OpenCommandPalette(false)
		state := a.quickOpen
		if state.task.Pending() {
			t.Fatal("command palette started file discovery")
		}
		a.releaseTask(state.task)
		state.task = NewTask[[]ui.PaletteFile](a, asynctask.TaskOptions{
			Run: func(fn func()) { work = append(work, fn) },
		})
		return state.palette
	}
	p := open()
	for _, input := range []string{"?", ":10", ">"} {
		p.Input.SetText(input)
	}
	if len(work) != 0 {
		t.Fatal("non-file modes started discovery")
	}
	p.Input.SetText("first")
	p.Input.SetText("second")
	if len(work) != 1 || len(p.Items) != 0 {
		t.Fatal("file mode did not queue exactly one scan")
	}
	work[0]()
	work = nil
	if len(p.Items) != 0 || len(delivery) != 1 {
		t.Fatal("worker applied files outside UI delivery")
	}
	delivery[0]()
	delivery = nil
	if len(p.Items) != 1 || p.Items[0].ID != filepath.Join(dir, "second.txt") {
		t.Fatalf("files did not use latest query: %+v", p.Items)
	}
	p.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", 0))
	if a.quickOpen != nil || len(a.tasks.tasks) != taskCount {
		t.Fatal("Escape did not release palette task")
	}
	p = open()
	p.Input.SetText("")
	work[0]()
	work = nil
	a.DismissDialog()
	reopened := open()
	delivery[0]()
	if len(p.Items) != 0 || reopened.Input.Text() != ">" || a.quickOpen.task.Pending() {
		t.Fatal("dismissed scan affected closed or reopened palette")
	}
}

func TestQuickOpenDismissCancelsActiveTask(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	t.Cleanup(a.CloseTasks)
	a.OpenCommandPalette(false)
	state := a.quickOpen
	var work func()
	var captured context.Context
	a.releaseTask(state.task)
	a.tasks.post = func(func()) error { return nil }
	state.task = NewTask[[]ui.PaletteFile](a, asynctask.TaskOptions{Run: func(fn func()) { work = fn }})
	state.task.Schedule(func(ctx context.Context) ([]ui.PaletteFile, error) {
		captured = ctx
		return nil, ctx.Err()
	}, func([]ui.PaletteFile, error) { t.Fatal("cancelled task applied files") })
	work()
	a.DismissDialog()
	if captured.Err() != context.Canceled {
		t.Fatal("dismiss did not cancel scan context")
	}
}

func TestQuickOpenDirectEntryLoadsFiles(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	t.Cleanup(a.CloseTasks)
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	a.Workspace = workspace.New([]string{dir})
	a.OpenCommandPalette(true)
	p := a.Root.TopOverlayWidget().(*ui.SelectDialogWidget)
	if p.Input.Text() != "" || len(p.Items) != 1 || p.Items[0].ID != path {
		t.Fatalf("direct file entry = %q, %+v", p.Input.Text(), p.Items)
	}
}
