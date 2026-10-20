package plugin

import (
	"regexp"
	"regexp/syntax"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"
)

const regexTypeName = "ttt.regex"

func setupRegexModule(L *lua.LState) {
	loader := func(L *lua.LState) int {
		mt := L.NewTypeMetatable(regexTypeName)
		L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
			"find":     regexFind,
			"find_all": regexFindAll,
		}))
		mod := L.NewTable()
		L.SetField(mod, "compile", L.NewFunction(regexCompile))
		L.Push(mod)
		return 1
	}
	L.PreloadModule("ttt.regex", loader)
}

// luaRegex is the value behind a compiled regex. after is the pattern with
// one character in front, for findFrom. contextual is set when the pattern
// has an anchor or word boundary, which depends on the text before the match.
type luaRegex struct {
	re         *regexp.Regexp
	after      *regexp.Regexp
	contextual bool
}

func regexCompile(L *lua.LState) int {
	pattern := L.CheckString(1)
	if opts := L.OptTable(2, nil); opts != nil && lua.LVAsBool(opts.RawGetString("ignore_case")) {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	after, err := regexp.Compile("(?s:.)(?:" + pattern + ")")
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	ud := L.NewUserData()
	ud.Value = &luaRegex{re: re, after: after, contextual: hasContextOp(parsed)}
	L.SetMetatable(ud, L.GetTypeMetatable(regexTypeName))
	L.Push(ud)
	return 1
}

func checkRegex(L *lua.LState) *luaRegex {
	ud := L.CheckUserData(1)
	if re, ok := ud.Value.(*luaRegex); ok {
		return re
	}
	L.ArgError(1, "regex expected")
	return nil
}

// regexFind returns the first match that starts at or after byte offset init.
// init is 1-based and a negative init counts from the end, as in string.find.
func regexFind(L *lua.LState) int {
	r := checkRegex(L)
	text := L.CheckString(2)
	init := float64(L.OptNumber(3, 1))
	if !(init <= float64(len(text)+1)) {
		L.Push(lua.LNil)
		return 1
	}
	if init < 0 {
		init = max(float64(len(text))+init+1, 1)
	}
	start := max(int(init), 1) - 1

	loc := r.findFrom(text, start)
	if loc == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(loc[0] + 1))
	L.Push(lua.LNumber(loc[1]))
	L.Push(regexCaptures(L, text, loc))
	return 3
}

// findFrom returns the first match that starts at or after byte offset start.
// It matches from the character before start, so that ^, \b and \B see it,
// unless start is the beginning of text or inside a character. Inside a
// character, a contextual pattern has no match, because the context it
// depends on is gone; other patterns keep matching from the raw offset.
func (r *luaRegex) findFrom(text string, start int) []int {
	if start > 0 {
		_, w := utf8.DecodeLastRuneInString(text[:start])
		base := start - w
		if _, fw := utf8.DecodeRuneInString(text[base:]); base+fw == start {
			loc := shiftedMatch(r.after, text, base)
			if loc != nil {
				_, dw := utf8.DecodeRuneInString(text[loc[0]:])
				loc[0] += dw
			}
			return loc
		}
		if r.contextual {
			return nil
		}
	}
	return shiftedMatch(r.re, text, start)
}

func hasContextOp(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpBeginLine, syntax.OpBeginText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return true
	}
	for _, sub := range re.Sub {
		if hasContextOp(sub) {
			return true
		}
	}
	return false
}

// shiftedMatch matches re against text[start:] and returns offsets into text.
func shiftedMatch(re *regexp.Regexp, text string, start int) []int {
	loc := re.FindStringSubmatchIndex(text[start:])
	for i := range loc {
		if loc[i] >= 0 {
			loc[i] += start
		}
	}
	return loc
}

func regexFindAll(L *lua.LState) int {
	r := checkRegex(L)
	text := L.CheckString(2)
	n := L.OptInt(3, -1)
	matches := L.NewTable()
	for _, loc := range r.re.FindAllStringSubmatchIndex(text, n) {
		m := L.NewTable()
		L.SetField(m, "s", lua.LNumber(loc[0]+1))
		L.SetField(m, "e", lua.LNumber(loc[1]))
		L.SetField(m, "caps", regexCaptures(L, text, loc))
		matches.Append(m)
	}
	L.Push(matches)
	return 1
}

// regexCaptures lists the capture groups of a match, with false for a group
// that did not take part, so the table has no holes.
func regexCaptures(L *lua.LState, text string, loc []int) *lua.LTable {
	caps := L.NewTable()
	for i := 2; i < len(loc); i += 2 {
		if loc[i] < 0 {
			caps.Append(lua.LFalse)
		} else {
			caps.Append(lua.LString(text[loc[i]:loc[i+1]]))
		}
	}
	return caps
}
