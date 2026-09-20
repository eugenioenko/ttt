package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/gitpod-io/xterm-go"
)

func BenchmarkTerminalRender(b *testing.B) {
	for _, size := range []struct{ cols, rows int }{{120, 40}, {200, 50}} {
		b.Run(fmt.Sprintf("%dx%d", size.cols, size.rows), func(b *testing.B) {
			t, err := terminal.New("/bin/cat", size.cols, size.rows, 1000, nil, "")
			if err != nil {
				b.Fatal(err)
			}
			defer t.Close()

			var sb strings.Builder
			for i := 0; i < size.rows*3; i++ {
				fmt.Fprintf(&sb, "\x1b[3%dmline %d \x1b[0m%s\r\n", i%8, i, strings.Repeat("abcdefghij ", size.cols/11))
			}
			t.Snapshot(func(xt *xterm.Terminal) { xt.WriteString(sb.String()) })

			tw := NewTerminalWidget(t, &TerminalColorPalette{})
			tw.SetRect(Rect{W: size.cols, H: size.rows})
			cells := make([][]term.Cell, size.rows)
			for y := range cells {
				cells[y] = make([]term.Cell, size.cols)
			}
			surface := NewRenderSurface(cells, Rect{W: size.cols, H: size.rows})

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				tw.Render(surface)
			}
		})
	}
}

func TestLineCellKeepsReplacementCharInCombinedCell(t *testing.T) {
	xt := xterm.New(xterm.WithCols(10), xterm.WithRows(2))
	xt.WriteString("�́a")
	buf := xt.Buffer()
	bl := buf.Lines.Get(buf.YBase)

	tw := &TerminalWidget{Palette: &TerminalColorPalette{}}
	cd := xterm.NewCellData()
	if got := tw.lineCell(bl, 0, cd).Ch; got != '�' {
		t.Errorf("combined cell rune = %U, want U+FFFD", got)
	}
	if got := tw.lineCell(bl, 1, cd).Ch; got != 'a' {
		t.Errorf("next cell rune = %q, want 'a'", got)
	}
}
