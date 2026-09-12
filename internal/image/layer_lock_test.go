package image

import (
	"bytes"
	"slices"
	"testing"
)

type lockCall struct {
	rect   Rect
	locked bool
}

func recordingLayer(t *testing.T) (*Layer, Placement, *[]lockCall) {
	t.Helper()
	l, _, p := layerFixture(t)
	var calls []lockCall
	l.SetLockFunc(func(r Rect, locked bool) {
		calls = append(calls, lockCall{rect: r, locked: locked})
	})
	return l, p, &calls
}

func TestLayerLocksCoverLivePlacements(t *testing.T) {
	l, p, calls := recordingLayer(t)
	commit(t, l, p)
	if len(*calls) != 1 {
		t.Fatalf("expected 1 lock call, got %d", len(*calls))
	}
	got := (*calls)[0]
	if !got.locked || got.rect != p.Cell {
		t.Errorf("expected lock(true) on %+v, got %+v", p.Cell, got)
	}
}

func TestLayerLocksReassertedOnUnchangedFrame(t *testing.T) {
	l, p, calls := recordingLayer(t)
	commit(t, l, p)
	// Locks are re-asserted on every Commit because resize drops them all.
	out := commit(t, l, p)
	if len(out) != 0 {
		t.Fatalf("unchanged frame wrote %d bytes", len(out))
	}
	if len(*calls) != 2 {
		t.Fatalf("expected locks re-asserted on second commit, got %d calls", len(*calls))
	}
	if !(*calls)[1].locked || (*calls)[1].rect != p.Cell {
		t.Errorf("expected lock(true) on %+v, got %+v", p.Cell, (*calls)[1])
	}
}

func TestLayerUnlocksRemovedPlacement(t *testing.T) {
	l, p, calls := recordingLayer(t)
	commit(t, l, p)
	*calls = nil
	commit(t, l)
	if len(*calls) != 1 {
		t.Fatalf("expected 1 unlock call, got %d", len(*calls))
	}
	got := (*calls)[0]
	if got.locked || got.rect != p.Cell {
		t.Errorf("expected lock(false) on %+v, got %+v", p.Cell, got)
	}
}

func TestLayerCloseUnlocksLivePlacements(t *testing.T) {
	l, p, calls := recordingLayer(t)
	commit(t, l, p)
	*calls = nil
	var buf bytes.Buffer
	if err := l.Close(&buf); err != nil {
		t.Fatalf("Close: %v", err)
	}
	found := false
	for _, c := range *calls {
		if !c.locked && c.rect == p.Cell {
			found = true
		}
	}
	if !found {
		t.Errorf("expected lock(false) on %+v in %v", p.Cell, *calls)
	}
}

func TestLayerNilLockFuncCommits(t *testing.T) {
	l, _, p := layerFixture(t)
	if out := commit(t, l, p); len(out) == 0 {
		t.Fatal("commit without a lock func should still emit bytes")
	}
}

func TestLayerMovedPlacementUnlocksOldRect(t *testing.T) {
	l, _, p := layerFixture(t)
	var log []lockCall
	l.SetLockFunc(func(r Rect, locked bool) { log = append(log, lockCall{r, locked}) })
	commit(t, l, p)
	moved := p
	moved.Cell.Y += 10
	log = nil
	commit(t, l, moved)
	if !slices.Contains(log, lockCall{p.Cell, false}) {
		t.Errorf("old rect should be unlocked, got %v", log)
	}
	if !slices.Contains(log, lockCall{moved.Cell, true}) {
		t.Errorf("new rect should be locked, got %v", log)
	}
}

func TestLayerCloseClearsSources(t *testing.T) {
	l, _, p := layerFixture(t)
	commit(t, l, p)
	var buf bytes.Buffer
	if err := l.Close(&buf); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(l.sources) != 0 {
		t.Errorf("Close should clear sources, kept %d", len(l.sources))
	}
}
