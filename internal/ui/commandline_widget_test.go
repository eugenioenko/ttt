package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

func newCommandLineSurface(w, h int) ([][]term.Cell, Surface) {
	cells := make([][]term.Cell, h)
	for y := range cells {
		cells[y] = make([]term.Cell, w)
		for x := range cells[y] {
			cells[y][x] = term.Cell{Ch: ' '}
		}
	}
	return cells, NewRenderSurface(cells, Rect{X: 0, Y: 0, W: w, H: h})
}

func cellRow(cells [][]term.Cell, y int) string {
	var b strings.Builder
	for _, c := range cells[y] {
		ch := c.Ch
		if ch == 0 {
			ch = ' '
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func typeInto(c *CommandLineWidget, s string) {
	for _, r := range s {
		c.HandleEvent(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
}

func TestCommandLineRendersAboveStatusBar(t *testing.T) {
	c := NewCommandLineWidget(":")
	typeInto(c, "%s/old/new/g")

	cells, surface := newCommandLineSurface(40, 10)
	c.SetRect(Rect{X: 0, Y: 0, W: 40, H: 10})
	c.Render(surface)

	// 3 rows ending one row above the status bar row (h-1).
	top, text, bottom := cellRow(cells, 6), cellRow(cells, 7), cellRow(cells, 8)
	if !strings.HasSuffix(strings.TrimRight(top, " "), "╮") {
		t.Errorf("expected top border on row 6, got %q", top)
	}
	if !strings.Contains(text, ":%s/old/new/g") {
		t.Errorf("expected prefix+text on row 7, got %q", text)
	}
	if !strings.HasSuffix(strings.TrimRight(bottom, " "), "╯") {
		t.Errorf("expected bottom border on row 8, got %q", bottom)
	}
	// The status bar row must be left untouched.
	if strings.TrimSpace(cellRow(cells, 9)) != "" {
		t.Errorf("expected row 9 (status bar) untouched, got %q", cellRow(cells, 9))
	}
}

func TestCommandLineRendersAtVariousWidths(t *testing.T) {
	for _, w := range []int{12, 40, 200} {
		c := NewCommandLineWidget("/")
		typeInto(c, "abc")

		cells, surface := newCommandLineSurface(w, 10)
		c.SetRect(Rect{X: 0, Y: 0, W: w, H: 10})
		c.Render(surface)

		row := cellRow(cells, 7)
		if !strings.Contains(row, "/abc") {
			t.Errorf("width %d: expected %q to contain \"/abc\"", w, row)
		}
		if len([]rune(row)) != w {
			t.Errorf("width %d: row width %d", w, len([]rune(row)))
		}
	}
}

func TestCommandLineTinyTerminalDoesNotPanic(t *testing.T) {
	for _, size := range [][2]int{{3, 10}, {40, 3}, {1, 1}, {0, 0}} {
		c := NewCommandLineWidget(":")
		typeInto(c, "x")
		_, surface := newCommandLineSurface(size[0], size[1])
		c.SetRect(Rect{X: 0, Y: 0, W: size[0], H: size[1]})
		c.Render(surface)
		if _, _, visible := c.CursorPosition(); visible {
			t.Errorf("size %v: cursor should be hidden when there is no room", size)
		}
	}
}

func TestCommandLineTypingAndBackspace(t *testing.T) {
	c := NewCommandLineWidget(":")
	typeInto(c, "wqa")
	if c.Text() != "wqa" {
		t.Fatalf("expected %q, got %q", "wqa", c.Text())
	}
	c.HandleEvent(tcell.NewEventKey(tcell.KeyBackspace2, "", tcell.ModNone))
	if c.Text() != "wq" {
		t.Fatalf("expected %q after backspace, got %q", "wq", c.Text())
	}
}

func TestCommandLineOnChangeFiresPerKeystroke(t *testing.T) {
	c := NewCommandLineWidget("/")
	var seen []string
	c.OnChange = func(text string) { seen = append(seen, text) }

	typeInto(c, "abc")
	c.HandleEvent(tcell.NewEventKey(tcell.KeyBackspace2, "", tcell.ModNone))

	want := []string{"a", "ab", "abc", "ab"}
	if len(seen) != len(want) {
		t.Fatalf("expected %v, got %v", want, seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, seen)
		}
	}
}

func TestCommandLineEnterSubmits(t *testing.T) {
	c := NewCommandLineWidget(":")
	submitted := ""
	calls := 0
	c.OnSubmit = func(text string) { submitted = text; calls++ }
	c.OnCancel = func() { t.Fatal("cancel must not fire on Enter") }

	typeInto(c, "s/a/b/")
	c.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	if calls != 1 || submitted != "s/a/b/" {
		t.Fatalf("expected 1 submit of %q, got %d of %q", "s/a/b/", calls, submitted)
	}
}

func TestCommandLineEscapeCancels(t *testing.T) {
	c := NewCommandLineWidget(":")
	cancelled := 0
	c.OnCancel = func() { cancelled++ }
	c.OnSubmit = func(string) { t.Fatal("submit must not fire on Escape") }

	typeInto(c, "q")
	if res := c.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone)); res != EventConsumed {
		t.Fatalf("expected Escape to be consumed, got %v", res)
	}
	if cancelled != 1 {
		t.Fatalf("expected 1 cancel, got %d", cancelled)
	}
}

func TestCommandLineSetTextNotifies(t *testing.T) {
	c := NewCommandLineWidget(":")
	var last string
	c.OnChange = func(text string) { last = text }
	c.SetText("noh")
	if c.Text() != "noh" || last != "noh" {
		t.Fatalf("expected text and OnChange %q, got %q / %q", "noh", c.Text(), last)
	}
}

func TestCommandLineCursorFollowsText(t *testing.T) {
	c := NewCommandLineWidget(":")
	typeInto(c, "abc")

	_, surface := newCommandLineSurface(40, 10)
	c.SetRect(Rect{X: 0, Y: 0, W: 40, H: 10})
	c.Render(surface)

	x, y, visible := c.CursorPosition()
	if !visible {
		t.Fatal("expected cursor to be visible")
	}
	// box x=0, inner starts at 2, one prefix rune, three typed runes.
	if x != 2+1+3 || y != 7 {
		t.Fatalf("expected cursor at (6,7), got (%d,%d)", x, y)
	}
}

func TestCommandLineDefaultPrefix(t *testing.T) {
	c := NewCommandLineWidget("")
	if c.Prefix != ":" {
		t.Fatalf("expected default prefix %q, got %q", ":", c.Prefix)
	}
}

// OnKey must see Enter and Escape too. An incremental search binds RET to
// "exit at the match" and C-g to "abort to origin", so a hook that only got
// ordinary keys could not implement either.
func TestCommandLineOnKeyPreemptsSubmitAndCancel(t *testing.T) {
	w := NewCommandLineWidget("/")

	submitted, cancelled := 0, 0
	w.OnSubmit = func(string) { submitted++ }
	w.OnCancel = func() { cancelled++ }

	var seen []tcell.Key
	w.OnKey = func(ev *tcell.EventKey) bool {
		seen = append(seen, ev.Key())
		return true
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone))

	if len(seen) != 2 {
		t.Fatalf("expected OnKey to see both keys, got %d", len(seen))
	}
	if submitted != 0 || cancelled != 0 {
		t.Fatalf("expected OnKey to preempt submit/cancel, got %d/%d", submitted, cancelled)
	}
}

func TestCommandLineOnKeyDeclineFallsThrough(t *testing.T) {
	w := NewCommandLineWidget("/")

	submitted := 0
	w.OnSubmit = func(string) { submitted++ }
	w.OnKey = func(*tcell.EventKey) bool { return false }

	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModNone))
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	if submitted != 1 {
		t.Fatalf("expected declined keys to fall through to normal handling, got %d submits", submitted)
	}
	if got := w.Text(); got != "a" {
		t.Fatalf("expected the declined rune to reach the input, got %q", got)
	}
}

func TestCommandLineWithoutOnKeyIsUnchanged(t *testing.T) {
	w := NewCommandLineWidget("/")

	submitted := 0
	w.OnSubmit = func(string) { submitted++ }

	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	w.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	if submitted != 1 || w.Text() != "x" {
		t.Fatalf("expected unchanged behaviour with no hook, got %d submits and %q", submitted, w.Text())
	}
}
