package e2e

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

const logBenchLineBytes = 256

func logBenchFile(b *testing.B, dir string, sizeMiB int) string {
	b.Helper()
	path := filepath.Join(dir, fmt.Sprintf("log-%dMiB.log", sizeMiB))
	size := int64(sizeMiB) * 1024 * 1024
	if info, err := os.Stat(path); err == nil && info.Size() == size {
		return path
	}
	file, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	line := fmt.Sprintf("%-*s\n", logBenchLineBytes-1, "2026-10-12T12:00:00.000Z INFO service=api request_id=0123456789abcdef message=\"Processed request\" status=200 latency_ms=12 path=/api/events payload="+strings.Repeat("abcdefgh", 8))
	if len(line) != logBenchLineBytes {
		b.Fatal("log entry exceeds its fixed byte length")
	}
	block := bytes.Repeat([]byte(line), 1024*1024/logBenchLineBytes)
	for range sizeMiB {
		if _, err := file.Write(block); err != nil {
			b.Fatal(err)
		}
	}
	return path
}

func openLogBenchFile(b *testing.B, h *testHarness, path string, lines int) {
	b.Helper()
	h.app.EditorGroup.OpenFile(path)
	h.redraw()
	if len(h.app.EditorGroup.Editor.Buf.Lines) < lines {
		b.Fatalf("log did not open: want at least %d lines", lines)
	}
}

func BenchmarkLog(b *testing.B) {
	// CI shares generated files across base/PR passes; -short only skips
	// the manual 1 GiB case, keeping the smaller files at their real sizes.
	dir := os.Getenv("TTT_BENCH_LOG_DIR")
	if dir == "" {
		dir = b.TempDir()
	}
	sizes := []int{1, 128, 256}
	if !testing.Short() {
		sizes = append(sizes, 1024)
	}
	for _, sizeMiB := range sizes {
		b.Run(fmt.Sprintf("%dMiB", sizeMiB), func(b *testing.B) {
			path := logBenchFile(b, dir, sizeMiB)
			lines := sizeMiB * 1024 * 1024 / logBenchLineBytes
			b.Run("open", func(b *testing.B) {
				h := newBenchHarness(b)
				b.ReportAllocs()
				b.SetBytes(int64(sizeMiB) * 1024 * 1024)
				for b.Loop() {
					openLogBenchFile(b, h, path, lines)
					b.StopTimer()
					h.exec("tab.close")
					b.StartTimer()
				}
			})
			b.Run("wheel_scroll", func(b *testing.B) {
				h := newBenchHarness(b)
				openLogBenchFile(b, h, path, lines)
				anchors := []int{0, lines / 2, lines - 1000}
				step := 0
				b.ReportAllocs()
				for b.Loop() {
					b.StopTimer()
					gotoLine(h, anchors[step%len(anchors)])
					top := h.app.EditorGroup.Editor.Viewport.TopLine
					b.StartTimer()
					h.app.Root.HandleEvent(tcell.NewEventMouse(benchWidth/2, benchHeight/2, tcell.WheelDown, tcell.ModNone))
					h.redraw()
					b.StopTimer()
					if h.app.EditorGroup.Editor.Viewport.TopLine <= top {
						b.Fatal("wheel event did not scroll the log")
					}
					step++
					b.StartTimer()
				}
			})
		})
	}
}
