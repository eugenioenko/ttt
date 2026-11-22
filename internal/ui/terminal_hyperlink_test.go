package ui

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/xterm-go"
	"github.com/gdamore/tcell/v3"
)

func hyperlinkWidget(t *testing.T, cols, rows int) (*TerminalWidget, func(string), func() [][]term.Cell) {
	t.Helper()
	dev, err := terminal.New("/bin/cat", cols, rows, 20, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dev.Close() })
	tw := NewTerminalWidget(dev, nil)
	tw.SetRect(Rect{X: 3, Y: 2, W: cols, H: rows})
	write := func(text string) { dev.Snapshot(func(xt *xterm.Terminal) { xt.WriteString(text) }) }
	render := func() [][]term.Cell {
		cells := make([][]term.Cell, rows)
		for y := range cells {
			cells[y] = make([]term.Cell, cols)
		}
		tw.Render(NewRenderSurface(cells, Rect{W: cols, H: rows}))
		return cells
	}
	return tw, write, render
}

func osc8(uri, label, end string) string {
	return "\x1b]8;id=shared;" + uri + end + label + "\x1b]8;;" + end
}

func TestTerminalHyperlinkRenderAndClick(t *testing.T) {
	for _, end := range []string{"\a", "\x1b\\"} {
		t.Run(end, func(t *testing.T) {
			tw, write, render := hyperlinkWidget(t, 40, 3)
			write("界 " + osc8("https://example.com/target", "Click here", end) + " plain")
			cells := render()
			for x := 3; x < 13; x++ {
				if cells[0][x].Link.URL() != "https://example.com/target" || cells[0][x].Link.ID() == "" {
					t.Fatalf("linked cell %d: %+v", x, cells[0][x])
				}
			}
			if cells[0][13].Link.URL() != "" {
				t.Fatal("link leaked into plain text")
			}
			opened := ""
			tw.OnOpenURL = func(uri string) { opened = uri }
			tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.ButtonNone, tcell.ModCtrl))
			if tw.PointerShapeAt(6, 2) != term.PointerHand {
				t.Fatal("missing link pointer")
			}
			if render()[0][3].Attrs&term.CellAttrUnderline == 0 {
				t.Fatal("missing hover underline")
			}
			tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.Button1, tcell.ModCtrl))
			if opened != "https://example.com/target" || tw.HasSelection() {
				t.Fatalf("opened=%q, selection=%t", opened, tw.HasSelection())
			}
			tw.SuperShortcuts = true
			opened = ""
			tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.Button1, tcell.ModMeta))
			if opened != "https://example.com/target" {
				t.Fatal("Super+click did not open link")
			}
		})
	}
}

func TestTerminalHyperlinkUsesCurrentTarget(t *testing.T) {
	tw, write, render := hyperlinkWidget(t, 40, 2)
	tw.linkModifierHeld = true
	label := "https://label.example"
	write(osc8("https://first.example", label, "\a"))
	render()
	assertTarget := func(want string) {
		t.Helper()
		got := tw.linkAt(0, 2)
		if got == nil || got.URL != want {
			t.Fatalf("link = %+v, want %q", got, want)
		}
	}
	assertTarget("https://first.example")
	write("\r" + osc8("https://second.example", label, "\a"))
	assertTarget("https://second.example")
	if got := render()[0][0].Link.URL(); got != "https://second.example" {
		t.Fatalf("rendered URL=%q", got)
	}
	write("\r" + label)
	assertTarget(label)
	if render()[0][0].Link.URL() != "" {
		t.Fatal("overwrite retained hyperlink")
	}
	write("\r" + osc8("javascript:alert(1)", label, "\a"))
	if tw.linkAt(0, 2) != nil || render()[0][0].Link.URL() != "" {
		t.Fatal("unsupported explicit target fell back to its label")
	}
}

func TestTerminalHyperlinkScrollbackAndWideCells(t *testing.T) {
	tw, write, render := hyperlinkWidget(t, 8, 2)
	write(osc8("https://example.com", "界abcdefghi", "\a"))
	cells := render()
	if cells[0][0].Link.URL() == "" || cells[0][1].Link.URL() == "" || cells[1][0].Link.URL() == "" {
		t.Fatal("wide or wrapped cells lost hyperlink")
	}
	write("\r\nnext\r\nlast")
	tw.scrollOffset = 2
	cells = render()
	if cells[0][0].Link.URL() != "https://example.com" {
		t.Fatal("scrollback lost hyperlink")
	}
	opened := ""
	tw.OnOpenURL = func(uri string) { opened = uri }
	tw.HandleEvent(tcell.NewEventMouse(4, 2, tcell.Button1, tcell.ModCtrl))
	if opened != "https://example.com" {
		t.Fatalf("scrollback click opened %q", opened)
	}
	tw.scrollOffset = 0
	write("\x1b[?1049h\x1b[H" + osc8("https://alternate.example", "Alt", "\a"))
	if render()[0][0].Link.URL() != "https://alternate.example" {
		t.Fatal("alternate screen lost hyperlink")
	}
	write("\x1b[?1049l")
	if strings.Contains(render()[0][0].Link.URL(), "alternate") {
		t.Fatal("alternate hyperlink leaked into normal buffer")
	}
}

func TestTerminalHyperlinkFileTarget(t *testing.T) {
	tw, write, _ := hyperlinkWidget(t, 40, 2)
	path := filepath.Join(t.TempDir(), "a b.go")
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	write(osc8(uri, "source", "\a"))
	opened := ""
	tw.OnOpenFile = func(path string, line, col int) { opened = path }
	tw.HandleEvent(tcell.NewEventMouse(3, 2, tcell.Button1, tcell.ModCtrl))
	if opened != path {
		t.Fatalf("opened %q, want %q", opened, path)
	}
	for _, raw := range []string{"file://other-host/tmp/a", "file:///tmp/a%00b", "file:///tmp/a?query", "file:///tmp/a#fragment", "https:///missing-host", "https://example.com/\x1b", "javascript:alert(1)", "/relative"} {
		if got := terminalLinkTarget(raw); got != nil {
			t.Errorf("target(%q)=%+v", raw, got)
		}
	}
}

func TestTerminalHyperlinkClippingAndPaneIDs(t *testing.T) {
	tw, write, render := hyperlinkWidget(t, 4, 2)
	_, otherWrite, otherRender := hyperlinkWidget(t, 4, 2)
	text := osc8("https://example.com", "abc", "\a")
	write(text)
	otherWrite(text)
	if render()[0][0].Link.ID() == otherRender()[0][0].Link.ID() {
		t.Fatal("different panes share hyperlink ID")
	}
	c := term.Cell{Ch: '界', Link: term.NewHyperlink("https://example.com", "id")}
	tw.clipWideRune(&c, 3, 4)
	if c.Ch != ' ' || c.Link.URL() != "" || c.Link.ID() != "" {
		t.Fatalf("clipped cell retained link: %+v", c)
	}
}

func TestTerminalHyperlinkClippedCellCannotOpen(t *testing.T) {
	tw, write, _ := hyperlinkWidget(t, 5, 2)
	write("abc" + osc8("https://clipped.example", "界", "\a") + "\r\n" + osc8("https://interior.example", "界", "\a"))
	cells := [][]term.Cell{make([]term.Cell, 4), make([]term.Cell, 4)}
	tw.Render(NewRenderSurface(cells, Rect{W: 4, H: 2}))
	if cells[0][3].Ch != ' ' || cells[0][3].Link.URL() != "" {
		t.Fatalf("clipped cell = %+v", cells[0][3])
	}
	opened := ""
	tw.OnOpenURL = func(uri string) { opened = uri }
	tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.Button1, tcell.ModCtrl))
	if opened != "" || tw.PointerShapeAt(6, 2) == term.PointerHand {
		t.Fatal("clipped blank still acts as a link")
	}
	for _, x := range []int{3, 4} {
		tw.ClearSelection()
		tw.HandleEvent(tcell.NewEventMouse(x, 3, tcell.Button1, tcell.ModCtrl))
		if opened != "https://interior.example" {
			t.Fatalf("interior wide cell at %d opened %q", x, opened)
		}
		opened = ""
	}
}
