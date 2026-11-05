package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func linkEditor(lines []string, w, h int) *EditorPaneWidget {
	e := newRowsEditor(lines, w, h)
	e.OnOpenURL = func(string) {}
	e.Undo = &undo.UndoStack{}
	return e
}

func hoverEditorLinks(e *EditorPaneWidget) [][]term.Cell {
	renderGrid(e)
	r := e.GetRect()
	e.HandleEvent(tcell.NewEventMouse(r.X+e.GutterWidth(), r.Y, 0, tcell.ModCtrl))
	return renderGrid(e)
}

func TestEditorLinksGeometry(t *testing.T) {
	for _, tt := range []struct {
		text string
		left int
		wrap bool
	}{
		{"(https://example.com), plain", 0, false},
		{"界\t https://example.com/世界. plain", 0, false},
		{"\xff\t https://example.com/path", 0, false},
		{"界\t https://example.com/世界. plain", 12, false},
		{"界\t https://example.com/世界. plain", 0, true},
	} {
		t.Run(fmt.Sprint(tt), func(t *testing.T) {
			e := linkEditor([]string{tt.text}, 23, 8)
			e.WordWrap, e.Viewport.LeftCol = tt.wrap, tt.left
			var opened []string
			e.OnOpenURL = func(s string) { opened = append(opened, s) }
			grid := hoverEditorLinks(e)
			if len(e.links.hits) == 0 {
				t.Fatal("no visible links")
			}
			for p, span := range e.links.hits {
				if !grid[p.y][p.x].Underline || e.PointerShapeAt(p.x, p.y) != term.PointerHand {
					t.Fatalf("missing link feedback at %v", p)
				}
				for range 2 {
					e.HandleEvent(tcell.NewEventMouse(p.x, p.y, tcell.Button1, tcell.ModCtrl))
				}
				e.HandleEvent(tcell.NewEventMouse(p.x, p.y, 0, tcell.ModCtrl))
				if len(opened) != 1 || opened[0] != span.URL || e.mouseDown || e.Selection.Active {
					t.Fatalf("opened %v for %q, selecting=%v", opened, span.URL, e.mouseDown)
				}
				opened = nil
			}
		})
	}
}

func TestEditorLinksExcludeNonText(t *testing.T) {
	e := linkEditor([]string{"https://visible.test", "fold", "https://hidden.test"}, 30, 8)
	e.Folds = fold.NewState()
	e.Folds.SetRanges([]fold.Range{{StartLine: 1, EndLine: 2}})
	e.Folds.Toggle(1)
	e.LineNumbers = true
	e.SetDiffOverlay(&DiffOverlay{Deleted: map[int][]diff.SideLine{0: {{Text: "https://deleted.test"}}}})
	grid := hoverEditorLinks(e)
	if len(e.links.hits) == 0 {
		t.Fatal("real link missing")
	}
	for p, span := range e.links.hits {
		if span.URL != "https://visible.test" || p.x < e.GutterWidth() || e.rowMap[p.y].isPhantom() {
			t.Fatalf("invalid hit %v: %+v", p, span)
		}
	}
	for y, row := range grid {
		for x, cell := range row {
			if _, hit := e.editorLinkAt(x, y); cell.Underline != hit {
				t.Fatalf("decoration and hit target disagree at %d,%d", x, y)
			}
		}
	}
	e.SetDiffOverlay(&DiffOverlay{Labels: map[int]string{0: "https://gap.test"}})
	grid = hoverEditorLinks(e)
	for x, cell := range grid[0] {
		if _, hit := e.editorLinkAt(x, 0); cell.Underline || hit {
			t.Fatal("gap label became interactive")
		}
	}
}

func TestEditorLinksPreserveEditingAndStyles(t *testing.T) {
	e := linkEditor([]string{"https://example.com/界"}, 40, 4)
	e.Selection.Start(0, 1)
	e.Cursor.Col = 3
	e.SearchMatches = []FindMatch{{Line: 0, Col: 5, Len: 4}}
	e.searchByLine = map[int][]int{0: {0}}
	e.Diagnostics = []Diagnostic{{EndCol: 5, Severity: DiagError}}
	e.diagByLine = map[int][]int{0: {0}}
	e.SetDiffOverlay(&DiffOverlay{Kinds: []diff.LineKind{diff.Added}})
	before, after := renderGrid(e), hoverEditorLinks(e)
	for p := range e.links.hits {
		cell := after[p.y][p.x]
		cell.Underline = false
		if before[p.y][p.x].Ch != ' ' && cell != before[p.y][p.x] {
			t.Fatalf("link changed existing styling at %v", p)
		}
	}
	x := e.GutterWidth() + 1
	e.HandleEvent(tcell.NewEventMouse(x, 0, tcell.Button1, tcell.ModCtrl))
	e.HandleEvent(tcell.NewEventMouse(x+5, 0, tcell.Button1, 0))
	if e.mouseDown {
		t.Fatal("releasing Ctrl started a text drag")
	}
	e.HandleEvent(tcell.NewEventMouse(x+5, 0, 0, 0))
	e.HandleEvent(tcell.NewEventMouse(x, 0, tcell.Button1, 0))
	e.HandleEvent(tcell.NewEventMouse(x+4, 0, tcell.Button1, 0))
	e.HandleEvent(tcell.NewEventMouse(x+4, 0, 0, 0))
	if !e.Selection.Active || e.Cursor.Col != 5 || e.links.active {
		t.Fatal("ordinary drag selection changed")
	}
	e.HandleEvent(tcell.NewEventMouse(x+7, 0, tcell.Button1, tcell.ModAlt))
	if !e.isMultiActive() {
		t.Fatal("Alt-click did not create a cursor")
	}
}

func TestEditorLinksClearThroughRoot(t *testing.T) {
	for _, ev := range []tcell.Event{
		tcell.NewEventMouse(100, 100, 0, tcell.ModCtrl),
		tcell.NewEventMouse(2, 0, 0, 0),
		tcell.NewEventKey(tcell.KeyEscape, "", 0), tcell.NewEventFocus(false), nil,
	} {
		e := linkEditor([]string{"https://example.com"}, 40, 4)
		r := NewRoot(e)
		e.OnLinkHover = r.TrackEditorLinkHover
		hoverEditorLinks(e)
		if ev == nil {
			r.PushOverlay(Overlay{Widget: &BaseWidget{}, Modal: true})
		} else {
			r.HandleEvent(ev)
		}
		if e.links.active || len(e.links.hits) != 0 || r.linkHover != nil {
			t.Fatalf("feedback survived %T", ev)
		}
	}
}

func TestEditorLinkReleaseClearsDiffOwners(t *testing.T) {
	for _, embedded := range []bool{false, true} {
		t.Run(fmt.Sprint(embedded), func(t *testing.T) {
			patch := diff.Parse("--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+https://example.com\n")
			view := NewDiffEditorWidget("file.txt", patch, nil, nil, false)
			var widget Widget = view
			var detail *CommitDetailWidget
			if embedded {
				detail = NewCommitDetailWidget("", "HEAD", "HEAD", false)
				detail.SetDetail("Subject", []CommitDetailFile{{Path: "file.txt", Diff: patch}}, "")
				view, widget = detail.Files[0].view, detail
			}
			r := NewRoot(widget)
			opened := 0
			configure := func(e *EditorPaneWidget, _ string) {
				e.OnOpenURL = func(string) { opened++ }
				e.OnLinkHover = r.TrackEditorLinkHover
			}
			for _, pane := range view.viewPanes() {
				configure(pane, "")
			}
			if detail != nil {
				detail.configurePaneLinks = configure
			}
			r.SetSize(80, 20)
			grid := makeGrid(80, 20)
			r.Render(grid)
			x, y, ok := detailFind(grid, "https://example.com")
			if !ok {
				t.Fatal("URL missing from diff")
			}
			r.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
			r.Render(grid)
			r.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModCtrl))
			r.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
			if opened != 1 || view.pressed || (detail != nil && detail.primaryPressed) {
				t.Fatalf("link gesture left an owner pressed: opened=%d diff=%v detail=%v", opened, view.pressed, detail != nil && detail.primaryPressed)
			}
		})
	}
}

func TestEditorLinksPathFallback(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(t.TempDir(), "source.go")
	g := &EditorGroupWidget{LinkWorkDir: func() string { return workspace }}
	for _, tt := range []struct{ path, dir string }{
		{file, filepath.Dir(file)}, {"", workspace}, {"src/source.go", workspace}, {"Preview/Title", workspace},
	} {
		e := linkEditor(nil, 40, 4)
		g.configurePaneLinks(e, tt.path)
		if e.LinkWorkDir != tt.dir {
			t.Errorf("link directory for %q = %q, want %q", tt.path, e.LinkWorkDir, tt.dir)
		}
	}
}

func TestEditorLinksInsideLongToken(t *testing.T) {
	prefix := strings.Repeat("x", 3*editorLinkLookaround) + `="`
	url := "https://example.com/"
	for _, tt := range []struct{ suffix, want string }{
		{url + `"` + strings.Repeat("x", 3*editorLinkLookaround), url},
		{url + strings.Repeat("a", 3*editorLinkLookaround), ""},
		{url + strings.Repeat("a", 35+editorLinkLookaround-len(url)-1) + ".tail", ""},
	} {
		e := linkEditor([]string{prefix + tt.suffix}, 36, 4)
		e.Viewport.LeftCol = len(prefix)
		hoverEditorLinks(e)
		if len(e.links.hits) == 0 && tt.want != "" {
			t.Fatal("complete URL inside a long token was discarded")
		}
		for _, span := range e.links.hits {
			if span.URL != tt.want {
				t.Fatalf("opened a truncated URL %q, want %q", span.URL, tt.want)
			}
		}
	}
}

func TestEditorFileLinkCache(t *testing.T) {
	e := linkEditor([]string{"target.go:12:3"}, 12, 8)
	e.WordWrap = true
	e.LinkWorkDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(e.LinkWorkDir, "target.go"), []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	probes := 0
	e.links.detect = func(text, dir string) []linkSpan { probes++; return detectLinks(text, dir) }
	e.OnOpenFile = func(string, int, int) {}
	hoverEditorLinks(e)
	for range 3 {
		renderGrid(e)
	}
	if probes != 1 || len(e.links.hits) == 0 {
		t.Fatal("wrapped segments repeated file resolution or lost links")
	}
	e.LinkWorkDir = t.TempDir()
	renderGrid(e)
	if probes != 2 || len(e.links.hits) != 0 {
		t.Fatal("directory change did not refresh file resolution")
	}
}

func TestEditorLinksBoundedCache(t *testing.T) {
	lines := make([]string, 100_000)
	for i := range lines {
		lines[i] = fmt.Sprintf("https://line%d.test", i)
	}
	e := linkEditor(lines, 30, 4)
	var visited []string
	e.links.detect = func(s, dir string) []linkSpan { visited = append(visited, s); return detectLinks(s, dir) }
	renderGrid(e)
	if len(visited) != 0 || len(e.links.cells) != 0 {
		t.Fatal("link work ran without Ctrl")
	}
	hoverEditorLinks(e)
	for range 3 {
		renderGrid(e)
	}
	if len(visited) != e.Viewport.Height || visited[0] != lines[0] {
		t.Fatalf("detection visited %d lines", len(visited))
	}
	for i, text := range visited {
		if text != lines[i] {
			t.Fatalf("detected offscreen text %q", text)
		}
	}
	for _, tt := range []struct {
		change func()
		url    string
	}{
		{func() { e.Viewport.TopLine = 1 }, "https://line1.test"},
		{func() { e.SetRect(Rect{W: 20, H: 4}) }, "https://line1.test"},
		{func() { e.exec(&undo.InsertStringCommand{Line: 1, Col: 8, Text: "new-"}) }, "https://new-line1.test"},
	} {
		tt.change()
		renderGrid(e)
		if span, ok := e.editorLinkAt(e.GutterWidth()+1, 0); !ok || span.URL != tt.url {
			t.Fatalf("viewport/edit link = %+v, want %s", span, tt.url)
		}
	}
	for _, text := range []string{
		"https://" + strings.Repeat("x", 2_000_000),
		strings.Repeat("x", 1_000_000) + " https://tail.test " + strings.Repeat("x", 1_000_000),
	} {
		e = linkEditor([]string{text}, 35, 4)
		e.Viewport.LeftCol = 1_000_005
		calls := 0
		e.links.detect = func(s, dir string) []linkSpan {
			calls++
			if len(s) > 2*editorLinkLookaround+35 {
				t.Fatalf("scanned %d hidden bytes", len(s))
			}
			return detectLinks(s, dir)
		}
		hoverEditorLinks(e)
		renderGrid(e)
		if calls != 1 || (strings.Contains(text, "tail.test") != (len(e.links.hits) > 0)) {
			t.Fatalf("long-line detection: calls=%d hits=%d", calls, len(e.links.hits))
		}
	}
}
