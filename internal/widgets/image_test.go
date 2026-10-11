package widgets

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	stdimage "image"
	"image/png"
	"strings"
	"sync"
	"testing"

	"github.com/eugenioenko/ttt/internal/image"
)

type placement struct {
	x, y, w, h int
	id         uint64
}

type placements struct {
	placed   []placement
	released []uint64
}

type placerSurface struct {
	*testSurface
	*placements
}

func newPlacerSurface(w, h int) *placerSurface {
	return &placerSurface{testSurface: newTestSurface(w, h), placements: &placements{}}
}

func (s *placerSurface) Sub(r Rect) Surface {
	return &placerSurface{testSurface: s.testSurface.Sub(r).(*testSurface), placements: s.placements}
}

func (s *placerSurface) PlaceImage(x, y, w, h int, src *image.Source) {
	s.placed = append(s.placed, placement{x, y, w, h, src.ID})
}

func (s *placerSurface) ImageReleaser() func(uint64) {
	return func(id uint64) { s.released = append(s.released, id) }
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, stdimage.NewNRGBA(stdimage.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func surfaceText(s *testSurface) string {
	var sb strings.Builder
	for _, row := range s.cells {
		for _, c := range row {
			if c.Ch != 0 {
				sb.WriteRune(c.Ch)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func kittyConfig(data []byte) ImageConfig {
	return ImageConfig{Data: string(data), Protocol: image.Kitty, CellW: 10, CellH: 20}
}

func TestImageWidgetPlacesCenteredContain(t *testing.T) {
	s := newPlacerSurface(60, 20)
	w := NewImageWidget(kittyConfig(pngBytes(t, 160, 120)))
	w.Render(s)
	if len(s.placed) != 1 {
		t.Fatalf("placements = %d, want 1", len(s.placed))
	}
	// 160x120 px at 10x20 px cells is 16x6 cells, centered in 60x20.
	if got := s.placed[0]; got.x != 22 || got.y != 7 || got.w != 16 || got.h != 6 {
		t.Errorf("placement = %+v, want 16x6 at (22,7)", got)
	}
}

func TestImageWidgetFitNoneUsesNaturalSize(t *testing.T) {
	s := newPlacerSurface(10, 5)
	cfg := kittyConfig(pngBytes(t, 205, 130))
	cfg.Fit = ImageFitNone
	NewImageWidget(cfg).Render(s)
	if got := s.placed[0]; got.x != 0 || got.y != 0 || got.w != 21 || got.h != 7 {
		t.Errorf("placement = %+v, want natural 21x7 at the origin", got)
	}
}

func TestImageWidgetReusesSourceForSameBytes(t *testing.T) {
	data := pngBytes(t, 40, 40)
	s := newPlacerSurface(60, 20)
	w := NewImageWidget(kittyConfig(data))
	w.Render(s)
	w.SetConfig(kittyConfig(append([]byte(nil), data...)))
	w.Render(s)
	if s.placed[0].id != s.placed[1].id {
		t.Errorf("identical bytes produced sources %d and %d", s.placed[0].id, s.placed[1].id)
	}
	if len(s.released) != 0 {
		t.Errorf("released %v for unchanged content", s.released)
	}

	w.SetConfig(kittyConfig(pngBytes(t, 50, 40)))
	w.Render(s)
	if s.placed[2].id == s.placed[0].id {
		t.Error("changed bytes reused the old source")
	}
	if len(s.released) != 1 || s.released[0] != s.placed[0].id {
		t.Errorf("released = %v, want the old source %d", s.released, s.placed[0].id)
	}

	w.Close()
	if len(s.released) != 2 || s.released[1] != s.placed[2].id {
		t.Errorf("close released %v, want the current source too", s.released)
	}
}

func TestImageWidgetFallbacks(t *testing.T) {
	data := pngBytes(t, 16, 12)
	cases := []struct {
		name    string
		surface Surface
		cfg     ImageConfig
		want    string
	}{
		{"alt without graphics", nil, ImageConfig{Data: string(data), Alt: "Page 3 of 10"}, "Page 3 of 10"},
		{"info without graphics", nil, ImageConfig{Data: string(data)}, "PNG image 16x12"},
		{"unknown cell size", nil, ImageConfig{Data: string(data), Protocol: image.Kitty}, "PNG image 16x12"},
		{"surface cannot place", newTestSurface(80, 4), kittyConfig(data), "PNG image 16x12"},
		{"corrupt data", nil, kittyConfig([]byte("not an image")), "image"},
		{"oversized", nil, kittyConfig(oversizedPNGHeader(t)), "exceed maximum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps := newPlacerSurface(80, 4)
			ts := ps.testSurface
			var surface Surface = ps
			if tc.surface != nil {
				surface = tc.surface
				ts = tc.surface.(*testSurface)
			}
			NewImageWidget(tc.cfg).Render(surface)
			if ps, ok := surface.(*placerSurface); ok && len(ps.placed) != 0 {
				t.Fatalf("placed an image: %+v", ps.placed)
			}
			if got := surfaceText(ts); !strings.Contains(got, tc.want) {
				t.Errorf("text = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// A valid PNG signature and IHDR chunk declaring 20000x20000 pixels, without pixel data.
func oversizedPNGHeader(t *testing.T) []byte {
	t.Helper()
	full := pngBytes(t, 1, 1)
	ihdr := bytes.Index(full, []byte("IHDR"))
	data := append([]byte(nil), full[:ihdr+4+13+4]...)
	copy(data[ihdr+4:], []byte{0, 0, 0x4e, 0x20, 0, 0, 0x4e, 0x20})
	binary.BigEndian.PutUint32(data[ihdr+17:], crc32.ChecksumIEEE(data[ihdr:ihdr+17]))
	return data
}

func TestImageWidgetDecodesOffTheRenderPath(t *testing.T) {
	woke := make(chan struct{}, 1)
	cfg := kittyConfig(pngBytes(t, 40, 40))
	cfg.Redraw = func() { woke <- struct{}{} }
	s := newPlacerSurface(60, 20)
	w := NewImageWidget(cfg)

	w.Render(s)
	if len(s.placed) != 0 {
		t.Fatalf("first frame placed %d images before the decode finished", len(s.placed))
	}
	if got := surfaceText(s.testSurface); !strings.Contains(got, "PNG image 40x40") {
		t.Errorf("text while decoding = %q, want the image info", got)
	}

	<-woke
	w.Render(s)
	if len(s.placed) != 1 {
		t.Fatalf("placements after decode = %d, want 1", len(s.placed))
	}

	stale := cfg
	stale.Data = string(pngBytes(t, 50, 40))
	w.SetConfig(stale)
	w.Render(s)
	<-woke
	w.Render(s)
	if len(s.placed) != 2 || s.placed[1].id == s.placed[0].id {
		t.Errorf("changed data placements = %+v, want a new source", s.placed)
	}
}

func TestImageWidgetRunsOneDecodeAtATime(t *testing.T) {
	var mu sync.Mutex
	running, peak := 0, 0
	gate := make(chan struct{})
	decodeImage = func(data []byte, name string) (*image.Source, error) {
		mu.Lock()
		running++
		peak = max(peak, running)
		mu.Unlock()
		<-gate
		mu.Lock()
		running--
		mu.Unlock()
		return image.Decode(data, name)
	}
	t.Cleanup(func() { decodeImage = image.Decode })

	woke := make(chan struct{}, 4)
	cfg := kittyConfig(pngBytes(t, 40, 40))
	cfg.Redraw = func() { woke <- struct{}{} }
	s := newPlacerSurface(60, 20)
	w := NewImageWidget(cfg)
	w.Render(s)
	for _, size := range []int{50, 60} {
		cfg.Data = string(pngBytes(t, size, 40))
		w.SetConfig(cfg)
		w.Render(s)
	}

	gate <- struct{}{}
	<-woke
	w.Render(s)
	if len(s.placed) != 0 {
		t.Fatalf("placed a stale decode: %+v", s.placed)
	}
	gate <- struct{}{}
	<-woke
	w.Render(s)
	if peak != 1 {
		t.Errorf("peak concurrent decodes = %d, want 1", peak)
	}
	if len(s.placed) != 1 || w.src == nil || w.src.W != 60 {
		t.Errorf("placements = %+v, want the latest 60px image", s.placed)
	}
}
