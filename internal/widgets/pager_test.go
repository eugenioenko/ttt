package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

func pagerColumn(s *testSurface, r rune) int {
	row := rowText(s, 0)
	return textwidth.String(row[:strings.IndexRune(row, r)])
}

func TestPagerShowsRangeAndDisablesEdges(t *testing.T) {
	p := NewPagerWidget(PagerConfig{Offset: 0, Limit: 20, Total: 85})
	s := renderWidget(p, 0, 0, 40, 1)

	if got := rowText(s, 0); got != " «   ‹   1–20 of 85   ›   »" {
		t.Fatalf("pager text = %q", got)
	}
	if s.cells[0][pagerColumn(s, '«')].Style != term.StyleMuted || s.cells[0][pagerColumn(s, '‹')].Style != term.StyleMuted {
		t.Error("first and previous should be muted on the first page")
	}
	if s.cells[0][pagerColumn(s, '›')].Style != term.StyleButton || s.cells[0][pagerColumn(s, '»')].Style != term.StyleButton {
		t.Error("next and last should be enabled")
	}
	if got := p.Width(); got != textwidth.String(rowText(s, 0))+1 {
		t.Errorf("width = %d, want the drawn row plus its trailing space", got)
	}
}

func TestPagerClickAndKeysChangeOffset(t *testing.T) {
	var got []int
	p := NewPagerWidget(PagerConfig{Offset: 20, Limit: 20, Total: 85, OnChange: func(o int) { got = append(got, o) }})
	s := renderWidget(p, 3, 5, 40, 1)

	last := 3 + pagerColumn(s, '»')
	p.HandleEvent(mouseClick(last, 5))
	p.HandleEvent(mouseRelease(last, 5))
	if len(got) != 1 || got[0] != 80 {
		t.Fatalf("clicking last should jump to the last page offset 80, got %v", got)
	}

	renderWidget(p, 3, 5, 40, 1)
	p.SetFocused(true)
	p.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if len(got) != 1 {
		t.Fatalf("next on the last page should do nothing, got %v", got)
	}
	p.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyHome, "", tcell.ModNone))
	p.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModNone))
	if len(got) != 4 || got[1] != 60 || got[2] != 0 || got[3] != 80 {
		t.Fatalf("left, home, end should go to 60, 0, 80, got %v", got)
	}
}

func TestPagerIgnoresKeysWithoutFocus(t *testing.T) {
	calls := 0
	p := NewPagerWidget(PagerConfig{Limit: 10, Total: 100, OnChange: func(int) { calls++ }})
	renderWidget(p, 0, 0, 40, 1)
	if r := p.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone)); r != EventIgnored || calls != 0 {
		t.Fatalf("unfocused pager handled a key: %v, %d calls", r, calls)
	}
}

func TestPagerHeldButtonDoesNotRepeat(t *testing.T) {
	calls := 0
	p := NewPagerWidget(PagerConfig{Offset: 0, Limit: 10, Total: 100, OnChange: func(int) { calls++ }})
	s := renderWidget(p, 0, 0, 40, 1)
	x := pagerColumn(s, '›')

	p.HandleEvent(mouseClick(x, 0))
	p.HandleEvent(mouseClick(x, 0))
	if calls != 1 {
		t.Fatalf("a held button should page once, got %d", calls)
	}
}

func TestPagerEmpty(t *testing.T) {
	s := renderWidget(NewPagerWidget(PagerConfig{Limit: 20}), 0, 0, 40, 1)
	if got := rowText(s, 0); !strings.Contains(got, "No items") {
		t.Fatalf("empty pager = %q", got)
	}
}

func TestPagerClampsAnOffsetPastTheTotal(t *testing.T) {
	var got []int
	p := NewPagerWidget(PagerConfig{Offset: 80, Limit: 20, Total: 50, OnChange: func(o int) { got = append(got, o) }})
	s := renderWidget(p, 0, 0, 40, 1)
	if row := rowText(s, 0); !strings.Contains(row, "41–50 of 50") {
		t.Fatalf("pager text = %q, want the last valid page", row)
	}
	p.SetFocused(true)
	p.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	if len(got) != 1 || got[0] != 20 {
		t.Fatalf("previous from the clamped last page = %v, want 20", got)
	}
}

func TestPagerIgnoresClicksOnItsPadding(t *testing.T) {
	calls := 0
	p := NewPagerWidget(PagerConfig{Limit: 10, Total: 100, OnChange: func(int) { calls++ }})
	p.Box.PaddingTop = 1
	s := renderWidget(p, 0, 0, 40, 2)
	x := textwidth.String(rowText(s, 1)[:strings.IndexRune(rowText(s, 1), '›')])
	p.HandleEvent(mouseClick(x, 0))
	p.HandleEvent(mouseRelease(x, 0))
	if calls != 0 {
		t.Fatal("a click on the padding row above the arrow paged")
	}
	p.HandleEvent(mouseClick(x, 1))
	if calls != 1 {
		t.Fatal("a click on the arrow did not page")
	}
}
