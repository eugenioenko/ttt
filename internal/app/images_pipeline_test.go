package app

import (
	"bytes"
	"image"
	"testing"

	tttimage "github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

// Replicates tcell's draw pass over a real CellBuffer: locked cells are never Dirty, so they keep stale content like a real terminal.
type terminalSim struct {
	buf     tcell.CellBuffer
	visible [][]string
	w, h    int
}

func newTerminalSim(w, h int) *terminalSim {
	s := &terminalSim{w: w, h: h}
	s.buf.Resize(w, h)
	s.visible = make([][]string, h)
	for y := range s.visible {
		s.visible[y] = make([]string, w)
	}
	return s
}

func (s *terminalSim) lockRegion(x, y, w, h int, lock bool) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			if lock {
				s.buf.LockCell(i, j)
			} else {
				s.buf.UnlockCell(i, j)
			}
		}
	}
}

func (s *terminalSim) show() {
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			if s.buf.Dirty(x, y) {
				str, _, _ := s.buf.Get(x, y)
				s.visible[y][x] = str
				s.buf.SetDirty(x, y, false)
			}
		}
	}
}

func (s *terminalSim) resize(w, h int) {
	s.buf.Resize(w, h)
	s.w, s.h = w, h
	visible := make([][]string, h)
	for y := range visible {
		visible[y] = make([]string, w)
		if y < len(s.visible) {
			copy(visible[y], s.visible[y])
		}
	}
	s.visible = visible
}

type putScreen struct {
	sim *terminalSim
}

func (p *putScreen) Size() (int, int) { return p.sim.w, p.sim.h }
func (p *putScreen) SetCell(x, y int, c term.Cell) {
	ch := c.Ch
	if ch == 0 {
		ch = ' '
	}
	p.sim.buf.Put(x, y, string(ch), tcell.StyleDefault)
}
func (p *putScreen) Show()                           {}
func (p *putScreen) Clear()                          {}
func (p *putScreen) ShowCursor(x, y int)             {}
func (p *putScreen) HideCursor()                     {}
func (p *putScreen) SetCursorStyle(term.CursorStyle) {}

// imageFrameDriver mirrors RunEventLoop's redraw order.
type lockRecord struct {
	rect   tttimage.Rect
	locked bool
}

type imageFrameDriver struct {
	renderer  *render.Renderer
	layer     *tttimage.Layer
	sim       *terminalSim
	screen    *putScreen
	w, h      int
	lastBytes int
	lockLog   []lockRecord
}

func newImageFrameDriver(w, h int) *imageFrameDriver {
	sim := newTerminalSim(w, h)
	layer := tttimage.NewLayer()
	d := &imageFrameDriver{
		renderer: &render.Renderer{},
		layer:    layer,
		sim:      sim,
		w:        w,
		h:        h,
	}
	d.screen = &putScreen{sim: sim}
	layer.SetLockFunc(func(r tttimage.Rect, locked bool) {
		d.lockLog = append(d.lockLog, lockRecord{rect: r, locked: locked})
		sim.lockRegion(r.X, r.Y, r.W, r.H, locked)
	})
	return d
}

func (d *imageFrameDriver) frame(cells [][]term.Cell, place func(*ui.RenderSurface)) {
	d.layer.Begin()
	grid := make([][]term.Cell, d.h)
	for y := range grid {
		grid[y] = make([]term.Cell, d.w)
	}
	surface := ui.NewRenderSurface(grid, ui.Rect{X: 0, Y: 0, W: d.w, H: d.h})
	surface.SetImageLayer(d.layer)
	if place != nil {
		place(surface)
	}
	d.renderer.SetCurrent(cells)
	d.renderer.Render(d.screen)
	d.sim.show()
	var buf bytes.Buffer
	if err := d.layer.Commit(&buf); err != nil {
		panic(err)
	}
	d.sim.show()
	d.lastBytes = buf.Len()
}

func textGrid(w, h int, imageRows map[int]bool) [][]term.Cell {
	cells := make([][]term.Cell, h)
	for y := range cells {
		cells[y] = make([]term.Cell, w)
		for x := range cells[y] {
			ch := rune('a' + y%26)
			if imageRows[y] {
				ch = ' '
			}
			cells[y][x] = term.Cell{Ch: ch}
		}
	}
	return cells
}

func testImageSource() *tttimage.Source {
	return &tttimage.Source{
		ID:  42,
		Pix: image.NewNRGBA(image.Rect(0, 0, 32, 16)),
		W:   32,
		H:   16,
	}
}

func assertVisibleExcept(t *testing.T, sim *terminalSim, want [][]term.Cell, except map[[2]int]bool, msg string) {
	t.Helper()
	for y := range want {
		for x := range want[y] {
			if except[[2]int{x, y}] {
				continue
			}
			if sim.visible[y][x] != string(want[y][x].Ch) {
				t.Errorf("%s: cell (%d,%d) = %q, want %q", msg, x, y, sim.visible[y][x], string(want[y][x].Ch))
				return
			}
		}
	}
}

func rectSet(x, y, w, h int) map[[2]int]bool {
	m := make(map[[2]int]bool)
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			m[[2]int{i, j}] = true
		}
	}
	return m
}

func TestImagePipelineScrollHealsInOneFrame(t *testing.T) {
	d := newImageFrameDriver(40, 20)
	src := testImageSource()
	grid := textGrid(40, 20, map[int]bool{5: true, 6: true, 7: true, 8: true, 9: true, 10: true, 11: true, 12: true})
	placeAt := func(y int) func(*ui.RenderSurface) {
		return func(s *ui.RenderSurface) {
			s.PlaceImage(20, y, 16, 8, src)
		}
	}

	d.frame(grid, placeAt(5))
	d.frame(grid, placeAt(5))
	assertVisibleExcept(t, d.sim, grid, rectSet(20, 5, 16, 8), "settled image frame")

	d.frame(grid, placeAt(7))
	assertVisibleExcept(t, d.sim, grid, rectSet(20, 7, 16, 8), "scroll frame")
	if d.sim.visible[5][20] != " " {
		t.Errorf("vacated row should repaint to spaces, got %q", d.sim.visible[5][20])
	}
	d.frame(grid, placeAt(7))
	assertVisibleExcept(t, d.sim, grid, rectSet(20, 7, 16, 8), "settled scrolled frame")
}

func TestImagePipelineTabAwayRestoresText(t *testing.T) {
	d := newImageFrameDriver(40, 20)
	src := testImageSource()
	grid := textGrid(40, 20, map[int]bool{5: true, 6: true})
	place := func(s *ui.RenderSurface) {
		s.PlaceImage(0, 5, 40, 2, src)
	}

	d.frame(grid, place)
	d.frame(grid, place)
	text := textGrid(40, 20, nil)
	d.frame(text, nil)
	assertVisibleExcept(t, d.sim, text, nil, "tab switch should restore all text in one frame")
}

func TestImagePipelineResizeKeepsTextCurrent(t *testing.T) {
	d := newImageFrameDriver(40, 20)
	src := testImageSource()
	grid := textGrid(40, 20, map[int]bool{5: true, 6: true})
	place := func(s *ui.RenderSurface) {
		s.PlaceImage(0, 5, 40, 2, src)
	}
	d.frame(grid, place)

	// Resize wipes tcell's locks; Clear forces a full repaint.
	d.sim.resize(40, 20)
	d.renderer.Clear()
	d.layer.Invalidate()
	d.lockLog = nil
	d.frame(grid, place)
	assertVisibleExcept(t, d.sim, grid, rectSet(0, 5, 40, 2), "post-resize frame")
	if d.lastBytes == 0 {
		t.Error("post-resize frame should re-place the image")
	}
	found := false
	for _, c := range d.lockLog {
		if c.locked && c.rect == (tttimage.Rect{X: 0, Y: 5, W: 40, H: 2}) {
			found = true
		}
	}
	if !found {
		t.Errorf("locks were not re-asserted after resize, log: %v", d.lockLog)
	}
}

type modalImageStub struct {
	ui.BaseWidget
	src *tttimage.Source
}

func (s *modalImageStub) Render(surface ui.Surface) {
	w, h := surface.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ch := rune('a' + y%26)
			if y >= 5 && y < 7 {
				ch = ' '
			}
			surface.SetCell(x, y, term.Cell{Ch: ch})
		}
	}
	if rs, ok := surface.(*ui.RenderSurface); ok {
		rs.PlaceImage(0, 5, 40, 2, s.src)
	}
}

type modalTextStub struct {
	ui.BaseWidget
}

func (s *modalTextStub) Render(surface ui.Surface) {
	for y := 5; y < 7; y++ {
		surface.DrawText(0, y, "DIALOG OVER IMAGE", 0, term.StyleDefault)
	}
}

func (d *imageFrameDriver) frameRoot(r *ui.Root) {
	d.layer.Begin()
	cells := make([][]term.Cell, d.h)
	for y := range cells {
		cells[y] = make([]term.Cell, d.w)
	}
	r.Render(cells)
	d.renderer.SetCurrent(cells)
	d.renderer.Render(d.screen)
	d.sim.show()
	var buf bytes.Buffer
	if err := d.layer.Commit(&buf); err != nil {
		panic(err)
	}
	d.sim.show()
	d.lastBytes = buf.Len()
}

func TestImagePipelineModalShowsDialogTextSameFrame(t *testing.T) {
	d := newImageFrameDriver(40, 20)
	src := testImageSource()
	r := ui.NewRoot(&modalImageStub{src: src})
	r.SetSize(40, 20)
	r.ImageLayer = d.layer

	d.frameRoot(r)
	d.frameRoot(r)
	assertVisibleExcept(t, d.sim, textGrid(40, 20, map[int]bool{5: true, 6: true}), rectSet(0, 5, 40, 2), "settled image frame")

	r.PushOverlay(ui.Overlay{Widget: &modalTextStub{}, Modal: true})
	d.frameRoot(r)
	for y := 5; y < 7; y++ {
		row := ""
		for x := 0; x < 17; x++ {
			row += d.sim.visible[y][x]
		}
		if row != "DIALOG OVER IMAGE" {
			t.Fatalf("dialog text missing on modal frame row %d: %q", y, row)
		}
	}
	if d.lastBytes == 0 {
		t.Error("modal frame should delete the suppressed placement")
	}

	r.PopOverlay()
	d.frameRoot(r)
	assertVisibleExcept(t, d.sim, textGrid(40, 20, map[int]bool{5: true, 6: true}), rectSet(0, 5, 40, 2), "post-modal frame")
}
