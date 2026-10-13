package highlight

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
)

func TestMergedEditFoldsChangesIntoOneSplice(t *testing.T) {
	tests := []struct {
		name                  string
		edit                  func(b *buffer.Buffer)
		start, removed, added int
	}{
		{"type on one line", func(b *buffer.Buffer) { b.SetLine(3, "x") }, 3, 1, 1},
		{"insert a line", func(b *buffer.Buffer) { b.Splice(2, 0, "new") }, 2, 0, 1},
		{"delete a line", func(b *buffer.Buffer) { b.Splice(5, 1) }, 5, 1, 0},
		{"edits on two lines", func(b *buffer.Buffer) { b.SetLine(2, "a"); b.SetLine(6, "b") }, 2, 5, 5},
		{"insert above an edit", func(b *buffer.Buffer) { b.SetLine(6, "a"); b.Splice(1, 0, "new") }, 1, 6, 7},
		{"delete below an edit", func(b *buffer.Buffer) { b.SetLine(1, "a"); b.Splice(8, 2) }, 1, 9, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &buffer.Buffer{}
			b.SetLines([]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"})
			version := b.Version()
			tt.edit(b)
			start, removed, added, ok := mergedEdit(b, version, 10)
			if !ok || start != tt.start || removed != tt.removed || added != tt.added {
				t.Fatalf("mergedEdit = (%d, %d, %d, %v), want (%d, %d, %d, true)", start, removed, added, ok, tt.start, tt.removed, tt.added)
			}
		})
	}
}

func TestSyncBufferMatchesFreshHighlighting(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	choices := []string{"x := 1", "/* open", "close */", `s := "a`, `b"`, "func f() {", "}", "", "// note"}
	b := &buffer.Buffer{}
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = choices[random.Intn(len(choices))]
	}
	b.SetLines(lines)
	h := New("a.go")
	h.sync = true
	h.SyncBuffer(b)

	for iteration := range 300 {
		for range 1 + random.Intn(3) {
			at := random.Intn(len(b.Lines))
			switch random.Intn(3) {
			case 0:
				b.SetLine(at, choices[random.Intn(len(choices))])
			case 1:
				b.Splice(at, 0, choices[random.Intn(len(choices))])
			default:
				if len(b.Lines) > 1 {
					b.Splice(at, 1)
				}
			}
		}
		h.SyncBuffer(b)
		if iteration%5 != 0 {
			continue
		}
		fresh := New("a.go")
		fresh.sync = true
		for i := range b.Lines {
			if got, want := h.HighlightLineAt(b.Lines, i), fresh.HighlightLineAt(b.Lines, i); !reflect.DeepEqual(got, want) {
				t.Fatalf("edit %d line %d %q: spans %v, want %v", iteration, i, b.Lines[i], got, want)
			}
		}
	}
}
