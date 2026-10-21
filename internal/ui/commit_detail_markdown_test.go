package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

func TestCommitDetailMarkdownStyles(t *testing.T) {
	d := NewCommitDetailWidget("", "", "", false)
	d.SetDetail("# Heading\n\n**bold** *italic* `code` [guide](https://example.com)\n\n- item\n\n```\n  literal **code**\n```", nil, "")
	grid := renderDetail(d, 100, 25)
	for _, tt := range []struct {
		text  string
		style term.Style
	}{{"# Heading", term.StyleHoverBold}, {"bold", term.StyleHoverBold}, {"italic", term.StyleHoverItalic}, {"code", term.StyleHoverCode}, {"guide", term.StylePaletteItem}, {"• item", term.StylePaletteItem}, {"  literal **code**", term.StyleHoverCode}} {
		x, y, ok := detailFind(grid, tt.text)
		if !ok || grid[y][x].Style != tt.style {
			t.Errorf("%q missing or has wrong style; screen:\n%s", tt.text, detailTestText(grid))
		}
	}
	if text := detailTestText(grid); strings.Contains(text, "```") || strings.Contains(text, "[guide]") {
		t.Fatalf("Markdown syntax leaked into preview:\n%s", text)
	}
}

func TestCommitDetailMarkdownWrapAndCopy(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		d := NewCommitDetailWidget("", "", "", false)
		d.SetWrapped(wrapped)
		text := "**A界B語C** alpha beta gamma delta epsilon"
		d.SetDetail(text, nil, "")
		rect := Rect{X: 7, Y: 3, W: 18, H: 20}
		d.SetRect(rect)
		grid := makeGrid(rect.X+rect.W, rect.Y+rect.H)
		d.Render(NewRenderSurface(grid, rect))
		visible := "A界B語C alpha beta gamma delta epsilon"
		var lines []int
		var joined strings.Builder
		for y := range d.visualRows {
			v := d.visualRows[y]
			if d.rows[v.row].kind == commitDetailMessageRow {
				lines = append(lines, y)
				for x := 1; x < d.layoutViewW-1; {
					ch := grid[rect.Y+y][rect.X+x].Ch
					joined.WriteRune(ch)
					x += textwidth.Rune(ch)
				}
			}
		}
		if len(lines) < 3 || !strings.Contains(strings.Join(strings.Fields(joined.String()), " "), visible) {
			t.Fatalf("wrap=%v: incomplete wrapped preview:\n%s", wrapped, detailTestText(grid))
		}
		for _, x := range []int{2, 3} {
			pos, ok := d.screenToSelection(rect.X+x, rect.Y+lines[0])
			if !ok || pos.Col != 1 {
				t.Fatalf("wide rune at x=%d maps to %+v, ok=%v", x, pos, ok)
			}
		}
		first, last := rect.Y+lines[0], rect.Y+lines[len(lines)-1]
		d.HandleEvent(tcell.NewEventMouse(rect.X+1, first, tcell.Button1, 0))
		d.HandleEvent(tcell.NewEventMouse(rect.X+rect.W-2, last, tcell.Button1, 0))
		d.HandleEvent(tcell.NewEventMouse(rect.X+rect.W-2, last, tcell.ButtonNone, 0))
		if got := d.CopySelection(); got != visible {
			t.Fatalf("wrap=%v: copied %q, want %q", wrapped, got, visible)
		}
	}
}

func TestCommitDetailMarkdownCodeScrollsWithoutMovingProse(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		d := NewCommitDetailWidget("", "", "", false)
		d.SetMode(mode)
		d.SetWrapped(true)
		d.SetDetail("Prose stays here.\n\n```\n  "+strings.Repeat("x", 80)+"TAIL\n```", twoHunkDetail().Files, "")
		grid := renderDetail(d, 30, 15)
		_, proseY, _ := detailFind(grid, "Prose stays here.")
		codeRows := 0
		for _, v := range d.visualRows {
			if strings.HasPrefix(d.rows[v.row].text, "  xxx") {
				codeRows++
			}
		}
		if codeRows != 1 || d.hscrollbar.TotalCols <= d.hscrollbar.Width {
			t.Fatal("long code must retain one row and horizontal scrolling with diff wrap enabled")
		}
		for range 100 {
			d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", 0))
		}
		grid = renderDetail(d, 30, 15)
		if _, _, ok := detailFind(grid, "TAIL"); !ok || grid[proseY][1].Ch != 'P' {
			t.Fatalf("code tail inaccessible or prose moved:\n%s", detailTestText(grid))
		}
		d.LeftCol = 0
		renderDetail(d, 30, 15)
		bar := d.hscrollbar
		d.HandleEvent(tcell.NewEventMouse(bar.X, bar.Y, tcell.Button1, 0))
		d.HandleEvent(tcell.NewEventMouse(bar.X+bar.Width-1, bar.Y, tcell.Button1, 0))
		d.HandleEvent(tcell.NewEventMouse(bar.X+bar.Width-1, bar.Y, tcell.ButtonNone, 0))
		if d.LeftCol != d.maxLeftCol || d.OwnsPointerCapture() {
			t.Fatal("horizontal scrollbar failed to reach the code's end or release capture")
		}
		d.SetWrapped(false)
		grid = renderDetail(d, 30, 15)
		if !strings.Contains(detailTestText(grid), "old one") || !strings.Contains(detailTestText(grid), "new one") {
			t.Fatal("disabling wrap lost the embedded diff")
		}
	}
}

func TestCommitDetailMarkdownWidthAndWordBoundaries(t *testing.T) {
	for _, tt := range []struct{ pane, limit, want int }{{120, 80, 80}, {42, 80, 40}, {82, 24, 24}, {102, 0, 100}, {2, 80, 0}, {3, 80, 1}} {
		t.Run(fmt.Sprint(tt), func(t *testing.T) {
			d := NewCommitDetailWidget("", "", "", false)
			d.SetMarkdownMaxWidth(tt.limit)
			d.SetDetail(strings.Repeat("x", 180), nil, "")
			grid := renderDetail(d, tt.pane, 240)
			if got := strings.Count(stringGridLine(grid[1]), "x"); got != tt.want {
				t.Fatalf("first prose row width %d, want %d", got, tt.want)
			}
		})
	}
	d := NewCommitDetailWidget("", "", "", false)
	d.SetDetail("alpha beta\n\n界界界界界", nil, "")
	grid := renderDetail(d, 10, 5)
	if got := strings.TrimRight(stringGridLine(grid[1][:d.layoutViewW]), " "); got != " alpha" {
		t.Fatalf("word-wrap row leaked the next segment: %q", got)
	}
	if got := strings.TrimRight(stringGridLine(grid[2][:d.layoutViewW]), " "); got != " beta" {
		t.Fatalf("continuation row = %q", got)
	}
	pos, ok := d.screenToSelection(7, 1)
	if !ok || pos.Col != len("alpha ") {
		t.Fatalf("padding hit escaped its segment: %+v, ok=%v", pos, ok)
	}
	for _, row := range grid {
		if row[d.layoutViewW-1].Ch == '界' {
			t.Fatal("fullwidth rune straddles the message boundary")
		}
	}
}

func stringGridLine(row []term.Cell) string {
	var text strings.Builder
	for _, cell := range row {
		text.WriteRune(cell.Ch)
	}
	return text.String()
}

func TestCommitDetailMarkdownSelectionAndResize(t *testing.T) {
	d := NewCommitDetailWidget("", "", "", false)
	d.SetDetail("**bold** alpha beta gamma\n\n``\n\n```\n  one\n\t界two\n```", nil, "")
	grid := renderDetail(d, 16, 20)
	x, y, ok := detailFind(grid, "bold")
	if !ok {
		t.Fatal("missing styled text")
	}
	d.HandleEvent(tcell.NewEventMouse(x+4, y, tcell.Button1, 0))
	d.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, 0))
	d.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, 0))
	grid = renderDetail(d, 24, 20)
	if cell := grid[y][1]; cell.Style != term.StyleHoverBold || cell.BgStyle != term.StyleSelection {
		t.Fatalf("resized selection lost Markdown style or background: %+v", cell)
	}
	if got := d.CopySelection(); got != "bold" {
		t.Fatalf("reverse selection after resize copied %q", got)
	}
	var first, last int
	for i, row := range d.rows {
		if row.text == "bold alpha beta gamma" {
			first = i
		}
		if row.text == "\t界two" {
			last = i
		}
	}
	d.hasSelection = true
	d.selection = diffTextSelection{Anchor: diffSelPos{Line: first}, Current: diffSelPos{Line: last, Col: len([]rune("\t界two"))}}
	if got := d.CopySelection(); got != "bold alpha beta gamma\n\n``\n\n  one\n\t界two" {
		t.Fatalf("paragraph/code copy changed text: %q", got)
	}
	rows := &d.messageRows[0]
	d.CollapseAllFiles()
	if rows != &d.messageRows[0] {
		t.Fatal("unchanged message was parsed again when files collapsed")
	}
	d.SetDetail("replacement", nil, "")
	if got := detailTestText(renderDetail(d, 40, 10)); !strings.Contains(got, "replacement") || strings.Contains(got, "bold") {
		t.Fatalf("message cache was not refreshed:\n%s", got)
	}
}
