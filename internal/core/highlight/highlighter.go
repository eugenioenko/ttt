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
	lexer chroma.Lexer
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

func (h *Highlighter) HighlightLine(line string) []Span {
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
	case t == chroma.Punctuation:
		return term.StyleSyntaxPunctuation
	default:
		return term.StyleDefault
	}
}
