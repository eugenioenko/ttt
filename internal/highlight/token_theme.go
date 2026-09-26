package highlight

import (
	"slices"
	"strings"
	"sync/atomic"

	"github.com/eugenioenko/ttt/internal/term"
)

// TokenRule is one selector of a VS Code tokenColors entry. An empty
// Foreground keeps the enclosing color; a nil FontStyle keeps the enclosing
// font style, and an empty one resets it.
type TokenRule struct {
	Selector   string
	Foreground string
	FontStyle  *string
}

// TokenStyle is the color of one token style slot. An empty Fg means the
// theme's default foreground.
type TokenStyle struct {
	Fg     string
	Bold   bool
	Italic bool
}

// TokenTheme resolves scope stacks the way VS Code resolves tokenColors.
// Styles lists every foreground and font-style combination the rules can
// produce, so style slots are fixed when the theme is compiled even though a
// token may take its color and its font style from different rules.
type TokenTheme struct {
	rules  []tokenRule
	fgs    map[string]int
	Styles []TokenStyle
}

type tokenRule struct {
	scope     string
	parents   []string
	fg        string
	hasFont   bool
	bold      bool
	italic    bool
	specifity int
	order     int
}

const fontStyles = 4 // none, bold, italic, bold italic

// NewTokenTheme returns nil when rules is empty or the rules would need more
// style slots than term reserves; tokens then use the syntax styles.
func NewTokenTheme(rules []TokenRule) *TokenTheme {
	t := &TokenTheme{fgs: map[string]int{"": 0}}
	fgList := []string{""}
	for order, rule := range rules {
		parts := strings.Fields(rule.Selector)
		if len(parts) == 0 || strings.HasPrefix(rule.Selector, "-") || strings.Contains(rule.Selector, " -") {
			continue
		}
		fg := normalizeColor(rule.Foreground)
		if _, ok := t.fgs[fg]; !ok {
			t.fgs[fg] = len(fgList)
			fgList = append(fgList, fg)
		}
		compiled := tokenRule{
			scope: parts[len(parts)-1], parents: parts[:len(parts)-1],
			fg: fg, specifity: strings.Count(parts[len(parts)-1], ".") + 1, order: order,
		}
		if rule.FontStyle != nil {
			compiled.hasFont = true
			compiled.bold = strings.Contains(*rule.FontStyle, "bold")
			compiled.italic = strings.Contains(*rule.FontStyle, "italic")
		}
		t.rules = append(t.rules, compiled)
	}
	if len(t.rules) == 0 || len(fgList)*fontStyles > term.MaxTokenStyles {
		return nil
	}
	for _, fg := range fgList {
		for font := 0; font < fontStyles; font++ {
			t.Styles = append(t.Styles, TokenStyle{Fg: fg, Bold: font&1 != 0, Italic: font&2 != 0})
		}
	}
	return t
}

// resolve returns the slot style for names, or StyleDefault when no rule
// matches. At each scope level the matching rules apply from least to most
// specific, each overriding only the settings it sets, so a rule that sets
// just a font style keeps the color of a broader one, as in vscode-textmate.
// A level without a match keeps the enclosing level's settings.
func (t *TokenTheme) resolve(names []string) term.Style {
	fg, bold, italic, matched := "", false, false, false
	for i := range names {
		for _, rule := range t.matching(names, i) {
			matched = true
			if rule.fg != "" {
				fg = rule.fg
			}
			if rule.hasFont {
				bold, italic = rule.bold, rule.italic
			}
		}
	}
	if !matched {
		return term.StyleDefault
	}
	font := 0
	if bold {
		font |= 1
	}
	if italic {
		font |= 2
	}
	return term.TokenStyle(t.fgs[fg]*fontStyles + font)
}

// matching returns the rules that apply at level, least specific first.
func (t *TokenTheme) matching(names []string, level int) []*tokenRule {
	var matched []*tokenRule
	for i := range t.rules {
		rule := &t.rules[i]
		if hasScopePrefix(names[level], rule.scope) && parentsMatch(names[:level], rule.parents) {
			matched = append(matched, rule)
		}
	}
	slices.SortStableFunc(matched, func(a, b *tokenRule) int {
		if a.specifity != b.specifity {
			return a.specifity - b.specifity
		}
		if len(a.parents) != len(b.parents) {
			return len(a.parents) - len(b.parents)
		}
		return a.order - b.order
	})
	return matched
}

// parentsMatch reports whether the parent selectors appear among ancestors
// in order, each matching some ancestor further out than the previous one.
func parentsMatch(ancestors, parents []string) bool {
	pos := len(ancestors)
	for i := len(parents) - 1; i >= 0; i-- {
		for pos > 0 && !hasScopePrefix(ancestors[pos-1], parents[i]) {
			pos--
		}
		if pos == 0 {
			return false
		}
		pos--
	}
	return true
}

func normalizeColor(color string) string {
	color = strings.ToLower(color)
	if len(color) == 4 && color[0] == '#' {
		color = "#" + strings.Repeat(color[1:2], 2) + strings.Repeat(color[2:3], 2) + strings.Repeat(color[3:4], 2)
	}
	if len(color) > 7 {
		color = color[:7]
	}
	return color
}

var currentTokenTheme atomic.Pointer[TokenTheme]

// SetTokenTheme installs the token theme used by every Highlighter. It must be
// installed together with a style map built from the same theme, because the
// style slots it returns are indexes into that map.
func SetTokenTheme(t *TokenTheme) {
	currentTokenTheme.Store(t)
}

// CurrentTokenTheme returns the installed token theme, or nil.
func CurrentTokenTheme() *TokenTheme {
	return currentTokenTheme.Load()
}
