package app

import (
	"os"
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/lsp"
)

// Call on the UI thread; async requests keep this immutable text snapshot.
func (a *App) lspLines(path string) []string {
	if buf := a.EditorGroup.BufferForPath(path); buf != nil {
		return slices.Clone(buf.Lines)
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
}

func (a *App) lspColumn(path string, line, col int) int {
	return lsp.PositionToUTF16(a.lspLines(path), lsp.Position{Line: line, Character: col}).Character
}

func runeTextEdits(lines []string, edits []lsp.TextEdit) []lsp.TextEdit {
	result := slices.Clone(edits)
	for i := range result {
		result[i].Range = lsp.RangeToRunes(lines, result[i].Range)
	}
	return result
}

func runeSymbols(lines []string, symbols []lsp.DocumentSymbol) {
	for i := range symbols {
		symbols[i].Range = lsp.RangeToRunes(lines, symbols[i].Range)
		symbols[i].SelectionRange = lsp.RangeToRunes(lines, symbols[i].SelectionRange)
		runeSymbols(lines, symbols[i].Children)
	}
}
