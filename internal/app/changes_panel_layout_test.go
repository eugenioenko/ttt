package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestChangesPanelPlacesCommitInputAboveWorkingTree(t *testing.T) {
	cp := NewChangesPanel()
	if cp.Adapter.Inner() != cp.Split {
		t.Fatalf("changes panel root = %T, want its content split", cp.Adapter.Inner())
	}
	top, ok := cp.Split.Top.(*widgets.VStackWidget)
	if !ok {
		t.Fatalf("changes panel top = %T, want *widgets.VStackWidget", cp.Split.Top)
	}
	if len(top.Children) != 3 {
		t.Fatalf("changes panel top has %d children, want 3", len(top.Children))
	}
	if top.Children[0] != cp.Input {
		t.Errorf("first child = %T, want commit input", top.Children[0])
	}
	if _, ok := top.Children[1].(*widgets.DividerWidget); !ok {
		t.Errorf("second child = %T, want divider", top.Children[1])
	}
	if top.Children[2] != cp.Tree {
		t.Errorf("third child = %T, want working-tree changes", top.Children[2])
	}

	bottom, ok := cp.Split.Bottom.(*widgets.VStackWidget)
	if !ok {
		t.Fatalf("changes panel bottom = %T, want *widgets.VStackWidget", cp.Split.Bottom)
	}
	if len(bottom.Children) != 2 {
		t.Fatalf("changes panel bottom has %d children, want 2", len(bottom.Children))
	}
	if _, ok := bottom.Children[0].(*widgets.TitleWidget); !ok {
		t.Errorf("first bottom child = %T, want commit-history title", bottom.Children[0])
	}
	if box, ok := bottom.Children[1].(*widgets.BoxWidget); !ok {
		t.Errorf("second bottom child = %T, want commit-history box", bottom.Children[1])
	} else if box.FixedHeight != 0 {
		t.Errorf("commit-history box has fixed height %d, want grow layout", box.FixedHeight)
	}

	// Focus follows visual order: the primary action is ready immediately, and
	// one Tab reaches the working-tree actions beneath it.
	cp.Adapter.SetFocused(true)
	if got := cp.Adapter.FocusedWidget(); got != cp.Input {
		t.Errorf("initial focus = %T, want commit input", got)
	}
	cp.Adapter.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	if got := cp.Adapter.FocusedWidget(); got != cp.Tree {
		t.Errorf("focus after Tab = %T, want working-tree changes", got)
	}
	cp.Adapter.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	if got := cp.Adapter.FocusedWidget(); got != cp.CommitLog {
		t.Errorf("focus after second Tab = %T, want commit history", got)
	}
}
