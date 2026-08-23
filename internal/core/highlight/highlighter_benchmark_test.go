package highlight

import (
	"fmt"
	"testing"
)

var benchmarkSpans []Span

func BenchmarkHighlightLine(b *testing.B) {
	const line = `func render(value string, count int) string { return fmt.Sprintf("%s:%d", value, count) } // display`

	b.Run("cold", func(b *testing.B) {
		h := New("benchmark.go")
		b.ReportAllocs()
		for b.Loop() {
			h.ClearCache()
			benchmarkSpans = h.HighlightLine(line)
		}
	})

	b.Run("warm", func(b *testing.B) {
		h := New("benchmark.go")
		benchmarkSpans = h.HighlightLine(line)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			benchmarkSpans = h.HighlightLine(line)
		}
	})
}

func BenchmarkHighlightLineAtAfterInvalidation(b *testing.B) {
	for _, lineCount := range []int{50_000, 200_000} {
		b.Run(fmt.Sprintf("lines=%d/edit=end", lineCount), func(b *testing.B) {
			lines := benchmarkLines(lineCount)
			h := New("benchmark.go")
			last := len(lines) - 1
			benchmarkSpans = h.HighlightLineAt(lines, last)
			variants := [2]string{"var tail = 1", "var tail = 2"}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				lines[last] = variants[i&1]
				h.ClearCache()
				benchmarkSpans = h.HighlightLineAt(lines, last)
			}
		})

		b.Run(fmt.Sprintf("lines=%d/edit=start", lineCount), func(b *testing.B) {
			lines := benchmarkLines(lineCount)
			h := New("benchmark.go")
			last := len(lines) - 1
			benchmarkSpans = h.HighlightLineAt(lines, last)
			variants := [2]string{"/* open", "var head = 1"}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				lines[0] = variants[i&1]
				h.ClearCache()
				benchmarkSpans = h.HighlightLineAt(lines, last)
			}
		})
	}
}

func benchmarkLines(count int) []string {
	lines := make([]string, count)
	for i := range lines {
		lines[i] = "var value = 1"
	}
	return lines
}
