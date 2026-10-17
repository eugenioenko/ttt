package widgets

import (
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

func surfaceRows(s *testSurface) []string {
	rows := make([]string, len(s.cells))
	for y, line := range s.cells {
		var sb strings.Builder
		for _, c := range line {
			if c.Ch == 0 {
				sb.WriteRune(' ')
			} else {
				sb.WriteRune(c.Ch)
			}
		}
		rows[y] = strings.TrimRight(sb.String(), " ")
	}
	return rows
}

func TestWrapInputRowsBreaksAtSpacesAndNewlines(t *testing.T) {
	for _, tc := range []struct {
		text  string
		width int
		want  []string
	}{
		{"one two three", 8, []string{"one two ", "three"}},
		{"abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"a\nb", 10, []string{"a", "b"}},
		{"a\n", 10, []string{"a", ""}},
		{"世界世界世", 4, []string{"世界", "世界", "世"}},
	} {
		runes := []rune(tc.text)
		var got []string
		for _, r := range wrapInputRows(runes, tc.width) {
			got = append(got, string(runes[r.start:r.end]))
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("wrap(%q, %d) = %q, want %q", tc.text, tc.width, got, tc.want)
		}
	}
}

func typeText(inp *InputWidget, text string) {
	for _, r := range text {
		inp.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
}

func TestWrappingInputGrowsAndScrolls(t *testing.T) {
	inp := NewInputWidget(InputConfig{Prefix: "> ", Wrap: true, MaxRows: 2})
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 12, inp.Height())
	if inp.Height() != 1 {
		t.Fatalf("empty height = %d, want 1", inp.Height())
	}
	typeText(inp, "one two three four five")
	if inp.Height() != 2 {
		t.Fatalf("height = %d, want MaxRows 2", inp.Height())
	}
	s := renderWidget(inp, 0, 0, 12, inp.Height())
	if got, want := surfaceRows(s), []string{"> three    ↑", "  four five"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q, want the last two rows, where the cursor is", got)
	}
	if x, y, _ := inp.CursorPosition(); x != 11 || y != 1 {
		t.Fatalf("cursor at %d,%d, want 11,1", x, y)
	}
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	s = renderWidget(inp, 0, 0, 12, inp.Height())
	if got, want := surfaceRows(s), []string{"> one two", "  three    ↓"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after Up rows = %q, want %q", got, want)
	}
	if x, y, _ := inp.CursorPosition(); x != 9 || y != 0 {
		t.Fatalf("cursor at %d,%d after Up, want 9,0: on the row's last rune, not the next row", x, y)
	}
}

func TestWrappingInputEnterSendsShiftEnterBreaksTheLine(t *testing.T) {
	var sent string
	inp := NewInputWidget(InputConfig{Wrap: true, OnSubmit: func(s string) { sent = s }})
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 20, 1)
	typeText(inp, "a")
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModShift))
	typeText(inp, "b")
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModAlt))
	typeText(inp, "c")
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "m", tcell.ModAlt|tcell.ModCtrl))
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlJ, "", tcell.ModCtrl))
	if inp.Text() != "a\nb\nc\n\n" || sent != "" {
		t.Fatalf("text %q sent %q, want line breaks and nothing sent", inp.Text(), sent)
	}
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if sent != "a\nb\nc\n\n" {
		t.Fatalf("sent %q", sent)
	}
	inp.PasteText("x\ny\n")
	if inp.Text() != "a\nb\nc\n\nx\ny" {
		t.Fatalf("after paste %q, want the pasted line break kept", inp.Text())
	}
}

func TestSingleRowInputIgnoresShiftEnterAndFlattensPastes(t *testing.T) {
	inp := NewInputWidget(InputConfig{})
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 20, 1)
	typeText(inp, "a")
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModShift))
	inp.PasteText("x\ny")
	if inp.Text() != "ax y" || inp.Height() != 1 {
		t.Fatalf("text %q height %d", inp.Text(), inp.Height())
	}
}

func TestWrappingInputClickPlacesTheCursor(t *testing.T) {
	inp := NewInputWidget(InputConfig{Prefix: "> ", Wrap: true})
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 12, 1)
	typeText(inp, "one two three")
	renderWidget(inp, 0, 0, 12, inp.Height())
	inp.HandleEvent(mouseClick(4, 1))
	if inp.cursorPos != 10 {
		t.Fatalf("cursor at rune %d, want 10 (the second r of three)", inp.cursorPos)
	}
}

func TestWrappingInputCursorReturnsToThePlaceholderRow(t *testing.T) {
	inp := NewInputWidget(InputConfig{Prefix: "> ", Placeholder: "Ask", Wrap: true, MaxRows: 2})
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 12, 1)
	typeText(inp, "one two three four five six seven")
	renderWidget(inp, 0, 0, 12, inp.Height())
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlA, "", tcell.ModCtrl))
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyBackspace2, "", tcell.ModNone))
	renderWidget(inp, 0, 0, 12, inp.Height())
	if _, y, _ := inp.CursorPosition(); y != 0 {
		t.Fatalf("cursor row %d after clearing, want 0", y)
	}
}

func TestWrappingInputHeightFollowsTheLayoutWidth(t *testing.T) {
	inp := NewInputWidget(InputConfig{Prefix: "> ", Wrap: true})
	inp.SetText("hello")
	if inp.Height() != 1 {
		t.Fatalf("height before any render = %d, want 1", inp.Height())
	}
	inp.SetText("one two three four")
	if h := inp.HeightAtWidth(40); h != 1 {
		t.Fatalf("height at 40 columns = %d, want 1", h)
	}
	if h := inp.HeightAtWidth(14); h != 2 {
		t.Fatalf("height at 14 columns = %d, want 2", h)
	}
	stack := NewVStackWidget(inp)
	renderWidget(stack, 0, 0, 40, 5)
	renderWidget(stack, 0, 0, 14, 5)
	if r := inp.GetRect(); r.H != 2 {
		t.Fatalf("laid out %d rows high in the narrower stack's first render, want 2", r.H)
	}
}

func TestWrapInputRowsNeverExceedTheWidth(t *testing.T) {
	runes := []rune(" abc世")
	for _, r := range wrapInputRows(runes, 4) {
		if w := textwidth.Runes(runes[r.start:r.end]); w > 4 {
			t.Fatalf("row %q is %d columns, over 4", string(runes[r.start:r.end]), w)
		}
	}
}

func TestPasteResetsTheGoalColumn(t *testing.T) {
	inp := NewInputWidget(InputConfig{Prefix: "> ", Wrap: true})
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 20, 1)
	inp.PasteText("abcdefgh\nab\nabcdefgh")
	renderWidget(inp, 0, 0, 20, inp.Height())
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	inp.PasteText("X")
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if inp.cursorPos != 16 {
		t.Fatalf("cursor at %d after Down, want 16: column 3, where the paste left it", inp.cursorPos)
	}
}

func TestWrappingInputMarksRowsOutOfView(t *testing.T) {
	inp := NewInputWidget(InputConfig{Prefix: "> ", Wrap: true, MaxRows: 2})
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 12, 1)
	typeText(inp, "one two three four five")
	s := renderWidget(inp, 0, 0, 12, inp.Height())
	if got := surfaceRows(s); got[0] != "> three    ↑" || got[1] != "  four five" {
		t.Fatalf("scrolled to the end, rows = %q, want an up mark only", got)
	}
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	s = renderWidget(inp, 0, 0, 12, inp.Height())
	if got := surfaceRows(s); got[0] != "> one two" || got[1] != "  three    ↓" {
		t.Fatalf("scrolled to the top, rows = %q, want a down mark only", got)
	}
}

func TestWrapInputRowsHangASpaceThatDoesNotFit(t *testing.T) {
	runes := []rune("hello world")
	var got []string
	for _, r := range wrapInputRows(runes, 5) {
		got = append(got, string(runes[r.start:r.end]))
	}
	if want := []string{"hello ", "world"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q, want %q: no row of a lone space", got, want)
	}
}

func TestNestedWrappingInputGetsTheLayoutWidth(t *testing.T) {
	inp := NewInputWidget(InputConfig{Prefix: "> ", Wrap: true})
	inp.SetText("one two three four five six seven eight")
	label := NewLabelWidget(LabelConfig{Text: "below"})
	box := NewBoxWidget(BoxModel{})
	box.Child = inp
	for name, outer := range map[string]Widget{
		"vstack": NewVStackWidget(NewVStackWidget(inp), label),
		"box":    NewVStackWidget(box, label),
	} {
		renderWidget(outer, 0, 0, 60, 10)
		renderWidget(outer, 0, 0, 14, 10)
		if got, want := inp.GetRect().H, inp.HeightAtWidth(14); got != want {
			t.Errorf("%s: laid out %d rows at 14 columns, want %d", name, got, want)
		}
	}
}

func TestWrappingInputLeavesModifiedUpDownAlone(t *testing.T) {
	inp := NewInputWidget(InputConfig{Wrap: true})
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 20, 1)
	inp.PasteText("ab\ncd")
	for _, mod := range []tcell.ModMask{tcell.ModAlt, tcell.ModAlt | tcell.ModShift, tcell.ModCtrl} {
		if res := inp.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", mod)); res != EventIgnored {
			t.Errorf("Up with %v was consumed, want it left for global bindings", mod)
		}
	}
	if res := inp.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModShift)); res != EventConsumed || !inp.hasSelection() {
		t.Error("Shift+Up did not select up a row")
	}
}

func TestWrappingInputCtrlHomeEndReachTheWholeText(t *testing.T) {
	inp := NewInputWidget(InputConfig{Wrap: true})
	inp.SetFocused(true)
	renderWidget(inp, 0, 0, 20, 1)
	inp.PasteText("abc\ndef")
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyHome, "", tcell.ModCtrl))
	if inp.cursorPos != 0 {
		t.Fatalf("Ctrl+Home left the cursor at %d", inp.cursorPos)
	}
	inp.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModCtrl))
	if inp.cursorPos != 7 {
		t.Fatalf("Ctrl+End left the cursor at %d", inp.cursorPos)
	}
}

func TestScrollMarksStayOffTextAndCursor(t *testing.T) {
	tiny := NewInputWidget(InputConfig{Prefix: "> ", Wrap: true, MaxRows: 1})
	tiny.SetFocused(true)
	renderWidget(tiny, 0, 0, 3, 1)
	typeText(tiny, "abcdef")
	if got := surfaceRows(renderWidget(tiny, 0, 0, 3, 1)); strings.ContainsAny(got[0], "↑↓↕^v:") {
		t.Errorf("tiny input row %q has a scroll mark over its text", got[0])
	}

	one := NewInputWidget(InputConfig{Prefix: "> ", Wrap: true, MaxRows: 1})
	one.SetFocused(true)
	renderWidget(one, 0, 0, 12, 1)
	typeText(one, "one two three four five")
	one.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	if got := surfaceRows(renderWidget(one, 0, 0, 12, 1)); !strings.HasSuffix(got[0], "↕") {
		t.Errorf("one-row input scrolled to the middle shows %q, want both directions marked", got[0])
	}
}
