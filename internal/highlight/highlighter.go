package highlight

import (
	"github.com/eugenioenko/ttt/internal/term"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

type Span struct {
	Start int
	End   int
	Style term.Style
}

// spanKey keys the span cache; the same text differs by incoming state.
type spanKey struct {
	line    string
	inBlock bool
}

type Highlighter struct {
	lexer chroma.Lexer
	cache map[spanKey][]Span

	// Empty when the language has no block comment; state tracking is then skipped.
	blockOpen  string
	blockClose string

	// Rune index where a line opens an unclosed block comment, -1 for none.
	// Pure function of the text, so it survives ClearCache.
	opens map[string]int

	// states[i] reports whether line i starts inside a block comment.
	states []bool
	// stateSrc[i] is the text of line i that produced states[i+1], so an edit
	// can be located by comparison instead of discarding the whole table.
	stateSrc []string
	// Set by ClearCache; the table is revalidated on the next lookup.
	statesDirty bool
}

func New(filename string) *Highlighter {
	lexer := lexers.Match(filename)
	if lexer == nil {
		return nil
	}
	lexer = chroma.Coalesce(lexer)
	h := &Highlighter{lexer: lexer}
	h.blockOpen, h.blockClose = detectBlockComment(lexer)
	return h
}

func (h *Highlighter) Language() string {
	return h.lexer.Config().Name
}

// HighlightLine highlights a line in isolation, ignoring multiline comments.
func (h *Highlighter) HighlightLine(line string) []Span {
	return h.highlight(line, false)
}

// HighlightLineAt highlights lines[idx], carrying block comment state down
// from the top of the buffer.
func (h *Highlighter) HighlightLineAt(lines []string, idx int) []Span {
	if idx < 0 || idx >= len(lines) {
		return nil
	}
	return h.highlight(lines[idx], h.stateAt(lines, idx))
}

func (h *Highlighter) ClearCache() {
	h.cache = make(map[spanKey][]Span)
	h.statesDirty = true
}

func (h *Highlighter) highlight(line string, inBlock bool) []Span {
	key := spanKey{line: line, inBlock: inBlock}
	if h.cache != nil {
		if cached, ok := h.cache[key]; ok {
			return cached
		}
	}
	spans := h.computeSpans(line, inBlock)
	if h.cache == nil {
		h.cache = make(map[spanKey][]Span)
	}
	h.cache[key] = spans
	return spans
}

func (h *Highlighter) computeSpans(line string, inBlock bool) []Span {
	if inBlock {
		end := h.closesAt(line)
		if end < 0 {
			if line == "" {
				return nil
			}
			return []Span{{Start: 0, End: len([]rune(line)), Style: term.StyleSyntaxComment}}
		}
		spans := []Span{{Start: 0, End: end, Style: term.StyleSyntaxComment}}
		for _, s := range h.computeSpans(string([]rune(line)[end:]), false) {
			spans = append(spans, Span{Start: s.Start + end, End: s.End + end, Style: s.Style})
		}
		return spans
	}

	spans := h.lexLine(line)
	open := h.opensAt(line)
	if open < 0 {
		return spans
	}
	// Comment opens here and runs past end of line. Fresh slice: spans is cached.
	out := make([]Span, 0, len(spans)+1)
	for _, s := range spans {
		if s.End <= open {
			out = append(out, s)
		} else if s.Start < open {
			out = append(out, Span{Start: s.Start, End: open, Style: s.Style})
		}
	}
	return append(out, Span{Start: open, End: len([]rune(line)), Style: term.StyleSyntaxComment})
}

func (h *Highlighter) lexLine(line string) []Span {
	iter, err := h.lexer.Tokenise(nil, line+"\n")
	if err != nil {
		return nil
	}
	var spans []Span
	pos := 0
	for _, tok := range iter.Tokens() {
		text := strings.TrimRight(tok.Value, "\n")
		if text == "" {
			continue
		}
		runeLen := len([]rune(text))
		style := mapTokenType(tok.Type)
		if style != term.StyleDefault {
			spans = append(spans, Span{
				Start: pos,
				End:   pos + runeLen,
				Style: style,
			})
		}
		pos += runeLen
	}
	return spans
}

// stateAt reports whether lines[idx] starts inside a block comment, extending
// the state table as needed. Transitions are memoized, so this is a map lookup
// per line rather than a re-lex.
func (h *Highlighter) stateAt(lines []string, idx int) bool {
	if h.blockOpen == "" || idx <= 0 {
		return false
	}
	if h.statesDirty {
		h.statesDirty = false
		h.truncateToEdit(lines)
	}
	if len(h.states) == 0 {
		h.states = append(h.states, false)
	}
	for len(h.states) <= idx && len(h.states) <= len(lines) {
		i := len(h.states) - 1
		h.states = append(h.states, h.nextState(lines[i], h.states[i]))
		h.stateSrc = append(h.stateSrc, lines[i])
	}
	if idx < len(h.states) {
		return h.states[idx]
	}
	return false
}

// truncateToEdit drops the state table from the first line whose text changed,
// keeping everything above it. Comparing strings that were never rewritten hits
// Go's identical-pointer fast path, so unchanged lines cost no scanning.
func (h *Highlighter) truncateToEdit(lines []string) {
	keep := min(len(h.stateSrc), len(lines))
	for i := 0; i < keep; i++ {
		if h.stateSrc[i] != lines[i] {
			keep = i
			break
		}
	}
	h.stateSrc = h.stateSrc[:keep]
	if len(h.states) > keep+1 {
		h.states = h.states[:keep+1]
	}
}

// nextState advances block comment state across one line.
func (h *Highlighter) nextState(line string, inBlock bool) bool {
	for inBlock {
		end := h.closesAt(line)
		if end < 0 {
			return true
		}
		line = string([]rune(line)[end:])
		inBlock = false
	}
	return h.opensAt(line) >= 0
}

// closesAt returns the rune index just past the closing delimiter, or -1.
// A plain search is correct: nothing is special inside a block comment.
func (h *Highlighter) closesAt(line string) int {
	if h.blockClose == "" {
		return -1
	}
	i := strings.Index(line, h.blockClose)
	if i < 0 {
		return -1
	}
	return len([]rune(line[:i+len(h.blockClose)]))
}

// opensAt returns the rune index where line opens an unclosed block comment,
// or -1. Appending the closer makes the comment well formed, so chroma's own
// rules decide: an opener inside a string or after a line comment is ignored.
func (h *Highlighter) opensAt(line string) int {
	if h.blockOpen == "" || !strings.Contains(line, h.blockOpen) {
		return -1
	}
	if idx, ok := h.opens[line]; ok {
		return idx
	}
	idx := h.computeOpensAt(line)
	if h.opens == nil {
		h.opens = make(map[string]int)
	}
	h.opens[line] = idx
	return idx
}

func (h *Highlighter) computeOpensAt(line string) int {
	iter, err := h.lexer.Tokenise(nil, line+h.blockClose)
	if err != nil {
		return -1
	}
	pos, start := 0, -1
	for _, tok := range iter.Tokens() {
		switch {
		case isCommentToken(tok.Type) && strings.HasPrefix(tok.Value, h.blockOpen):
			start = pos
		case !isCommentToken(tok.Type):
			start = -1
		}
		pos += len([]rune(tok.Value))
	}
	return start
}

// Probed in order to discover the language's block comment delimiters.
var blockCommentCandidates = [][2]string{
	{"/*", "*/"},
	{"<!--", "-->"},
	{"{-", "-}"},
	{"(*", "*)"},
	{"--[[", "]]"},
	{"<#", "#>"},
}

// detectBlockComment finds which candidate pair the lexer honours. Chroma's
// comment rules vary too much in shape to read delimiters off directly.
// The probe spans two lines because a line comment cannot, which stops
// Haskell's "--" from matching the "--[[" candidate.
func detectBlockComment(lx chroma.Lexer) (string, string) {
	for _, c := range blockCommentCandidates {
		probe := c[0] + " a\nb " + c[1]
		iter, err := lx.Tokenise(nil, probe)
		if err != nil {
			continue
		}
		toks := iter.Tokens()
		if len(toks) == 0 {
			continue
		}
		if isCommentToken(toks[0].Type) && len([]rune(toks[0].Value)) >= len([]rune(probe)) {
			return c[0], c[1]
		}
	}
	return "", ""
}

func isCommentToken(t chroma.TokenType) bool {
	return t == chroma.Comment || t.InSubCategory(chroma.Comment)
}

func mapTokenType(t chroma.TokenType) term.Style {
	switch {
	case t == chroma.KeywordType:
		return term.StyleSyntaxType
	case t == chroma.Keyword || t.InSubCategory(chroma.Keyword):
		return term.StyleSyntaxKeyword
	case t == chroma.Comment || t.InSubCategory(chroma.Comment):
		return term.StyleSyntaxComment
	case t == chroma.String || t.InSubCategory(chroma.String):
		return term.StyleSyntaxString
	case t == chroma.Number || t.InSubCategory(chroma.Number):
		return term.StyleSyntaxNumber
	case t == chroma.Operator || t.InSubCategory(chroma.Operator):
		return term.StyleSyntaxOperator
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic:
		return term.StyleSyntaxFunction
	case t == chroma.NameBuiltin || t == chroma.NameBuiltinPseudo:
		return term.StyleSyntaxBuiltin
	case t == chroma.NameClass || t == chroma.NameDecorator:
		return term.StyleSyntaxType
	case t == chroma.NameTag:
		return term.StyleSyntaxTag
	case t == chroma.NameAttribute:
		return term.StyleSyntaxAttribute
	case t == chroma.NameVariable || t.InSubCategory(chroma.NameVariable):
		return term.StyleSyntaxVariable
	case t == chroma.GenericHeading || t == chroma.GenericSubheading:
		return term.StyleSyntaxKeyword
	case t == chroma.GenericStrong:
		return term.StyleSyntaxType
	case t == chroma.GenericEmph:
		return term.StyleSyntaxString
	case t == chroma.GenericInserted:
		return term.StyleDiffAdded
	case t == chroma.GenericDeleted:
		return term.StyleDiffDeleted
	case t == chroma.Punctuation:
		return term.StyleSyntaxPunctuation
	default:
		return term.StyleDefault
	}
}
