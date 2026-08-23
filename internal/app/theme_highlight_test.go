package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestCachedHighlightStyleFollowsLiveStyleMap(t *testing.T) {
	h := highlight.New("main.go")
	line := "func main() {}"
	spans := h.HighlightLine(line)
	keyword := highlightStyleAt(spans, 0)
	if keyword != term.StyleSyntaxKeyword {
		t.Fatalf("func style = %v, want syntax keyword", keyword)
	}

	firstTheme := config.DefaultTheme()
	firstTheme.Syntax.Keyword = config.StyleDef{Fg: "#112233"}
	firstMap := BuildStyleMap(firstTheme)
	if got := firstMap[keyword].GetForeground(); got != tcell.GetColor("#112233") {
		t.Fatalf("first keyword foreground = %v, want #112233", got)
	}

	secondTheme := config.DefaultTheme()
	secondTheme.Syntax.Keyword = config.StyleDef{Fg: "#abcdef"}
	secondMap := BuildStyleMap(secondTheme)
	cachedKeyword := highlightStyleAt(h.HighlightLine(line), 0)
	if cachedKeyword != keyword {
		t.Fatalf("cached keyword style = %v, want stable style slot %v", cachedKeyword, keyword)
	}
	if got := secondMap[cachedKeyword].GetForeground(); got != tcell.GetColor("#abcdef") {
		t.Fatalf("cached keyword foreground after style-map change = %v, want #abcdef", got)
	}
}

func highlightStyleAt(spans []highlight.Span, col int) term.Style {
	for _, span := range spans {
		if col >= span.Start && col < span.End {
			return span.Style
		}
	}
	return term.StyleDefault
}
