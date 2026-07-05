package spell

import (
	"context"
	"reflect"
	"testing"
)

func TestParsePipeOutput(t *testing.T) {
	out := []byte(`@(#) International Ispell Version 3.1.20 (but really Aspell 0.60.8.2)
& helllo 3 1: hello, hell, he'll
& tesst 2 23: tests, test

# xqzt 8

& wrld 2 12: weld, world
`)
	got := parsePipeOutput(out)
	want := []Misspelling{
		{Line: 0, Col: 0, Word: "helllo", Suggestions: []string{"hello", "hell", "he'll"}},
		{Line: 0, Col: 22, Word: "tesst", Suggestions: []string{"tests", "test"}},
		{Line: 1, Col: 7, Word: "xqzt"},
		{Line: 2, Col: 11, Word: "wrld", Suggestions: []string{"weld", "world"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestParsePipeOutputBlankLinesOnly(t *testing.T) {
	out := []byte("@(#) banner\n\n\n\n")
	if got := parsePipeOutput(out); len(got) != 0 {
		t.Errorf("expected no misspellings, got %+v", got)
	}
}

func TestParseResultMalformed(t *testing.T) {
	for _, s := range []string{"", "&", "& word", "& word 3", "# word", "& word 3 x: a", "* ok", "& w 1 0: a"} {
		if m, ok := parseResult(s); ok {
			t.Errorf("parseResult(%q) unexpectedly ok: %+v", s, m)
		}
	}
}

func TestParseResultSuggestionCap(t *testing.T) {
	m, ok := parseResult("& word 12 5: a, b, c, d, e, f, g, h, i, j, k, l")
	if !ok || len(m.Suggestions) != maxSuggestions {
		t.Errorf("expected %d suggestions, got %+v ok=%v", maxSuggestions, m, ok)
	}
}

func TestModeForLanguage(t *testing.T) {
	cases := []struct {
		lang string
		mode string
		ok   bool
	}{
		{"", "", true},
		{"plaintext", "", true},
		{"markdown", "markdown", true},
		{"HTML", "html", true},
		{"XML", "sgml", true},
		{"TeX", "tex", true},
		{"Go", "", false},
		{"Python", "", false},
	}
	for _, c := range cases {
		mode, ok := ModeForLanguage(c.lang)
		if mode != c.mode || ok != c.ok {
			t.Errorf("ModeForLanguage(%q) = %q,%v want %q,%v", c.lang, mode, ok, c.mode, c.ok)
		}
	}
}

func TestCheckRealAspell(t *testing.T) {
	if !Available() {
		t.Skip("aspell not installed")
	}
	lines := []string{"helllo world", "", "a naïve tesst"}
	got, err := Check(context.Background(), lines, "en_US", "")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	byWord := map[string]Misspelling{}
	for _, m := range got {
		byWord[m.Word] = m
	}
	if m, ok := byWord["helllo"]; !ok || m.Line != 0 || m.Col != 0 || len(m.Suggestions) == 0 {
		t.Errorf("helllo: %+v ok=%v", m, ok)
	}
	if m, ok := byWord["tesst"]; !ok || m.Line != 2 || m.Col != 8 {
		t.Errorf("tesst: want line 2 col 8 (rune offset after naïve), got %+v ok=%v", m, ok)
	}
	if _, ok := byWord["world"]; ok {
		t.Error("world flagged as misspelled")
	}
}
