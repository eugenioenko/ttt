package lsp

// InsertTextFormat values from the LSP CompletionItem spec.
const (
	InsertTextFormatPlainText = 1
	InsertTextFormatSnippet   = 2
)

// SnippetStop is a tab stop in an expanded snippet. Offset and Length are rune
// offsets into SnippetExpansion.Text.
type SnippetStop struct {
	Index  int
	Offset int
	Length int
}

type SnippetExpansion struct {
	Text  string
	Stops []SnippetStop
}

// FirstStop returns the stop the cursor belongs to after insertion: the lowest
// numbered stop above zero, falling back to $0, then to no stop.
func (e SnippetExpansion) FirstStop() (SnippetStop, bool) {
	var best SnippetStop
	found := false
	for _, stop := range e.Stops {
		if stop.Index == 0 {
			continue
		}
		if !found || stop.Index < best.Index || (stop.Index == best.Index && stop.Offset < best.Offset) {
			best = stop
			found = true
		}
	}
	if found {
		return best, true
	}
	for _, stop := range e.Stops {
		if stop.Index == 0 {
			return stop, true
		}
	}
	return SnippetStop{}, false
}

// ExpandSnippet turns an LSP snippet into plain text and the stops it
// contains. Variables and mirrored stops are left as text; choices collapse
// to their first option.
func ExpandSnippet(src string) SnippetExpansion {
	p := snippetParser{src: []rune(src)}
	text, stops, _ := p.segment(false)
	return SnippetExpansion{Text: string(text), Stops: stops}
}

type snippetParser struct {
	src []rune
	i   int
	// unclosed holds start offsets of braced defaults that never closed, so a
	// malformed snippet cannot be reparsed from the same position.
	unclosed map[int]bool
}

func (p *snippetParser) segment(stopAtBrace bool) ([]rune, []SnippetStop, bool) {
	var out []rune
	var stops []SnippetStop
	for p.i < len(p.src) {
		r := p.src[p.i]
		if stopAtBrace && r == '}' {
			return out, stops, true
		}
		if r == '\\' && p.i+1 < len(p.src) {
			next := p.src[p.i+1]
			if next == '$' || next == '}' || next == '\\' {
				out = append(out, next)
				p.i += 2
				continue
			}
		}
		if r == '$' {
			text, placeholderStops, ok := p.placeholder()
			if ok {
				base := len(out)
				out = append(out, text...)
				for _, stop := range placeholderStops {
					stop.Offset += base
					stops = append(stops, stop)
				}
				continue
			}
		}
		out = append(out, r)
		p.i++
	}
	return out, stops, false
}

func (p *snippetParser) placeholder() ([]rune, []SnippetStop, bool) {
	start := p.i
	p.i++ // '$'
	if p.i >= len(p.src) {
		p.i = start
		return nil, nil, false
	}
	if p.src[p.i] == '{' {
		return p.bracedPlaceholder(start)
	}
	index, ok := p.number()
	if !ok {
		p.i = start
		return nil, nil, false
	}
	return nil, []SnippetStop{{Index: index}}, true
}

func (p *snippetParser) bracedPlaceholder(start int) ([]rune, []SnippetStop, bool) {
	if p.unclosed[start] {
		p.i = start
		return nil, nil, false
	}
	p.i++ // '{'
	index, ok := p.number()
	if !ok || p.i >= len(p.src) {
		p.i = start
		return nil, nil, false
	}
	switch p.src[p.i] {
	case '}':
		p.i++
		return nil, []SnippetStop{{Index: index}}, true
	case ':':
		p.i++
		text, childStops, closed := p.segment(true)
		if !closed {
			if p.unclosed == nil {
				p.unclosed = make(map[int]bool)
			}
			p.unclosed[start] = true
			p.i = start
			return nil, nil, false
		}
		p.i++ // '}'
		stops := make([]SnippetStop, 0, 1+len(childStops))
		stops = append(stops, SnippetStop{Index: index, Length: len(text)})
		stops = append(stops, childStops...)
		return text, stops, true
	case '|':
		return p.choice(start, index)
	default:
		p.i = start
		return nil, nil, false
	}
}

func (p *snippetParser) choice(start, index int) ([]rune, []SnippetStop, bool) {
	p.i++ // '|'
	choiceStart := p.i
	for p.i < len(p.src) {
		if p.src[p.i] == '\\' && p.i+1 < len(p.src) {
			p.i += 2
			continue
		}
		if p.src[p.i] == '|' && p.i+1 < len(p.src) && p.src[p.i+1] == '}' {
			raw := p.src[choiceStart:p.i]
			p.i += 2
			text := firstChoice(raw)
			return text, []SnippetStop{{Index: index, Length: len(text)}}, true
		}
		p.i++
	}
	p.i = start
	return nil, nil, false
}

func firstChoice(raw []rune) []rune {
	var text []rune
	for i := 0; i < len(raw); i++ {
		r := raw[i]
		if r == '\\' && i+1 < len(raw) {
			next := raw[i+1]
			switch next {
			case ',', '|', '\\', '}', '$':
				text = append(text, next)
				i++
				continue
			}
		}
		if r == ',' {
			break
		}
		text = append(text, r)
	}
	return text
}

func (p *snippetParser) number() (int, bool) {
	start := p.i
	n := 0
	for p.i < len(p.src) {
		c := p.src[p.i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
		p.i++
	}
	return n, p.i > start
}
