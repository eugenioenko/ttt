package textmate

import (
	"strconv"
	"strings"

	"github.com/eugenioenko/textmate-go/oniguruma"
)

type regexpSourceAnchorCache struct {
	a0g0 string
	a0g1 string
	a1g0 string
	a1g1 string
}

type regexpSource struct {
	source            string
	ruleID            ruleID
	hasAnchor         bool
	hasBackReferences bool
	anchorCache       *regexpSourceAnchorCache
}

func newRegExpSource(source string, id ruleID) *regexpSource {
	result := &regexpSource{source: source, ruleID: id}
	if source != "" {
		var output strings.Builder
		lastPushedPos := 0
		for pos := 0; pos < len(source); pos++ {
			if source[pos] != '\\' || pos+1 >= len(source) {
				continue
			}

			switch source[pos+1] {
			case 'z':
				output.WriteString(source[lastPushedPos:pos])
				output.WriteString(`$(?!\n)(?<!\n)`)
				lastPushedPos = pos + 2
			case 'A', 'G':
				result.hasAnchor = true
			}
			pos++
		}
		if lastPushedPos != 0 {
			output.WriteString(source[lastPushedPos:])
			result.source = output.String()
		}
	}

	if result.hasAnchor {
		result.anchorCache = result.buildAnchorCache()
	}
	result.hasBackReferences = hasNumericBackReference(result.source)
	return result
}

func (r *regexpSource) clone() *regexpSource {
	return newRegExpSource(r.source, r.ruleID)
}

func (r *regexpSource) setSource(source string) {
	if r.source == source {
		return
	}
	r.source = source
	if r.hasAnchor {
		r.anchorCache = r.buildAnchorCache()
	}
}

func (r *regexpSource) resolveBackReferences(lineText string, captures []oniguruma.Capture) string {
	lineRunes := []rune(lineText)
	return replaceNumericBackReferences(r.source, func(index int) string {
		if index < 0 || index >= len(captures) {
			return ""
		}
		capture := captures[index]
		if capture.Start < 0 || capture.End < capture.Start || capture.End > len(lineRunes) {
			return ""
		}
		return escapeRegExpCharacters(string(lineRunes[capture.Start:capture.End]))
	})
}

func (r *regexpSource) buildAnchorCache() *regexpSourceAnchorCache {
	var a0g0, a0g1, a1g0, a1g1 strings.Builder
	for pos := 0; pos < len(r.source); pos++ {
		ch := r.source[pos]
		if ch != '\\' || pos+1 >= len(r.source) {
			a0g0.WriteByte(ch)
			a0g1.WriteByte(ch)
			a1g0.WriteByte(ch)
			a1g1.WriteByte(ch)
			continue
		}

		next := r.source[pos+1]
		a0g0.WriteByte(ch)
		a0g1.WriteByte(ch)
		a1g0.WriteByte(ch)
		a1g1.WriteByte(ch)
		switch next {
		case 'A':
			a0g0.WriteRune('\uffff')
			a0g1.WriteRune('\uffff')
			a1g0.WriteByte('A')
			a1g1.WriteByte('A')
		case 'G':
			a0g0.WriteRune('\uffff')
			a0g1.WriteByte('G')
			a1g0.WriteRune('\uffff')
			a1g1.WriteByte('G')
		default:
			a0g0.WriteByte(next)
			a0g1.WriteByte(next)
			a1g0.WriteByte(next)
			a1g1.WriteByte(next)
		}
		pos++
	}
	return &regexpSourceAnchorCache{
		a0g0: a0g0.String(),
		a0g1: a0g1.String(),
		a1g0: a1g0.String(),
		a1g1: a1g1.String(),
	}
}

func (r *regexpSource) resolveAnchors(allowA, allowG bool) string {
	if !r.hasAnchor || r.anchorCache == nil {
		return r.source
	}
	if allowA {
		if allowG {
			return r.anchorCache.a1g1
		}
		return r.anchorCache.a1g0
	}
	if allowG {
		return r.anchorCache.a0g1
	}
	return r.anchorCache.a0g0
}

type regexpSourceListAnchorCache struct {
	a0g0 *compiledRule
	a0g1 *compiledRule
	a1g0 *compiledRule
	a1g1 *compiledRule
}

type regexpSourceList struct {
	items          []*regexpSource
	hasAnchors     bool
	cached         *compiledRule
	anchorCache    regexpSourceListAnchorCache
	scannerOptions []oniguruma.ScannerOption
}

func newRegExpSourceList(options ...oniguruma.ScannerOption) *regexpSourceList {
	return &regexpSourceList{scannerOptions: options}
}

func (l *regexpSourceList) dispose() {
	l.disposeCaches()
}

func (l *regexpSourceList) disposeCaches() {
	if l.cached != nil {
		l.cached.dispose()
		l.cached = nil
	}
	if l.anchorCache.a0g0 != nil {
		l.anchorCache.a0g0.dispose()
		l.anchorCache.a0g0 = nil
	}
	if l.anchorCache.a0g1 != nil {
		l.anchorCache.a0g1.dispose()
		l.anchorCache.a0g1 = nil
	}
	if l.anchorCache.a1g0 != nil {
		l.anchorCache.a1g0.dispose()
		l.anchorCache.a1g0 = nil
	}
	if l.anchorCache.a1g1 != nil {
		l.anchorCache.a1g1.dispose()
		l.anchorCache.a1g1 = nil
	}
}

func (l *regexpSourceList) push(item *regexpSource) {
	l.items = append(l.items, item)
	l.hasAnchors = l.hasAnchors || item.hasAnchor
}

func (l *regexpSourceList) unshift(item *regexpSource) {
	l.items = append(l.items, nil)
	copy(l.items[1:], l.items)
	l.items[0] = item
	l.hasAnchors = l.hasAnchors || item.hasAnchor
}

func (l *regexpSourceList) length() int {
	return len(l.items)
}

func (l *regexpSourceList) setSource(index int, source string) {
	if l.items[index].source == source {
		return
	}
	l.disposeCaches()
	l.items[index].setSource(source)
}

func (l *regexpSourceList) compile() *compiledRule {
	if l.cached == nil {
		l.cached = l.compileResolved(false, false, false)
	}
	return l.cached
}

func (l *regexpSourceList) compileAG(allowA, allowG bool) *compiledRule {
	if !l.hasAnchors {
		return l.compile()
	}
	if allowA {
		if allowG {
			if l.anchorCache.a1g1 == nil {
				l.anchorCache.a1g1 = l.compileResolved(true, allowA, allowG)
			}
			return l.anchorCache.a1g1
		}
		if l.anchorCache.a1g0 == nil {
			l.anchorCache.a1g0 = l.compileResolved(true, allowA, allowG)
		}
		return l.anchorCache.a1g0
	}
	if allowG {
		if l.anchorCache.a0g1 == nil {
			l.anchorCache.a0g1 = l.compileResolved(true, allowA, allowG)
		}
		return l.anchorCache.a0g1
	}
	if l.anchorCache.a0g0 == nil {
		l.anchorCache.a0g0 = l.compileResolved(true, allowA, allowG)
	}
	return l.anchorCache.a0g0
}

func (l *regexpSourceList) compileResolved(resolveAnchors, allowA, allowG bool) *compiledRule {
	sources := make([]string, len(l.items))
	rules := make([]ruleID, len(l.items))
	for i, item := range l.items {
		sources[i] = item.source
		if resolveAnchors {
			sources[i] = item.resolveAnchors(allowA, allowG)
		}
		rules[i] = item.ruleID
	}
	return newCompiledRule(sources, rules, l.scannerOptions...)
}

type compiledRule struct {
	scanner *oniguruma.OnigScanner
	regexps []string
	rules   []ruleID
}

func newCompiledRule(
	regexps []string,
	rules []ruleID,
	options ...oniguruma.ScannerOption,
) *compiledRule {
	return &compiledRule{
		scanner: oniguruma.NewScanner(regexps, options...),
		regexps: append([]string(nil), regexps...),
		rules:   append([]ruleID(nil), rules...),
	}
}

// dispose mirrors vscode-oniguruma's optional scanner cleanup. The pure-Go
// scanner owns no resources outside the garbage-collected heap.
func (r *compiledRule) dispose() {}

func (r *compiledRule) String() string {
	var result strings.Builder
	for i := range r.rules {
		if i != 0 {
			result.WriteByte('\n')
		}
		result.WriteString("   - ")
		result.WriteString(strconv.Itoa(int(r.rules[i])))
		result.WriteString(": ")
		result.WriteString(r.regexps[i])
	}
	return result.String()
}

type findNextMatchResult struct {
	ruleID         ruleID
	captureIndices []oniguruma.Capture
}

func (r *compiledRule) findNextMatch(
	input *oniguruma.String,
	startPosition int,
	options oniguruma.FindOption,
	dst []oniguruma.Capture,
) (findNextMatchResult, bool) {
	match, ok := r.scanner.FindNextMatchInto(input, startPosition, options, dst)
	if !ok {
		return findNextMatchResult{}, false
	}
	return findNextMatchResult{
		ruleID:         r.rules[match.Index],
		captureIndices: match.Captures,
	}, true
}

func hasNumericBackReference(source string) bool {
	for pos := 0; pos+1 < len(source); pos++ {
		if source[pos] == '\\' && source[pos+1] >= '0' && source[pos+1] <= '9' {
			return true
		}
	}
	return false
}

func replaceNumericBackReferences(source string, value func(int) string) string {
	var result strings.Builder
	last := 0
	for pos := 0; pos+1 < len(source); pos++ {
		if source[pos] != '\\' || source[pos+1] < '0' || source[pos+1] > '9' {
			continue
		}
		end := pos + 2
		for end < len(source) && source[end] >= '0' && source[end] <= '9' {
			end++
		}
		index, _ := strconv.Atoi(source[pos+1 : end])
		result.WriteString(source[last:pos])
		result.WriteString(value(index))
		last = end
		pos = end - 1
	}
	if last == 0 {
		return source
	}
	result.WriteString(source[last:])
	return result.String()
}

func escapeRegExpCharacters(value string) string {
	var result strings.Builder
	for _, ch := range value {
		switch ch {
		case '\\', '|', '(', '[', '{', '}', ']', ')', '.', '?', '*', '+', '^', '$':
			result.WriteByte('\\')
		}
		result.WriteRune(ch)
	}
	return result.String()
}
