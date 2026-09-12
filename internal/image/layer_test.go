package image

import (
	"bytes"
	"image"
	"strings"
	"testing"
)

func layerFixture(t *testing.T) (*Layer, *Source, Placement) {
	t.Helper()
	l := NewLayer()
	s := testSource(t, 4, 4)
	l.AddSource(s)
	p := Placement{
		SourceID:    s.ID,
		PlacementID: 1,
		Cell:        Rect{X: 0, Y: 0, W: 4, H: 4},
		Src:         image.Rect(0, 0, 4, 4),
	}
	return l, s, p
}

func commit(t *testing.T, l *Layer, placements ...Placement) string {
	t.Helper()
	l.Begin()
	for _, p := range placements {
		l.Place(p)
	}
	var buf bytes.Buffer
	if err := l.Commit(&buf); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return buf.String()
}

func TestLayerUnchangedFrameWritesZeroBytes(t *testing.T) {
	l, _, p := layerFixture(t)
	first := commit(t, l, p)
	if len(first) == 0 {
		t.Fatal("first frame should transmit and place")
	}
	if !strings.Contains(first, "a=t") {
		t.Error("first frame should transmit image data")
	}
	second := commit(t, l, p)
	if len(second) != 0 {
		t.Errorf("unchanged frame wrote %d bytes: %q", len(second), second)
	}
}

func TestLayerMoveDeletesAndPlacesWithoutRetransmit(t *testing.T) {
	l, _, p := layerFixture(t)
	commit(t, l, p)
	moved := p
	moved.Cell.X = 5
	out := commit(t, l, moved)
	if !strings.Contains(out, "d=i") {
		t.Errorf("moved placement should delete old id, got %q", out)
	}
	if !strings.Contains(out, "a=p") {
		t.Errorf("moved placement should place new rect, got %q", out)
	}
	if strings.Contains(out, "a=t") {
		t.Errorf("moved placement must not re-transmit image data, got %q", out)
	}
	if n := commit(t, l, moved); len(n) != 0 {
		t.Errorf("settled frame wrote %d bytes", len(n))
	}
}

func TestLayerRemoveDeletesPlacementKeepsImage(t *testing.T) {
	l := NewLayer()
	kept := testSource(t, 2, 2)
	dropped := testSource(t, 2, 2)
	l.AddSource(kept)
	l.AddSource(dropped)
	kp := Placement{SourceID: kept.ID, PlacementID: 1, Cell: Rect{X: 0, Y: 0, W: 2, H: 2}, Src: image.Rect(0, 0, 2, 2)}
	dp := Placement{SourceID: dropped.ID, PlacementID: 2, Cell: Rect{X: 5, Y: 0, W: 2, H: 2}, Src: image.Rect(0, 0, 2, 2)}
	commit(t, l, kp, dp)
	out := commit(t, l, kp)
	if !strings.Contains(out, "d=i") {
		t.Errorf("removed placement should be deleted, got %q", out)
	}
	// Image data stays in the terminal so a tab switch or modal does not re-transmit.
	if strings.Contains(out, "d=I") {
		t.Errorf("dropped source must not be freed per frame, got %q", out)
	}
	if strings.Contains(out, "a=p") {
		t.Errorf("kept placement should emit nothing, got %q", out)
	}
	if again := commit(t, l, kp, dp); strings.Contains(again, "a=t") {
		t.Errorf("re-placing a known source must not re-transmit, got %q", again)
	}
}

func TestLayerInvalidate(t *testing.T) {
	l, _, p := layerFixture(t)
	commit(t, l, p)
	if n := commit(t, l, p); len(n) != 0 {
		t.Fatalf("settled frame wrote %d bytes", len(n))
	}
	l.Invalidate()
	out := commit(t, l, p)
	// Screen clears drop placements, not image data.
	if strings.Contains(out, "a=t") {
		t.Errorf("invalidated frame must not re-transmit, got %q", out)
	}
	if !strings.Contains(out, "a=p") {
		t.Errorf("invalidated frame should re-place, got %q", out)
	}
}

func TestLayerClose(t *testing.T) {
	l, _, p := layerFixture(t)
	commit(t, l, p)
	var buf bytes.Buffer
	if err := l.Close(&buf); err != nil {
		t.Fatalf("Close: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "d=i") {
		t.Errorf("Close should delete placements, got %q", out)
	}
	if !strings.Contains(out, "d=I") {
		t.Errorf("Close should free image data, got %q", out)
	}
	if n := commit(t, l); len(n) != 0 {
		t.Errorf("frame after Close wrote %d bytes", len(n))
	}
}

func TestLayerUnknownSourcePlacesWithoutTransmit(t *testing.T) {
	l := NewLayer()
	p := Placement{SourceID: 999, PlacementID: 1, Cell: Rect{X: 0, Y: 0, W: 2, H: 2}, Src: image.Rect(0, 0, 2, 2)}
	out := commit(t, l, p)
	if strings.Contains(out, "a=t") {
		t.Errorf("unknown source cannot be transmitted, got %q", out)
	}
	if !strings.Contains(out, "a=p") {
		t.Errorf("unknown source should still be placed, got %q", out)
	}
}

func TestLayerForgetFreesImageOnNextCommit(t *testing.T) {
	l, s, p := layerFixture(t)
	commit(t, l, p)
	l.Forget(s.ID)
	out := commit(t, l)
	if !strings.Contains(out, "d=I") {
		t.Errorf("forgotten source should be freed in the terminal, got %q", out)
	}
	if _, ok := l.sources[s.ID]; ok {
		t.Error("forgotten source should be dropped in-process")
	}
	l.AddSource(s)
	if again := commit(t, l, p); !strings.Contains(again, "a=t") {
		t.Errorf("re-placing a forgotten source must re-transmit, got %q", again)
	}
}
