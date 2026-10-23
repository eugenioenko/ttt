package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

func TestTitleHeight(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{Title: "Test"})
	if h := tw.Height(); h != 1 {
		t.Fatalf("expected Height()=1, got %d", h)
	}
}

func TestTitleWidthReturnsZero(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{Title: "Test"})
	if w := tw.Width(); w != 0 {
		t.Fatalf("expected Width()=0 (grow), got %d", w)
	}
}

func TestTitleRender(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{
		Title: "Hello",
		Style: term.StyleScrollbar, // use a recognizable style
	})
	s := renderWidget(tw, 0, 0, 20, 1)

	// Verify title text appears
	for i, ch := range []rune("Hello") {
		if s.cells[0][i].Ch != ch {
			t.Errorf("cell[0][%d]: expected %c, got %c", i, ch, s.cells[0][i].Ch)
		}
		if s.cells[0][i].Style != term.StyleScrollbar {
			t.Errorf("cell[0][%d]: expected style %d, got %d", i, term.StyleScrollbar, s.cells[0][i].Style)
		}
		if !s.cells[0][i].Bold {
			t.Errorf("cell[0][%d]: expected bold", i)
		}
	}
}

func TestTitleRenderTruncatesLongTitle(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{Title: "Very Long Title That Exceeds Width"})
	s := renderWidget(tw, 0, 0, 5, 1)

	// Only first 5 chars should render
	for i, ch := range []rune("Very ") {
		if s.cells[0][i].Ch != ch {
			t.Errorf("cell[0][%d]: expected %c, got %c", i, ch, s.cells[0][i].Ch)
		}
	}
}

func TestTitleWithDropdown(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{
		Title: "Section",
		Menu: []MenuEntry{
			{Label: "Option 1"},
			{Label: "Option 2"},
		},
	})

	if tw.dropdown == nil {
		t.Fatal("expected dropdown to be created when Menu entries are provided")
	}

	// Render and verify dropdown indicator appears
	s := renderWidget(tw, 0, 0, 20, 1)

	// The dropdown uses the default icon "..." which renders as a button.
	// The dropdown widget takes some width from the right side.
	ddW := tw.dropdown.Width()
	if ddW <= 0 {
		t.Fatal("dropdown should have positive width")
	}

	// Title text should still render on the left
	if s.cells[0][0].Ch != 'S' {
		t.Errorf("expected first char 'S', got %c", s.cells[0][0].Ch)
	}
}

func TestTitleWithoutDropdown(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{Title: "Plain"})

	if tw.dropdown != nil {
		t.Fatal("expected no dropdown when Menu is empty")
	}
}

func TestTitleHandleEventIgnoredWithoutDropdown(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{Title: "Test"})
	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	if r := tw.HandleEvent(ev); r != EventIgnored {
		t.Fatalf("expected EventIgnored without dropdown, got %d", r)
	}
}

func TestTitleDropdownClickTriggersMenu(t *testing.T) {
	menuCalled := false
	tw := NewTitleWidget(TitleConfig{
		Title: "Section",
		Menu: []MenuEntry{
			{Label: "Option 1"},
		},
		OnMenu: func(entries []MenuEntry, sx, sy int) {
			menuCalled = true
		},
	})

	// Render to set rects
	renderWidget(tw, 0, 0, 20, 1)

	// Click on the dropdown area (right side)
	ddRect := tw.dropdown.GetRect()
	click := tcell.NewEventMouse(ddRect.X, ddRect.Y, tcell.Button1, tcell.ModNone)
	result := tw.HandleEvent(click)

	if result != EventConsumed {
		t.Fatalf("expected EventConsumed for dropdown click, got %d", result)
	}
	if !menuCalled {
		t.Fatal("expected OnMenu to be called")
	}
}

func TestTitleDropdownClickOutsideIgnored(t *testing.T) {
	menuCalled := false
	tw := NewTitleWidget(TitleConfig{
		Title: "Section",
		Menu: []MenuEntry{
			{Label: "Option 1"},
		},
		OnMenu: func(entries []MenuEntry, sx, sy int) {
			menuCalled = true
		},
	})

	renderWidget(tw, 0, 0, 20, 1)

	// Click on the title text area (not the dropdown)
	click := tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone)
	result := tw.HandleEvent(click)

	if result == EventConsumed {
		t.Fatal("click on title text should not be consumed")
	}
	if menuCalled {
		t.Fatal("OnMenu should not be called for click outside dropdown")
	}
}

func TestTitleCustomIcon(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{
		Title: "Section",
		Icon:  "+",
		Menu: []MenuEntry{
			{Label: "Add"},
		},
	})

	if tw.dropdown == nil {
		t.Fatal("expected dropdown with custom icon")
	}
	// The button label should use the custom icon
	if tw.dropdown.button.label != "+" {
		t.Fatalf("expected dropdown label '+', got '%s'", tw.dropdown.button.label)
	}
}

func TestTitleHeightWithBoxModel(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{Title: "Test"})
	tw.SetBoxModel(BoxModel{PaddingTop: 1, PaddingBottom: 1})
	// Height = 1 (title) + padding overhead
	if h := tw.Height(); h != 3 {
		t.Fatalf("expected Height()=3 with padding, got %d", h)
	}
}

func TestTitleRenderFullwidth(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{Title: "タイトル", Badge: "バッジ"})
	s := renderWidget(tw, 0, 0, 20, 1)

	if row := rowText(s, 0); row != "タイトル      バッジ" {
		t.Errorf("row = %q, want %q", row, "タイトル      バッジ")
	}
	for x := 0; x < 8; x += 2 {
		if !s.cells[0][x].Bold {
			t.Errorf("cell[0][%d]: expected bold", x)
		}
	}
}

func TestTitleRenderFullwidthStopsBeforeLastColumn(t *testing.T) {
	tw := NewTitleWidget(TitleConfig{Title: "タイトル"})
	s := renderWidget(tw, 0, 0, 7, 1)

	if row := rowText(s, 0); row != "タイト" {
		t.Errorf("row = %q, want %q", row, "タイト")
	}
	if ch := s.cells[0][6].Ch; textwidth.Rune(ch) > 1 {
		t.Errorf("last column = %q, want no fullwidth rune starting there", ch)
	}
}
