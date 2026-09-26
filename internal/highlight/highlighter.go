package highlight

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/textmate-go/grammars"

	"github.com/eugenioenko/ttt/internal/term"
)

type Span struct {
	Start int
	End   int
	Style term.Style
}

// Minified files put a whole program on one line; past this limit a line is
// left uncoloured rather than stalling the render.
var tokenizeOptions = textmate.TokenizeOptions{
	TimeLimit:    50 * time.Millisecond,
	MaxLineRunes: 20_000,
}

const maxIsolatedCache = 4_096

var (
	registryOnce sync.Once
	registry     *textmate.Registry
)

func sharedRegistry() *textmate.Registry {
	registryOnce.Do(func() {
		registry = textmate.NewRegistry(textmate.RegistryOptions{LoadGrammar: grammars.Load})
	})
	return registry
}

type Highlighter struct {
	grammar  *textmate.Grammar
	language string
	doc      *textmate.Document
	dirty    bool
	isolated map[string][]Span
	styles   map[*textmate.ScopeStack]term.Style
}

// Conventional extensionless names the grammar catalog does not declare.
var extraFilenames = map[string]string{
	"gemfile":     "ruby",
	"rakefile":    "ruby",
	"guardfile":   "ruby",
	"podfile":     "ruby",
	"vagrantfile": "ruby",
	"brewfile":    "ruby",
	"jenkinsfile": "groovy",
}

var backupSuffixes = []string{".bak", ".orig", ".old", "~"}

// New returns nil when no embedded grammar matches the file name.
func New(filename string) *Highlighter {
	info, ok := infoForFilename(filepath.Base(filename))
	if !ok {
		return nil
	}
	return newFromInfo(info)
}

func infoForFilename(name string) (grammars.GrammarInfo, bool) {
	for {
		if info, ok := grammars.InfoForFilename(name); ok {
			return info, true
		}
		if id, ok := extraFilenames[strings.ToLower(name)]; ok {
			return grammars.InfoForID(id)
		}
		trimmed := name
		for _, suffix := range backupSuffixes {
			trimmed = strings.TrimSuffix(trimmed, suffix)
		}
		if trimmed == name || trimmed == "" {
			return grammars.GrammarInfo{}, false
		}
		name = trimmed
	}
}

// NewForLanguage selects a grammar by language ID or alias, such as a Markdown
// code-fence label.
func NewForLanguage(lang string) *Highlighter {
	info, ok := grammars.InfoForID(lang)
	if !ok {
		info, ok = grammars.InfoForAlias(lang)
	}
	if !ok {
		return nil
	}
	return newFromInfo(info)
}

func newFromInfo(info grammars.GrammarInfo) *Highlighter {
	g, err := sharedRegistry().LoadGrammar(info.ScopeName)
	if err != nil || g == nil {
		return nil
	}
	return &Highlighter{
		grammar:  g,
		language: info.DisplayName,
		doc:      textmate.NewDocument(g, textmate.DocumentOptions{TokenizeOptions: tokenizeOptions}),
		dirty:    true,
	}
}

func (h *Highlighter) Language() string {
	return h.language
}

// HighlightLine highlights a line in isolation, as the first line of a file.
func (h *Highlighter) HighlightLine(line string) []Span {
	if spans, ok := h.isolated[line]; ok {
		return spans
	}
	res := h.grammar.TokenizeLineWithOptions(line, nil, tokenizeOptions)
	spans := h.tokensToSpans(res.Tokens)
	if !res.Stopped {
		if h.isolated == nil || len(h.isolated) >= maxIsolatedCache {
			h.isolated = make(map[string][]Span)
		}
		h.isolated[line] = spans
	}
	return spans
}

// HighlightLineAt highlights lines[idx], carrying grammar state down from the
// top of the buffer.
func (h *Highlighter) HighlightLineAt(lines []string, idx int) []Span {
	if idx < 0 || idx >= len(lines) {
		return nil
	}
	if h.dirty || h.doc.Len() != len(lines) {
		h.dirty = false
		h.doc.SetLines(lines)
	}
	res, ok := h.doc.Line(idx)
	if !ok {
		return nil
	}
	return h.tokensToSpans(res.Tokens)
}

// ClearCache marks the buffer as edited; the next lookup hands the new lines
// to the document, which keeps state above the edit and reuses the unchanged
// tail once tokenizer state converges.
func (h *Highlighter) ClearCache() {
	h.dirty = true
}

func (h *Highlighter) tokensToSpans(tokens []textmate.Token) []Span {
	var spans []Span
	for _, tok := range tokens {
		if tok.End <= tok.Start {
			continue
		}
		style := h.styleFor(tok.ScopeStack)
		if style == term.StyleDefault {
			continue
		}
		if n := len(spans); n > 0 && spans[n-1].End == tok.Start && spans[n-1].Style == style {
			spans[n-1].End = tok.End
			continue
		}
		spans = append(spans, Span{Start: tok.Start, End: tok.End, Style: style})
	}
	return spans
}

// styleFor refines the engine's coarse category with the token's own scope.
// Only the innermost scope is consulted, so an enclosing scope such as
// meta.decorator cannot recolor a string or number nested inside it.
func (h *Highlighter) styleFor(stack *textmate.ScopeStack) term.Style {
	if style, ok := h.styles[stack]; ok {
		return style
	}
	style := categoryStyles[stack.Category()]
	if innermost, ok := stack.At(stack.Len() - 1); ok {
		for _, rule := range scopeStyles {
			if hasScopePrefix(innermost, rule.prefix) {
				style = rule.style
				break
			}
		}
	}
	if h.styles == nil || len(h.styles) >= maxIsolatedCache {
		h.styles = make(map[*textmate.ScopeStack]term.Style)
	}
	h.styles[stack] = style
	return style
}

func hasScopePrefix(scope, prefix string) bool {
	return strings.HasPrefix(scope, prefix) &&
		(len(scope) == len(prefix) || scope[len(prefix)] == '.')
}

var scopeStyles = []struct {
	prefix string
	style  term.Style
}{
	{"keyword.control", term.StyleSyntaxControl},
	{"storage.type.annotation", term.StyleSyntaxDecorator},
	{"storage", term.StyleSyntaxStorage},
	{"constant.language", term.StyleSyntaxConstant},
	{"constant.character.escape", term.StyleSyntaxEscape},
	{"variable.parameter", term.StyleSyntaxParameter},
	{"variable.other.property", term.StyleSyntaxProperty},
	{"variable.other.object.property", term.StyleSyntaxProperty},
	{"variable.language", term.StyleSyntaxSelf},
	{"entity.name.namespace", term.StyleSyntaxNamespace},
	{"entity.name.module", term.StyleSyntaxNamespace},
	{"entity.name.function.decorator", term.StyleSyntaxDecorator},
	{"punctuation.decorator", term.StyleSyntaxDecorator},
	{"markup.underline.link", term.StyleSyntaxLink},
	{"string.other.link", term.StyleSyntaxLink},
	{"markup.inline.raw", term.StyleSyntaxCode},
	{"punctuation.definition.template-expression", term.StyleSyntaxInterpolation},
	{"punctuation.definition.inserted", term.StyleSyntaxInserted},
	{"punctuation.definition.deleted", term.StyleSyntaxDeleted},
}

var categoryStyles = [...]term.Style{
	textmate.TokenCategoryPlain:       term.StyleDefault,
	textmate.TokenCategoryComment:     term.StyleSyntaxComment,
	textmate.TokenCategoryString:      term.StyleSyntaxString,
	textmate.TokenCategoryRegexp:      term.StyleSyntaxRegexp,
	textmate.TokenCategoryNumber:      term.StyleSyntaxNumber,
	textmate.TokenCategoryKeyword:     term.StyleSyntaxKeyword,
	textmate.TokenCategoryOperator:    term.StyleSyntaxOperator,
	textmate.TokenCategoryFunction:    term.StyleSyntaxFunction,
	textmate.TokenCategoryTag:         term.StyleSyntaxTag,
	textmate.TokenCategoryAttribute:   term.StyleSyntaxAttribute,
	textmate.TokenCategoryType:        term.StyleSyntaxType,
	textmate.TokenCategoryBuiltin:     term.StyleSyntaxBuiltin,
	textmate.TokenCategoryVariable:    term.StyleSyntaxVariable,
	textmate.TokenCategoryHeading:     term.StyleSyntaxHeading,
	textmate.TokenCategoryBold:        term.StyleSyntaxBold,
	textmate.TokenCategoryItalic:      term.StyleSyntaxItalic,
	textmate.TokenCategoryQuote:       term.StyleSyntaxQuote,
	textmate.TokenCategoryInserted:    term.StyleSyntaxInserted,
	textmate.TokenCategoryDeleted:     term.StyleSyntaxDeleted,
	textmate.TokenCategoryPunctuation: term.StyleSyntaxPunctuation,
	textmate.TokenCategoryInvalid:     term.StyleSyntaxInvalid,
}
