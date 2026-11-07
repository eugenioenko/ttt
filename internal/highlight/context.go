package highlight

import (
	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/ttt/internal/core/buffer"
)

func (h *Highlighter) StringAt(buf *buffer.Buffer, line, col int) (inside, known bool) {
	if line < 0 || line >= len(buf.Lines) {
		return false, false
	}
	runes := []rune(buf.Lines[line])
	if col < 0 || col > len(runes) {
		return false, false
	}
	h.SyncBuffer(buf)
	h.HighlightLineAt(buf.Lines, line)
	// A deferred render's isolated spans cannot establish multiline string context.
	if h.doc.MaterializedLines() <= line {
		return false, false
	}
	state, ok := h.doc.StateAt(line)
	if !ok {
		return false, false
	}
	// Probe the insertion point, rather than the quote delimiter on either side.
	res := h.grammar.TokenizeLineWithOptions(string(runes[:col])+"x", state, tokenizeOptions)
	if res.Stopped {
		return false, false
	}
	for _, tok := range res.Tokens {
		if tok.Start <= col && col < tok.End {
			return tok.Category() == textmate.TokenCategoryString, true
		}
	}
	return false, false
}
