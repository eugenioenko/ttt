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
	return parseDocumentMasked(lines, nil, nextID)
}

// parseDocumentMasked parses lines, treating HTML tags inside inline code spans
// as literal text. masked must keep the offsets of lines; when it is missing or
// misaligned the mask is rebuilt from lines.
func parseDocumentMasked(lines, masked []string, nextID *int) *Document {
	if masked == nil || len(masked) != len(lines) {
		masked = maskCodeSpans(lines)
	}
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

		trimmedMasked := strings.TrimLeft(masked[i], " ")
		if indent <= 3 && len(trimmedMasked) == len(trimmedLeft) && detailsStartRegex.MatchString(trimmedMasked) {
			flushContent()

			loc := detailsStartRegex.FindStringIndex(trimmedLeft)
			tag := trimmedLeft[loc[0]:loc[1]]
			isOpen := openAttrRegex.MatchString(tag)
			afterDetails := trimmedLeft[loc[1]:]

			var summaryText string
			var summarySpans []Span
			var innerLines []string
			var innerMasked []string

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
			sameLineBody := ""
			if trimmedAfterSummary != "" {
				if closeLoc := findDetailsClose(trimmedAfterSummary); closeLoc != nil {
					inside := trimmedAfterSummary[:closeLoc[0]]
					if inside != "" {
						innerLines = append(innerLines, inside)
					}
					singleLineClosed = true
				} else {
					sameLineBody = trimmedAfterSummary
				}
			}

			if !singleLineClosed {
				depth := 1
				innerFence := ""
				// Masking starts at the body boundary so backticks in the
				// summary cannot pair with text outside the disclosure. Body
				// text on the summary line is the first body line, so it joins
				// the mask and keeps innerMasked aligned with innerLines.
				bodyStart := i + 1
				bodyLines := lines[bodyStart:]
				offset := 0
				if sameLineBody != "" {
					bodyLines = append([]string{sameLineBody}, bodyLines...)
					offset = 1
				}
				maskedBody := maskCodeSpans(bodyLines)
				appendInner := func(raw, maskedLine string) {
					innerLines = append(innerLines, raw)
					innerMasked = append(innerMasked, maskedLine)
				}
				if offset == 1 {
					appendInner(sameLineBody, maskedBody[0])
				}
				i = bodyStart
				for i < len(lines) {
					curr := lines[i]
					currMasked := maskedBody[i-bodyStart+offset]
					currTrimmedLeft := strings.TrimLeft(curr, " ")
					currIndent := len(curr) - len(currTrimmedLeft)
					currTagText := strings.TrimLeft(currMasked, " ")

					if innerFence != "" {
						if currIndent <= 3 && matchFenceEnd(currTrimmedLeft, innerFence) {
							innerFence = ""
						}
						appendInner(curr, currMasked)
						i++
						continue
					}

					if currIndent <= 3 {
						if fence, ok := matchFenceStart(currTrimmedLeft); ok {
							innerFence = fence
							appendInner(curr, currMasked)
							i++
							continue
						}
					}

					if currIndent <= 3 {
						hasStart := detailsStartRegex.MatchString(currTagText)
						hasClose := detailsCloseRegex.MatchString(currTagText)
						if hasStart && hasClose {
							appendInner(curr, currMasked)
							i++
							continue
						}
						if hasStart {
							depth++
							appendInner(curr, currMasked)
							i++
							continue
						}
						if hasClose {
							depth--
							if depth == 0 {
								break
							}
							appendInner(curr, currMasked)
							i++
							continue
						}
					}

					appendInner(curr, currMasked)
					i++
				}
			}

			detID := *nextID
			(*nextID)++
			childDoc := parseDocumentMasked(innerLines, innerMasked, nextID)

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

// maskCodeSpans blanks out inline code spans, including spans that continue
// across lines within one paragraph, keeping byte offsets and line breaks
// intact. An opener with no matching run is left as literal text; fenced and
// indented code blocks are skipped, and a span never crosses into another
// block, so callers can match HTML tags on the masked copy.
func maskCodeSpans(lines []string) []string {
	masked := make([]string, len(lines))
	copy(masked, lines)

	inFence := ""
	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)

		if inFence != "" {
			if indent <= 3 && matchFenceEnd(trimmed, inFence) {
				inFence = ""
			}
			i++
			continue
		}
		if indent <= 3 {
			if fence, ok := matchFenceStart(trimmed); ok {
				inFence = fence
				i++
				continue
			}
		}
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}
		if indent >= 4 {
			// Indented code block: inline markup is literal, so no span
			// starts or continues here.
			i++
			continue
		}
		if interruptsParagraph(trimmed) {
			maskParagraphCodeSpans(lines[i:i+1], masked[i:i+1])
			i++
			continue
		}

		end := i + 1
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
			next := strings.TrimLeft(lines[end], " ")
			if len(lines[end])-len(next) <= 3 {
				if _, ok := matchFenceStart(next); ok {
					break
				}
				if interruptsParagraph(next) {
					break
				}
			}
			end++
		}
		maskParagraphCodeSpans(lines[i:end], masked[i:end])
		i = end
	}
	return masked
}

// interruptsParagraph reports whether a line begins a block that ends the
// paragraph above it. A code span cannot cross such a boundary, so the masker
// starts a fresh paragraph there instead of pairing backticks across blocks.
func interruptsParagraph(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '>' {
		return true
	}
	if headingLevel(s) > 0 {
		return true
	}
	trimmed := strings.TrimRight(s, " \t")
	if isThematicBreak(trimmed) || isSetextUnderline(trimmed) {
		return true
	}
	return isListItemStart(s)
}

func headingLevel(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0
	}
	if n == len(s) || s[n] == ' ' || s[n] == '\t' {
		return n
	}
	return 0
}

func isThematicBreak(s string) bool {
	var marker byte
	count := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '*', '-', '_':
			if marker == 0 {
				marker = s[i]
			}
			if s[i] != marker {
				return false
			}
			count++
		case ' ', '\t':
		default:
			return false
		}
	}
	return count >= 3
}

func isSetextUnderline(s string) bool {
	if s == "" {
		return false
	}
	marker := s[0]
	if marker != '=' && marker != '-' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] != marker {
			return false
		}
	}
	return true
}

func isListItemStart(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '-' || s[0] == '+' || s[0] == '*' {
		if len(s) == 1 || s[1] == ' ' || s[1] == '\t' {
			// An empty list item (a bare marker, optionally followed by
			// spaces) does not interrupt a paragraph (CommonMark 0.31.2
			// example 285).
			return strings.TrimSpace(s[1:]) != ""
		}
		return false
	}
	value := 0
	digits := 0
	for digits < len(s) && digits < 9 && s[digits] >= '0' && s[digits] <= '9' {
		value = value*10 + int(s[digits]-'0')
		digits++
	}
	if digits == 0 || digits >= len(s) {
		return false
	}
	if s[digits] != '.' && s[digits] != ')' {
		return false
	}
	if digits+1 != len(s) && s[digits+1] != ' ' && s[digits+1] != '\t' {
		return false
	}
	// Only a non-empty ordered list item starting at one interrupts a
	// paragraph; a bare "1." or "1)" line stays inside the paragraph.
	return value == 1 && strings.TrimSpace(s[digits+1:]) != ""
}

// maskParagraphCodeSpans masks the code spans of one paragraph. A span opens on
// a run of n backticks and closes on the next run of exactly n backticks in the
// same paragraph; an unmatched opener stays literal.
func maskParagraphCodeSpans(lines []string, masked []string) {
	bufs := make([][]byte, len(lines))
	for i, line := range lines {
		bufs[i] = []byte(line)
	}

	line, col := 0, 0
	for line < len(lines) {
		if col >= len(lines[line]) {
			line++
			col = 0
			continue
		}
		if lines[line][col] != '`' {
			if lines[line][col] == '<' {
				if end := htmlTagEnd(lines[line], col); end >= 0 {
					col = end
					continue
				}
			}
			col++
			continue
		}

		backslashes := 0
		for b := col - 1; b >= 0 && lines[line][b] == '\\'; b-- {
			backslashes++
		}
		if backslashes%2 == 1 {
			col++
			continue
		}

		open := col
		for col < len(lines[line]) && lines[line][col] == '`' {
			col++
		}
		run := col - open

		closeLine, closeCol, ok := findCodeSpanClose(lines, line, col, run)
		if !ok {
			continue
		}
		for l := line; l <= closeLine; l++ {
			from, to := 0, len(bufs[l])
			if l == line {
				from = open
			}
			if l == closeLine {
				to = closeCol + run
			}
			for m := from; m < to; m++ {
				bufs[l][m] = ' '
			}
		}
		line, col = closeLine, closeCol+run
	}

	for i := range bufs {
		masked[i] = string(bufs[i])
	}
}

// findCodeSpanClose returns the start of the next backtick run of exactly n
// backticks, searching from col on line through the end of the paragraph.
// Once a span is open, tag-shaped text is code content, so the search matches
// the next backtick run even when it sits inside what looks like an HTML tag.
func findCodeSpanClose(lines []string, line, col, n int) (int, int, bool) {
	for ; line < len(lines); line++ {
		for col < len(lines[line]) {
			if lines[line][col] != '`' {
				col++
				continue
			}
			start := col
			for col < len(lines[line]) && lines[line][col] == '`' {
				col++
			}
			if col-start == n {
				return line, start, true
			}
		}
		col = 0
	}
	return 0, 0, false
}

// htmlTagEnd reports where the HTML tag starting at line[start] ends. It
// returns the index just past the tag's '>', or len(line) when the tag is
// still open at the end of the line, and -1 when line[start] does not begin
// a tag at all. Raw HTML is consumed before code-span delimiters are
// matched, so a backtick inside a tag (for example in a quoted attribute)
// is literal text and never opens or closes a code span.
func htmlTagEnd(line string, start int) int {
	if start+1 >= len(line) {
		return -1
	}
	c := line[start+1]
	if isTagNameStart(c) {
		// Open tag.
	} else if c == '/' && start+2 < len(line) && isTagNameStart(line[start+2]) {
		// Closing tag.
	} else {
		return -1
	}
	for i := start + 1; i < len(line); i++ {
		switch line[i] {
		case '"':
			if j := strings.IndexByte(line[i+1:], '"'); j >= 0 {
				i += j + 1
			} else {
				return len(line)
			}
		case '\'':
			if j := strings.IndexByte(line[i+1:], '\''); j >= 0 {
				i += j + 1
			} else {
				return len(line)
			}
		case '>':
			return i + 1
		}
	}
	return len(line)
}

func isTagNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
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
