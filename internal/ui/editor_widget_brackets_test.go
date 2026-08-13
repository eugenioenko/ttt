package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/view"
)

func bracketEditor(lines []string, line, col int) *EditorPaneWidget {
	buf := &buffer.Buffer{Lines: lines}
	cur := &cursor.Cursor{Line: line, Col: col}
	vp := &view.Viewport{TopLine: 0, LeftCol: 0, Width: 40, Height: 10}
	return NewEditorPaneWidget(buf, cur, vp)
}

func TestFindMatchingBracket(t *testing.T) {
	tests := []struct {
		name              string
		lines             []string
		line, col         int
		wantLine, wantCol int
		wantOK            bool
	}{
		{
			name:  "same line forward",
			lines: []string{"foo(bar)"},
			line:  0, col: 3,
			wantLine: 0, wantCol: 7, wantOK: true,
		},
		{
			name:  "same line backward from closer",
			lines: []string{"foo(bar)"},
			line:  0, col: 7,
			wantLine: 0, wantCol: 3, wantOK: true,
		},
		{
			name:  "multiline nested",
			lines: []string{"{", "  {", "  }", "}"},
			line:  0, col: 0,
			wantLine: 3, wantCol: 0, wantOK: true,
		},
		{
			name:  "skips over nested pair",
			lines: []string{"[[]]"},
			line:  0, col: 0,
			wantLine: 0, wantCol: 3, wantOK: true,
		},
		{
			name:  "unmatched returns false",
			lines: []string{"{ no closer"},
			line:  0, col: 0,
			wantOK: false,
		},
		{
			name:  "cursor not on a bracket",
			lines: []string{"plain text"},
			line:  0, col: 2,
			wantOK: false,
		},
		{
			name:  "spans blank lines",
			lines: []string{"(", "", "", ")"},
			line:  0, col: 0,
			wantLine: 3, wantCol: 0, wantOK: true,
		},
		{
			name:  "backward across blank lines",
			lines: []string{"(", "", "", ")"},
			line:  3, col: 0,
			wantLine: 0, wantCol: 0, wantOK: true,
		},
		{
			name:  "fullwidth runes counted as rune indices",
			lines: []string{"（｛[日本語]｝）"},
			line:  0, col: 2,
			wantLine: 0, wantCol: 6, wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := bracketEditor(tt.lines, tt.line, tt.col)
			gotLine, gotCol, gotOK := e.findMatchingBracket()
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if gotLine != tt.wantLine || gotCol != tt.wantCol {
				t.Errorf("match = (%d,%d), want (%d,%d)", gotLine, gotCol, tt.wantLine, tt.wantCol)
			}
		})
	}
}

// The cached result must not outlive the cursor move or the edit that
// invalidates it.
func TestFindMatchingBracketCacheInvalidation(t *testing.T) {
	e := bracketEditor([]string{"(a)", "[b]"}, 0, 0)

	if _, col, ok := e.findMatchingBracket(); !ok || col != 2 {
		t.Fatalf("initial scan = (%d, %v), want (2, true)", col, ok)
	}

	// Moving the cursor re-keys the cache.
	e.Cursor.Line, e.Cursor.Col = 1, 0
	line, col, ok := e.findMatchingBracket()
	if !ok || line != 1 || col != 2 {
		t.Fatalf("after cursor move = (%d,%d,%v), want (1,2,true)", line, col, ok)
	}

	// Editing the buffer under a stationary cursor must invalidate too.
	e.Buf.Lines[1] = "[bbbb]"
	e.bufferDirty = true
	e.FlushOnChange()
	if _, col, ok = e.findMatchingBracket(); !ok || col != 5 {
		t.Fatalf("after edit = (%d, %v), want (5, true)", col, ok)
	}

	// InvalidateBracketColors is the other invalidation hook (tab switch,
	// settings change) and must drop the match cache as well.
	e.Buf.Lines[1] = "[b]"
	e.InvalidateBracketColors()
	if _, col, ok = e.findMatchingBracket(); !ok || col != 2 {
		t.Fatalf("after invalidate = (%d, %v), want (2, true)", col, ok)
	}
}

// Guards the regression from issue #444: the scan must be linear in the
// characters it steps over, not in characters times line length.
func BenchmarkFindMatchingBracketLargeJSON(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 8000; i++ {
		sb.WriteString(`  "node_modules/some-package-name": { "version": "1.0.0", "resolved": "https://registry.npmjs.org/some/-/some-1.0.0.tgz" },`)
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	lines := strings.Split(sb.String(), "\n")

	e := bracketEditor(lines, 0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.bracketGen++ // defeat the cache so the scan itself is measured
		if _, _, ok := e.findMatchingBracket(); !ok {
			b.Fatal("expected a match")
		}
	}
}
