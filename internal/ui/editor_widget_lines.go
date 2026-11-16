package ui

import (
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/highlight"
)

func (e *EditorPaneWidget) MoveLineUp() {
	if e.isMultiActive() {
		e.moveLinesMulti(-1)
		return
	}
	e.moveLineBlock(-1)
}

func (e *EditorPaneWidget) MoveLineDown() {
	if e.isMultiActive() {
		e.moveLinesMulti(1)
		return
	}
	e.moveLineBlock(1)
}

// moveLineBlock moves the cursor's line (or selected lines) one visible line
// up or down. Collapsed folds move as units: a folded header carries its
// hidden body, and a folded neighbor is stepped over whole. Swapping raw lines
// instead would reorder code hidden inside a fold (BUG-027).
func (e *EditorPaneWidget) moveLineBlock(delta int) {
	start, end, hasSel := e.selectedLineRange()
	if !hasSel {
		start, end = e.Cursor.Line, e.Cursor.Line
	}
	last := len(e.Buf.Lines) - 1
	folded := e.hasFolds()
	if folded {
		for l := start; l <= end; l++ {
			if r := e.Folds.FoldAt(l); r != nil && e.Folds.IsCollapsed(l) && r.EndLine > end {
				end = min(r.EndLine, last)
			}
		}
	}

	var lo, hi int
	if delta < 0 {
		if start <= 0 {
			return
		}
		lo, hi = start-1, end
		if folded {
			for r := e.Folds.ContainingFold(lo); r != nil; r = e.Folds.ContainingFold(lo) {
				lo = r.StartLine
			}
		}
	} else {
		if end >= last {
			return
		}
		lo, hi = start, end+1
		if folded && e.Folds.IsCollapsed(hi) {
			if r := e.Folds.FoldAt(hi); r != nil {
				hi = min(r.EndLine, last)
			}
		}
	}

	block := e.Buf.Lines[start : end+1]
	var neighbor []string
	if delta < 0 {
		neighbor = e.Buf.Lines[lo:start]
	} else {
		neighbor = e.Buf.Lines[end+1 : hi+1]
	}
	old := append([]string(nil), e.Buf.Lines[lo:hi+1]...)
	moved := make([]string, 0, len(old))
	if delta < 0 {
		moved = append(append(moved, block...), neighbor...)
	} else {
		moved = append(append(moved, neighbor...), block...)
	}
	blockShift := len(neighbor) * delta
	e.exec(&moveLinesCommand{
		replace:       undo.ReplaceLinesCommand{Start: lo, OldLines: old, NewLines: moved},
		folds:         e.Folds,
		start:         start,
		end:           end,
		hi:            hi,
		blockShift:    blockShift,
		neighborShift: -len(block) * delta,
	})
	e.refreshFolds()
	e.Cursor.Line += blockShift
	if hasSel {
		e.Selection.Anchor.Line += blockShift
	}
	e.clampCursor()
	e.scrollViewport()
}

// moveLinesCommand moves collapsed fold state along with the lines on apply
// and back on undo; undo only recomputes fold ranges, so without this a
// collapsed fold would land on whatever text now occupies its old line.
type moveLinesCommand struct {
	replace                   undo.ReplaceLinesCommand
	folds                     *fold.State
	start, end, hi            int
	blockShift, neighborShift int
}

func (c *moveLinesCommand) Apply(b *buffer.Buffer) {
	c.replace.Apply(b)
	c.remapFolds(c.start, c.end, 1)
}

func (c *moveLinesCommand) Undo(b *buffer.Buffer) {
	c.replace.Undo(b)
	c.remapFolds(c.start+c.blockShift, c.end+c.blockShift, -1)
}

func (c *moveLinesCommand) UndoCursor() (int, int) {
	return c.start, 0
}

func (c *moveLinesCommand) RedoCursor() (int, int) {
	return c.start + c.blockShift, 0
}

func (c *moveLinesCommand) remapFolds(blockStart, blockEnd, sign int) {
	if c.folds == nil {
		return
	}
	lo := c.replace.Start
	c.folds.RemapCollapsed(func(l int) int {
		switch {
		case l >= blockStart && l <= blockEnd:
			return l + sign*c.blockShift
		case l >= lo && l <= c.hi:
			return l + sign*c.neighborShift
		}
		return l
	})
}

// moveLinesMulti shifts every buffer line touched by a cursor (or its
// selection) by delta (-1 or +1) and moves all cursors with it, so each
// cursor stays on its own text. The whole move is a no-op if any touched
// line would cross a buffer edge. Without this, line commands leave
// e.Multi.Cursors at stale offsets and the next keystroke corrupts (BUG-005).
func (e *EditorPaneWidget) moveLinesMulti(delta int) {
	e.syncToMulti()

	touched := make(map[int]bool)
	for _, cs := range e.Multi.Cursors {
		lo, hi := cs.Line, cs.Line
		if cs.Sel.Active {
			s, en := cs.Sel.Range(cs.Line, cs.Col)
			lo, hi = s.Line, en.Line
			if en.Col == 0 && hi > lo {
				hi--
			}
		}
		for l := lo; l <= hi; l++ {
			touched[e.Buf.ClampLine(l)] = true
		}
	}
	if len(touched) == 0 {
		return
	}
	lines := make([]int, 0, len(touched))
	for l := range touched {
		lines = append(lines, l)
	}
	sort.Ints(lines)

	lastReal := len(e.Buf.Lines) - 1
	if lastReal > 0 && e.Buf.Lines[lastReal] == "" {
		lastReal--
	}
	if delta < 0 && lines[0] <= 0 {
		return
	}
	if delta > 0 && lines[len(lines)-1] >= lastReal {
		return
	}
	if e.hasFolds() {
		for _, l := range lines {
			for _, t := range []int{l, l + delta} {
				if e.Folds.IsCollapsed(t) || e.Folds.ContainingFold(t) != nil {
					return
				}
			}
		}
	}

	// One BatchCommand so a single undo reverses the whole multicursor move,
	// mirroring the multiExec* handlers.
	var cmds []undo.EditCommand
	if delta < 0 {
		for _, l := range lines {
			cmd := &undo.SwapLineCommand{Line1: l, Line2: l - 1}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
		}
	} else {
		for i := len(lines) - 1; i >= 0; i-- {
			cmd := &undo.SwapLineCommand{Line1: lines[i], Line2: lines[i] + 1}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
		}
	}
	if e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds})
	}
	e.bufferDirty = true

	for i := range e.Multi.Cursors {
		cs := &e.Multi.Cursors[i]
		cs.Line = e.Buf.ClampLine(cs.Line + delta)
		if cs.Sel.Active {
			cs.Sel.Anchor.Line += delta
		}
	}
	e.Multi.Deduplicate()
	e.syncFromMulti()
	e.clampCursor()
	e.scrollViewport()
}

// collapseMultiForLineOp drops multi-cursor mode before a line command that
// has no meaningful multi-cursor semantics (duplicate/delete/join/sort a
// line at N cursors is ambiguous). Leaving e.Multi.Cursors in place while
// such a command reshapes the buffer strands them at stale offsets and the
// next keystroke corrupts (BUG-005).
func (e *EditorPaneWidget) collapseMultiForLineOp() {
	if e.isMultiActive() {
		e.collapseMulti()
	}
}

func (e *EditorPaneWidget) DuplicateLine() {
	e.collapseMultiForLineOp()
	startLine, endLine, hasSel := e.selectedLineRange()
	if !hasSel {
		startLine = e.Buf.ClampLine(e.Cursor.Line)
		endLine = startLine
	}
	texts := e.copyLines(startLine, endLine)
	for i, text := range texts {
		e.exec(&undo.InsertLineCommand{Idx: endLine + 1 + i, Text: text})
	}
	blockSize := endLine - startLine + 1
	e.Cursor.Line += blockSize
	if hasSel {
		e.Selection.Anchor.Line += blockSize
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) DeleteLine() {
	e.collapseMultiForLineOp()
	startLine, endLine, hasSel := e.selectedLineRange()
	if !hasSel {
		startLine = e.Buf.ClampLine(e.Cursor.Line)
		endLine = startLine
	}
	for i := endLine; i >= startLine; i-- {
		if len(e.Buf.Lines) <= 1 {
			e.exec(&undo.DeleteSelectionCommand{
				StartLine: 0, StartCol: 0,
				EndLine: 0, EndCol: len([]rune(e.Buf.Lines[0])),
			})
			break
		}
		e.exec(&undo.DeleteLineCommand{Idx: i})
	}
	if hasSel {
		e.Selection.Active = false
	}
	e.Cursor.Line = e.Buf.ClampLine(startLine)
	e.Cursor.Col = 0
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) JoinLines() {
	e.collapseMultiForLineOp()
	if e.Undo != nil {
		e.Undo.BreakGroup()
	}
	hasSel := e.Selection != nil && e.Selection.Active
	if hasSel {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		endLine := end.Line
		if end.Col == 0 && endLine > start.Line {
			endLine--
		}
		// Join all lines in the selection range from the first line
		for endLine > start.Line {
			cmd := &undo.JoinNextLineCommand{Line: start.Line}
			e.exec(cmd)
			endLine--
		}
		e.Selection.Active = false
		e.Cursor.Line = start.Line
		lineLen := len([]rune(e.Buf.Lines[start.Line]))
		if e.Cursor.Col > lineLen {
			e.Cursor.Col = lineLen
		}
	} else {
		if e.Cursor.Line >= len(e.Buf.Lines)-1 {
			return
		}
		cmd := &undo.JoinNextLineCommand{Line: e.Cursor.Line}
		e.exec(cmd)
		e.Cursor.Col = cmd.JoinCol
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) InsertLineBelow() {
	e.exec(&undo.InsertLineCommand{Idx: e.Cursor.Line + 1, Text: ""})
	e.Cursor.Line++
	e.Cursor.Col = 0
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) InsertLineAbove() {
	e.exec(&undo.InsertLineCommand{Idx: e.Cursor.Line, Text: "", Above: true})
	e.Cursor.Col = 0
	e.clampCursor()
	e.scrollViewport()
}

// commentSyntax returns the delimiters used to comment a single line. A
// non-empty postfix marks a language whose comments need both delimiters on
// the same line, such as HTML's <!-- ... -->; otherwise the comment runs to the
// end of the line and postfix is empty.
func (e *EditorPaneWidget) commentSyntax() (prefix, postfix string) {
	prefix = "//"
	if e.Highlighter != nil {
		switch e.commentLanguage() {
		case "awk", "cmake", "coffee", "crystal", "desktop", "docker", "dotenv",
			"elixir", "fish", "gdscript", "gherkin", "graphql", "julia", "make",
			"mojo", "nix", "nushell", "perl", "powershell", "puppet", "python",
			"r", "ruby", "shellscript", "systemd", "toml", "yaml":
			prefix = "#"
		case "elm", "haskell", "lean", "lua", "plsql", "sql", "vhdl":
			prefix = "--"
		case "angular-html", "html", "html-derivative", "xml":
			return "<!--", "-->"
		case "css", "less", "scss":
			return "/*", "*/"
		case "coq", "ocaml":
			return "(*", "*)"
		case "jinja":
			return "{#", "#}"
		case "smalltalk":
			return "\"", "\""
		case "ahk", "clojure", "common-lisp", "fennel", "hy", "reg", "scheme":
			prefix = ";"
		case "bat":
			prefix = "REM"
		case "erlang", "tex":
			prefix = "%"
		case "vb":
			prefix = "'"
		}
	}
	return prefix, ""
}

// commentLanguage returns the canonical language ID that refines comment
// classification, or "" without a highlighter.
func (e *EditorPaneWidget) commentLanguage() string {
	if e.Highlighter == nil {
		return ""
	}
	return highlight.CanonicalLanguageID(e.Highlighter.LanguageID())
}

// escapeXMLCommentText spells every character that may not appear inside an
// XML comment as an entity, so wrapping the result in <!-- ... --> can neither
// close early nor form an invalid "--" run. Comments cannot nest and have no
// backslash escape, so entities are the only spelling. "&" is encoded too, so
// pre-existing entity text such as &amp;lt;!-- is not decoded by the unwrap
// pass: unescapeXMLCommentText reverses exactly what this encoded.
//
// Every hyphen of a run of two or more is encoded, which covers the "<!--"
// and "-->" delimiters, the "--" run XML forbids, and guarantees no two
// literal hyphens stay adjacent in the output.
func escapeXMLCommentText(text string) string {
	return escapeXMLCommentTextPrefix(text, len(text))
}

// escapeXMLCommentTextPrefix encodes text as escapeXMLCommentText does but
// stops after the first end bytes, using the whole line as context: a hyphen
// run straddling end is still a run, so a cursor inside it moves past every
// encoded hyphen before it instead of landing inside an entity.
func escapeXMLCommentTextPrefix(text string, end int) string {
	var b strings.Builder
	b.Grow(end + 8)
	for i := 0; i < len(text) && i < end; {
		switch {
		case text[i] == '&':
			b.WriteString("&amp;")
			i++
		case text[i] == '-':
			j := i
			for j < len(text) && text[j] == '-' {
				j++
			}
			if j-i < 2 {
				b.WriteByte('-')
				i++
				continue
			}
			for ; i < j && i < end; i++ {
				b.WriteString("&#45;")
			}
		default:
			b.WriteByte(text[i])
			i++
		}
	}
	return b.String()
}

func unescapeXMLCommentText(text string) string {
	text = strings.ReplaceAll(text, "&#45;", "-")
	return strings.ReplaceAll(text, "&amp;", "&")
}

// isWordCommentPrefix reports whether prefix is a keyword rather than a
// punctuation delimiter, such as Batch's REM. Word prefixes are matched
// case-insensitively and only as a whole word, so REMOVE_ME=1 is code.
func isWordCommentPrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	for _, r := range prefix {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// commentSyntaxNests reports whether an opener inside a comment starts a
// nested comment, as OCaml's (* ... *) does. Other paired syntaxes end at the
// first closer, so text after a separate pair is not commented.
func commentSyntaxNests(prefix, postfix string) bool {
	return prefix == "(*" && postfix == "*)"
}

// isWholeLineComment reports whether text is one comment wrapping the whole
// line. Nesting syntax balances to depth zero only at the end of the line, so an
// outer wrapper with inner closers still counts, but delimiters quoted in a
// string, character, or quoted-string literal do not nest; non-nesting syntax
// requires the first closer to end the line. Quoted strings are OCaml-only: Coq
// shares the delimiters but has no {| |} literal, so there the brace is comment
// text and (* {| *) is a complete comment.
func isWholeLineComment(text, prefix, postfix, lang string) bool {
	if !commentSyntaxNests(prefix, postfix) {
		rest := text[len(prefix):]
		closer := strings.Index(rest, postfix)
		return closer >= 0 && closer+len(postfix) == len(rest)
	}
	depth := 0
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], prefix):
			depth++
			i += len(prefix)
		case strings.HasPrefix(text[i:], postfix):
			depth--
			if depth == 0 {
				return i+len(postfix) == len(text)
			}
			i += len(postfix)
		case text[i] == '"':
			i = skipStringLiteral(text, i)
		case text[i] == '\'':
			if next, ok := skipCharLiteral(text, i); ok {
				i = next
			} else {
				i++
			}
		case text[i] == '{' && lang == "ocaml":
			if next, ok := skipQuotedString(text, i); ok {
				i = next
			} else {
				i++
			}
		default:
			i++
		}
	}
	return false
}

// skipStringLiteral returns the index just past the string literal that opens
// at i. Comment delimiters inside a literal are text, not nesting, so the scan
// must jump over it; an unterminated literal runs to the end of the line.
func skipStringLiteral(text string, i int) int {
	for i++; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return len(text)
}

// skipCharLiteral returns the index just past the character literal opening at
// i, and false when text[i] does not open one. A quote character such as '"'
// must be skipped first, or it would be mistaken for a string opener. OCaml
// also spells type variables with a bare quote, so the closing quote is
// required before this counts as a literal.
func skipCharLiteral(text string, i int) (int, bool) {
	j := i + 1
	if j >= len(text) {
		return i, false
	}
	if text[j] == '\\' {
		j++
		if j >= len(text) {
			return i, false
		}
	}
	_, size := utf8.DecodeRuneInString(text[j:])
	j += size
	if j < len(text) && text[j] == '\'' {
		return j + 1, true
	}
	return i, false
}

// skipQuotedString returns the index just past the OCaml quoted string literal
// that opens at i, and false when text[i] does not open one. The id before the
// opening "|" must repeat (up to Unicode normalization) in the closing "|id}",
// and a literal with no closer runs to the end of the line. Quotes inside a
// read as string openers. Ids are
// lowercase letters and underscores only, so {1| is not a literal opener and
// must not swallow the rest of the line. Combining marks are scanned with the
// id and the whole id is validated after NFC, so a decomposed é opens the same
// literal as a composed one. Byte indices stay in the original line.
func skipQuotedString(text string, i int) (int, bool) {
	j := i + 1
	for j < len(text) {
		r, size := utf8.DecodeRuneInString(text[j:])
		if !isQuotedStringIDRune(r) && !isCombiningMark(r) {
			break
		}
		j += size
	}
	if j >= len(text) || text[j] != '|' {
		return i, false
	}
	id := text[i+1 : j]
	if !isQuotedStringID(norm.NFC.String(id)) {
		return i, false
	}
	end := findQuotedStringCloser(text, j+1, id)
	if end < 0 {
		return len(text), true
	}
	return end, true
}

// isQuotedStringIDRune accepts OCaml's quoted-string id characters: lowercase
// ASCII, the lowercase letters of Latin-9, and underscore. Other scripts are not
// id characters, so {α| is not a literal opener and its later "*)' stays visible
// to the comment scan.
func isQuotedStringIDRune(r rune) bool {
	if r == '_' || r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 0xDF && r <= 0xFF && r != 0xF7 {
		return true
	}
	switch r {
	case 'œ', 'š', 'ž':
		return true
	}
	return false
}

// isQuotedStringID reports whether a normalized id is made of id characters
// only; the raw bytes may also carry combining marks, which NFC folds away.
func isQuotedStringID(id string) bool {
	for _, r := range id {
		if !isQuotedStringIDRune(r) {
			return false
		}
	}
	return true
}

func isCombiningMark(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r)
}

// findQuotedStringCloser returns the original byte index just past the closing
// "|id}" for the opener id, or -1. Ids are compared in NFC because the same id
// can be spelled composed (é) or decomposed (e + combining acute) and OCaml
// treats the lexer-normalized forms alike; normalizing must not leak into the
// returned offsets, which slice the untouched line.
func findQuotedStringCloser(text string, from int, id string) int {
	want := norm.NFC.String(id)
	for p := from; p < len(text); {
		skip := strings.IndexByte(text[p:], '|')
		if skip < 0 {
			return -1
		}
		p += skip
		if n, ok := matchQuotedStringID(text[p+1:], want); ok && p+1+n < len(text) && text[p+1+n] == '}' {
			return p + 1 + n + 1
		}
		p++
	}
	return -1
}

// matchQuotedStringID returns how many bytes of s hold an id whose NFC form is
// want. It consumes whole clusters so a combining mark stays with its base rune
// and the prefix's normalized form is always the normalization of the prefix.
func matchQuotedStringID(s, want string) (int, bool) {
	for i := 0; ; {
		got := norm.NFC.String(s[:i])
		if got == want {
			return i, true
		}
		if i >= len(s) || !strings.HasPrefix(want, got) {
			return 0, false
		}
		i += runeClusterLen(s[i:])
	}
}

func runeClusterLen(s string) int {
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if i > 0 && !isCombiningMark(r) {
			break
		}
		i += size
	}
	return i
}

func hasCommentPrefix(trimmed, prefix string) bool {
	if !isWordCommentPrefix(prefix) {
		return strings.HasPrefix(trimmed, prefix)
	}
	if len(trimmed) < len(prefix) || !strings.EqualFold(trimmed[:len(prefix)], prefix) {
		return false
	}
	rest := trimmed[len(prefix):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

func (e *EditorPaneWidget) ToggleLineComment() {
	e.collapseMultiForLineOp()
	prefix, postfix := e.commentSyntax()
	prefixLen := len([]rune(prefix))
	postfixLen := len([]rune(postfix))

	startLine, endLine := e.Cursor.Line, e.Cursor.Line
	if e.Selection != nil && e.Selection.Active {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		startLine = start.Line
		endLine = end.Line
		if end.Col == 0 && endLine > startLine {
			endLine--
		}
	}
	startLine = e.Buf.ClampLine(startLine)
	endLine = e.Buf.ClampLine(endLine)

	allCommented := true
	for l := startLine; l <= endLine; l++ {
		trimmed := strings.TrimRight(strings.TrimLeft(e.Buf.Lines[l], " \t"), " \t")
		if trimmed == "" {
			continue
		}
		commented := hasCommentPrefix(trimmed, prefix)
		if commented && postfix != "" {
			// Uncomment only when a single wrapper covers the whole line.
			commented = isWholeLineComment(trimmed, prefix, postfix, e.commentLanguage())
		}
		if !commented {
			allCommented = false
			break
		}
	}

	var cmds []undo.EditCommand
	cursorDelta := 0

	for l := startLine; l <= endLine; l++ {
		runes := []rune(e.Buf.Lines[l])
		trimmed := strings.TrimLeft(string(runes), " \t")
		if trimmed == "" {
			continue
		}
		indent := len(runes) - len([]rune(trimmed))

		if allCommented {
			removeLen := prefixLen
			if indent+removeLen < len(runes) && runes[indent+removeLen] == ' ' {
				removeLen++
			}
			cmd := &undo.DeleteSelectionCommand{
				StartLine: l, StartCol: indent,
				EndLine: l, EndCol: indent + removeLen,
			}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			if l == e.Cursor.Line {
				cursorDelta = -removeLen
			}
			if postfix != "" {
				tail := []rune(strings.TrimRight(e.Buf.Lines[l], " \t"))
				postStart := len(tail) - postfixLen
				postRemoveLen := postfixLen
				if postStart > 0 && tail[postStart-1] == ' ' {
					postStart--
					postRemoveLen++
				}
				if postStart >= 0 {
					del := &undo.DeleteSelectionCommand{
						StartLine: l, StartCol: postStart,
						EndLine: l, EndCol: postStart + postRemoveLen,
					}
					del.Apply(e.Buf)
					cmds = append(cmds, del)
					if l == e.Cursor.Line {
						// Compare in post-prefix coordinates; a cursor inside the
						// removed suffix collapses to its start.
						transformed := e.Cursor.Col + cursorDelta
						switch {
						case transformed >= postStart+postRemoveLen:
							cursorDelta -= postRemoveLen
						case transformed >= postStart:
							cursorDelta = postStart - e.Cursor.Col
						}
					}
				}
				if prefix == "<!--" {
					if unescaped := unescapeXMLCommentText(e.Buf.Lines[l]); unescaped != e.Buf.Lines[l] {
						if l == e.Cursor.Line {
							line := []rune(e.Buf.Lines[l])
							c := max(0, min(e.Cursor.Col+cursorDelta, len(line)))
							cursorDelta += len([]rune(unescapeXMLCommentText(string(line[:c])))) - c
						}
						replace := &undo.ReplaceLinesCommand{
							Start: l, OldLines: []string{e.Buf.Lines[l]}, NewLines: []string{unescaped},
						}
						replace.Apply(e.Buf)
						cmds = append(cmds, replace)
					}
				}
			}
		} else {
			if prefix == "<!--" {
				if escaped := escapeXMLCommentText(e.Buf.Lines[l]); escaped != e.Buf.Lines[l] {
					if l == e.Cursor.Line {
						c := min(e.Cursor.Col, len(runes))
						line := string(runes)
						end := len(string(runes[:c]))
						cursorDelta += len([]rune(escapeXMLCommentTextPrefix(line, end))) - c
					}
					replace := &undo.ReplaceLinesCommand{
						Start: l, OldLines: []string{e.Buf.Lines[l]}, NewLines: []string{escaped},
					}
					replace.Apply(e.Buf)
					cmds = append(cmds, replace)
				}
			}
			cmd := &undo.InsertStringCommand{Line: l, Col: indent, Text: prefix + " "}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			if l == e.Cursor.Line {
				cursorDelta += prefixLen + 1
			}
			if postfix != "" {
				endCol := len([]rune(strings.TrimRight(e.Buf.Lines[l], " \t")))
				add := &undo.InsertStringCommand{Line: l, Col: endCol, Text: " " + postfix}
				add.Apply(e.Buf)
				cmds = append(cmds, add)
			}
		}
	}

	if len(cmds) > 0 && e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds})
		e.bufferDirty = true
	}

	e.Cursor.Col += cursorDelta
	if e.Cursor.Col < 0 {
		e.Cursor.Col = 0
	}
	e.clampCursor()
	e.scrollViewport()
}

// selectedLineRange returns the start and end line indices for the current
// selection applying the col-0 convention, and true. If no selection is
// active it returns (0, 0, false).
func (e *EditorPaneWidget) selectedLineRange() (int, int, bool) {
	if e.Selection != nil && e.Selection.Active {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		endLine := end.Line
		if end.Col == 0 && endLine > start.Line {
			endLine--
		}
		return e.Buf.ClampLine(start.Line), e.Buf.ClampLine(endLine), true
	}
	return 0, 0, false
}

// lineRange returns the start and end line indices for the current selection,
// or the full buffer range if no selection is active.
func (e *EditorPaneWidget) lineRange() (int, int) {
	if start, end, ok := e.selectedLineRange(); ok {
		return start, end
	}
	end := len(e.Buf.Lines) - 1
	if end > 0 && e.Buf.Lines[end] == "" {
		end--
	}
	return 0, end
}

// copyLines returns a copy of the buffer lines in the given range (inclusive).
func (e *EditorPaneWidget) copyLines(start, end int) []string {
	lines := make([]string, end-start+1)
	copy(lines, e.Buf.Lines[start:end+1])
	return lines
}

func (e *EditorPaneWidget) SortLinesAsc() {
	e.collapseMultiForLineOp()
	start, end := e.lineRange()
	old := e.copyLines(start, end)
	sorted := make([]string, len(old))
	copy(sorted, old)
	sort.Strings(sorted)
	e.exec(&undo.ReplaceLinesCommand{Start: start, OldLines: old, NewLines: sorted})
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) SortLinesDesc() {
	e.collapseMultiForLineOp()
	start, end := e.lineRange()
	old := e.copyLines(start, end)
	sorted := make([]string, len(old))
	copy(sorted, old)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted)))
	e.exec(&undo.ReplaceLinesCommand{Start: start, OldLines: old, NewLines: sorted})
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) ReverseLines() {
	e.collapseMultiForLineOp()
	start, end := e.lineRange()
	old := e.copyLines(start, end)
	reversed := make([]string, len(old))
	for i, line := range old {
		reversed[len(old)-1-i] = line
	}
	e.exec(&undo.ReplaceLinesCommand{Start: start, OldLines: old, NewLines: reversed})
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) UniqueLines() {
	e.collapseMultiForLineOp()
	start, end := e.lineRange()
	old := e.copyLines(start, end)
	seen := make(map[string]bool)
	var unique []string
	for _, line := range old {
		if !seen[line] {
			seen[line] = true
			unique = append(unique, line)
		}
	}
	e.exec(&undo.ReplaceLinesCommand{Start: start, OldLines: old, NewLines: unique})
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) TrimTrailingWhitespaceLines() {
	if e.ReadOnly {
		return
	}
	e.collapseMultiForLineOp()
	start, end := e.lineRange()
	old := e.copyLines(start, end)
	trimmed := make([]string, len(old))
	for i, line := range old {
		trimmed[i] = strings.TrimRight(line, " \t")
	}
	if slices.Equal(trimmed, old) {
		return
	}
	e.exec(&undo.ReplaceLinesCommand{Start: start, OldLines: old, NewLines: trimmed})
	e.clampCursor()
	if lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line])); e.Cursor.Col > lineLen {
		e.Cursor.Col = lineLen
	}
	e.scrollViewport()
}
