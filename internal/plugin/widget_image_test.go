package plugin

import (
	"bytes"
	stdimage "image"
	"image/png"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

type placerMock struct {
	*mockSurface
	placed   []uint64
	released []uint64
}

func (s *placerMock) Sub(widgets.Rect) widgets.Surface { return s }
func (s *placerMock) PlaceImage(x, y, w, h int, src *image.Source) {
	s.placed = append(s.placed, src.ID)
}
func (s *placerMock) ImageReleaser() func(uint64) {
	return func(id uint64) { s.released = append(s.released, id) }
}

func testPNG(t *testing.T, w, h int) lua.LString {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, stdimage.NewNRGBA(stdimage.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return lua.LString(buf.String())
}

func imagePanel(t *testing.T, src string) (*Plugin, *PluginPanelWidget) {
	t.Helper()
	p := &Plugin{Name: "img", Enabled: true}
	p.State = NewSandbox()
	t.Cleanup(p.State.Close)
	setupTTTModule(p.State, p)
	p.Graphics = func() GraphicsInfo { return GraphicsInfo{Protocol: image.Kitty, CellW: 10, CellH: 20} }
	if err := p.State.DoString(src); err != nil {
		t.Fatal(err)
	}
	render, ok := p.State.GetGlobal("render").(*lua.LFunction)
	if !ok {
		t.Fatal("test plugin must define a global render function")
	}
	return p, NewPluginPanelWidget(p, render, nil)
}

func TestPanelImageReusesAndReleasesSources(t *testing.T) {
	p, pw := imagePanel(t, `
		mode = "image"
		function render(panel)
			if mode == "image" then
				panel:vstack({ render = function(s) s:image({ data = img }) end })
			else
				panel:label("no image")
			end
		end
	`)
	s := &placerMock{mockSurface: newMockSurface(60, 20)}

	p.State.SetGlobal("img", testPNG(t, 40, 40))
	pw.Render(s)
	pw.Render(s)
	if len(s.placed) != 2 || s.placed[0] != s.placed[1] {
		t.Fatalf("identical bytes placed sources %v, want one source twice", s.placed)
	}
	first := s.placed[0]

	p.State.SetGlobal("img", testPNG(t, 50, 40))
	pw.Render(s)
	if s.placed[2] == first || len(s.released) != 1 || s.released[0] != first {
		t.Fatalf("changed bytes: placed %v released %v", s.placed, s.released)
	}
	second := s.placed[2]

	p.State.SetGlobal("mode", lua.LString("label"))
	pw.Render(s)
	if len(s.released) != 2 || s.released[1] != second {
		t.Fatalf("dropping the image widget released %v, want %d", s.released, second)
	}

	p.State.SetGlobal("mode", lua.LString("image"))
	pw.Render(s)
	third := s.placed[len(s.placed)-1]
	pw.Close()
	if s.released[len(s.released)-1] != third {
		t.Errorf("closing the panel released %v, want %d last", s.released, third)
	}
}

func TestPanelImageArgumentErrors(t *testing.T) {
	cases := map[string]string{
		"missing data": `panel:image({ alt = "x" })`,
		"bad fit":      `panel:image({ data = "x", fit = "cover" })`,
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			p, pw := imagePanel(t, `function render(panel) `+call+` end`)
			s := newMockSurface(40, 5)
			pw.Render(s)
			if p.LastError == nil {
				t.Fatal("expected a render error")
			}
			found := false
			for _, txt := range s.texts {
				if strings.Contains(txt.text, "Plugin error") {
					found = true
				}
			}
			if !found {
				t.Errorf("render error not shown: %+v", s.texts)
			}
		})
	}
}

func TestTTTGraphics(t *testing.T) {
	p, _ := imagePanel(t, `
		local ttt = require("ttt")
		g = ttt.graphics()
		function render() end
	`)
	g := p.State.GetGlobal("g").(*lua.LTable)
	if g.RawGetString("protocol").String() != "kitty" || g.RawGetString("cell_w").String() != "10" || g.RawGetString("cell_h").String() != "20" {
		t.Errorf("ttt.graphics() = kitty/%s/%s", g.RawGetString("cell_w"), g.RawGetString("cell_h"))
	}

	p.Graphics = nil
	if err := p.State.DoString(`g = require("ttt").graphics()`); err != nil {
		t.Fatal(err)
	}
	g = p.State.GetGlobal("g").(*lua.LTable)
	if g.RawGetString("protocol").String() != "none" || g.RawGetString("cell_w").String() != "0" {
		t.Errorf("without a host, ttt.graphics() = %s/%s, want none/0", g.RawGetString("protocol"), g.RawGetString("cell_w"))
	}
}

func TestPanelImageInsideScrollViewFallsBack(t *testing.T) {
	p, pw := imagePanel(t, `
		function render(panel)
			panel:scrollview({ render = function(s) s:image({ data = img, alt = "scrolled alt", height = 3 }) end })
		end
	`)
	p.State.SetGlobal("img", testPNG(t, 40, 40))
	s := &placerMock{mockSurface: newMockSurface(60, 20)}
	pw.Render(s)
	if len(s.placed) != 0 {
		t.Fatalf("image inside a scrollview was placed: %v", s.placed)
	}
	var row strings.Builder
	for x := 0; x < 60; x++ {
		if c, ok := s.cells[[2]int{x, 0}]; ok && c.Ch != 0 {
			row.WriteRune(c.Ch)
		}
	}
	if !strings.Contains(row.String(), "scrolled alt") {
		t.Errorf("first row = %q, want the alt text", row.String())
	}
}

func TestPanelImageKeepsItsSourceWhenItChangesSlot(t *testing.T) {
	p, pw := imagePanel(t, `
		loading = true
		function render(panel)
			if loading then panel:label("Loading…") end
			panel:image({ data = img })
			panel:vstack({ render = function(s)
				if loading then s:label("Loading…") end
				s:image({ data = img2 })
			end })
		end
	`)
	s := &placerMock{mockSurface: newMockSurface(60, 20)}
	p.State.SetGlobal("img", testPNG(t, 40, 40))
	p.State.SetGlobal("img2", testPNG(t, 30, 30))
	pw.Render(s)

	p.State.SetGlobal("loading", lua.LFalse)
	pw.Render(s)
	if len(s.placed) != 4 || s.placed[2] != s.placed[0] {
		t.Fatalf("placed %v, want the top-level image's source kept", s.placed)
	}
	// A container that changes slot is rebuilt with everything in it.
	if len(s.released) != 1 || s.released[0] != s.placed[1] {
		t.Errorf("released %v, want only the image inside the moved vstack", s.released)
	}
}
