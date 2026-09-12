package ui

import (
	"bytes"
	"image"
	"strings"
	"testing"

	tttimage "github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func testImageSource(id uint64, w, h int) *tttimage.Source {
	return &tttimage.Source{
		ID:  id,
		Pix: image.NewNRGBA(image.Rect(0, 0, w, h)),
		W:   w,
		H:   h,
	}
}

func commitLayer(t *testing.T, l *tttimage.Layer) string {
	t.Helper()
	var buf bytes.Buffer
	if err := l.Commit(&buf); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return buf.String()
}

func TestPlaceImageClipsAtViewportEdge(t *testing.T) {
	l := tttimage.NewLayer()
	src := testImageSource(7, 100, 100)
	l.AddSource(src)
	grid := makeGrid(40, 20)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 20})
	surface.SetImageLayer(l)
	// The sub-region starts two rows above the surface origin, and the placement four rows above the clip.
	viewport := surface.Sub(Rect{X: 0, Y: -2, W: 40, H: 8})
	placer, ok := viewport.(widgets.ImagePlacer)
	if !ok {
		t.Fatal("sub-surface should carry ImagePlacer")
	}

	l.Begin()
	placer.PlaceImage(0, -4, 20, 10, src)
	out := commitLayer(t, l)

	// 20x10 cells cropped to 20x6, source cropped to 100x60 at pixel row 40.
	for _, want := range []string{"x=0,y=40,w=100,h=60", "c=20,r=6"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected clipped placement %q in %q", want, out)
		}
	}
}

func TestPlaceImageEmptyIntersectionRecordsNothing(t *testing.T) {
	l := tttimage.NewLayer()
	src := testImageSource(7, 100, 100)
	l.AddSource(src)
	grid := makeGrid(40, 20)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 20})
	surface.SetImageLayer(l)
	viewport := surface.Sub(Rect{X: 0, Y: -12, W: 40, H: 10})
	placer, ok := viewport.(widgets.ImagePlacer)
	if !ok {
		t.Fatal("sub-surface should carry ImagePlacer")
	}

	l.Begin()
	placer.PlaceImage(0, 0, 20, 10, src)
	if out := commitLayer(t, l); len(out) != 0 {
		t.Errorf("off-screen placement wrote %d bytes: %q", len(out), out)
	}
}

func TestPlaceImageWithoutLayerRecordsNothing(t *testing.T) {
	grid := makeGrid(40, 20)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 20})
	if _, ok := Surface(surface).(widgets.ImagePlacer); !ok {
		t.Fatal("RenderSurface should implement ImagePlacer")
	}
	surface.PlaceImage(0, 0, 20, 10, testImageSource(7, 100, 100))
}

func TestPlaceImageUnchangedFrameWritesZeroBytes(t *testing.T) {
	l := tttimage.NewLayer()
	src := testImageSource(7, 100, 100)
	l.AddSource(src)
	grid := makeGrid(40, 20)
	surface := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 40, H: 20})
	surface.SetImageLayer(l)

	l.Begin()
	surface.PlaceImage(0, 0, 20, 10, src)
	if first := commitLayer(t, l); len(first) == 0 {
		t.Fatal("first frame should transmit and place")
	}
	l.Begin()
	surface.PlaceImage(0, 0, 20, 10, src)
	if second := commitLayer(t, l); len(second) != 0 {
		t.Errorf("unchanged frame wrote %d bytes: %q", len(second), second)
	}
}

type placingStubWidget struct {
	BaseWidget
	src *tttimage.Source
}

func (s *placingStubWidget) Render(surface Surface) {
	if placer, ok := surface.(widgets.ImagePlacer); ok {
		placer.PlaceImage(0, 0, 4, 4, s.src)
	}
}

func (s *placingStubWidget) HandleEvent(ev tcell.Event) EventResult { return EventIgnored }

func renderRootFrame(t *testing.T, r *Root, l *tttimage.Layer) string {
	t.Helper()
	cells := make([][]term.Cell, r.Height)
	for y := range cells {
		cells[y] = make([]term.Cell, r.Width)
	}
	l.Begin()
	r.Render(cells)
	return commitLayer(t, l)
}

func TestModalOverlaySuppressesPlacements(t *testing.T) {
	l := tttimage.NewLayer()
	src := testImageSource(9, 16, 16)
	l.AddSource(src)
	r := NewRoot(&placingStubWidget{src: src})
	r.SetSize(40, 20)
	r.ImageLayer = l

	if out := renderRootFrame(t, r, l); !strings.Contains(out, "a=p") {
		t.Fatalf("expected a placement without overlays, got %q", out)
	}
	r.PushOverlay(Overlay{Widget: &placingStubWidget{}, Modal: true})
	if out := renderRootFrame(t, r, l); len(out) == 0 {
		t.Fatalf("expected the modal frame to delete the placement, got silence")
	}
	if out := renderRootFrame(t, r, l); len(out) != 0 {
		t.Errorf("modal overlay frame placed %d bytes: %q", len(out), out)
	}
	r.PopOverlay()
	if out := renderRootFrame(t, r, l); !strings.Contains(out, "a=p") {
		t.Errorf("expected the placement to return after dismiss, got %q", out)
	}
}

func TestAnyOverlaySuppressesPlacements(t *testing.T) {
	l := tttimage.NewLayer()
	src := testImageSource(9, 16, 16)
	l.AddSource(src)
	r := NewRoot(&placingStubWidget{src: src})
	r.SetSize(40, 20)
	r.ImageLayer = l
	r.PushOverlay(Overlay{Widget: &placingStubWidget{}, Modal: false})

	l.Begin()
	cells := make([][]term.Cell, r.Height)
	for y := range cells {
		cells[y] = make([]term.Cell, r.Width)
	}
	r.Render(cells)
	// Non-modal overlays (find bar) would otherwise be hidden under a z=0 image.
	if out := commitLayer(t, l); strings.Contains(out, "a=p") {
		t.Errorf("an overlay should suppress placements, got %q", out)
	}
}
