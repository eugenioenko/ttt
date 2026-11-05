package e2e

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func inputDrag(h *testHarness, x1, y1, x2, y2 int) {
	h.t.Helper()
	for _, event := range []*tcell.EventMouse{
		tcell.NewEventMouse(x1, y1, tcell.Button1, 0),
		tcell.NewEventMouse(x2, y2, tcell.Button1, 0),
		tcell.NewEventMouse(x2, y2, tcell.ButtonNone, 0),
	} {
		h.app.Root.HandleEvent(event)
		h.redraw()
	}
}

func TestSearchBarInputDrag(t *testing.T) {
	for _, command := range []string{"search.find", "search.replace"} {
		t.Run(command, func(t *testing.T) {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			h.exec("file.new")
			pasteText(h, pasteKeys("abcdef"))
			h.exec(command)
			var input *ui.InputWidget
			switch bar := h.app.Root.TopOverlayWidget().(type) {
			case *ui.FindBarWidget:
				input = bar.Input
			case *ui.ReplaceBarWidget:
				input = bar.SearchInput
			}
			input.SetText("abcdef")
			h.redraw()
			inputDrag(h, 83, 5, 86, 5)
			if !input.HasSelection() || h.app.Root.PointerCaptureActive() {
				t.Fatal("drag did not leave a selection and release capture")
			}
			h.pressRune('X')
			if input.Text != "Xdef" {
				t.Fatalf("query=%q", input.Text)
			}
			if got := editorText(h); got != "abcdef" {
				t.Fatalf("drag edited buffer: %q", got)
			}
			inputDrag(h, 84, 5, 0, 20)
			if h.app.Root.PointerCaptureActive() {
				t.Fatal("release outside bar kept pointer captured")
			}
			h.app.Root.HandleEvent(tcell.NewEventMouse(84, 5, tcell.Button1, 0))
			h.app.Root.SetSize(121, 30)
			if h.app.Root.PointerCaptureActive() {
				t.Fatal("resize kept input drag captured")
			}
		})
	}
}

func TestDialogInputDragOutside(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.exec("file.new")
	h.app.ShowInputDialogEx("Input", "", "abcdef", "OK", func(string) {})
	h.redraw()
	adapter := h.app.Root.TopOverlayWidget().(*ui.WidgetAdapter)
	input := adapter.FocusedWidget()
	cursor := input.(interface{ CursorPosition() (int, int, bool) })
	x, y, _ := cursor.CursorPosition()
	inputDrag(h, x-6, y, x-3, y)
	h.pressRune('X')
	if got := input.(interface{ Text() string }).Text(); got != "Xdef" {
		t.Fatalf("dialog value=%q", got)
	}
	inputDrag(h, x-6, y, 0, 0)
	if h.app.Root.PointerCaptureActive() {
		t.Fatal("dialog release outside kept capture")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(x-3, y, tcell.Button1, 0))
	h.app.DismissDialog()
	if h.app.Root.PointerCaptureActive() {
		t.Fatal("closing dialog kept capture")
	}
}

func TestWorkspaceSearchInputDragStaysInInput(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	h.exec("sidebar.search")
	input := h.app.Search.Input
	input.OnChange = nil
	input.SetText("abcdef")
	h.redraw()
	x, y := -1, -1
	for row, line := range strings.Split(h.screenText(), "\n") {
		if col := displayColumnOf(line, "abcdef"); col >= 0 {
			x, y = col, row
			break
		}
	}
	if x < 0 {
		t.Fatal("search input not visible")
	}
	width := h.app.Sidebar.GetRect().W
	h.app.FocusEditor()
	inputDrag(h, x, y, x+3, y)
	h.pressRune('X')
	if input.Text != "Xdef" {
		t.Fatalf("search input=%q", input.Text)
	}
	inputDrag(h, x+1, y, 100, y)
	h.pressRune('Y')
	if input.Text != "XY" || editorText(h) != "" || h.app.EditorGroup.Editor.Selection.Active {
		t.Fatal("input drag leaked into editor")
	}
	if h.app.Sidebar.GetRect().W != width {
		t.Fatal("input drag resized sidebar")
	}
}
