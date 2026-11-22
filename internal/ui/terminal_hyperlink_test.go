package ui

import (
	"os"
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

func TestTerminalHyperlinkUsesOuterTerminal(t *testing.T) {
	for _, end := range []string{"\a", "\x1b\\"} {
		for _, target := range []string{"https://target.example", "file:///tmp/source.go", "javascript:alert(1)"} {
			t.Run(target+end, func(t *testing.T) {
				tw, write, render := hyperlinkWidget(t, 40, 3)
				write("界 " + osc8(target, "https://label.example", end) + " plain")
				tw.OnOpenFile = func(string, int, int) { t.Error("ttt opened an OSC 8 file") }
				tw.SuperShortcuts = true
				want := target
				if strings.HasPrefix(target, "javascript:") {
					want = ""
				}
				for _, mod := range []tcell.ModMask{tcell.ModNone, tcell.ModCtrl, tcell.ModMeta} {
					tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.ButtonNone, mod))
					if tw.PointerShapeAt(6, 2) == term.PointerHand {
						t.Fatal("ttt supplied an OSC 8 hover pointer")
					}
					cells := render()
					for x := 3; x < 24; x++ {
						if got := cells[0][x].Link.URL(); got != want {
							t.Fatalf("forwarded URL at %d=%q, want %q", x, got, want)
						}
						if want != "" && cells[0][x].Link.ID() == "" {
							t.Fatal("forwarded link lost its ID")
						}
						if cells[0][x].Attrs&term.CellAttrUnderline != 0 {
							t.Fatal("ttt added an OSC 8 underline")
						}
					}
					if cells[0][24].Link.URL() != "" {
						t.Fatal("link leaked into plain text")
					}
					tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.Button1, mod))
					tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.ButtonNone, mod))
					tw.ClearSelection()
				}
			})
		}
	}
}

func TestTerminalHyperlinkRejectsInvalidTargets(t *testing.T) {
	for _, raw := range []string{"file:///tmp/a%00b", "https:///missing-host", "https://example.com/\x1b", "javascript:alert(1)", "/relative"} {
		if got := terminalLinkURI(raw); got != nil {
			t.Errorf("URI(%q)=%+v", raw, got)
		}
	}
}

func TestTerminalHyperlinkUsesCurrentTarget(t *testing.T) {
	tw, write, render := hyperlinkWidget(t, 40, 2)
	tw.linkModifierHeld = true
	label := "https://label.example"
	for _, target := range []string{"https://first.example", "https://second.example"} {
		write("\r" + osc8(target, label, "\a"))
		if got := render()[0][0].Link.URL(); got != target {
			t.Fatalf("rendered URL=%q, want %q", got, target)
		}
		if tw.linkAt(0, 2) != nil {
			t.Fatal("OSC 8 label became a ttt link")
		}
	}
	write("\r" + label)
	cells := render()
	if cells[0][0].Link.URL() != label {
		t.Fatal("overwrite did not replace explicit target with detected URL")
	}
	if cells[0][0].Attrs&term.CellAttrUnderline != 0 {
		t.Fatal("ttt added a plain URL hover underline")
	}
	if tw.linkAt(0, 2) != nil {
		t.Fatal("plain URL became an editor link")
	}
	write("\r\x1b[2Kplain")
	if render()[0][0].Link.URL() != "" {
		t.Fatal("erased URL retained hyperlink")
	}
}

func TestTerminalHyperlinkPreservesExplicitUnderline(t *testing.T) {
	for _, style := range []string{"4", "4:3", "21"} {
		t.Run(style, func(t *testing.T) {
			_, write, render := hyperlinkWidget(t, 40, 2)
			write(osc8("https://example.com", "\x1b["+style+"munderlined\x1b[24m plain", "\a"))
			cells := render()
			if cells[0][0].Attrs&term.CellAttrUnderline == 0 {
				t.Fatal("explicit SGR underline lost")
			}
			if cells[0][11].Attrs&term.CellAttrUnderline != 0 {
				t.Fatal("SGR 24 did not clear underline")
			}
			if cells[0][0].Link.URL() == "" || cells[0][11].Link.URL() == "" {
				t.Fatal("SGR changed hyperlink metadata")
			}
		})
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
	if tw.HandleEvent(tcell.NewEventMouse(4, 2, tcell.Button1, tcell.ModCtrl)) != EventCaptured {
		t.Fatal("scrollback link click intercepted by ttt")
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

func TestTerminalHyperlinkFileLabel(t *testing.T) {
	tw, write, render := hyperlinkWidget(t, 80, 2)
	tw.WorkDir = t.TempDir()
	path := filepath.Join(tw.WorkDir, "source.go")
	if err := os.WriteFile(path, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	label := "source.go:12:3"
	write(osc8("file://remote-host/tmp/other.go", label, "\a") + " " + label)
	tw.OnOpenFile = func(string, int, int) { t.Error("ttt opened an OSC 8 file label") }
	tw.HandleEvent(tcell.NewEventMouse(3, 2, tcell.Button1, tcell.ModCtrl))
	tw.HandleEvent(tcell.NewEventMouse(3, 2, tcell.ButtonNone, tcell.ModCtrl))
	tw.ClearSelection()
	cells := render()
	if cells[0][0].Link.URL() != "file://remote-host/tmp/other.go" {
		t.Fatal("remote file target lost")
	}
	if cells[0][0].Attrs&term.CellAttrUnderline != 0 {
		t.Fatal("OSC 8 file label got fallback underline")
	}
	opened := false
	tw.OnOpenFile = func(got string, line, col int) {
		opened = true
		if got != path || line != 12 || col != 3 {
			t.Errorf("plain file opened %q:%d:%d", got, line, col)
		}
	}
	tw.HandleEvent(tcell.NewEventMouse(18, 2, tcell.Button1, tcell.ModCtrl))
	if !opened {
		t.Fatal("plain file no longer opens")
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
	tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.Button1, tcell.ModCtrl))
	if tw.PointerShapeAt(6, 2) == term.PointerHand {
		t.Fatal("clipped blank still acts as a link")
	}
	for _, x := range []int{3, 4} {
		tw.ClearSelection()
		if tw.HandleEvent(tcell.NewEventMouse(x, 3, tcell.Button1, tcell.ModCtrl)) != EventCaptured {
			t.Fatalf("interior wide cell at %d intercepted by ttt", x)
		}
	}
}

func TestTerminalPlainURLsForwarded(t *testing.T) {
	tw, write, render := hyperlinkWidget(t, 80, 3)
	write("界 https://example.com/界, http://other.example/path. plain")
	for _, mod := range []tcell.ModMask{tcell.ModNone, tcell.ModCtrl, tcell.ModMeta} {
		tw.SuperShortcuts = true
		tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.ButtonNone, mod))
		cells := render()
		for _, pos := range []int{3, 23, 24} {
			if got := cells[0][pos].Link.URL(); got != "https://example.com/界" {
				t.Fatalf("URL at %d=%q", pos, got)
			}
		}
		if cells[0][2].Link.URL() != "" || cells[0][25].Link.URL() != "" {
			t.Fatal("hyperlink included prefix or punctuation")
		}
		if got := cells[0][27].Link.URL(); got != "http://other.example/path" {
			t.Fatalf("second URL=%q", got)
		}
		if cells[0][3].Attrs&term.CellAttrUnderline != 0 || tw.PointerShapeAt(6, 2) == term.PointerHand {
			t.Fatal("ttt supplied web link hover")
		}
		if got := tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.Button1, mod)); got != EventCaptured {
			t.Fatalf("URL click handled locally: %v", got)
		}
		tw.HandleEvent(tcell.NewEventMouse(6, 2, tcell.ButtonNone, mod))
		tw.ClearSelection()
	}
}

func TestTerminalPlainURLWrapAndScrollback(t *testing.T) {
	tw, write, render := hyperlinkWidget(t, 16, 3)
	url := "https://example.com/a-long-path"
	write("prefix " + url + " suffix")
	cells := render()
	for _, pos := range [][2]int{{0, 7}, {0, 15}, {1, 0}, {1, 15}, {2, 0}} {
		if got := cells[pos[0]][pos[1]].Link.URL(); got != url {
			t.Fatalf("wrapped URL at %v=%q", pos, got)
		}
	}
	write("\r\nnext\r\nlast")
	tw.scrollOffset = 2
	if got := render()[0][7].Link.URL(); got != url {
		t.Fatalf("scrollback URL=%q", got)
	}
	tw.scrollOffset = 0
	write("\x1b[?1049h\x1b[Hhttps://alt.example")
	if got := render()[0][0].Link.URL(); got != "https://alt.example" {
		t.Fatalf("alternate URL=%q", got)
	}
}

func TestTerminalPlainURLHardNewline(t *testing.T) {
	_, write, render := hyperlinkWidget(t, 40, 3)
	write("https://example.com\r\n/not-part-of-the-url")
	cells := render()
	if cells[0][0].Link.URL() != "https://example.com" || cells[1][0].Link.URL() != "" {
		t.Fatal("URL detection crossed hard newline")
	}
}

func TestTerminalPlainURLWideWrapPadding(t *testing.T) {
	_, write, render := hyperlinkWidget(t, 21, 3)
	url := "https://example.com/界/last"
	write(url)
	cells := render()
	if got := cells[0][0].Link.URL(); got != url {
		t.Fatalf("first row target=%q, want %q", got, url)
	}
	if got := cells[1][0].Link.URL(); got != url {
		t.Fatalf("wrapped row target=%q, want %q", got, url)
	}
	if cells[0][20].Link.URL() != "" {
		t.Fatal("wide-wrap padding is clickable")
	}
}

func TestTerminalPlainURLCombiningCharacters(t *testing.T) {
	_, write, render := hyperlinkWidget(t, 60, 2)
	url := "https://example.com/e\u0301"
	write("e\u0301 " + url + " plain")
	cells := render()
	if cells[0][1].Link.URL() != "" {
		t.Fatal("prefix space became a hyperlink")
	}
	for _, col := range []int{2, 22} {
		if got := cells[0][col].Link.URL(); got != url {
			t.Fatalf("URL at %d=%q", col, got)
		}
	}
	if cells[0][23].Link.URL() != "" {
		t.Fatal("hyperlink leaked past combining character")
	}
}
