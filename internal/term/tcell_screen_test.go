package term

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/gdamore/tcell/v3/tty"
)

func TestTcellScreen_SetCellWritesRune(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(4, 1)
	ts := NewTcellScreenFrom(sim)
	for _, r := range []rune{'a', ' ', 'é', '中'} {
		ts.SetCell(0, 0, Cell{Ch: r})
		if got, _, _ := sim.Get(0, 0); got != string(r) {
			t.Errorf("SetCell(%q): screen has %q", r, got)
		}
	}
}

func TestTcellScreen_SetCellWritesCompleteGrapheme(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(4, 1)
	screen := NewTcellScreenFrom(sim)
	for _, text := range []string{"e\u0301", "q\u0301", "e\u0300", "e"} {
		screen.SetCell(0, 0, Cell{Ch: rune(text[0]), Text: text})
		if got, _, width := sim.Get(0, 0); got != text || width != 1 {
			t.Fatalf("SetCell(%q) = %q, width %d", text, got, width)
		}
	}
	screen.SetCell(0, 0, Cell{Ch: 'e'})
	if got, _, _ := sim.Get(0, 0); got != "e" {
		t.Fatalf("plain cell retained combining mark: %q", got)
	}
}

func TestTcellScreen_SetCellASCIIDoesNotAllocate(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(4, 1)
	ts := NewTcellScreenFrom(sim)
	cell := Cell{Ch: 'a'}
	ts.SetCell(0, 0, cell)
	if allocs := testing.AllocsPerRun(100, func() { ts.SetCell(0, 0, cell) }); allocs != 0 {
		t.Errorf("SetCell of an ASCII rune allocated %v times, want 0", allocs)
	}
}

func TestTcellScreen_SetCellReversesAStyledCell(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(2, 1)
	ts := NewTcellScreenFrom(sim)
	ts.SetCell(0, 0, Cell{Ch: 'a', Attrs: CellAttrReverse})
	ts.SetCell(1, 0, Cell{Ch: 'b'})
	if _, style, _ := sim.Get(0, 0); !style.HasReverse() {
		t.Error("cell with CellAttrReverse is not reversed")
	}
	if _, style, _ := sim.Get(1, 0); style.HasReverse() {
		t.Error("plain cell is reversed")
	}
}

func TestTcellScreen_SetCellInheritsReverseFromBgStyle(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(2, 1)
	ts := NewTcellScreenFrom(sim)
	styles := DefaultStyleMap()
	styles[StyleSelection] = tcell.StyleDefault.Reverse(true)
	styles[StyleActiveLine] = tcell.StyleDefault.Background(color.Green)
	ts.SetStyleMap(styles)

	ts.SetCell(0, 0, Cell{Ch: 'a', BgStyle: StyleSelection})
	ts.SetCell(1, 0, Cell{Ch: 'b', BgStyle: StyleActiveLine})

	if _, style, _ := sim.Get(0, 0); !style.HasReverse() {
		t.Error("cell with reversed BgStyle is not reversed")
	}
	if _, style, _ := sim.Get(1, 0); style.HasReverse() {
		t.Error("cell with normal BgStyle is reversed")
	}
}

func TestTcellScreen_HideUsesTheRenderedBackground(t *testing.T) {
	for _, tc := range []struct {
		name string
		cell Cell
		want tcell.Color
	}{
		{"plain", Cell{Ch: 'a', Attrs: CellAttrHide}, color.Blue},
		{"layered", Cell{Ch: 'a', BgStyle: StyleActiveLine, Attrs: CellAttrHide}, color.Green},
		{"reverse", Cell{Ch: 'a', Attrs: CellAttrHide | CellAttrReverse}, color.Red},
		{"named style", Cell{Ch: 'a', Style: StyleHide}, color.Blue},
		{"wide", Cell{Ch: '界', Attrs: CellAttrHide}, color.Blue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sim := NewSimScreen()
			sim.SetSize(4, 1)
			ts := NewTcellScreenFrom(sim)
			styles := DefaultStyleMap()
			styles[StyleDefault] = tcell.StyleDefault.Foreground(color.Red).Background(color.Blue)
			styles[StyleHide] = styles[StyleDefault]
			styles[StyleActiveLine] = tcell.StyleDefault.Background(color.Green)
			ts.SetStyleMap(styles)
			cell := tc.cell
			cell.Underline, cell.Bold = true, true
			cell.UlStyle = StyleDiagError
			ts.SetCell(0, 0, cell)
			text, style, _ := sim.Get(0, 0)
			if text != string(cell.Ch) {
				t.Fatalf("text = %q, want %q", text, cell.Ch)
			}
			if style.GetForeground() != tc.want || style.GetBackground() != tc.want {
				t.Fatalf("foreground/background = %v/%v, want %v", style.GetForeground(), style.GetBackground(), tc.want)
			}
			if style.GetAttributes() != 0 || style.GetUnderlineStyle() != tcell.UnderlineStyleNone {
				t.Fatal("hidden text retained visible attributes")
			}
		})
	}
}

func TestTcellScreen_HideWithDefaultBackgroundClearsWideCharacters(t *testing.T) {
	sim := NewSimScreen()
	sim.SetSize(4, 1)
	ts := NewTcellScreenFrom(sim)
	ts.SetCell(0, 0, Cell{Ch: '界'})
	ts.SetCell(2, 0, Cell{Ch: 'x'})
	ts.SetCell(0, 0, Cell{Ch: '界', Attrs: CellAttrHide})
	for x := range 2 {
		if text, style, _ := sim.Get(x, 0); text != " " || style.GetBackground() != tcell.ColorDefault {
			t.Errorf("column %d = %q with background %v, want a default-background space", x, text, style.GetBackground())
		}
	}
	if text, _, _ := sim.Get(2, 0); text != "x" {
		t.Fatal("hiding a wide character changed its neighbor")
	}
	ts.SetCell(0, 0, Cell{Ch: '界'})
	if text, _, _ := sim.Get(0, 0); text != "界" {
		t.Fatal("clearing concealment did not restore the character")
	}
}

func TestTcellScreen_HideReversedDefaultForegroundKeepsBackground(t *testing.T) {
	for _, fromStyle := range []bool{false, true} {
		name := "cell attribute"
		if fromStyle {
			name = "theme style"
		}
		t.Run(name, func(t *testing.T) {
			sim := NewSimScreen()
			sim.SetSize(2, 1)
			ts := NewTcellScreenFrom(sim)
			styles := DefaultStyleMap()
			styles[StyleDefault] = tcell.StyleDefault.Background(color.Blue).Reverse(fromStyle)
			ts.SetStyleMap(styles)
			cell := Cell{Ch: '界', Attrs: CellAttrHide, Bold: true, Underline: true}
			if !fromStyle {
				cell.Attrs |= CellAttrReverse
			}
			ts.SetCell(0, 0, cell)
			for x := range 2 {
				text, style, _ := sim.Get(x, 0)
				if text != " " || !style.HasReverse() || style.GetForeground() != tcell.ColorDefault {
					t.Errorf("column %d: concealment lost the terminal-default foreground as its displayed background", x)
				}
				if style.HasBold() || style.GetUnderlineStyle() != tcell.UnderlineStyleNone {
					t.Errorf("column %d: fallback space retained visible attributes", x)
				}
			}
		})
	}
}

type mockTty struct {
	buf bytes.Buffer
}

func (m *mockTty) Start() error             { return nil }
func (m *mockTty) Stop() error              { return nil }
func (m *mockTty) Drain() error             { return nil }
func (m *mockTty) NotifyResize(chan<- bool) {}
func (m *mockTty) WindowSize() (tty.WindowSize, error) {
	return tty.WindowSize{Width: 80, Height: 24}, nil
}
func (m *mockTty) Read(p []byte) (int, error)  { return 0, io.EOF }
func (m *mockTty) Write(p []byte) (int, error) { return m.buf.Write(p) }
func (m *mockTty) Close() error                { return nil }

func TestSetPointerShapeEmitsOSC22AndCaches(t *testing.T) {
	sim := NewSimScreen()
	mtty := &mockTty{}
	sim.SetTty(mtty)
	ts := NewTcellScreenFrom(sim)

	if !ts.PointerShapesEnabled() {
		t.Fatal("expected pointerShapesEnabled to default to true")
	}

	ts.SetPointerShape("text")
	if got := mtty.buf.String(); got != "\x1b]22;text\x1b\\" {
		t.Fatalf("first SetPointerShape: got %q, want %q", got, "\x1b]22;text\x1b\\")
	}

	// Repeated identical shape should be suppressed by cache
	mtty.buf.Reset()
	ts.SetPointerShape("text")
	if got := mtty.buf.String(); got != "" {
		t.Fatalf("cached SetPointerShape emitted: %q", got)
	}

	// Different shape writes new OSC 22
	ts.SetPointerShape("col-resize")
	if got := mtty.buf.String(); got != "\x1b]22;col-resize\x1b\\" {
		t.Fatalf("new SetPointerShape: got %q, want %q", got, "\x1b]22;col-resize\x1b\\")
	}

	// Fini restores default
	mtty.buf.Reset()
	ts.Fini()
	if got := mtty.buf.String(); got != "\x1b]22;default\x1b\\" {
		t.Fatalf("Fini did not restore default pointer: got %q, want %q", got, "\x1b]22;default\x1b\\")
	}
}

func TestTcellScreenFiniBeginsOSCShutdown(t *testing.T) {
	sim := NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	reader := NewOSCReader(&mockTty{})
	screen := NewTcellScreenFrom(sim)
	screen.osc = reader
	screen.Fini()
	if !reader.shuttingDown.Load() {
		t.Fatal("Fini did not enable shutdown writes")
	}
}

func TestSetPointerShapesDisabledFromStartupEmitsNoOSC22(t *testing.T) {
	sim := NewSimScreen()
	mtty := &mockTty{}
	sim.SetTty(mtty)
	ts := NewTcellScreenFrom(sim)

	ts.SetPointerShapesEnabled(false)
	if mtty.buf.Len() > 0 {
		t.Fatalf("SetPointerShapesEnabled(false) at startup emitted: %q", mtty.buf.String())
	}
	if ts.PointerShapesEnabled() {
		t.Fatal("expected PointerShapesEnabled() to be false")
	}

	ts.SetPointerShape("text")
	ts.SetPointerShape("col-resize")
	ts.Fini()

	if mtty.buf.Len() > 0 {
		t.Fatalf("disabled session emitted OSC 22 sequences: %q", mtty.buf.String())
	}
}

func TestSetPointerShapesLiveDisableResetsOnce(t *testing.T) {
	sim := NewSimScreen()
	mtty := &mockTty{}
	sim.SetTty(mtty)
	ts := NewTcellScreenFrom(sim)

	ts.SetPointerShape("text")
	if got := mtty.buf.String(); got != "\x1b]22;text\x1b\\" {
		t.Fatalf("initial SetPointerShape: got %q", got)
	}

	mtty.buf.Reset()
	ts.SetPointerShapesEnabled(false)
	if got := mtty.buf.String(); got != "\x1b]22;default\x1b\\" {
		t.Fatalf("live disable did not restore default: got %q, want %q", got, "\x1b]22;default\x1b\\")
	}

	mtty.buf.Reset()
	ts.SetPointerShape("hand")
	ts.SetPointerShapesEnabled(false)
	ts.Fini()

	if mtty.buf.Len() > 0 {
		t.Fatalf("subsequent operations after live disable emitted: %q", mtty.buf.String())
	}
}

func TestSetPointerShapesLiveDisableWhenUnchangedEmitsNothing(t *testing.T) {
	sim := NewSimScreen()
	mtty := &mockTty{}
	sim.SetTty(mtty)
	ts := NewTcellScreenFrom(sim)

	ts.SetPointerShapesEnabled(false)
	if mtty.buf.Len() > 0 {
		t.Fatalf("live disable when pointer was never changed emitted: %q", mtty.buf.String())
	}
}

func TestSetPointerShapesLiveDisableWhenAlreadyDefaultEmitsNothing(t *testing.T) {
	sim := NewSimScreen()
	mtty := &mockTty{}
	sim.SetTty(mtty)
	ts := NewTcellScreenFrom(sim)

	ts.SetPointerShape("default")
	mtty.buf.Reset()

	ts.SetPointerShapesEnabled(false)
	if mtty.buf.Len() > 0 {
		t.Fatalf("live disable when pointer is already default emitted: %q", mtty.buf.String())
	}
}

func TestSetPointerShapesReenableResumesWithoutStaleCache(t *testing.T) {
	sim := NewSimScreen()
	mtty := &mockTty{}
	sim.SetTty(mtty)
	ts := NewTcellScreenFrom(sim)

	ts.SetPointerShape("text")
	ts.SetPointerShapesEnabled(false)
	mtty.buf.Reset()

	ts.SetPointerShapesEnabled(true)
	if !ts.PointerShapesEnabled() {
		t.Fatal("expected PointerShapesEnabled() to be true")
	}

	// Resuming with the same shape ("text") must NOT be suppressed by stale cache
	ts.SetPointerShape("text")
	if got := mtty.buf.String(); got != "\x1b]22;text\x1b\\" {
		t.Fatalf("re-enabled SetPointerShape: got %q, want %q", got, "\x1b]22;text\x1b\\")
	}

	// Now cache should suppress repeated shape
	mtty.buf.Reset()
	ts.SetPointerShape("text")
	if mtty.buf.Len() > 0 {
		t.Fatalf("re-enabled cache failed to suppress repeated shape: %q", mtty.buf.String())
	}
}

type winMockTty struct {
	mockTty
	stopQ chan struct{}
}

func (w *winMockTty) Drain() error {
	select {
	case <-w.stopQ:
	default:
		close(w.stopQ)
	}
	return nil
}

func (w *winMockTty) Read(p []byte) (int, error) {
	<-w.stopQ
	return 0, nil
}

func TestTerminfoScreenFiniWithWindowsDrainedTty(t *testing.T) {
	tty := &winMockTty{stopQ: make(chan struct{})}
	screen, _, err := NewTerminfoScreenFromTty(tty, tcell.OptNegotiation(false))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		screen.Fini()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("screen.Fini hung when Tty.Drain yields 0-byte reads (Windows console shutdown)")
	}
}
