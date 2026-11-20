package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestEditorDropZoneAt(t *testing.T) {
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	group.SetRect(Rect{X: 10, Y: 5, W: 80, H: 23})
	group.Render(NewRenderSurface(makeGrid(100, 40), group.GetRect()))

	tabs := group.TabBar.GetRect()
	content := group.ContentRect()

	tests := []struct {
		name      string
		x, y      int
		sameGroup bool
		canSplit  bool
		want      EditorDropZone
	}{
		{"outside", 0, 0, false, true, EditorDropNone},
		{"same tabbar", tabs.X + 4, tabs.Y + 1, true, true, EditorDropNone},
		{"other tabbar", tabs.X + 4, tabs.Y + 1, false, true, EditorDropCenter},
		{"same center", content.X + content.W/2, content.Y + content.H/2, true, true, EditorDropNone},
		{"other center", content.X + content.W/2, content.Y + content.H/2, false, true, EditorDropCenter},
		{"left edge", content.X + 2, content.Y + content.H/2, true, true, EditorDropLeft},
		{"right edge", content.X + content.W - 3, content.Y + content.H/2, true, true, EditorDropRight},
		{"top edge", content.X + content.W/2, content.Y + 1, true, true, EditorDropTop},
		{"bottom edge", content.X + content.W/2, content.Y + content.H - 2, true, true, EditorDropBottom},
		{"corner prefers closer axis", content.X + 10, content.Y, true, true, EditorDropTop},
		{"same edge no split", content.X + 2, content.Y + content.H/2, true, false, EditorDropNone},
		{"other edge no split", content.X + 2, content.Y + content.H/2, false, false, EditorDropCenter},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := group.DropZoneAt(tc.x, tc.y, tc.sameGroup, tc.canSplit); got != tc.want {
				t.Fatalf("DropZoneAt(%d,%d, same=%v, split=%v) = %v, want %v", tc.x, tc.y, tc.sameGroup, tc.canSplit, got, tc.want)
			}
		})
	}
}

func TestEditorGroupRenderDropPreview(t *testing.T) {
	borders := term.RoundedBorderSet()
	group := NewEditorGroupWidget(&borders, 4, true, "relative")
	group.ActiveBuffer().Lines = []string{"hello world", "second line"}
	group.SetRect(Rect{X: 0, Y: 0, W: 40, H: 15})

	for _, zone := range []EditorDropZone{EditorDropLeft, EditorDropRight, EditorDropTop, EditorDropBottom, EditorDropCenter} {
		group.SetDropZone(zone)
		cells := makeGrid(40, 15)
		group.Render(NewRenderSurface(cells, group.GetRect()))

		content := group.ContentRect()
		pr := DropPreviewRect(content.W, content.H, zone)
		absX, absY := content.X+pr.X, content.Y+pr.Y

		if got := cells[absY][absX]; got.Ch != borders.TopLeft || got.Style != term.StyleBorderActive || got.BgStyle != term.StyleHighlightStrong {
			t.Fatalf("zone %v top-left corner = %+v", zone, got)
		}
		if got := cells[absY+pr.H-1][absX+pr.W-1]; got.Ch != borders.BottomRight || got.Style != term.StyleBorderActive || got.BgStyle != term.StyleHighlightStrong {
			t.Fatalf("zone %v bottom-right corner = %+v", zone, got)
		}
		if got := cells[absY+pr.H/2][absX+pr.W/2]; got.BgStyle != term.StyleHighlightStrong {
			t.Fatalf("zone %v interior cell not tinted: %+v", zone, got)
		}
		if zone == EditorDropRight && cells[content.Y+1][content.X+1].BgStyle == term.StyleHighlightStrong {
			t.Fatal("right preview tinted cells in the left half")
		}
		topHighlighted := cells[0][0].Ch == borders.Horizontal && cells[0][0].Style == term.StyleBorderActive
		if topHighlighted != (zone == EditorDropCenter) {
			t.Fatalf("zone %v tabbar top border highlight = %v, want %v", zone, topHighlighted, zone == EditorDropCenter)
		}
	}
}

func TestEditorLayoutSplitGroupAt(t *testing.T) {
	tests := []struct {
		name  string
		zone  EditorDropZone
		check func(orig, added Rect) bool
	}{
		{"left", EditorDropLeft, func(orig, added Rect) bool { return added.X < orig.X && added.Y == orig.Y }},
		{"right", EditorDropRight, func(orig, added Rect) bool { return added.X > orig.X && added.Y == orig.Y }},
		{"top", EditorDropTop, func(orig, added Rect) bool { return added.Y < orig.Y && added.X == orig.X }},
		{"bottom", EditorDropBottom, func(orig, added Rect) bool { return added.Y > orig.Y && added.X == orig.X }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			orig := NewEditorGroupWidget(nil, 4, true, "relative")
			added := NewEditorGroupWidget(nil, 4, true, "relative")
			layout := NewEditorLayoutWidget(orig, nil)
			if !layout.SplitGroupAt(orig, added, tc.zone) {
				t.Fatalf("SplitGroupAt(%v) returned false", tc.zone)
			}
			layout.SetRect(Rect{W: 100, H: 40})
			layout.Render(NewRenderSurface(makeGrid(100, 40), layout.GetRect()))
			if !tc.check(orig.GetRect(), added.GetRect()) {
				t.Fatalf("%s split rects: added=%+v orig=%+v", tc.name, added.GetRect(), orig.GetRect())
			}
		})
	}
}
