package plugin

import (
	"strings"
	"testing"
)

func runTextScript(t *testing.T, script string) string {
	t.Helper()
	p, cleanup := newTestPluginBase(PermissionSet{})
	defer cleanup()
	setupTextModule(p.State)
	if err := p.State.DoString(`local text = require("ttt.text")` + "\n" + script); err != nil {
		t.Fatal(err)
	}
	return p.State.GetGlobal("result").String()
}

func TestTextWidth(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want string
	}{
		{"", "0"},
		{"abc", "3"},
		{"日本語", "6"},
		{"a日b", "4"},
		{"ｈｅｌｌｏ", "10"},
		{"héllo", "5"},
	} {
		if got := runTextScript(t, `result = text.width("`+tt.in+`")`); got != tt.want {
			t.Errorf("width(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestTextTruncate(t *testing.T) {
	for _, tt := range []struct {
		call string
		want string
	}{
		{`text.truncate("hello", 10)`, "hello"},
		{`text.truncate("hello", 5)`, "hello"},
		{`text.truncate("hello world", 5)`, "hello"},
		{`text.truncate("hello world", 6, "…")`, "hello…"},
		{`text.truncate("日本語テキスト", 5)`, "日本"},
		{`text.truncate("日本語テキスト", 6, "…")`, "日本…"},
		{`text.truncate("a日本", 2)`, "a"},
		{`text.truncate("hello", 0)`, ""},
		{`text.truncate("hello", 2, "...")`, "he"},
	} {
		if got := runTextScript(t, `result = `+tt.call); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.call, got, tt.want)
		}
	}
}

func TestTextWrap(t *testing.T) {
	for _, tt := range []struct {
		call string
		want []string
	}{
		{`text.wrap("hello world", 20)`, []string{"hello world"}},
		{`text.wrap("hello world", 5)`, []string{"hello", "world"}},
		{`text.wrap("the quick brown fox", 10)`, []string{"the quick", "brown fox"}},
		{`text.wrap("abcdefghij", 4)`, []string{"abcd", "efgh", "ij"}},
		{`text.wrap("日本語のテキスト", 5)`, []string{"日本", "語の", "テキ", "スト"}},
		{`text.wrap("日本 語", 4)`, []string{"日本", "語"}},
		{`text.wrap("a日", 2)`, []string{"a", "日"}},
		{`text.wrap("日", 1)`, []string{"日"}},
		{`text.wrap("one\ntwo three", 5)`, []string{"one", "two", "three"}},
		{`text.wrap("", 5)`, []string{""}},
		{`text.wrap("a\n\nb", 5)`, []string{"a", "", "b"}},
		{`text.wrap("abc ", 3)`, []string{"abc"}},
	} {
		got := runTextScript(t, `result = table.concat(`+tt.call+`, "|")`)
		if want := strings.Join(tt.want, "|"); got != want {
			t.Errorf("%s = %q, want %q", tt.call, got, want)
		}
	}
}

func TestTextInvalidWidth(t *testing.T) {
	for _, call := range []string{
		`text.wrap("x", 0)`,
		`text.truncate("x", -1)`,
		`text.width()`,
	} {
		p, cleanup := newTestPluginBase(PermissionSet{})
		setupTextModule(p.State)
		if err := p.State.DoString(`local text = require("ttt.text"); ` + call); err == nil {
			t.Errorf("%s: expected an error", call)
		}
		cleanup()
	}
}
