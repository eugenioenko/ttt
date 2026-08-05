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

type Highlighter struct {
	lexer     chroma.Lexer
	cache     map[string][]Span
	lineSpans [][]Span
}

func New(filename string) *Highlighter {
	lexer := lexers.Match(filename)
	if lexer == nil {
		return nil
	}
	lexer = chroma.Coalesce(lexer)
	return &Highlighter{lexer: lexer}
}

func (h *Highlighter) Language() string {
	return h.lexer.Config().Name
}

// HighlightAll tokenizes all lines at once so that multiline constructs
// (block comments, heredocs, etc.) are correctly recognized. The result is
// cached and reused until ClearCache is called.
func (h *Highlighter) HighlightAll(lines []string) [][]Span {
	if h.lineSpans != nil && len(h.lineSpans) == len(lines) {
		return h.lineSpans
	}

	text := strings.Join(lines, "\n") + "\n"
	iter, err := h.lexer.Tokenise(nil, text)
	if err != nil {
		return nil
	}

	h.lineSpans = make([][]Span, len(lines))
	lineIdx := 0
	col := 0

	for tok := iter(); tok != chroma.EOF; tok = iter() {
		value := tok.Value
		for {
			nlIdx := strings.Index(value, "\n")
			if nlIdx < 0 {
				part := value
				runeLen := len([]rune(part))
				if runeLen > 0 {
					style := mapTokenType(tok.Type)
					if style != term.StyleDefault {
						h.lineSpans[lineIdx] = append(h.lineSpans[lineIdx], Span{
							Start: col,
							End:   col + runeLen,
							Style: style,
						})
					}
				}
				col += runeLen
				break
			}

			part := value[:nlIdx]
			runeLen := len([]rune(part))
			if runeLen > 0 {
				style := mapTokenType(tok.Type)
				if style != term.StyleDefault {
					h.lineSpans[lineIdx] = append(h.lineSpans[lineIdx], Span{
						Start: col,
						End:   col + runeLen,
						Style: style,
					})
				}
			}

			lineIdx++
			if lineIdx >= len(lines) {
				return h.lineSpans
			}
			col = 0
			value = value[nlIdx+1:]
		}
	}

	return h.lineSpans
}

func (h *Highlighter) HighlightLine(line string) []Span {
	if h.cache != nil {
		if cached, ok := h.cache[line]; ok {
			return cached
		}
	}
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
	if h.cache == nil {
		h.cache = make(map[string][]Span)
	}
	h.cache[line] = spans
	return spans
}

func (h *Highlighter) ClearCache() {
	h.cache = make(map[string][]Span)
	h.lineSpans = nil
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
