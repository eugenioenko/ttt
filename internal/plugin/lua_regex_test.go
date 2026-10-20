package plugin

import (
	"strconv"
	"testing"
)

// runRegexScript runs a script in a plugin with no permissions and returns the
// global `result`.
func runRegexScript(t *testing.T, script string) string {
	t.Helper()
	p, cleanup := newTestPluginBase(PermissionSet{})
	defer cleanup()
	setupRegexModule(p.State)
	if err := p.State.DoString(`local regex = require("ttt.regex")` + "\n" + script); err != nil {
		t.Fatal(err)
	}
	return p.State.GetGlobal("result").String()
}

func TestRegexCompileInvalidPattern(t *testing.T) {
	got := runRegexScript(t, `
		local re, err = regex.compile("a(b")
		result = tostring(re) .. " " .. err
	`)
	if got != "nil error parsing regexp: missing closing ): `a(b`" {
		t.Errorf("got %q", got)
	}
}

func TestRegexFind(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{"match", `s, e = re:find("say foo or bar")`, "5 7"},
		{"alternation", `s, e = re:find("say bar")`, "5 7"},
		{"no match", `s, e = re:find("baz")`, "nil nil"},
		{"init", `s, e = re:find("foo foo", 2)`, "5 7"},
		{"init at end", `s, e = re:find("foo", 4)`, "nil nil"},
		{"init past end", `s, e = re:find("foo", 5)`, "nil nil"},
		{"negative init", `s, e = re:find("foo foo", -3)`, "5 7"},
		{"multi-byte", `s, e = re:find("日本 foo")`, "8 10"},
		{"huge init", `s, e = re:find("foo", 1/0)`, "nil nil"},
		{"nan init", `s, e = re:find("foo", 0/0)`, "nil nil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runRegexScript(t, `
				local re = regex.compile("foo|bar")
				`+tt.script+`
				result = tostring(s) .. " " .. tostring(e)
			`)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A search from init still sees the character before it, as a forward search
// from the cursor needs
func TestRegexFindKeepsContextBeforeInit(t *testing.T) {
	tests := []struct {
		pattern, text string
		init          int
		want          string
	}{
		{`\\bbar`, "foobar bar", 4, "8 10"},
		{`\\Bbar`, "foobar bar", 4, "4 6"},
		{`^foo`, "xfoo", 2, "nil nil"},
		{`(?m)^foo`, "x\\nfoo", 3, "3 5"},
		{`aa`, "aaa", 2, "2 3"},
		{`\\bfoo`, "éfoo", 3, "3 5"},
		{`.`, "é", 2, "2 2"},
	}
	for _, tt := range tests {
		got := runRegexScript(t, `
			local re = regex.compile("`+tt.pattern+`")
			local s, e = re:find("`+tt.text+`", `+strconv.Itoa(tt.init)+`)
			result = tostring(s) .. " " .. tostring(e)
		`)
		if got != tt.want {
			t.Errorf("%s in %q from %d: got %q, want %q", tt.pattern, tt.text, tt.init, got, tt.want)
		}
	}
}

func TestRegexFindMatchesStringFindOffsets(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile("é+")
		local s, e = re:find("café éé")
		local ls, le = string.find("café éé", "éé")
		result = s .. " " .. e .. " " .. ls .. " " .. le
	`)
	if got != "4 5 7 10" {
		t.Errorf("got %q", got)
	}
}

func TestRegexFindCaptures(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile("(\\w+)@(\\w+)(\\.com)?")
		local s, e, caps = re:find("mail bob@example now")
		result = s .. " " .. e .. " " .. #caps .. " " .. caps[1] .. " " .. caps[2] .. " " .. tostring(caps[3])
	`)
	if got != "6 16 3 bob example false" {
		t.Errorf("got %q", got)
	}
}

func TestRegexFindMultiByteCapture(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile("(\\p{Han}+) (é+)")
		local s, e, caps = re:find("x 日本 éé")
		result = s .. " " .. e .. " " .. caps[1] .. " " .. caps[2]
	`)
	if got != "3 13 日本 éé" {
		t.Errorf("got %q", got)
	}
}

func TestRegexFindEmptyMatch(t *testing.T) {
	got := runRegexScript(t, `
		local s, e = regex.compile("x*"):find("abc", 2)
		result = s .. " " .. e
	`)
	if got != "2 1" {
		t.Errorf("got %q", got)
	}
}

func TestRegexFindAll(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile("(\\d+)")
		local parts = {}
		for _, m in ipairs(re:find_all("a1 b22 c333")) do
			parts[#parts + 1] = m.s .. "-" .. m.e .. ":" .. m.caps[1]
		end
		result = table.concat(parts, ",")
	`)
	if got != "2-2:1,5-6:22,9-11:333" {
		t.Errorf("got %q", got)
	}
}

func TestRegexFindAllLimit(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile("\\d")
		result = #re:find_all("1 2 3 4", 2)
	`)
	if got != "2" {
		t.Errorf("got %q", got)
	}
}

func TestRegexFindAllEmptyMatches(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile("x*")
		local parts = {}
		for _, m in ipairs(re:find_all("axb")) do
			parts[#parts + 1] = m.s .. "-" .. m.e
		end
		result = table.concat(parts, ",")
	`)
	if got != "1-0,2-2,4-3" {
		t.Errorf("got %q", got)
	}
}

func TestRegexIgnoreCase(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile("hello", { ignore_case = true })
		local s = re:find("Say HeLLo")
		local strict = regex.compile("hello")
		result = s .. " " .. tostring(strict:find("Say HeLLo"))
	`)
	if got != "5 nil" {
		t.Errorf("got %q", got)
	}
}

func TestRegexIsReusable(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile("o")
		local count = 0
		for _, line in ipairs({"one", "two", "three"}) do
			if re:find(line) then count = count + 1 end
		end
		result = count
	`)
	if got != "2" {
		t.Errorf("got %q", got)
	}
}

func TestRegexModuleLoadsInPlugins(t *testing.T) {
	p := &Plugin{Name: "regex", Enabled: true}
	if err := p.InitFromSource(`
		local regex = require("ttt.regex")
		result = regex.compile("b+"):find("abbc")
	`); err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	if got := p.State.GetGlobal("result").String(); got != "2" {
		t.Errorf("got %q", got)
	}
}

func TestRegexFindInsideCharacterContext(t *testing.T) {
	tests := []struct {
		pattern, text string
		init          int
		want          string
	}{
		{`^.`, "é", 2, "nil nil"},
		{`^.`, "€x", 3, "nil nil"},
		{`\A.`, "é", 2, "nil nil"},
		{`\b.`, "é", 2, "nil nil"},
		{`\B.`, "é", 2, "nil nil"},
		{`\B.`, "€", 2, "nil nil"},
		{`\B.`, "€", 3, "nil nil"},
		{`(?m)^x`, "é\nx", 2, "4 4"},
		{`(?m)^x`, "€\nx", 2, "5 5"},
		{`(?m)^x`, "€\nx", 3, "5 5"},
		{`^z|x`, "éx", 2, "3 3"},
		{`\bfoo`, "éfoo", 2, "3 5"},
		{`\Bbar`, "€foobar", 3, "7 9"},
		{`\bx`, "🙂x", 4, "5 5"},
		{`\B`, "€", 3, "4 3"},
		{`.`, "€", 2, "2 2"},
		{`.`, "€", 3, "3 3"},
		{`[^a]`, "€", 3, "3 3"},
	}
	for _, tt := range tests {
		got := runRegexScript(t, `
			local re = regex.compile([=[`+tt.pattern+`]=])
			local s, e = re:find(`+strconv.Quote(tt.text)+`, `+strconv.Itoa(tt.init)+`)
			result = tostring(s) .. " " .. tostring(e)
		`)
		if got != tt.want {
			t.Errorf("%s in %q from %d: got %q, want %q", tt.pattern, tt.text, tt.init, got, tt.want)
		}
	}
}

func TestRegexFindInsideCharacterCaptures(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile([[(?m)^(x)(y)?]])
		local s, e, caps = re:find("€\nx", 3)
		result = s .. " " .. e .. " " .. caps[1] .. " " .. tostring(caps[2])
	`)
	if got != "5 5 x false" {
		t.Errorf("got %q", got)
	}
}

func TestRegexFindInvalidByteKeepsContext(t *testing.T) {
	got := runRegexScript(t, `
		local re = regex.compile([[\B.]])
		local s, e = re:find("é" .. string.char(128), 3)
		result = tostring(s) .. " " .. tostring(e)
	`)
	if got != "3 3" {
		t.Errorf("got %q", got)
	}
}
