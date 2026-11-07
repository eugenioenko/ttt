package app

import (
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestEditorFileLinksRenderAndInputWhileResolving(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	delivery := make(chan func(), 1)
	a.tasks.post = func(fn func()) error { delivery <- fn; return nil }
	var work func()
	a.fileLinks = NewTask[map[string]string](a, asynctask.TaskOptions{
		KeepRunning: true, Run: func(fn func()) { work = fn },
	})
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	a.resolveFileLink = func(path, dir string) string {
		close(started)
		<-release
		return filepath.Join(dir, path)
	}
	e := a.EditorGroup.Editor
	e.Buf = &buffer.Buffer{Lines: []string{"target.go:12:3 https://example.com"}}
	e.LinkWorkDir = t.TempDir()
	e.ResolveFileLinks = a.EditorGroup.ResolveFileLinks
	e.SetRect(ui.Rect{W: 80, H: 4})
	openedURL, openedFile := false, false
	e.OnOpenURL = func(string) { openedURL = true }
	e.OnOpenFile = func(path string, line, col int) {
		openedFile = path == filepath.Join(e.LinkWorkDir, "target.go") && line == 12 && col == 3
	}
	cells := make([][]term.Cell, 4)
	for y := range cells {
		cells[y] = make([]term.Cell, 80)
	}
	render := func() { e.Render(ui.NewRenderSurface(cells, e.GetRect())) }
	render()
	x := e.GutterWidth()
	e.HandleEvent(tcell.NewEventMouse(x, 0, 0, tcell.ModCtrl))
	render()
	if work == nil || e.PointerShapeAt(x, 0) == term.PointerHand {
		t.Fatal("resolution was not queued independently of file hit targets")
	}
	go func() { work(); close(done) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("resolver did not start")
	}
	a.Root.AddGlobalKey(tcell.KeyRight, tcell.ModCtrl, 0, func() { e.MoveWordRight(false) })
	a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModCtrl))
	render()
	if e.Cursor.Col == 0 {
		t.Fatal("input could not move the cursor while resolution was blocked")
	}
	urlX := x + len("target.go:12:3 ")
	e.HandleEvent(tcell.NewEventMouse(urlX, 0, tcell.Button1, tcell.ModCtrl))
	e.HandleEvent(tcell.NewEventMouse(urlX, 0, 0, tcell.ModCtrl))
	if !openedURL || openedFile {
		t.Fatal("URL availability depended on file resolution")
	}
	unblock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("resolver did not finish")
	}
	render()
	if e.PointerShapeAt(x, 0) == term.PointerHand {
		t.Fatal("worker applied its result before main-thread delivery")
	}
	(<-delivery)()
	render()
	if e.PointerShapeAt(x, 0) != term.PointerHand {
		t.Fatal("main-thread delivery did not enable the resolved file")
	}
	e.HandleEvent(tcell.NewEventMouse(x, 0, tcell.Button1, tcell.ModCtrl))
	if !openedFile {
		t.Fatal("resolved file lost path or line/column navigation")
	}
}

func TestEditorFileLinkResolutionBoundsWorkAndCloses(t *testing.T) {
	a := &App{}
	var work, delivery []func()
	a.tasks.post = func(fn func()) error { delivery = append(delivery, fn); return nil }
	a.fileLinks = NewTask[map[string]string](a, asynctask.TaskOptions{
		KeepRunning: true, Run: func(fn func()) { work = append(work, fn) },
	})
	var resolved, applied []string
	a.resolveFileLink = func(path, dir string) string { resolved = append(resolved, path); return path }
	apply := func(paths map[string]string) {
		for path := range paths {
			applied = append(applied, path)
		}
	}
	for _, path := range []string{"first", "discarded", "latest"} {
		a.resolveEditorFileLinks([]string{path}, "", apply)
	}
	if len(work) != 1 {
		t.Fatalf("started %d workers for changing viewports", len(work))
	}
	work[0]()
	delivery[0]()
	if len(work) != 2 {
		t.Fatal("latest viewport was not queued behind the running batch")
	}
	work[1]()
	delivery[1]()
	if !slices.Equal(resolved, []string{"first", "latest"}) || !slices.Equal(applied, resolved) {
		t.Fatalf("resolved %v, applied %v", resolved, applied)
	}
	a.resolveEditorFileLinks([]string{"closed"}, "", apply)
	a.CloseTasks()
	work[2]()
	if len(resolved) != 2 || len(applied) != 2 {
		t.Fatal("shutdown did not discard outstanding resolution")
	}
}
