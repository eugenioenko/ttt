package highlight

import (
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"strings"
	"testing"
	"time"
)

func TestStringAtInsertionPoint(t *testing.T) {
	for _, tc := range []struct {
		name, file, text string
		line, col        int
		inside           bool
	}{
		{"code", "a.go", "var x = ", 0, 8, false},
		{"before opening quote", "a.go", `var s = "text"`, 0, 8, false},
		{"after opening quote", "a.go", `var s = "text"`, 0, 9, true},
		{"before closing quote", "a.go", `var s = "text"`, 0, 13, true},
		{"after closing quote", "a.go", `var s = "text"`, 0, 14, false},
		{"empty string", "a.go", `""`, 0, 1, true},
		{"unterminated string", "a.go", `var s = "`, 0, 9, true},
		{"escaped quote", "a.go", `"a\" b"`, 0, 4, true},
		{"unicode string", "a.go", `"界🙂 "`, 0, 3, true},
		{"multiline raw string", "a.go", "var s = `\nhello\n`", 1, 0, true},
		{"after raw string", "a.go", "var s = `\nhello\n`", 2, 1, false},
		{"python multiline", "a.py", "s = \"\"\"\nhello\n\"\"\"", 1, 0, true},
		{"javascript template", "a.js", "const x = `\nhello\n`;", 1, 0, true},
		{"comment", "a.go", "// \"hello", 0, 9, false},
		{"comment quote does not leak", "a.go", "// \"\nvar x = ", 1, 8, false},
		{"json string", "a.json", `{"key": "value"}`, 0, 9, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := New(tc.file)
			t.Cleanup(h.Close)
			b := &buffer.Buffer{Lines: strings.Split(tc.text, "\n")}
			inside, known := h.StringAt(b, tc.line, tc.col)
			if !known || inside != tc.inside {
				t.Fatalf("StringAt = %v,%v, want %v,true", inside, known, tc.inside)
			}
		})
	}
}

func TestStringAtIgnoresThemeAndTracksEdits(t *testing.T) {
	h := New("a.go")
	defer h.Close()
	b := &buffer.Buffer{Lines: []string{`"a "`}}
	SetTokenTheme(NewTokenTheme([]TokenRule{{Selector: "string", Foreground: "#ff0000"}}))
	defer SetTokenTheme(nil)
	if inside, known := h.StringAt(b, 0, 2); !inside || !known {
		t.Fatal("themed string not detected")
	}
	b.SetLine(0, "x = ")
	if inside, known := h.StringAt(b, 0, 4); inside || !known {
		t.Fatal("context did not follow buffer edit")
	}
}

func TestStringAtUnknownContextIsConservative(t *testing.T) {
	h := New("a.go")
	defer h.Close()
	b := &buffer.Buffer{Lines: []string{"x"}}
	for _, p := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 2}} {
		if _, known := h.StringAt(b, p[0], p[1]); known {
			t.Fatalf("invalid position %v has context", p)
		}
	}
	SetStepScheduler(func() {})
	defer SetStepScheduler(nil)
	b.SetLines(strings.Split(strings.Repeat("// line\n", 1000), "\n"))
	if _, known := h.StringAt(b, 900, 0); known {
		t.Fatal("deferred line must not claim accurate context")
	}
	if got := h.doc.MaterializedLines(); got > DefaultSyncLines+1 {
		t.Fatalf("context synchronously tokenized %d lines", got)
	}
	for Step(time.Second) {
	}
	if inside, known := h.StringAt(b, 900, 0); inside || !known {
		t.Fatal("settled context not recognized")
	}
}

func TestStringAtLongPrefixIsUnknown(t *testing.T) {
	h := New("a.go")
	defer h.Close()
	b := &buffer.Buffer{Lines: []string{strings.Repeat("a", 20001)}}
	if _, known := h.StringAt(b, 0, 20001); known {
		t.Fatal("tokenization beyond the line limit must be unknown")
	}
}
