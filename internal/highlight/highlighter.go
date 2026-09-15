package highlight

import (
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/eugenioenko/ttt/internal/term"
)

type Span struct {
	Start int
	End   int
	Style term.Style
}

// noRegion marks a line that starts outside every multi-line region.
const noRegion int8 = -1

// spanKey keys the span cache; the same text differs by incoming state.
type spanKey struct {
	line   string
	region int8
}

// A region is a delimiter pair the language lets span lines: a block comment,
// a docstring, a raw or template string. ttt tokenises one line at a time and
// chroma exposes no way to resume a lexer's state stack on the next line, so a
// line that starts inside a region is coloured from the region's own style
// until its closing delimiter, and only the remainder is lexed.
type region struct {
	open  string
	close string
	style term.Style
	// tokenType is what the lexer called the opening delimiter during the
	// probe. Matching it exactly is what keeps a backtick inside a
	// single-quoted string from opening a template literal.
	tokenType chroma.TokenType
	// escapes reports that a backslash escapes the closing delimiter: true for
	// strings, false for comments, where nothing is special.
	escapes bool
}

// openAt is where a line opens a region it never closes, region noRegion when
// it opens none.
type openAt struct {
	col    int
	region int8
}

var noOpen = openAt{col: -1, region: noRegion}

type Highlighter struct {
	lexer chroma.Lexer
	cache map[spanKey][]Span

	// Empty when the language has no multi-line region; state tracking is
	// then skipped entirely.
	regions []region
	// opens memoizes where a line opens a region that outlives it. A pure
	// function of the text, so it survives ClearCache.
	opens map[string]openAt

	// states[i] is the region line i starts inside, noRegion for none.
	states []int8
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
	h := &Highlighter{lexer: chroma.Coalesce(lexer)}
	h.regions = detectRegions(lexer)
	return h
}

func (h *Highlighter) Language() string {
	return h.lexer.Config().Name
}

// HighlightLine highlights a line in isolation, ignoring multi-line regions.
func (h *Highlighter) HighlightLine(line string) []Span {
	return h.highlight(line, noRegion)
}

// HighlightLineAt highlights lines[idx], carrying multi-line region state down
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

func (h *Highlighter) highlight(line string, reg int8) []Span {
	key := spanKey{line: line, region: reg}
	if h.cache != nil {
		if cached, ok := h.cache[key]; ok {
			return cached
		}
	}
	spans := h.computeSpans(line, reg)
	if h.cache == nil {
		h.cache = make(map[spanKey][]Span)
	}
	h.cache[key] = spans
	return spans
}

func (h *Highlighter) computeSpans(line string, reg int8) []Span {
	if reg >= 0 && int(reg) < len(h.regions) {
		r := h.regions[reg]
		end := closesAt(line, r)
		if end < 0 {
			if line == "" {
				return nil
			}
			return []Span{{Start: 0, End: len([]rune(line)), Style: r.style}}
		}
		spans := []Span{{Start: 0, End: end, Style: r.style}}
		for _, s := range h.computeSpans(string([]rune(line)[end:]), noRegion) {
			spans = append(spans, Span{Start: s.Start + end, End: s.End + end, Style: s.Style})
		}
		return spans
	}

	spans := h.lexLine(line)
	open := h.opensAt(line)
	if open.region < 0 {
		return spans
	}
	// A region opens here and runs past end of line. Fresh slice: spans is cached.
	out := make([]Span, 0, len(spans)+1)
	for _, s := range spans {
		if s.End <= open.col {
			out = append(out, s)
		} else if s.Start < open.col {
			out = append(out, Span{Start: s.Start, End: open.col, Style: s.Style})
		}
	}
	style := h.regions[open.region].style
	return append(out, Span{Start: open.col, End: len([]rune(line)), Style: style})
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

// stateAt reports which region lines[idx] starts inside, extending the state
// table as needed. Transitions are memoized, so this is a map lookup per line
// rather than a re-lex.
func (h *Highlighter) stateAt(lines []string, idx int) int8 {
	if len(h.regions) == 0 || idx <= 0 {
		return noRegion
	}
	if h.statesDirty {
		h.statesDirty = false
		h.truncateToEdit(lines)
	}
	if len(h.states) == 0 {
		h.states = append(h.states, noRegion)
	}
	for len(h.states) <= idx && len(h.states) <= len(lines) {
		i := len(h.states) - 1
		h.states = append(h.states, h.nextState(lines[i], h.states[i]))
		h.stateSrc = append(h.stateSrc, lines[i])
	}
	if idx < len(h.states) {
		return h.states[idx]
	}
	return noRegion
}

// truncateToEdit drops the state table from the first line whose text changed,
// keeping everything above it. Comparing strings that were never rewritten hits
// Go's identical-pointer fast path, so unchanged lines cost no scanning.
func (h *Highlighter) truncateToEdit(lines []string) {
	keep := min(len(h.stateSrc), len(lines))
	for i := range keep {
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

// nextState advances region state across one line.
func (h *Highlighter) nextState(line string, reg int8) int8 {
	for reg >= 0 && int(reg) < len(h.regions) {
		end := closesAt(line, h.regions[reg])
		if end < 0 {
			return reg
		}
		line = string([]rune(line)[end:])
		reg = noRegion
	}
	return h.opensAt(line).region
}

// closesAt returns the rune index just past the region's closing delimiter, or
// -1. Only a string region honours backslash escapes; inside a comment nothing
// is special, so that case is a plain substring search. Neither path
// materialises the line as runes: this runs once per line when the state table
// is rebuilt over a whole buffer.
func closesAt(line string, r region) int {
	if r.close == "" {
		return -1
	}
	if !r.escapes {
		i := strings.Index(line, r.close)
		if i < 0 {
			return -1
		}
		return utf8.RuneCountInString(line[:i+len(r.close)])
	}
	runes := 0
	for i := 0; i < len(line); {
		if line[i] == '\\' {
			i++
			runes++
			if i < len(line) {
				_, w := utf8.DecodeRuneInString(line[i:])
				i += w
				runes++
			}
			continue
		}
		if strings.HasPrefix(line[i:], r.close) {
			return runes + utf8.RuneCountInString(r.close)
		}
		_, w := utf8.DecodeRuneInString(line[i:])
		i += w
		runes++
	}
	return -1
}

func (h *Highlighter) opensAt(line string) openAt {
	if len(h.regions) == 0 {
		return noOpen
	}
	if at, ok := h.opens[line]; ok {
		return at
	}
	at := h.computeOpensAt(line)
	if h.opens == nil {
		h.opens = make(map[string]openAt)
	}
	h.opens[line] = at
	return at
}

// computeOpensAt returns the earliest region the line opens and never closes.
// Appending the closer makes the region well formed, so chroma's own rules
// decide whether the opener is real: one inside a string or after a line
// comment is ignored. The lexer is asked where a region starts and never
// whether it ends, because the appended closer coalesces with a closer the
// line already had; closesAt, the same oracle nextState and computeSpans use
// to end a region, decides that. An opener that only exists because the
// appended closer completed it is not on the line at all.
func (h *Highlighter) computeOpensAt(line string) openAt {
	best := noOpen
	runes := []rune(line)
	for i := range h.regions {
		r := h.regions[i]
		if !strings.Contains(line, r.open) {
			continue
		}
		start := h.trailingRegionStart(line+r.close, r)
		after := start + len([]rune(r.open))
		if start < 0 || after > len(runes) {
			continue
		}
		if closesAt(string(runes[after:]), r) >= 0 {
			continue
		}
		if best.col < 0 || start < best.col {
			best = openAt{col: start, region: int8(i)}
		}
	}
	return best
}

// trailingRegionStart returns the rune index where the region that reaches the
// end of text begins, or -1 when the text does not end inside one.
func (h *Highlighter) trailingRegionStart(text string, r region) int {
	iter, err := h.lexer.Tokenise(nil, text)
	if err != nil {
		return -1
	}
	pos, start := 0, -1
	for _, tok := range iter.Tokens() {
		switch {
		case tok.Type == r.tokenType && strings.HasPrefix(tok.Value, r.open):
			start = pos
		case tok.Type != r.tokenType:
			start = -1
		}
		pos += len([]rune(tok.Value))
	}
	return start
}

// Probed in order to discover which multi-line regions the language has. The
// probe spans two lines because a single-line construct cannot, which is what
// stops Haskell's "--" from matching the "--[[" candidate.
var regionCandidates = []struct {
	open      string
	close     string
	style     term.Style
	comment   bool
	ownsDelim bool
}{
	{open: "/*", close: "*/", style: term.StyleSyntaxComment, comment: true},
	{open: "<!--", close: "-->", style: term.StyleSyntaxComment, comment: true},
	{open: "{-", close: "-}", style: term.StyleSyntaxComment, comment: true},
	{open: "(*", close: "*)", style: term.StyleSyntaxComment, comment: true},
	{open: "--[[", close: "]]", style: term.StyleSyntaxComment, comment: true},
	{open: "<#", close: "#>", style: term.StyleSyntaxComment, comment: true},
	{open: `"""`, close: `"""`, style: term.StyleSyntaxString, ownsDelim: true},
	{open: `'''`, close: `'''`, style: term.StyleSyntaxString, ownsDelim: true},
	{open: "`", close: "`", style: term.StyleSyntaxString, ownsDelim: true},
}

// detectRegions asks the lexer, never the delimiter text, which candidates the
// language has. A language has one block comment, so the first comment
// candidate that matches wins: collecting them all hands TypeScript an
// "<!-- -->" region, and `a <!--b` then greys out the rest of the buffer.
func detectRegions(raw chroma.Lexer) []region {
	lx := chroma.Coalesce(raw)
	var out []region
	haveComment := false
	for _, c := range regionCandidates {
		if c.comment && haveComment {
			continue
		}
		tok, ok := wholeTextToken(lx, c.open+" a\nb "+c.close)
		if !ok {
			continue
		}
		if c.comment != isCommentToken(tok.Type) || (!c.comment && !isStringToken(tok.Type)) {
			continue
		}
		if c.ownsDelim && !emitsDelimToken(raw, c.open+" a\nb "+c.close, c.open) {
			continue
		}
		haveComment = haveComment || c.comment
		out = append(out, region{
			open:      c.open,
			close:     c.close,
			style:     c.style,
			tokenType: tok.Type,
			escapes:   !c.comment && escapesCloser(lx, c.open, c.close),
		})
	}
	return out
}

// escapesCloser reports whether a backslash before the closing delimiter
// escapes it. Asking the lexer is the only way to tell a JavaScript template
// literal, where it does, from a Go raw string, where a backslash is an
// ordinary character and assuming otherwise leaves the region open for the
// rest of the buffer.
func escapesCloser(lx chroma.Lexer, open, close string) bool {
	// The sentinel sits outside the region unless the backslash swallowed the
	// closer, so one token covering the probe means escapes are honoured.
	_, whole := wholeTextToken(lx, open+` a\`+close+" b")
	return whole
}

// wholeTextToken reports the first token when it covers the whole text, which
// is what "the lexer reads this as one region" means.
func wholeTextToken(lx chroma.Lexer, text string) (chroma.Token, bool) {
	iter, err := lx.Tokenise(nil, text)
	if err != nil {
		return chroma.Token{}, false
	}
	toks := iter.Tokens()
	if len(toks) == 0 {
		return chroma.Token{}, false
	}
	return toks[0], len([]rune(toks[0].Value)) >= len([]rune(text))
}

// emitsDelimToken requires the raw lexer to emit the opening delimiter as a
// token of its own, which is what separates a region from a run of literals
// that coalesce into one token: Go and Rust read `""" a\nb """` as `""`,
// `" a\nb "`, `""`, and neither has a triple-quoted literal. The test is
// conservative, not exact. Swift, Scala, Java and Kotlin do have multi-line
// `"""` strings and are rejected here, so they keep main's behaviour of losing
// the colour after the first line; admitting them would mean admitting Go and
// Rust, where the same input is three ordinary literals. Leading empty tokens,
// such as Python's string affix, are skipped.
func emitsDelimToken(lx chroma.Lexer, probe, delim string) bool {
	iter, err := lx.Tokenise(nil, probe)
	if err != nil {
		return false
	}
	for _, tok := range iter.Tokens() {
		if tok.Value == "" {
			continue
		}
		return tok.Value == delim
	}
	return false
}

func isCommentToken(t chroma.TokenType) bool {
	return t == chroma.Comment || t.InSubCategory(chroma.Comment)
}

func isStringToken(t chroma.TokenType) bool {
	return t == chroma.LiteralString || t.InSubCategory(chroma.LiteralString)
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
