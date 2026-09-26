package config

import (
	"encoding/json"
	"testing"

	"github.com/eugenioenko/ttt/internal/config/themes"
)

func TestFineSyntaxSlotsInheritTheirParent(t *testing.T) {
	th := ThemeConfig{
		Default: StyleDef{Fg: "#dddddd"},
		Danger:  StyleDef{Fg: "#ff0000"},
		Syntax: SyntaxStyles{
			Comment:     StyleDef{Fg: "#000001"},
			String:      StyleDef{Fg: "#000002"},
			Keyword:     StyleDef{Fg: "#000003", Italic: true},
			Function:    StyleDef{Fg: "#000006"},
			Type:        StyleDef{Fg: "#000007"},
			Variable:    StyleDef{Fg: "#000009"},
			Punctuation: StyleDef{Fg: "#00000a"},
		},
	}
	th.ResolveColors()
	s := th.Syntax

	for name, tc := range map[string]struct{ got, want StyleDef }{
		"regexp":        {s.Regexp, s.String},
		"quote":         {s.Quote, s.Comment},
		"control":       {s.Control, s.Keyword},
		"storage":       {s.Storage, s.Keyword},
		"constant":      {s.Constant, s.Keyword},
		"self":          {s.Self, s.Keyword},
		"escape":        {s.Escape, s.String},
		"parameter":     {s.Parameter, s.Variable},
		"property":      {s.Property, s.Variable},
		"namespace":     {s.Namespace, s.Type},
		"decorator":     {s.Decorator, s.Function},
		"link":          {s.Link, s.String},
		"code":          {s.Code, s.String},
		"interpolation": {s.Interpolation, s.Punctuation},
		"inserted":      {s.Inserted, th.Diff.Added},
		"deleted":       {s.Deleted, th.Diff.Deleted},
		"heading":       {s.Heading, StyleDef{Fg: "#000003", Italic: true, Bold: true}},
		"bold":          {s.Bold, StyleDef{Fg: "#dddddd", Bold: true}},
		"italic":        {s.Italic, StyleDef{Fg: "#dddddd", Italic: true}},
		"invalid":       {s.Invalid, StyleDef{Fg: "#ff0000"}},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %+v, want %+v", name, tc.got, tc.want)
		}
	}
}

func TestFineSyntaxSlotSetByThemeIsKept(t *testing.T) {
	th := ThemeConfig{Syntax: SyntaxStyles{
		Keyword: StyleDef{Fg: "#111111"},
		Control: StyleDef{Fg: "#222222"},
		Heading: StyleDef{Fg: "#333333"},
	}}
	th.ResolveColors()
	if th.Syntax.Control != (StyleDef{Fg: "#222222"}) {
		t.Errorf("control = %+v, want the theme's own value", th.Syntax.Control)
	}
	if th.Syntax.Heading != (StyleDef{Fg: "#333333"}) {
		t.Errorf("heading = %+v, want the theme's own value without implied bold", th.Syntax.Heading)
	}
}

func TestSampleThemesSetFineSyntaxSlots(t *testing.T) {
	for _, name := range []string{"default-dark.json", "vscode-dark-plus.json"} {
		data, err := themes.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var raw struct {
			Syntax map[string]StyleDef `json:"syntax"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{
			"regexp", "heading", "bold", "italic", "quote", "inserted", "deleted", "invalid",
			"control", "storage", "constant", "escape", "parameter", "property", "self",
			"namespace", "decorator", "link", "code", "interpolation",
		} {
			if raw.Syntax[key] == (StyleDef{}) {
				t.Errorf("%s: syntax.%s is not set", name, key)
			}
		}
	}
}
