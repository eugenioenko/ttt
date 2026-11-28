package highlight

import (
	"strings"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/ttt/internal/core/buffer"
)

// JSXMarkupAt reports whether lines[line] starts inside JSX markup rather
// than JavaScript or TypeScript code. Tokenization runs through the line
// regardless of the render deferral budget, because a comment toggle needs the
// context before it edits anything.
func (h *Highlighter) JSXMarkupAt(buf *buffer.Buffer, line int) bool {
	if h == nil || buf == nil || line < 0 || line >= len(buf.Lines) {
		return false
	}
	h.syncTokenTheme()
	h.SyncBuffer(buf)
	h.materialize(line, line+1)
	res, ok := h.doc.Line(line)
	if !ok || len(res.Tokens) == 0 {
		return false
	}
	runes := []rune(buf.Lines[line])
	for _, tok := range res.Tokens {
		if strings.TrimSpace(string(runes[tok.Start:tok.End])) == "" {
			continue
		}
		// Scopes enclosing an open {…} still name the outer tag, so only the
		// scopes inside the last embedded expression decide this token: a nested
		// element there is markup, but continuing code of that expression is not.
		start := 0
		for i, scope := range tok.Scopes {
			if hasScopePrefix(scope, "meta.embedded.expression") {
				start = i + 1
			}
		}
		for _, scope := range tok.Scopes[start:] {
			if hasScopePrefix(scope, "meta.tag") || hasScopePrefix(scope, "meta.jsx.children") {
				return true
			}
		}
		return false
	}
	return false
}

// SvelteContext identifies the embedded syntax a Svelte line starts in.
type SvelteContext int

const (
	SvelteMarkup SvelteContext = iota
	SvelteScript
	SvelteStyle
	SvelteExpression
)

// SvelteContextAt reports the context in which lines[line] starts. A line
// beginning inside a <script> or <style> block, or inside a template
// expression that opened on an earlier line, needs that embedded language's
// comment marker rather than an HTML one. The block's own tags and ordinary
// text stay markup.
func (h *Highlighter) SvelteContextAt(buf *buffer.Buffer, line int) SvelteContext {
	if h == nil || buf == nil || line < 0 || line >= len(buf.Lines) {
		return SvelteMarkup
	}
	h.syncTokenTheme()
	h.SyncBuffer(buf)
	h.materialize(line, line+1)
	res, ok := h.doc.Line(line)
	if !ok || len(res.Tokens) == 0 {
		return SvelteMarkup
	}
	runes := []rune(buf.Lines[line])
	var scopes []string
	for _, tok := range res.Tokens {
		if strings.TrimSpace(string(runes[tok.Start:tok.End])) == "" {
			continue
		}
		scopes = tok.Scopes
		break
	}
	for _, scope := range scopes {
		if hasScopePrefix(scope, "meta.tag.start") || hasScopePrefix(scope, "meta.tag.end") {
			return SvelteMarkup
		}
	}
	for _, scope := range scopes {
		switch {
		case hasScopePrefix(scope, "meta.script.svelte"):
			return SvelteScript
		case hasScopePrefix(scope, "meta.style.svelte"):
			return SvelteStyle
		case hasScopePrefix(scope, "meta.embedded.expression"):
			return SvelteExpression
		}
	}
	return SvelteMarkup
}

// TwigContext identifies the syntax a Twig line starts in. A .twig file is
// HTML with embedded Twig, JavaScript, and CSS, and each needs its own comment
// marker; Twig comments are literal inside a {% verbatim %} block.
type TwigContext int

const (
	TwigMarkup TwigContext = iota
	TwigScript
	TwigStyle
	TwigVerbatim
)

// TwigContextAt reports the context in which lines[line] starts. Script and
// style tags themselves count as markup, because their content begins on a
// later line; so does a line inside {% verbatim %}, where Twig does not parse
// the text.
func (h *Highlighter) TwigContextAt(buf *buffer.Buffer, line int) TwigContext {
	if h == nil || buf == nil || line < 0 || line >= len(buf.Lines) {
		return TwigMarkup
	}
	h.syncTokenTheme()
	h.SyncBuffer(buf)
	h.materialize(line, line+1)
	res, ok := h.doc.Line(line)
	if !ok || len(res.Tokens) == 0 {
		return TwigMarkup
	}
	runes := []rune(buf.Lines[line])
	var scopes []string
	for _, tok := range res.Tokens {
		if strings.TrimSpace(string(runes[tok.Start:tok.End])) == "" {
			continue
		}
		scopes = tok.Scopes
		break
	}
	for _, scope := range scopes {
		if hasScopePrefix(scope, "string.unquoted.verbatim") {
			return TwigVerbatim
		}
	}
	for _, scope := range scopes {
		if hasScopePrefix(scope, "punctuation.definition.tag") ||
			hasScopePrefix(scope, "entity.name.tag") ||
			hasScopePrefix(scope, "meta.tag") {
			return TwigMarkup
		}
	}
	for _, scope := range scopes {
		switch {
		case hasScopePrefix(scope, "source.js.embedded"):
			return TwigScript
		case hasScopePrefix(scope, "source.css.embedded"):
			return TwigStyle
		}
	}
	return TwigMarkup
}

// VueContext identifies the syntax a Vue single-file component line starts
// in: template markup, a block's <script> content, or its <style> content.
type VueContext int

const (
	VueMarkup VueContext = iota
	VueScript
	VueStyle
)

// VueContextAt reports the context in which lines[line] starts. The SFC block
// tags themselves count as markup; only their content uses the embedded
// language's comment syntax.
func (h *Highlighter) VueContextAt(buf *buffer.Buffer, line int) VueContext {
	if h == nil || buf == nil || line < 0 || line >= len(buf.Lines) {
		return VueMarkup
	}
	h.syncTokenTheme()
	h.SyncBuffer(buf)
	h.materialize(line, line+1)
	res, ok := h.doc.Line(line)
	if !ok || len(res.Tokens) == 0 {
		return VueMarkup
	}
	runes := []rune(buf.Lines[line])
	var scopes []string
	for _, tok := range res.Tokens {
		if strings.TrimSpace(string(runes[tok.Start:tok.End])) == "" {
			continue
		}
		scopes = tok.Scopes
		break
	}
	// Element selectors inside a <style> block carry entity.name.tag.css, which
	// the tag checks below would otherwise read as markup.
	for _, scope := range scopes {
		if hasScopePrefix(scope, "source.css") || hasScopePrefix(scope, "source.scss") ||
			hasScopePrefix(scope, "source.less") || hasScopePrefix(scope, "source.sass") ||
			hasScopePrefix(scope, "source.stylus") || hasScopePrefix(scope, "source.postcss") {
			return VueStyle
		}
	}
	for _, scope := range scopes {
		if hasScopePrefix(scope, "punctuation.definition.tag") ||
			hasScopePrefix(scope, "entity.name.tag") ||
			hasScopePrefix(scope, "meta.tag") {
			return VueMarkup
		}
	}
	for _, scope := range scopes {
		if hasScopePrefix(scope, "source.js") || hasScopePrefix(scope, "source.ts") {
			return VueScript
		}
	}
	return VueMarkup
}

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
