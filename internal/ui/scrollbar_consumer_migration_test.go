package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type consumerScrollbarHarness struct {
	widget Widget
	render func() [][]term.Cell
	shrink func() [][]term.Cell
}

func renderAt(widget Widget, rect Rect) [][]term.Cell {
	cells := makeGrid(rect.X+rect.W+4, rect.Y+rect.H+4)
	widget.SetRect(rect)
	widget.Render(NewRenderSurface(cells, rect))
	return cells
}

func findScrollbarThumb(t *testing.T, cells [][]term.Cell) (int, int) {
	t.Helper()
	x, y, ok := findVerticalScrollbarThumb(cells)
	if ok {
		return x, y
	}
	t.Fatal("rendered vertical scrollbar thumb not found")
	return 0, 0
}

func findVerticalScrollbarThumb(cells [][]term.Cell) (int, int, bool) {
	for y, row := range cells {
		for x, cell := range row {
			if cell.Ch == '█' && cell.Style == term.StyleScrollbarThumb {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func scrollbarConsumerHarnesses() map[string]func() consumerScrollbarHarness {
	return map[string]func() consumerScrollbarHarness{
		"editor": func() consumerScrollbarHarness {
			lines := make([]string, 20)
			for i := range lines {
				lines[i] = strings.Repeat("x", 40)
			}
			editor := NewEditorPaneWidget(
				&buffer.Buffer{Lines: lines},
				&cursor.Cursor{},
				&view.Viewport{},
			)
			rect := Rect{X: 3, Y: 2, W: 20, H: 6}
			return consumerScrollbarHarness{
				widget: editor,
				render: func() [][]term.Cell { return renderAt(editor, rect) },
				shrink: func() [][]term.Cell {
					editor.Buf.Lines = []string{"short"}
					editor.maxWidthSeen = 0
					return renderAt(editor, rect)
				},
			}
		},
		"diff": func() consumerScrollbarHarness {
			var patch strings.Builder
			patch.WriteString("--- a/test.go\n+++ b/test.go\n@@ -1,20 +1,20 @@\n")
			for i := range 20 {
				fmt.Fprintf(&patch, " line %02d %s\n", i, strings.Repeat("x", 30))
			}
			diffView := NewDiffViewWidget("test.go", diff.Parse(patch.String()), nil, nil, false)
			rect := Rect{X: 4, Y: 3, W: 34, H: 6}
			return consumerScrollbarHarness{
				widget: diffView,
				render: func() [][]term.Cell { return renderAt(diffView, rect) },
				shrink: func() [][]term.Cell {
					diffView.Loading = true
					return renderAt(diffView, rect)
				},
			}
		},
		"commit detail": func() consumerScrollbarHarness {
			var patch strings.Builder
			patch.WriteString("--- a/test.go\n+++ b/test.go\n@@ -1,20 +1,20 @@\n")
			for i := range 20 {
				fmt.Fprintf(&patch, " line %02d %s\n", i, strings.Repeat("x", 30))
			}
			detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
			detail.SetDetail("Subject", []CommitDetailFile{{Path: "test.go", Diff: diff.Parse(patch.String())}}, "")
			rect := Rect{X: 5, Y: 4, W: 40, H: 8}
			return consumerScrollbarHarness{
				widget: detail,
				render: func() [][]term.Cell { return renderAt(detail, rect) },
				shrink: func() [][]term.Cell {
					detail.Loading = true
					return renderAt(detail, rect)
				},
			}
		},
		"search": func() consumerScrollbarHarness {
			search := NewSearchWidget()
			for i := range 20 {
				search.Groups = append(search.Groups, SearchFileGroup{RelPath: fmt.Sprintf("file-%02d.go", i)})
				search.FlatList = append(search.FlatList, searchItem{IsFile: true, Group: i})
			}
			rect := Rect{X: 6, Y: 2, W: 30, H: 10}
			return consumerScrollbarHarness{
				widget: search,
				render: func() [][]term.Cell { return renderAt(search, rect) },
				shrink: func() [][]term.Cell {
					search.Groups = nil
					search.FlatList = nil
					return renderAt(search, rect)
				},
			}
		},
		"autocomplete": func() consumerScrollbarHarness {
			items := make([]CompletionItem, 20)
			for i := range items {
				items[i] = CompletionItem{Label: fmt.Sprintf("item-%02d", i)}
			}
			autocomplete := NewAutocompleteWidget(items, 2, 2)
			autocomplete.firstEvent = false
			rect := Rect{X: 7, Y: 3, W: 50, H: 20}
			return consumerScrollbarHarness{
				widget: autocomplete,
				render: func() [][]term.Cell { return renderAt(autocomplete, rect) },
				shrink: func() [][]term.Cell {
					autocomplete.SetItems(nil)
					return renderAt(autocomplete, rect)
				},
			}
		},
		"select dialog": func() consumerScrollbarHarness {
			commands := make([]command.Command, 20)
			for i := range commands {
				commands[i] = command.Command{ID: fmt.Sprintf("command.%02d", i), Title: fmt.Sprintf("Command %02d", i)}
			}
			dialog := NewSelectDialogWidget(commands)
			rect := Rect{X: 8, Y: 4, W: 80, H: 24}
			return consumerScrollbarHarness{
				widget: dialog,
				render: func() [][]term.Cell { return renderAt(dialog, rect) },
				shrink: func() [][]term.Cell {
					dialog.Items = dialog.Items[:1]
					dialog.Selected = 0
					return renderAt(dialog, rect)
				},
			}
		},
		"keybindings": func() consumerScrollbarHarness {
			commands := make([]command.Command, 20)
			for i := range commands {
				commands[i] = command.Command{ID: fmt.Sprintf("command.%02d", i), Title: fmt.Sprintf("Command %02d", i)}
			}
			keybindings := NewKeybindingsWidget(commands)
			rect := Rect{X: 9, Y: 5, W: 80, H: 24}
			return consumerScrollbarHarness{
				widget: keybindings,
				render: func() [][]term.Cell { return renderAt(keybindings, rect) },
				shrink: func() [][]term.Cell {
					keybindings.items = keybindings.items[:1]
					keybindings.selected = 0
					return renderAt(keybindings, rect)
				},
			}
		},
	}
}

func TestScrollbarConsumersPropagateCaptureLifecycle(t *testing.T) {
	for name, newHarness := range scrollbarConsumerHarnesses() {
		t.Run(name, func(t *testing.T) {
			h := newHarness()
			x, y := findScrollbarThumb(t, h.render())
			if got := h.widget.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0)); got != EventCaptured {
				t.Fatalf("thumb press = %v, want captured", got)
			}
			owner, ok := h.widget.(widgets.PointerCaptureOwner)
			if !ok || !owner.OwnsPointerCapture() {
				t.Fatalf("consumer does not report owned capture: owner=%v owns=%v", ok, ok && owner.OwnsPointerCapture())
			}
			if !widgets.CancelPointerCapture(h.widget) || owner.OwnsPointerCapture() {
				t.Fatal("consumer did not cancel its scrollbar capture")
			}
		})
	}
}

func TestScrollbarConsumersConsumeOwnedRelease(t *testing.T) {
	for name, newHarness := range scrollbarConsumerHarnesses() {
		t.Run(name, func(t *testing.T) {
			h := newHarness()
			x, y := findScrollbarThumb(t, h.render())
			if got := h.widget.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0)); got != EventCaptured {
				t.Fatalf("thumb press = %v, want captured", got)
			}
			if got := h.widget.HandleEvent(tcell.NewEventMouse(x+200, y+200, tcell.ButtonNone, 0)); got != EventConsumed {
				t.Fatalf("owned off-widget release = %v, want consumed", got)
			}
		})
	}
}

func TestScrollbarConsumersNotifyWhenRenderInvalidatesCapture(t *testing.T) {
	for name, newHarness := range scrollbarConsumerHarnesses() {
		t.Run(name, func(t *testing.T) {
			h := newHarness()
			invalidations := 0
			widgets.SetPointerCaptureInvalidated(h.widget, func() { invalidations++ })
			x, y := findScrollbarThumb(t, h.render())
			if got := h.widget.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0)); got != EventCaptured {
				t.Fatalf("thumb press = %v, want captured", got)
			}
			h.shrink()
			if invalidations != 1 {
				t.Fatalf("render invalidations = %d, want 1", invalidations)
			}
			if owner, ok := h.widget.(widgets.PointerCaptureOwner); !ok || owner.OwnsPointerCapture() {
				t.Fatalf("capture survived invalidating render: owner=%v owns=%v", ok, ok && owner.OwnsPointerCapture())
			}
		})
	}
}

func TestEditorHorizontalScrollbarUsesRenderedAbsoluteTrack(t *testing.T) {
	editor := NewEditorPaneWidget(
		&buffer.Buffer{Lines: []string{strings.Repeat("x", 50)}},
		&cursor.Cursor{},
		&view.Viewport{Width: 20, Height: 6},
	)
	rect := Rect{X: 5, Y: 3, W: 20, H: 6}
	cells := renderAt(editor, rect)
	trackX := rect.X + rect.W - 1
	trackY := rect.Y + rect.H - 1
	if cell := cells[trackY][trackX]; cell.Ch != '▄' || cell.Style != term.StyleScrollbar {
		t.Fatalf("horizontal track endpoint = %+v, want rendered scrollbar track", cell)
	}

	if got := editor.HandleEvent(tcell.NewEventMouse(trackX, trackY, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("horizontal endpoint press = %v, want captured", got)
	}
	if editor.Viewport.LeftCol != 31 {
		t.Fatalf("horizontal endpoint offset = %d, want 31", editor.Viewport.LeftCol)
	}
	if got := editor.HandleEvent(tcell.NewEventMouse(trackX+100, trackY+100, tcell.ButtonNone, 0)); got != EventConsumed {
		t.Fatalf("horizontal off-widget release = %v, want consumed", got)
	}
}

func TestEditorGroupPropagatesScrollbarCapture(t *testing.T) {
	for _, name := range []string{"editor", "autocomplete"} {
		t.Run(name, func(t *testing.T) {
			group := NewEditorGroupWidget(nil, 4, false, "minimal")
			if name == "editor" {
				group.Editor.Buf.Lines = make([]string, 20)
				for i := range group.Editor.Buf.Lines {
					group.Editor.Buf.Lines[i] = "line"
				}
			} else {
				items := make([]CompletionItem, 20)
				for i := range items {
					items[i] = CompletionItem{Label: fmt.Sprintf("item-%02d", i)}
				}
				group.Autocomplete = NewAutocompleteWidget(items, 1, 1)
				group.Autocomplete.firstEvent = false
			}

			rect := Rect{X: 4, Y: 2, W: 30, H: 18}
			cells := renderAt(group, rect)
			x, y := findScrollbarThumb(t, cells)
			if got := group.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0)); got != EventCaptured {
				t.Fatalf("group scrollbar press = %v, want captured", got)
			}
			if !group.OwnsPointerCapture() {
				t.Fatal("group did not report child capture")
			}
			if !group.CancelPointerCapture() || group.OwnsPointerCapture() {
				t.Fatal("group did not cancel child capture")
			}
		})
	}
}
