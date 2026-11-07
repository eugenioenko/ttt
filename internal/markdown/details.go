package markdown

import (
	"regexp"
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	gmtext "github.com/yuin/goldmark/text"
)

var (
	detailsStartRegex = regexp.MustCompile(`(?i)^<details(\s+[^>]*)?>`)
	detailsCloseRegex = regexp.MustCompile(`(?i)</details\s*>`)
	summaryStartRegex = regexp.MustCompile(`(?i)^<summary(\s+[^>]*)?>`)
	summaryCloseRegex = regexp.MustCompile(`(?i)</summary\s*>`)
	openAttrRegex     = regexp.MustCompile(`(?i)\bopen(\b|=)`)
	htmlTagRegex      = regexp.MustCompile(`<[^>]+>`)
)

type DetailBlock struct {
	ID           int
	Open         bool
	DefaultOpen  bool
	Summary      string
	SummarySpans []Span
	Content      *Document
}

type BlockKind int

const (
	BlockContent BlockKind = iota
	BlockDetail
)

type Block struct {
	Kind   BlockKind
	Lines  []Line
	Detail *DetailBlock
}

type Document struct {
	Blocks  []Block
	details []*DetailBlock
}

func (d *Document) Details() []*DetailBlock {
	if d == nil {
		return nil
	}
	return d.details
}

func (d *Document) Toggle(id int) bool {
	if d == nil {
		return false
	}
	for _, det := range d.details {
		if det.ID == id {
			det.Open = !det.Open
			return true
		}
	}
	return false
}

func (d *Document) SetOpen(id int, open bool) bool {
	if d == nil {
		return false
	}
	for _, det := range d.details {
		if det.ID == id {
			det.Open = open
			return true
		}
	}
	return false
}

func (d *Document) IsOpen(id int) bool {
	if d == nil {
		return false
	}
	for _, det := range d.details {
		if det.ID == id {
			return det.Open
		}
	}
	return false
}

func (d *Document) VisibleDetailIDs() []int {
	if d == nil {
		return nil
	}
	var ids []int
	d.collectVisibleIDs(&ids)
	return ids
}

func (d *Document) collectVisibleIDs(out *[]int) {
	for _, b := range d.Blocks {
		if b.Kind == BlockDetail && b.Detail != nil {
			*out = append(*out, b.Detail.ID)
			if b.Detail.Open && b.Detail.Content != nil {
				b.Detail.Content.collectVisibleIDs(out)
			}
		}
	}
}

func (d *Document) Render() []Line {
	if d == nil {
		return []Line{{Spans: []Span{{Text: "", Style: term.StyleDefault}}}}
	}
	var lines []Line
	for _, b := range d.Blocks {
		switch b.Kind {
		case BlockContent:
			lines = append(lines, b.Lines...)
		case BlockDetail:
			if b.Detail != nil {
				lines = append(lines, b.Detail.render()...)
			}
		}
	}
	for len(lines) > 0 && lines[len(lines)-1].Kind == KindBlank {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []Line{{Spans: []Span{{Text: "", Style: term.StyleDefault}}}}
	}
	return lines
}

func (det *DetailBlock) render() []Line {
	var lines []Line
	marker := "▶ "
	if det.Open {
		marker = "▼ "
	}
	spans := []Span{{Text: marker, Style: term.StyleHoverBold}}
	if len(det.SummarySpans) > 0 {
		spans = append(spans, det.SummarySpans...)
	} else {
		spans = append(spans, Span{Text: det.Summary, Style: term.StyleHoverBold})
	}

	lines = append(lines, Line{
		Kind:     KindSummary,
		Spans:    spans,
		DetailID: det.ID,
	})

	if det.Open && det.Content != nil {
		lines = append(lines, Line{Kind: KindBlank, Spans: []Span{{Text: "", Style: term.StyleDefault}}})
		contentLines := det.Content.Render()
		lines = append(lines, contentLines...)
	}
	lines = append(lines, Line{Kind: KindBlank, Spans: []Span{{Text: "", Style: term.StyleDefault}}})
	return lines
}

func Parse(text string) *Document {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	nextID := 1
	return parseDocument(lines, &nextID)
}

func parseDocument(lines []string, nextID *int) *Document {
	var blocks []Block
	var allDetails []*DetailBlock
	var contentLines []string
	inFence := ""
	i := 0

	flushContent := func() {
		if len(contentLines) > 0 {
			raw := strings.Join(contentLines, "\n")
			rendered := renderMarkdownRaw(raw)
			if len(rendered) > 0 {
				blocks = append(blocks, Block{
					Kind:  BlockContent,
					Lines: rendered,
				})
			}
			contentLines = nil
		}
	}

	for i < len(lines) {
		line := lines[i]
		trimmedLeft := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmedLeft)

		if inFence != "" {
			if indent <= 3 && matchFenceEnd(trimmedLeft, inFence) {
				inFence = ""
			}
			contentLines = append(contentLines, line)
			i++
			continue
		}

		if indent <= 3 {
			if fence, ok := matchFenceStart(trimmedLeft); ok {
				inFence = fence
				contentLines = append(contentLines, line)
				i++
				continue
			}
		}

		if indent <= 3 && detailsStartRegex.MatchString(trimmedLeft) {
			flushContent()

			loc := detailsStartRegex.FindStringIndex(trimmedLeft)
			tag := trimmedLeft[loc[0]:loc[1]]
			isOpen := openAttrRegex.MatchString(tag)
			afterDetails := trimmedLeft[loc[1]:]

			var summaryText string
			var summarySpans []Span
			var innerLines []string

			trimmedAfter := strings.TrimSpace(afterDetails)
			if summaryStartRegex.MatchString(trimmedAfter) {
				summaryText, summarySpans, afterDetails = extractSummaryFromLine(trimmedAfter)
			} else {
				foundSummary := false
				nextIdx := i + 1
				for nextIdx < len(lines) {
					trimmedNext := strings.TrimSpace(lines[nextIdx])
					if trimmedNext == "" {
						nextIdx++
						continue
					}
					if summaryStartRegex.MatchString(trimmedNext) {
						foundSummary = true
						var remainingAfter string
						summaryText, summarySpans, remainingAfter, nextIdx = extractSummaryMultiline(lines, nextIdx)
						i = nextIdx
						afterDetails = remainingAfter
					}
					break
				}
				if !foundSummary {
					summaryText = "Details"
					summarySpans = []Span{{Text: "Details", Style: term.StyleHoverBold}}
				}
			}

			if summaryText == "" {
				summaryText = "Details"
				summarySpans = []Span{{Text: "Details", Style: term.StyleHoverBold}}
			}

			trimmedAfterSummary := strings.TrimSpace(afterDetails)
			singleLineClosed := false
			if trimmedAfterSummary != "" {
				if closeLoc := findDetailsClose(trimmedAfterSummary); closeLoc != nil {
					inside := trimmedAfterSummary[:closeLoc[0]]
					if inside != "" {
						innerLines = append(innerLines, inside)
					}
					singleLineClosed = true
				} else {
					innerLines = append(innerLines, trimmedAfterSummary)
				}
			}

			if !singleLineClosed {
				depth := 1
				innerFence := ""
				i++
				for i < len(lines) {
					curr := lines[i]
					currTrimmedLeft := strings.TrimLeft(curr, " ")
					currIndent := len(curr) - len(currTrimmedLeft)

					if innerFence != "" {
						if currIndent <= 3 && matchFenceEnd(currTrimmedLeft, innerFence) {
							innerFence = ""
						}
						innerLines = append(innerLines, curr)
						i++
						continue
					}

					if currIndent <= 3 {
						if fence, ok := matchFenceStart(currTrimmedLeft); ok {
							innerFence = fence
							innerLines = append(innerLines, curr)
							i++
							continue
						}
					}

					if currIndent <= 3 {
						hasStart := detailsStartRegex.MatchString(currTrimmedLeft)
						hasClose := findDetailsClose(currTrimmedLeft) != nil
						if hasStart && hasClose {
							innerLines = append(innerLines, curr)
							i++
							continue
						}
						if hasStart {
							depth++
							innerLines = append(innerLines, curr)
							i++
							continue
						}
						if hasClose {
							depth--
							if depth == 0 {
								break
							}
							innerLines = append(innerLines, curr)
							i++
							continue
						}
					}

					innerLines = append(innerLines, curr)
					i++
				}
			}

			detID := *nextID
			(*nextID)++
			childDoc := parseDocument(innerLines, nextID)

			det := &DetailBlock{
				ID:           detID,
				Open:         isOpen,
				DefaultOpen:  isOpen,
				Summary:      summaryText,
				SummarySpans: summarySpans,
				Content:      childDoc,
			}

			blocks = append(blocks, Block{Kind: BlockDetail, Detail: det})
			allDetails = append(allDetails, det)
			allDetails = append(allDetails, childDoc.details...)

			i++
			continue
		}

		contentLines = append(contentLines, line)
		i++
	}

	flushContent()
	return &Document{Blocks: blocks, details: allDetails}
}

func findDetailsClose(s string) []int {
	return detailsCloseRegex.FindStringIndex(maskInlineCode(s))
}

// maskInlineCode blanks out inline code spans, keeping byte offsets intact.
func maskInlineCode(s string) string {
	var masked []byte
	for i := 0; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		backslashes := 0
		for b := i - 1; b >= 0 && s[b] == '\\'; b-- {
			backslashes++
		}
		if backslashes%2 == 1 {
			i++
			continue
		}
		open := i
		for i < len(s) && s[i] == '`' {
			i++
		}
		n := i - open
		for j := i; j < len(s); {
			if s[j] != '`' {
				j++
				continue
			}
			k := j
			for k < len(s) && s[k] == '`' {
				k++
			}
			if k-j == n {
				if masked == nil {
					masked = []byte(s)
				}
				for m := open; m < k; m++ {
					masked[m] = ' '
				}
				i = k
				break
			}
			j = k
		}
	}
	if masked == nil {
		return s
	}
	return string(masked)
}

func extractSummaryFromLine(line string) (summary string, spans []Span, remaining string) {
	startLoc := summaryStartRegex.FindStringIndex(line)
	if startLoc == nil {
		return "", nil, line
	}
	content := line[startLoc[1]:]
	closeLoc := summaryCloseRegex.FindStringIndex(content)
	if closeLoc != nil {
		rawSummary := content[:closeLoc[0]]
		remaining = content[closeLoc[1]:]
		summary, spans = parseSummarySpans(rawSummary)
		return summary, spans, remaining
	}
	summary, spans = parseSummarySpans(content)
	return summary, spans, ""
}

func extractSummaryMultiline(lines []string, startIdx int) (summary string, spans []Span, remaining string, endIdx int) {
	firstLine := strings.TrimSpace(lines[startIdx])
	startLoc := summaryStartRegex.FindStringIndex(firstLine)
	content := firstLine[startLoc[1]:]

	closeLoc := summaryCloseRegex.FindStringIndex(content)
	if closeLoc != nil {
		rawSummary := content[:closeLoc[0]]
		remaining = content[closeLoc[1]:]
		summary, spans = parseSummarySpans(rawSummary)
		return summary, spans, remaining, startIdx
	}

	var parts []string
	if strings.TrimSpace(content) != "" {
		parts = append(parts, content)
	}

	currIdx := startIdx + 1
	for currIdx < len(lines) {
		l := lines[currIdx]
		if cLoc := summaryCloseRegex.FindStringIndex(l); cLoc != nil {
			before := l[:cLoc[0]]
			if strings.TrimSpace(before) != "" {
				parts = append(parts, before)
			}
			remaining = l[cLoc[1]:]
			rawSummary := strings.Join(parts, " ")
			summary, spans = parseSummarySpans(rawSummary)
			return summary, spans, remaining, currIdx
		}
		parts = append(parts, l)
		currIdx++
	}

	rawSummary := strings.Join(parts, " ")
	summary, spans = parseSummarySpans(rawSummary)
	return summary, spans, "", currIdx
}

func parseSummarySpans(raw string) (string, []Span) {
	cleaned := htmlTagRegex.ReplaceAllString(raw, "")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "Details", []Span{{Text: "Details", Style: term.StyleHoverBold}}
	}

	src := []byte(cleaned)
	parsedDoc := md.Parser().Parse(gmtext.NewReader(src))
	r := &renderer{src: src}
	spans := r.collectInlineSpans(parsedDoc, term.StyleHoverBold)

	var text strings.Builder
	for _, s := range spans {
		text.WriteString(s.Text)
	}
	return text.String(), spans
}

func matchFenceStart(s string) (string, bool) {
	if len(s) < 3 {
		return "", false
	}
	ch := s[0]
	if ch != '`' && ch != '~' {
		return "", false
	}
	count := 0
	for count < len(s) && s[count] == ch {
		count++
	}
	if count < 3 {
		return "", false
	}
	fence := s[:count]
	if ch == '`' {
		if strings.Contains(s[count:], "`") {
			return "", false
		}
	}
	return fence, true
}

func matchFenceEnd(s, fence string) bool {
	if len(fence) == 0 {
		return false
	}
	ch := fence[0]
	count := 0
	for count < len(s) && s[count] == ch {
		count++
	}
	if count < len(fence) {
		return false
	}
	rest := strings.TrimSpace(s[count:])
	return len(rest) == 0
}
