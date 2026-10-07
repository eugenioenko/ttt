package e2e

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

// BenchmarkEditor drives the whole app through the simulated screen, so every
// operation includes event handling, highlighting, and a full frame render.
// Sub-benchmarks are named <language>/<scenario>; select one language with
// -bench 'Editor/ts/'. -short shrinks the large file so CI stays fast.

var benchLanguages = []struct{ name, fixture string }{
	{"go", "source.go"},
	{"ts", "client.ts"},
	{"cpp", "matrix.cpp"},
	{"python", "inventory.py"},
}

const (
	benchWidth  = 200
	benchHeight = 50
	mediumLines = 1_500
)

func largeLines() int {
	if testing.Short() {
		return 2_000
	}
	return 20_000
}

// repeatToLines repeats src until it has at least n lines. Each copy's lines
// end in a different run of spaces and tabs: identical lines would be served
// from the tokenizer's cache, which real files rarely allow.
func repeatToLines(src []byte, n int) []byte {
	lines := bytes.Split(bytes.TrimSuffix(src, []byte("\n")), []byte("\n"))
	var out bytes.Buffer
	for copyIndex := 0; copyIndex*len(lines) < n; copyIndex++ {
		var marker []byte
		for bits := copyIndex; bits > 0; bits >>= 1 {
			marker = append(marker, " \t"[bits&1])
		}
		for _, line := range lines {
			out.Write(line)
			out.Write(marker)
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

func newBenchHarness(b *testing.B) *testHarness {
	b.Helper()
	h := newTestHarness(b, benchWidth, benchHeight)
	b.Cleanup(h.stop)
	return h
}

func openBenchFile(b *testing.B, h *testHarness, name string, content []byte) {
	b.Helper()
	path := filepath.Join(h.dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		b.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()
}

func gotoLine(h *testHarness, line int) {
	ed := h.app.EditorGroup.Editor
	ed.Cursor.Line = min(line, len(ed.Buf.Lines)-1)
	ed.Cursor.Col = 0
	ed.EnsureCursorVisible()
	h.redraw()
}

func atLastLine(h *testHarness) bool {
	ed := h.app.EditorGroup.Editor
	return ed.Cursor.Line == len(ed.Buf.Lines)-1
}

func BenchmarkEditor(b *testing.B) {
	for _, lang := range benchLanguages {
		src, err := os.ReadFile(filepath.Join("testdata", "bench", lang.fixture))
		if err != nil {
			b.Fatal(err)
		}
		medium := repeatToLines(src, mediumLines)
		large := repeatToLines(src, largeLines())

		b.Run(lang.name, func(b *testing.B) {
			b.Run("redraw", func(b *testing.B) {
				h := newBenchHarness(b)
				openBenchFile(b, h, lang.fixture, medium)
				b.ReportAllocs()
				for b.Loop() {
					h.redraw()
				}
			})

			b.Run("cursor_down", func(b *testing.B) {
				h := newBenchHarness(b)
				openBenchFile(b, h, lang.fixture, medium)
				b.ReportAllocs()
				for b.Loop() {
					if atLastLine(h) {
						gotoLine(h, 0)
					}
					h.pressKey(tcell.KeyDown, tcell.ModNone)
				}
			})

			b.Run("page_down", func(b *testing.B) {
				h := newBenchHarness(b)
				openBenchFile(b, h, lang.fixture, medium)
				b.ReportAllocs()
				for b.Loop() {
					if atLastLine(h) {
						gotoLine(h, 0)
					}
					h.pressKey(tcell.KeyPgDn, tcell.ModNone)
				}
			})

			b.Run("wheel_scroll", func(b *testing.B) {
				h := newBenchHarness(b)
				openBenchFile(b, h, lang.fixture, medium)
				b.ReportAllocs()
				for b.Loop() {
					h.app.Root.HandleEvent(tcell.NewEventMouse(benchWidth/2, benchHeight/2, tcell.WheelDown, tcell.ModNone))
					h.redraw()
					if ed := h.app.EditorGroup.Editor; ed.Viewport.TopLine+ed.Viewport.Height >= len(ed.Buf.Lines) {
						gotoLine(h, 0)
					}
				}
			})

			b.Run("type_and_delete", func(b *testing.B) {
				h := newBenchHarness(b)
				openBenchFile(b, h, lang.fixture, medium)
				gotoLine(h, mediumLines/2)
				h.pressKey(tcell.KeyEnd, tcell.ModNone)
				b.ReportAllocs()
				for b.Loop() {
					h.pressRune('x')
					h.pressKey(tcell.KeyBackspace2, tcell.ModNone)
				}
			})

			// The first file of a language also compiles its grammar.
			b.Run("open_large_first", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					b.StopTimer()
					h := newTestHarness(b, benchWidth, benchHeight)
					b.StartTimer()
					openBenchFile(b, h, lang.fixture, large)
					b.StopTimer()
					h.stop()
					b.StartTimer()
				}
			})

			b.Run("open_large", func(b *testing.B) {
				h := newBenchHarness(b)
				openBenchFile(b, h, lang.fixture, large)
				h.exec("tab.close")
				b.ReportAllocs()
				for b.Loop() {
					openBenchFile(b, h, lang.fixture, large)
					b.StopTimer()
					h.exec("tab.close")
					b.StartTimer()
				}
			})

			// The first render at the end highlights every line above it.
			b.Run("jump_to_end_large", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					b.StopTimer()
					h := newTestHarness(b, benchWidth, benchHeight)
					openBenchFile(b, h, lang.fixture, large)
					b.StartTimer()
					gotoLine(h, largeLines())
					b.StopTimer()
					h.stop()
					b.StartTimer()
				}
			})

			// An edit at the top invalidates highlighting below it until the
			// tokenizer state converges with the cached tail.
			b.Run("edit_top_view_end_large", func(b *testing.B) {
				h := newBenchHarness(b)
				openBenchFile(b, h, lang.fixture, large)
				gotoLine(h, largeLines())
				b.ReportAllocs()
				for b.Loop() {
					gotoLine(h, 0)
					h.pressRune('/')
					gotoLine(h, largeLines())
				}
			})
		})
	}
}
