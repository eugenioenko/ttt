package plugin

import "github.com/eugenioenko/ttt/internal/term"

type styleEntry struct {
	Name  string
	Style term.Style
}

var pluginStyles = []styleEntry{
	{"default", term.StyleDefault},
	{"muted", term.StyleMuted},
	{"border", term.StyleBorder},
	{"success", term.StyleSuccess},
	{"danger", term.StyleDanger},
	{"warning", term.StyleWarning},
	{"selected", term.StyleSidebarSelected},
	{"item", term.StylePaletteItem},
	{"line", term.StyleLineNumber},
	{"input", term.StyleInput},
	{"bold", term.StyleHoverBold},
	{"italic", term.StyleHoverItalic},
	{"code", term.StyleHoverCode},
	{"syntax_comment", term.StyleSyntaxComment},
	{"syntax_string", term.StyleSyntaxString},
	{"syntax_keyword", term.StyleSyntaxKeyword},
	{"syntax_number", term.StyleSyntaxNumber},
	{"syntax_operator", term.StyleSyntaxOperator},
	{"syntax_function", term.StyleSyntaxFunction},
	{"syntax_type", term.StyleSyntaxType},
	{"syntax_builtin", term.StyleSyntaxBuiltin},
	{"syntax_variable", term.StyleSyntaxVariable},
	{"syntax_tag", term.StyleSyntaxTag},
	{"syntax_attribute", term.StyleSyntaxAttribute},
	{"syntax_regexp", term.StyleSyntaxRegexp},
	{"syntax_heading", term.StyleSyntaxHeading},
	{"syntax_bold", term.StyleSyntaxBold},
	{"syntax_italic", term.StyleSyntaxItalic},
	{"syntax_quote", term.StyleSyntaxQuote},
	{"syntax_inserted", term.StyleSyntaxInserted},
	{"syntax_deleted", term.StyleSyntaxDeleted},
	{"syntax_invalid", term.StyleSyntaxInvalid},
	{"syntax_control", term.StyleSyntaxControl},
	{"syntax_storage", term.StyleSyntaxStorage},
	{"syntax_constant", term.StyleSyntaxConstant},
	{"syntax_escape", term.StyleSyntaxEscape},
	{"syntax_parameter", term.StyleSyntaxParameter},
	{"syntax_property", term.StyleSyntaxProperty},
	{"syntax_self", term.StyleSyntaxSelf},
	{"syntax_namespace", term.StyleSyntaxNamespace},
	{"syntax_decorator", term.StyleSyntaxDecorator},
	{"syntax_link", term.StyleSyntaxLink},
	{"syntax_code", term.StyleSyntaxCode},
	{"syntax_interpolation", term.StyleSyntaxInterpolation},
}

var styleByName map[string]term.Style
var nameByStyle map[term.Style]string

func init() {
	styleByName = make(map[string]term.Style, len(pluginStyles))
	nameByStyle = make(map[term.Style]string, len(pluginStyles))
	for _, e := range pluginStyles {
		styleByName[e.Name] = e.Style
		nameByStyle[e.Style] = e.Name
	}
}

func StyleByName(name string) (term.Style, bool) {
	s, ok := styleByName[name]
	return s, ok
}

func NameByStyle(s term.Style) string {
	if name, ok := nameByStyle[s]; ok {
		return name
	}
	return "default"
}
