package ui

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/term"
)

func addLines(o *OutputWidget, n int) {
	for i := 0; i < n; i++ {
		o.AddLine(OutputLine{
			Time:       "12:00:00",
			PluginName: "test",
			Level:      "info",
			Message:    fmt.Sprintf("line %d", i),
		})
	}
}

func TestOutputAddLineKeepsTreeInSync(t *testing.T) {
	o := NewOutputWidget()
	addLines(o, 5)

	if got := o.tree.ItemCount(); got != 5 {
		t.Fatalf("expected 5 tree items, got %d", got)
	}
	if got := o.tree.FlatList()[4].Label; got != "line 4" {
		t.Errorf("expected last item %q, got %q", "line 4", got)
	}
}

func TestOutputTrimsToCapKeepingNewest(t *testing.T) {
	o := NewOutputWidget()
	addLines(o, outputMaxLines+1)

	want := outputMaxLines - outputTrimChunk
	if len(o.Lines) != want {
		t.Fatalf("expected %d lines after trim, got %d", want, len(o.Lines))
	}
	if o.tree.ItemCount() != want {
		t.Fatalf("expected %d tree items after trim, got %d", want, o.tree.ItemCount())
	}

	// The newest line must survive; the oldest must not.
	last := fmt.Sprintf("line %d", outputMaxLines)
	if got := o.Lines[len(o.Lines)-1].Message; got != last {
		t.Errorf("expected newest line %q, got %q", last, got)
	}
	if got := o.Lines[0].Message; got == "line 0" {
		t.Error("expected oldest line to be dropped")
	}

	// Lines and tree nodes must stay index-aligned, since renderItem indexes
	// Lines by the tree's flat index.
	for i, node := range o.tree.FlatList() {
		if node.Label != o.Lines[i].Message {
			t.Fatalf("index %d out of sync: node %q vs line %q", i, node.Label, o.Lines[i].Message)
		}
	}
}

func TestOutputTrimKeepsUniqueNodeIDs(t *testing.T) {
	o := NewOutputWidget()
	addLines(o, outputMaxLines+outputTrimChunk+10)

	seen := map[string]bool{}
	for _, node := range o.tree.FlatList() {
		if seen[node.ID] {
			t.Fatalf("duplicate node ID %q after trim", node.ID)
		}
		seen[node.ID] = true
	}
}

func TestOutputClearResetsState(t *testing.T) {
	o := NewOutputWidget()
	addLines(o, 10)
	o.Clear()

	if len(o.Lines) != 0 || o.tree.ItemCount() != 0 {
		t.Fatalf("expected empty after Clear, got %d lines / %d items", len(o.Lines), o.tree.ItemCount())
	}

	addLines(o, 1)
	if o.tree.ItemCount() != 1 {
		t.Errorf("expected 1 item after re-adding, got %d", o.tree.ItemCount())
	}
}

// A fullwidth rune in a log message advances two terminal columns. Measuring the
// message by rune count instead would put the rest of the line one column left of
// where the terminal actually draws it (issue #434).
func TestOutputRenderFullwidthMessage(t *testing.T) {
	o := NewOutputWidget()
	o.AddLine(OutputLine{Time: "12:00:00", PluginName: "p", Level: "info", Message: "가b"})

	grid := makeGrid(40, 1)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 1})
	o.renderItem(s, 0, 0, 40, false)

	// 1 (left pad) + len("12:00:00 [p]") + 1 (gap) = 14
	const msgX = 14
	if grid[0][msgX].Ch != '가' {
		t.Fatalf("grid[0][%d] = %q, want '가'", msgX, grid[0][msgX].Ch)
	}
	if grid[0][msgX+2].Ch != 'b' {
		t.Errorf("grid[0][%d] = %q, want 'b' two columns after the fullwidth rune", msgX+2, grid[0][msgX+2].Ch)
	}
}

// A style carries its own background, so a selected row that kept the muted
// prefix or the level color would show gaps in the selection bar.
func TestOutputSelectedRowUsesOneStyle(t *testing.T) {
	o := NewOutputWidget()
	o.AddLine(OutputLine{Time: "12:00:00", PluginName: "p", Level: "error", Message: "boom"})

	grid := makeGrid(40, 1)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 1})
	o.renderItem(s, 0, 0, 40, true)

	for x := 0; x < 40; x++ {
		if got := grid[0][x].Style; got != term.StyleSidebarSelected {
			t.Fatalf("column %d has style %d, want StyleSidebarSelected", x, got)
		}
	}
}

func TestOutputUnselectedRowKeepsLevelColor(t *testing.T) {
	o := NewOutputWidget()
	o.AddLine(OutputLine{Time: "12:00:00", PluginName: "p", Level: "error", Message: "boom"})

	grid := makeGrid(40, 1)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 1})
	o.renderItem(s, 0, 0, 40, false)

	if got := grid[0][1].Style; got != term.StyleMuted {
		t.Errorf("prefix style = %d, want StyleMuted", got)
	}
	// 1 (left pad) + len("12:00:00 [p]") + 1 (gap) = 14
	if got := grid[0][14].Style; got != term.StyleDanger {
		t.Errorf("message style = %d, want StyleDanger", got)
	}
}

func TestOutputCopySelected(t *testing.T) {
	clipboard.DisableSystem()
	o := NewOutputWidget()

	if o.CopySelected() {
		t.Error("expected CopySelected to report false on an empty panel")
	}

	o.AddLine(OutputLine{Time: "12:00:00", PluginName: "lsp:go", Level: "error", Message: "boom"})
	if !o.CopySelected() {
		t.Fatal("expected CopySelected to report true")
	}
	if got := clipboard.Get(); got != "12:00:00 [lsp:go] boom" {
		t.Errorf("unexpected clipboard contents: %q", got)
	}
}
