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
	cache map[string][]Span
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
}

// HighlightLineWithState highlights a line tracking multiline block comment state.
// If inBlockComment is true, content until */ is styled as Comment.
// Returns the spans and whether the line ends inside an unclosed block comment.
func (h *Highlighter) HighlightLineWithState(line string, inBlockComment bool) ([]Span, bool) {
	if inBlockComment {
		return h.highlightInBlockComment(line)
	}
	return h.highlightOutsideComment(line)
}

// highlightInBlockComment handles a line that starts inside a block comment.
func (h *Highlighter) highlightInBlockComment(line string) ([]Span, bool) {
	lineRunes := []rune(line)
	endIdx := strings.Index(line, "*/")
	if endIdx >= 0 {
		var spans []Span
		endRune := len([]rune(line[:endIdx+2]))
		spans = append(spans, Span{Start: 0, End: endRune, Style: term.StyleSyntaxComment})
		rest := line[endIdx+2:]
		if rest != "" {
			restSpans, _ := h.highlightOutsideComment(rest)
			for _, s := range restSpans {
				spans = append(spans, Span{Start: s.Start + endRune, End: s.End + endRune, Style: s.Style})
			}
		}
		return spans, false
	}
	if len(lineRunes) > 0 {
		return []Span{{Start: 0, End: len(lineRunes), Style: term.StyleSyntaxComment}}, true
	}
	return nil, true
}

// highlightOutsideComment handles a line that starts outside a block comment.
// It returns chroma spans and checks whether the line opens a multiline comment.
func (h *Highlighter) highlightOutsideComment(line string) ([]Span, bool) {
	spans := h.HighlightLine(line)

	// Find /* that opens a multiline comment (no */ on same line).
	// Only consider /* that is NOT inside a chroma Comment or String span —
	// chroma handles inline /* */ and // line comments correctly.
	for i := 0; i < len(line)-1; i++ {
		if line[i] != '/' || line[i+1] != '*' {
			continue
		}
		runeIdx := len([]rune(line[:i]))

		// Skip if inside a chroma Comment or String span
		insideHandledSpan := false
		for _, s := range spans {
			if (s.Style == term.StyleSyntaxComment || s.Style == term.StyleSyntaxString) &&
				runeIdx >= s.Start && runeIdx < s.End {
				insideHandledSpan = true
				break
			}
		}
		if insideHandledSpan {
			continue
		}

		// Check if */ exists on this line after /*
		rest := line[i+2:]
		endIdx := strings.Index(rest, "*/")
		if endIdx < 0 {
			// No closing */ — multiline comment starts here
			lineRunes := []rune(line)
			// Remove chroma spans that overlap with the comment region (from /* to EOL)
			filtered := spans[:0]
			for _, s := range spans {
				if s.End <= runeIdx {
					filtered = append(filtered, s)
				} else if s.Start < runeIdx && s.End > runeIdx {
					filtered = append(filtered, Span{Start: s.Start, End: runeIdx, Style: s.Style})
				}
			}
			spans = filtered
			spans = append(spans, Span{Start: runeIdx, End: len(lineRunes), Style: term.StyleSyntaxComment})
			return spans, true
		}
	}

	return spans, false
}

// ScanBlockCommentState scans lines from 0 to startLine and returns
// whether startLine begins inside a block comment. Uses simple string
// matching for speed — rare false positives (/* inside strings) are
// harmless since visible lines use the accurate chroma-based check.
func ScanBlockCommentState(lines []string, startLine int) bool {
	inComment := false
	for i := 0; i < startLine && i < len(lines); i++ {
		inComment = nextCommentState(lines[i], inComment)
	}
	return inComment
}

func nextCommentState(line string, inComment bool) bool {
	if inComment {
		idx := strings.Index(line, "*/")
		if idx < 0 {
			return true
		}
		return nextCommentState(line[idx+2:], false)
	}
	for i := 0; i < len(line)-1; i++ {
		if line[i] == '/' && line[i+1] == '*' {
			rest := line[i+2:]
			endIdx := strings.Index(rest, "*/")
			if endIdx < 0 {
				return true
			}
			return nextCommentState(rest[endIdx+2:], false)
		}
	}
	return false
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
