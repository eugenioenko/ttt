package highlight

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
)

// TestThemeParityWithVSCode compares the color ttt draws for every token of
// the samples below with the color the source VS Code theme gives the same
// scope stack. It needs the upstream theme files (for example VS Code's
// extensions/theme-defaults/themes and theme-monokai/themes) in one folder,
// and is skipped unless TTT_VSCODE_THEMES_DIR points at it. Bundled themes
// without tokenColors are skipped. Semantic highlighting is not compared.
func TestThemeParityWithVSCode(t *testing.T) {
	dir := os.Getenv("TTT_VSCODE_THEMES_DIR")
	if dir == "" {
		t.Skip("set TTT_VSCODE_THEMES_DIR to a folder of VS Code theme JSON files")
	}
	themes := map[string]string{
		"vscode-dark-plus": "dark_plus.json", "default-dark": "dark_modern.json",
		"default-light": "light_modern.json", "monokai": "monokai-color-theme.json",
		"solarized-dark": "solarized-dark-color-theme.json", "solarized-light": "solarized-light-color-theme.json",
		"dracula": "dracula.json", "nord": "nord-color-theme.json", "one-dark": "OneDark-Pro.json",
	}
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			ttt, err := config.LoadTheme(name)
			if err != nil {
				t.Fatal(err)
			}
			if len(ttt.TokenColors) == 0 {
				t.Skip("theme has no tokenColors yet")
			}
			vs := loadVSCodeTheme(t, filepath.Join(dir, themes[name]))
			tokens := tokenThemeFor(ttt)
			SetTokenTheme(tokens)
			defer SetTokenTheme(nil)
			type key struct{ scope, want, got string }
			mismatches := map[key]int{}
			for file, source := range paritySamples {
				h := New(file)
				if h == nil {
					t.Fatalf("no grammar for %s", file)
				}
				h.syncTokenTheme()
				lines := strings.Split(source, "\n")
				h.doc.SetLines(lines)
				for i, line := range lines {
					res, _ := h.doc.Line(i)
					for _, tok := range res.Tokens {
						if strings.TrimSpace(string([]rune(line)[tok.Start:tok.End])) == "" {
							continue
						}
						want, wantDefault := vs.resolve(tok.Scopes)
						def := styleDef(ttt, tokens, h.styleFor(tok.ScopeStack))
						got := strings.ToLower(def.Fg)
						gotDefault := got == "" || got == strings.ToLower(ttt.Default.Fg)
						if (wantDefault && gotDefault) || want == got {
							continue
						}
						mismatches[key{tok.Scopes[len(tok.Scopes)-1], want, got}]++
					}
				}
			}
			if len(mismatches) == 0 {
				return
			}
			var report []string
			for k, n := range mismatches {
				report = append(report, fmt.Sprintf("%-55s vscode=%s ttt=%s (%d)", k.scope, k.want, k.got, n))
			}
			sort.Strings(report)
			t.Errorf("%d scopes differ from VS Code:\n%s", len(report), strings.Join(report, "\n"))
		})
	}
}

// tokenThemeFor mirrors app.NewTokenTheme, which highlight cannot import.
func tokenThemeFor(theme config.ThemeConfig) *TokenTheme {
	var rules []TokenRule
	for _, entry := range theme.TokenColors {
		for _, selector := range entry.Scope {
			rules = append(rules, TokenRule{Selector: selector, Foreground: entry.Settings.Foreground, FontStyle: entry.Settings.FontStyle})
		}
	}
	return NewTokenTheme(rules)
}

// Keep in sync with the syntax fields of config.SyntaxStyles.
func styleDef(theme config.ThemeConfig, tokens *TokenTheme, style term.Style) config.StyleDef {
	if tokens != nil && style >= term.TokenStyle(0) {
		fg := tokens.Styles[style-term.TokenStyle(0)].Fg
		if fg == "" {
			fg = theme.Default.Fg
		}
		return config.StyleDef{Fg: fg}
	}
	s := theme.Syntax
	switch style {
	case term.StyleSyntaxComment:
		return s.Comment
	case term.StyleSyntaxString:
		return s.String
	case term.StyleSyntaxKeyword:
		return s.Keyword
	case term.StyleSyntaxNumber:
		return s.Number
	case term.StyleSyntaxOperator:
		return s.Operator
	case term.StyleSyntaxFunction:
		return s.Function
	case term.StyleSyntaxType:
		return s.Type
	case term.StyleSyntaxBuiltin:
		return s.Builtin
	case term.StyleSyntaxVariable:
		return s.Variable
	case term.StyleSyntaxPunctuation:
		return s.Punctuation
	case term.StyleSyntaxTag:
		return s.Tag
	case term.StyleSyntaxAttribute:
		return s.Attribute
	case term.StyleSyntaxRegexp:
		return s.Regexp
	case term.StyleSyntaxHeading:
		return s.Heading
	case term.StyleSyntaxBold:
		return s.Bold
	case term.StyleSyntaxItalic:
		return s.Italic
	case term.StyleSyntaxQuote:
		return s.Quote
	case term.StyleSyntaxInserted:
		return s.Inserted
	case term.StyleSyntaxDeleted:
		return s.Deleted
	case term.StyleSyntaxInvalid:
		return s.Invalid
	case term.StyleSyntaxControl:
		return s.Control
	case term.StyleSyntaxStorage:
		return s.Storage
	case term.StyleSyntaxConstant:
		return s.Constant
	case term.StyleSyntaxEscape:
		return s.Escape
	case term.StyleSyntaxParameter:
		return s.Parameter
	case term.StyleSyntaxProperty:
		return s.Property
	case term.StyleSyntaxSelf:
		return s.Self
	case term.StyleSyntaxNamespace:
		return s.Namespace
	case term.StyleSyntaxDecorator:
		return s.Decorator
	case term.StyleSyntaxLink:
		return s.Link
	case term.StyleSyntaxCode:
		return s.Code
	case term.StyleSyntaxInterpolation:
		return s.Interpolation
	case term.StyleSyntaxSelector:
		return s.Selector
	case term.StyleSyntaxReadonlyVariable:
		return s.ReadonlyVariable
	}
	return config.StyleDef{}
}

type vscodeRule struct {
	scope   string
	parents []string
	fg      string
	order   int
}

type vscodeTheme struct {
	rules    []vscodeRule
	fallback string
}

func loadVSCodeTheme(t *testing.T, path string) vscodeTheme {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Include     string            `json:"include"`
		Colors      map[string]string `json:"colors"`
		TokenColors []struct {
			Scope    json.RawMessage `json:"scope"`
			Settings struct {
				Foreground string `json:"foreground"`
			} `json:"settings"`
		} `json:"tokenColors"`
	}
	if err := json.Unmarshal(stripJSONC(data), &raw); err != nil {
		t.Fatal(err)
	}
	theme := vscodeTheme{fallback: normalizeHex(raw.Colors["editor.foreground"])}
	// An included theme's rules come first, so the including theme wins ties.
	if raw.Include != "" {
		base := loadVSCodeTheme(t, filepath.Join(filepath.Dir(path), raw.Include))
		theme.rules = base.rules
		if theme.fallback == "" {
			theme.fallback = base.fallback
		}
	}
	offset := len(theme.rules)
	for order, tc := range raw.TokenColors {
		var selectors []string
		var one string
		if json.Unmarshal(tc.Scope, &one) == nil {
			selectors = strings.Split(one, ",")
		} else {
			_ = json.Unmarshal(tc.Scope, &selectors)
		}
		if len(selectors) == 0 && tc.Settings.Foreground != "" && theme.fallback == "" {
			theme.fallback = normalizeHex(tc.Settings.Foreground)
		}
		for _, selector := range selectors {
			parts := strings.Fields(selector)
			if len(parts) == 0 || strings.Contains(selector, "-") && strings.Contains(selector, " -") {
				continue
			}
			theme.rules = append(theme.rules, vscodeRule{
				scope: parts[len(parts)-1], parents: parts[:len(parts)-1],
				fg: normalizeHex(tc.Settings.Foreground), order: offset + order,
			})
		}
	}
	return theme
}

// resolve applies VS Code's precedence: at each scope level the most specific
// matching rule wins, and a level without a match keeps the enclosing color.
func (v vscodeTheme) resolve(scopes []string) (color string, isDefault bool) {
	color, isDefault = v.fallback, true
	for i := range scopes {
		var matched []*vscodeRule
		for r := range v.rules {
			rule := &v.rules[r]
			if hasScopePrefix(scopes[i], rule.scope) && parentsMatch(scopes[:i], rule.parents) {
				matched = append(matched, rule)
			}
		}
		sort.SliceStable(matched, func(a, b int) bool {
			ka := [3]int{strings.Count(matched[a].scope, ".") + 1, len(matched[a].parents), matched[a].order}
			kb := [3]int{strings.Count(matched[b].scope, ".") + 1, len(matched[b].parents), matched[b].order}
			return ka[0] < kb[0] || ka[0] == kb[0] && (ka[1] < kb[1] || ka[1] == kb[1] && ka[2] < kb[2])
		})
		for _, rule := range matched {
			if rule.fg != "" {
				color, isDefault = rule.fg, rule.fg == v.fallback
			}
		}
	}
	return color, isDefault
}

func normalizeHex(color string) string {
	color = strings.ToLower(color)
	if len(color) == 4 {
		color = "#" + strings.Repeat(color[1:2], 2) + strings.Repeat(color[2:3], 2) + strings.Repeat(color[3:4], 2)
	}
	if len(color) > 7 {
		color = color[:7]
	}
	return color
}

var paritySamples = map[string]string{
	"a.ts": `import { Injectable } from "./core";
export enum Mode { Fast, Slow }
@Injectable()
export class Cart<T extends Item> implements Store {
  private readonly items: T[] = [];
  static count = 0;
  add(item: T, qty = 1): void {
    if (qty <= 0 || this.items.length > 99) throw new Error(` + "`bad ${qty}\\n`" + `);
    const total = this.items.reduce((sum, i) => sum + i.price, 0);
    let pattern = /ab+c/gi;
    return total > 10 ? void 0 : null;
  }
}`,
	"a.tsx": `export function App({ user }: Props) {
  const [open, setOpen] = useState(false);
  return (
    <Layout title="Home" onClose={() => setOpen(false)}>
      {open && <Modal user={user} />}
    </Layout>
  );
}`,
	"a.css": `.card > a:hover, #main::before {
  color: #fff;
  margin: 0 auto !important;
}
@media (max-width: 600px) { .card { display: none; } }`,
	"a.scss": `$gap: 8px;
.card {
  &:hover { color: red; }
  a, button { padding: $gap; }
}`,
	"a.html": `<!DOCTYPE html>
<div id="main" class="card" data-x='1'>
  <a href="/x">Link &amp; more</a>
  <!-- comment -->
</div>`,
	"a.go": `package main

import (
	"fmt"
	"runtime/debug"
)

type Point struct{ X, Y int }

func (p *Point) Move(dx int) error {
	if dx < 0 {
		return fmt.Errorf("bad %d\n", dx)
	}
	defer debug.FreeOSMemory()
	return nil
}`,
	"a.py": `import os
from typing import List

@dataclass
class Item(Base):
    """Docstring."""
    def total(self, qty: int = 1) -> float:
        if qty > 0 and not self.empty:
            return self.price * qty
        return None`,
	"a.md":   "# Title\n\nSome **bold**, *italic*, `code` and [a link](https://x.io).\n\n> quote\n\n- item\n\n```go\nfunc main() {}\n```",
	"a.json": `{"name": "x", "count": 3, "ok": true, "list": [null, 1.5]}`,
	"a.rs": `use std::io;
#[derive(Debug)]
pub struct Point { x: i32 }
impl Point {
    pub fn new(x: i32) -> Self { Self { x } }
}
fn main() { let v = vec![1, 2]; println!("{:?}", v); }`,
	"a.java": `package com.example;
import java.util.List;
@Override
public final class Main extends Base {
    private static final int MAX = 10;
    public void run(String[] args) { if (args.length > 0) return; }
}`,
	"a.cpp": `#include <vector>
namespace app {
template <typename T>
class Box { public: T value; };
int main() { auto v = std::vector<int>{1, 2}; return v.size() > 0 ? 0 : 1; }
}`,
	"a.yaml": `name: app
version: 1.2
enabled: true
items:
  - id: 1
    tags: [a, b]`,
	"a.sh": `#!/usr/bin/env bash
set -euo pipefail
for f in "$@"; do
  if [[ -f "$f" ]]; then echo "file: ${f##*/}"; fi
done`,
}

// stripJSONC removes comments and trailing commas, which VS Code theme files
// allow, while leaving string contents untouched.
func stripJSONC(data []byte) []byte {
	var out []byte
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case inString:
			out = append(out, c)
			if c == '\\' && i+1 < len(data) {
				i++
				out = append(out, data[i])
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			i--
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++
		default:
			out = append(out, c)
		}
	}
	return trailingCommas.ReplaceAll(out, []byte("$1"))
}

var trailingCommas = regexp.MustCompile(`,(\s*[}\]])`)
