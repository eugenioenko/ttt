package highlight

import (
	"fmt"
	"testing"
)

func benchDiffSides(n int) (old, updated []string) {
	for i := range n {
		old = append(old, fmt.Sprintf("func f%d() {", i), fmt.Sprintf("\tx := %d // value", i), "}", "")
	}
	updated = append([]string(nil), old...)
	for i := 40; i < len(updated); i += 400 {
		updated[i] = "\t/* changed */ y := 2"
	}
	return old, updated
}

func BenchmarkDiffSidesTokenize(b *testing.B) {
	old, updated := benchDiffSides(500)
	for _, shared := range []bool{false, true} {
		name := "separate"
		if shared {
			name = "shared"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				a, c := New("main.go"), New("main.go")
				if shared {
					a.ShareTokens(nil)
					c.ShareTokens(a)
				}
				for i := range old {
					a.HighlightLineAt(old, i)
				}
				for i := range updated {
					c.HighlightLineAt(updated, i)
				}
			}
		})
	}
}

func BenchmarkDeepLineFirstAnswer(b *testing.B) {
	lines := commentedFile(4000)
	deep := len(lines) - 2
	b.Run("synchronous", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			New("main.go").HighlightLineAt(lines, deep)
		}
	})
	b.Run("deferred", func(b *testing.B) {
		recordWakes(b)
		b.ReportAllocs()
		for b.Loop() {
			h := New("main.go")
			h.HighlightLineAt(lines, deep)
			b.StopTimer()
			h.Close()
			b.StartTimer()
		}
	})
}
