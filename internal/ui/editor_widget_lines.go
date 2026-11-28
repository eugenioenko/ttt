package ui

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

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
	prefix = ""
	if e.Highlighter != nil {
		switch e.commentLanguage() {
		case "awk", "cmake", "coffee", "crystal", "desktop", "docker", "dotenv",
			"elixir", "fish", "gdscript", "gherkin", "graphql", "julia", "make",
			"mojo", "nix", "nushell", "perl", "powershell", "puppet", "python",
			"r", "ruby", "shellscript", "systemd", "toml", "yaml":
			prefix = "#"
		case "elm", "haskell", "lean", "lua", "plsql", "sql", "vhdl":
			prefix = "--"
		// Markdown has no comment of its own; an HTML comment is what most
		// parsers pass through, so it also takes the XML escaping below.
		case "angular-html", "html", "html-derivative", "markdown", "svelte", "vue", "xml":
			return "<!--", "-->"
		case "css", "less", "scss":
			return "/*", "*/"
		// SAS has no line comment marker; a block comment is the only form
		// that safely wraps a statement containing a semicolon. The
		// "* ... ;" comment statement is still recognized when unwrapping.
		case "sas":
			return "/*", "*/"
		case "coq", "ocaml":
			return "(*", "*)"
		case "handlebars":
			return "{{!", "}}"
		case "jinja", "twig":
			return "{#", "#}"
		case "smalltalk":
			return "\"", "\""
		case "asm":
			prefix = e.assemblyCommentPrefix()
		case "ahk", "clojure", "common-lisp", "fennel", "hy", "ini", "reg", "scheme":
			prefix = ";"
		case "cobol":
			prefix = "*>"
		// WebAssembly text uses two semicolons; a single one is not a comment.
		case "wasm":
			prefix = ";;"
		case "rst":
			prefix = ".."
		case "bat":
			prefix = "REM"
		case "erlang", "tex":
			prefix = "%"
		case "vb":
			prefix = "'"
		// Legacy Vim script uses a double quote; Vim9 script, selected by a
		// leading vim9script command, uses a hash instead.
		case "viml":
			prefix = "\""
		// For plain text and languages without a standard comment line prefix
		// or comment syntax, treat them as having no prefix at all.
		case "bibtex", "csv", "diff", "json", "shellsession":
			prefix = ""
		default:
			prefix = "//"
		}
	}
	return prefix, ""
}

// assemblyCommentPrefix returns "#" for GNU assembly and ";" for NASM. The
// bundled x86 grammar claims .s, .asm, .inc, and .nasm alike, so the extension
// is the only dialect signal; GNU as reads ";" as a statement separator, which
// leaves the instruction after it executable. Files without a name, such as a
// language override, keep NASM's marker.
func (e *EditorPaneWidget) assemblyCommentPrefix() string {
	if strings.EqualFold(filepath.Ext(e.Highlighter.Filename()), ".s") {
		return "#"
	}
	return ";"
}

// commentLanguage returns the canonical language ID that refines comment
// classification, or "" without a highlighter.
func (e *EditorPaneWidget) commentLanguage() string {
	if e.Highlighter == nil {
		return ""
	}
	return highlight.CanonicalLanguageID(e.Highlighter.LanguageID())
}

// commentMarkerStart returns the rune column where a comment marker belongs
// and the line's text from there with trailing whitespace removed. COBOL
// fixed-format source may carry a sequence number field in its first six
// columns, which is not source text and must not receive the marker.
func (e *EditorPaneWidget) commentMarkerStart(line string, cobolFixed bool) (int, string) {
	runes := []rune(line)
	indent := len(runes) - len([]rune(strings.TrimLeft(line, " \t")))
	if indent == len(runes) {
		return indent, ""
	}
	if e.commentLanguage() == "php" {
		if col, ok := phpOpenTagContent(line); ok {
			return col, strings.TrimRight(string(runes[col:]), " \t")
		}
	}
	if cobolFixed && e.commentLanguage() == "cobol" {
		if col := cobolCommentColumn(line); col > indent {
			return col, strings.TrimRight(string(runes[col:]), " \t")
		}
	}
	return indent, strings.TrimRight(string(runes[indent:]), " \t")
}

// cobolCommentColumn places the marker after the indentation, or after a
// fixed-format sequence number field so it starts in the indicator area.
func cobolCommentColumn(line string) int {
	indent := len([]rune(line)) - len([]rune(strings.TrimLeft(line, " \t")))
	if indent < 6 && isCOBOLSequenceArea(line) {
		return 6
	}
	return indent
}

// isCOBOLSequenceArea reports whether the first six runes look like a fixed-
// format sequence number field: letters, digits, and blanks with at least one
// letter or digit. Sequence fields can be alphanumeric, and the columns must be
// preserved so the statement does not shift into the indicator column.
func isCOBOLSequenceArea(line string) bool {
	runes := []rune(line)
	if len(runes) < 6 {
		return false
	}
	alnum := false
	for _, r := range runes[:6] {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
			alnum = true
		case r == ' ':
		default:
			return false
		}
	}
	return alnum
}

// cobolFixedFormatLines reports whether each line is fixed-format source.
// COBOL defaults to fixed format; a >>SOURCE directive can switch it, and a
// later directive switches it back.
func cobolFixedFormatLines(lines []string) []bool {
	fixed := make([]bool, len(lines))
	state := true
	for i, line := range lines {
		if free, ok := cobolSourceFormat(line); ok {
			state = !free
		}
		fixed[i] = state
	}
	return fixed
}

// cobolSourceFormat reports whether a >>SOURCE directive selects free (true)
// or fixed (false) source format. FORMAT and IS are optional words.
func cobolSourceFormat(line string) (free, ok bool) {
	fields := strings.Fields(strings.ToUpper(strings.TrimSpace(line)))
	if len(fields) < 2 || fields[0] != ">>SOURCE" {
		return false, false
	}
	// Fixed-format COBOL source lines before a >>SOURCE directive may carry a
	// six-column sequence number field and a two-column indicator field, then
	// the directive text. Skip those fixed-format columns so a directive such as
	// "000100 >>SOURCE FREE" is still recognized.
	start := 1
	if isCOBOLSequenceArea(line) {
		start = 2 // skip the indicator field that follows the sequence field
	}
	for _, f := range fields[start:] {
		switch f {
		case "FREE":
			return true, true
		case "FIXED":
			return false, true
		}
	}
	return false, false
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

// An endpoint inside an entity selects the decoded character instead of
// retaining an offset into text that disappears during decoding.
func unescapeXMLCommentColumn(text string, col int, isStart bool) int {
	end := runeByteOffset(text, col)
	if start := strings.LastIndex(text[:end], "&"); start >= 0 {
		for _, entity := range []string{"&amp;", "&#45;"} {
			if strings.HasPrefix(text[start:], entity) && end < start+len(entity) {
				end = start
				if !isStart {
					end += len(entity)
				}
				break
			}
		}
	}
	return len([]rune(unescapeXMLCommentText(text[:end])))
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

// Match quoted-string delimiters byte-for-byte; Unicode normalization is
// intentionally unsupported to avoid linking its tables for this OCaml case.
func skipQuotedString(text string, i int) (int, bool) {
	j := i + 1
	for j < len(text) {
		r, size := utf8.DecodeRuneInString(text[j:])
		if !isQuotedStringIDRune(r) {
			break
		}
		j += size
	}
	if j >= len(text) || text[j] != '|' {
		return i, false
	}
	closer := "|" + text[i+1:j] + "}"
	end := strings.Index(text[j+1:], closer)
	if end < 0 {
		return len(text), true
	}
	return j + 1 + end + len(closer), true
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

func isJSXLanguage(lang string) bool {
	switch lang {
	case "javascriptreact", "jsx", "tsx", "typescriptreact":
		return true
	}
	return false
}

type lineCommentDelims struct {
	prefix, postfix string
	// col is the rune column for the prefix, or -1 for the range's shared
	// column. PHP opening tags sit before the code their line comments.
	col int
	// skip leaves a line untouched: it cannot carry a comment marker without
	// breaking the surrounding syntax, such as a lone <?php tag.
	skip bool
}

// commentDelimitersPerLine returns the delimiters to use for each line of the
// range. JSX and TSX depend on whether a line starts in markup or in code, and
// PHP on whether it starts inside a PHP block; every other language uses one
// form for the whole range.
func (e *EditorPaneWidget) commentDelimitersPerLine(startLine, endLine int, prefix, postfix, lang string) []lineCommentDelims {
	delims := make([]lineCommentDelims, endLine-startLine+1)
	for i := range delims {
		delims[i] = lineCommentDelims{prefix: prefix, postfix: postfix, col: -1}
	}
	if lang == "php" {
		e.applyPHPDelimiters(delims, startLine, endLine)
		return delims
	}
	if lang == "svelte" {
		e.applySvelteDelimiters(delims, startLine, endLine)
		return delims
	}
	if lang == "twig" {
		e.applyTwigDelimiters(delims, startLine, endLine)
		return delims
	}
	if lang == "vue" {
		e.applyVueDelimiters(delims, startLine, endLine)
		return delims
	}
	if lang == "viml" {
		if directive := vim9ScriptLine(e.Buf.Lines); directive >= 0 {
			for i := range delims {
				switch line := startLine + i; {
				case line < directive:
					delims[i] = lineCommentDelims{prefix: "\"", col: -1}
				case line == directive:
					// Commenting the directive would switch the file to legacy mode.
					delims[i] = lineCommentDelims{skip: true, col: -1}
				default:
					delims[i] = lineCommentDelims{prefix: "#", col: -1}
				}
			}
			return delims
		}
	}
	if !isJSXLanguage(lang) || e.Highlighter == nil {
		return delims
	}
	for i := range delims {
		if e.Highlighter.JSXMarkupAt(e.Buf, startLine+i) {
			delims[i] = lineCommentDelims{prefix: "{/*", postfix: "*/}", col: -1}
		}
	}
	return delims
}

func (e *EditorPaneWidget) applyVueDelimiters(delims []lineCommentDelims, startLine, endLine int) {
	for i := range delims {
		switch e.Highlighter.VueContextAt(e.Buf, startLine+i) {
		case highlight.VueScript:
			delims[i] = lineCommentDelims{prefix: "//", col: -1}
		case highlight.VueStyle:
			delims[i] = lineCommentDelims{prefix: "/*", postfix: "*/", col: -1}
		default:
			delims[i] = lineCommentDelims{prefix: "<!--", postfix: "-->", col: -1}
		}
	}
}

// vim9ScriptLine returns the index of the vim9script directive, or -1 when the
// buffer is legacy Vim script. Blank lines and comments may precede it.
func vim9ScriptLine(lines []string) int {
	const marker = "vim9script"
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "\"") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(trimmed, marker) {
			return -1
		}
		if len(trimmed) == len(marker) || trimmed[len(marker)] == ' ' || trimmed[len(marker)] == '\t' {
			return i
		}
		return -1
	}
	return -1
}

// isVim9Script reports whether lines are a Vim9 script, selected by a leading
// vim9script command.
func isVim9Script(lines []string) bool {
	return vim9ScriptLine(lines) >= 0
}

func (e *EditorPaneWidget) applyTwigDelimiters(delims []lineCommentDelims, startLine, endLine int) {
	for i := range delims {
		switch e.Highlighter.TwigContextAt(e.Buf, startLine+i) {
		case highlight.TwigScript:
			delims[i] = lineCommentDelims{prefix: "//", col: -1}
		case highlight.TwigStyle:
			delims[i] = lineCommentDelims{prefix: "/*", postfix: "*/", col: -1}
		case highlight.TwigVerbatim:
			delims[i] = lineCommentDelims{prefix: "<!--", postfix: "-->", col: -1}
		default:
			delims[i] = lineCommentDelims{prefix: "{#", postfix: "#}", col: -1}
		}
	}
}

func (e *EditorPaneWidget) applySvelteDelimiters(delims []lineCommentDelims, startLine, endLine int) {
	for i := range delims {
		switch e.Highlighter.SvelteContextAt(e.Buf, startLine+i) {
		case highlight.SvelteScript:
			delims[i] = lineCommentDelims{prefix: "//", col: -1}
		case highlight.SvelteExpression:
			// A line that carries the expression's closing brace cannot take a
			// // comment: it would comment out the brace and break the template.
			if svelteExpressionCloses(e.Buf.Lines[startLine+i]) {
				delims[i] = lineCommentDelims{skip: true, col: -1}
				continue
			}
			delims[i] = lineCommentDelims{prefix: "//", col: -1}
		case highlight.SvelteStyle:
			delims[i] = lineCommentDelims{prefix: "/*", postfix: "*/", col: -1}
		default:
			delims[i] = lineCommentDelims{prefix: "<!--", postfix: "-->", col: -1}
		}
	}
}

// svelteExpressionCloses reports whether line holds the } that ends a template
// expression opened on an earlier line. Braces nested within the line (an
// object literal) and braces inside strings and comments do not count.
func svelteExpressionCloses(line string) bool {
	depth := 0
	for i := 0; i < len(line); i++ {
		iBeforeLiteral := i
		switch c := line[i]; {
		case c == '/' && i+1 < len(line) && line[i+1] == '/':
			return false
		case c == '/' && i+1 < len(line) && line[i+1] == '*':
			end := strings.Index(line[i+2:], "*/")
			if end < 0 {
				return false
			}
			i += end + 3
		case c == '\'' || c == '"' || c == '`':
			i = skipScriptLiteral(line, i)
			if i <= iBeforeLiteral {
				i = iBeforeLiteral + 1
			}
		case c == '{':
			depth++
		case c == '}':
			if depth == 0 {
				return true
			}
			depth--
		}
	}
	return false
}

// skipScriptLiteral returns the index just past the JavaScript string or
// template literal opening at i.
func skipScriptLiteral(text string, i int) int {
	quote := text[i]
	for i++; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case quote:
			return i
		}
	}
	return len(text)
}

func (e *EditorPaneWidget) applyPHPDelimiters(delims []lineCommentDelims, startLine, endLine int) {
	starts := phpCodeStarts(e.Buf.Lines, endLine)
	for i := range delims {
		line := e.Buf.Lines[startLine+i]
		if starts[startLine+i] {
			runes := []rune(line)
			indent := len(runes) - len([]rune(strings.TrimLeft(line, " \t")))
			delims[i] = phpCodeDelims(line, indent, true)
			continue
		}
		if col, ok := phpOpenTagContent(line); ok {
			// <?= implies an echo, so a // after the tag would leave it with no
			// expression to print. Nothing safely disables a short echo from
			// inside the construct, so leave those lines alone.
			if phpShortEchoTag(line) {
				delims[i] = lineCommentDelims{skip: true, col: col}
				continue
			}
			delims[i] = phpCodeDelims(line, col, false)
			continue
		}
		if phpLeadingOpenTag(line) {
			delims[i].skip = true
			continue
		}
		delims[i] = lineCommentDelims{prefix: "<!--", postfix: "-->", col: -1}
	}
}

// phpCodeDelims picks the marker for one PHP code line. A // comment ends at
// the first ?>, even one inside a string or block comment, so a line carrying
// such a quoted close tag must use a block comment or be left alone: adding
// // would close the PHP block and leave the rest of the selection as output.
func phpCodeDelims(line string, col int, align bool) lineCommentDelims {
	runes := []rune(line)
	if col < 0 || col > len(runes) {
		col = min(max(col, 0), len(runes))
	}
	text := strings.TrimRight(string(runes[col:]), " \t")
	if phpBlockCommented(text) {
		return lineCommentDelims{prefix: "/*", postfix: "*/", col: col}
	}
	protected, real, balanced := phpCloseTagContext(text)
	if protected {
		// A nested */ would close the wrapper early, and a real closing tag
		// must stay effective, so those lines are skipped instead.
		if !real && balanced && !strings.Contains(text, "*/") {
			return lineCommentDelims{prefix: "/*", postfix: "*/", col: col}
		}
		return lineCommentDelims{skip: true, col: col}
	}
	if align {
		// The batch aligns plain line comments at the shallowest indent.
		col = -1
	}
	return lineCommentDelims{prefix: "//", col: col}
}

// phpBlockCommented reports whether text is one block comment covering the
// whole line, as phpCodeDelims leaves a line it wrapped.
func phpBlockCommented(text string) bool {
	if len(text) < 4 || !strings.HasPrefix(text, "/*") || !strings.HasSuffix(text, "*/") {
		return false
	}
	return strings.Index(text[2:], "*/") == len(text)-4
}

// phpCloseTagContext reports whether text has a ?> that PHP ignores because it
// is inside a string or block comment, whether it also has a real closing tag,
// and whether the strings and block comments balance on the line.
func phpCloseTagContext(text string) (protected, real, balanced bool) {
	block := false
	var quote byte
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '?' && i+1 < len(text) && text[i+1] == '>' {
			if block || quote != 0 {
				protected = true
			} else {
				real = true
			}
			i++
			continue
		}
		if block {
			if c == '*' && i+1 < len(text) && text[i+1] == '/' {
				block = false
				i++
			}
			continue
		}
		if quote != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch {
		case c == '\'' || c == '"':
			quote = c
		case c == '/' && i+1 < len(text) && text[i+1] == '/':
			return protected, real, true
		case c == '/' && i+1 < len(text) && text[i+1] == '*':
			block = true
			i++
		}
	}
	return protected, real, !block && quote == 0
}

// phpShortEchoTag reports whether line starts with the <?= short echo tag.
func phpShortEchoTag(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), "<?=")
}

// phpLeadingOpenTag reports whether line starts with a PHP opening tag.
func phpLeadingOpenTag(line string) bool {
	runes := []rune(line)
	i := 0
	for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	rest := string(runes[i:])
	return (len(rest) >= 5 && strings.EqualFold(rest[:5], "<?php")) || strings.HasPrefix(rest, "<?=")
}

// phpOpenTagContent returns the rune column just past a leading PHP opening
// tag and its following blanks when code follows on the same line. A marker
// placed there comments the code while leaving the tag effective.
func phpOpenTagContent(line string) (int, bool) {
	runes := []rune(line)
	i := 0
	for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	rest := string(runes[i:])
	var n int
	switch {
	case len(rest) >= 5 && strings.EqualFold(rest[:5], "<?php"):
		n = 5
	case strings.HasPrefix(rest, "<?="):
		n = 3
	default:
		return 0, false
	}
	j := i + n
	if j >= len(runes) || (runes[j] != ' ' && runes[j] != '\t') {
		return 0, false
	}
	for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
		j++
	}
	if j >= len(runes) {
		return 0, false
	}
	return j, true
}

// phpCodeStarts reports, for every line through end, whether it begins inside
// a PHP block. Strings and comments are skipped so a ?> inside them does not
// flip the state, except that a one-line comment still ends at ?> as PHP does.
func phpCodeStarts(lines []string, end int) []bool {
	if end >= len(lines) {
		end = len(lines) - 1
	}
	starts := make([]bool, end+1)
	var scan phpScan
	for i := 0; i <= end; i++ {
		starts[i] = scan.inCode
		scan.line(lines[i])
	}
	return starts
}

type phpScan struct {
	inCode bool
	block  bool
	quote  rune
}

func (s *phpScan) line(text string) {
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if s.block {
			if r == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				s.block = false
				i++
			}
			continue
		}
		if s.quote != 0 {
			if r == '\\' {
				i++
				continue
			}
			if r == s.quote {
				s.quote = 0
			}
			continue
		}
		if !s.inCode {
			if r == '<' && i+1 < len(runes) && runes[i+1] == '?' {
				rest := string(runes[i:])
				switch {
				case len(rest) >= 5 && strings.EqualFold(rest[:5], "<?php"):
					s.inCode = true
					i += 4
				case strings.HasPrefix(rest, "<?="):
					s.inCode = true
					i += 2
				}
			}
			continue
		}
		switch {
		case r == '?' && i+1 < len(runes) && runes[i+1] == '>':
			s.inCode = false
			i++
		case r == '\'' || r == '"' || r == '`':
			s.quote = r
		case r == '/' && i+1 < len(runes) && runes[i+1] == '/':
			if j := strings.Index(string(runes[i:]), "?>"); j >= 0 {
				s.inCode = false
				i += j + 1
			} else {
				return
			}
		case r == '#' && !(i+1 < len(runes) && runes[i+1] == '['):
			if j := strings.Index(string(runes[i:]), "?>"); j >= 0 {
				s.inCode = false
				i += j + 1
			} else {
				return
			}
		case r == '/' && i+1 < len(runes) && runes[i+1] == '*':
			s.block = true
			i++
		}
	}
}

// commentDelimitersFor returns the delimiters that make text a whole-line
// comment. Handlebars also has a long form, {{!-- ... --}}, which is the only
// spelling that can wrap an expression containing }}. JSX and TSX accept
// either form, but only the marker matching the line's context is inserted.
func commentDelimitersFor(text, prefix, postfix, lang string) (string, string, bool) {
	if isJSXLanguage(lang) {
		if hasCommentPrefix(text, "{/*") && isWholeLineComment(text, "{/*", "*/}", lang) {
			return "{/*", "*/}", true
		}
		if prefix == "//" && hasCommentPrefix(text, "//") {
			return "//", "", true
		}
		return "", "", false
	}
	// PHP code uses "//"; an HTML-context line uses the generic <!-- -->
	// branch below.
	if lang == "php" && prefix == "//" {
		if hasCommentPrefix(text, "//") {
			return "//", "", true
		}
		return "", "", false
	}
	// Twig files carry both Twig and HTML comments; markup, script, and style
	// lines are recognized by the marker each context uses.
	if lang == "twig" {
		if hasCommentPrefix(text, "{#") && isWholeLineComment(text, "{#", "#}", lang) {
			return "{#", "#}", true
		}
		if hasCommentPrefix(text, "<!--") && isWholeLineComment(text, "<!--", "-->", lang) {
			return "<!--", "-->", true
		}
	}
	if lang == "sas" {
		if hasCommentPrefix(text, "/*") && isWholeLineComment(text, "/*", "*/", lang) {
			return "/*", "*/", true
		}
		if hasCommentPrefix(text, "*") && isWholeLineComment(text, "*", ";", lang) {
			return "*", ";", true
		}
		return "", "", false
	}
	if lang == "handlebars" {
		if hasCommentPrefix(text, "{{!--") && isWholeLineComment(text, "{{!--", "--}}", lang) {
			return "{{!--", "--}}", true
		}
		if hasCommentPrefix(text, "{{!") && isWholeLineComment(text, "{{!", "}}", lang) {
			return "{{!", "}}", true
		}
		return "", "", false
	}
	// INI parsers accept either marker; "#" is common even though the default
	// marker this editor inserts is ";".
	if lang == "ini" {
		if hasCommentPrefix(text, "#") {
			return "#", "", true
		}
		if hasCommentPrefix(text, ";") {
			return ";", "", true
		}
		return "", "", false
	}
	if !hasCommentPrefix(text, prefix) {
		return "", "", false
	}
	if postfix != "" && !isWholeLineComment(text, prefix, postfix, lang) {
		return "", "", false
	}
	return prefix, postfix, true
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

// isRSTCommentMarker reports whether trimmed starts a reStructuredText
// comment: two periods followed by whitespace or the end of the line.
func isRSTCommentMarker(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "..") {
		return false
	}
	rest := trimmed[2:]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// toggleRSTComment toggles an explicit markup comment block. The marker goes
// on the first non-blank line and every other line is indented under it,
// because a ".. " on each line would start a separate comment. Both that block
// form and a list of per-line comments are recognized when uncommenting.
func (e *EditorPaneWidget) toggleRSTComment(startLine, endLine int, before *undo.EditPosition) {
	nonBlank := make([]int, 0, endLine-startLine+1)
	for l := startLine; l <= endLine; l++ {
		if strings.TrimSpace(e.Buf.Lines[l]) != "" {
			nonBlank = append(nonBlank, l)
		}
	}
	if len(nonBlank) == 0 {
		return
	}
	indent := func(l int) int {
		line := e.Buf.Lines[l]
		return len([]rune(line)) - len([]rune(strings.TrimLeft(line, " \t")))
	}
	trimmed := func(l int) string { return strings.TrimLeft(e.Buf.Lines[l], " \t") }

	perLine := true
	for _, l := range nonBlank {
		if !isRSTCommentMarker(trimmed(l)) {
			perLine = false
			break
		}
	}
	first := nonBlank[0]
	block := !perLine && isRSTCommentMarker(trimmed(first))
	if block {
		base := indent(first)
		for _, l := range nonBlank[1:] {
			if indent(l) <= base {
				block = false
				break
			}
		}
	}

	selected := e.Selection != nil && e.Selection.Active
	cursorAtStart := false
	if selected {
		start, _ := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		cursorAtStart = start.Line == e.Cursor.Line && start.Col == e.Cursor.Col
	}
	mapColumns := func(line int, transform func(int, bool) int) {
		if line == e.Cursor.Line {
			e.Cursor.Col = transform(e.Cursor.Col, selected && cursorAtStart)
		}
		if selected && line == e.Selection.Anchor.Line {
			e.Selection.Anchor.Col = transform(e.Selection.Anchor.Col, !cursorAtStart)
		}
	}
	var cmds []undo.EditCommand
	insert := func(line, col int, text string) {
		cmd := &undo.InsertStringCommand{Line: line, Col: col, Text: text}
		cmd.Apply(e.Buf)
		cmds = append(cmds, cmd)
		n := len([]rune(text))
		mapColumns(line, func(c int, isStart bool) int {
			if c > col || (c == col && !isStart) {
				return c + n
			}
			return c
		})
	}
	remove := func(line, col, count int) {
		if count <= 0 {
			return
		}
		cmd := &undo.DeleteSelectionCommand{
			StartLine: line, StartCol: col,
			EndLine: line, EndCol: col + count,
		}
		cmd.Apply(e.Buf)
		cmds = append(cmds, cmd)
		mapColumns(line, func(c int, _ bool) int {
			if c > col {
				return max(col, c-count)
			}
			return c
		})
	}

	markerLen := func(l int) int {
		runes := []rune(e.Buf.Lines[l])
		col := indent(l)
		n := 2
		if col+2 < len(runes) && (runes[col+2] == ' ' || runes[col+2] == '\t') {
			n++
		}
		return n
	}

	switch {
	case perLine:
		for _, l := range nonBlank {
			remove(l, indent(l), markerLen(l))
		}
	case block:
		base := indent(first)
		remove(first, base, markerLen(first))
		for _, l := range nonBlank[1:] {
			runes := []rune(e.Buf.Lines[l])
			n := 0
			for n < 3 && base+n < len(runes) && runes[base+n] == ' ' {
				n++
			}
			remove(l, base, n)
		}
	default:
		base := indent(first)
		for _, l := range nonBlank {
			base = min(base, indent(l))
		}
		insert(first, base, ".. ")
		for _, l := range nonBlank[1:] {
			insert(l, base, "   ")
		}
	}

	e.clampCursor()
	if len(cmds) > 0 && e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds, Before: before, After: e.undoSelectionPosition()})
		e.bufferDirty = true
	}
	e.scrollViewport()
}

func (e *EditorPaneWidget) ToggleLineComment() {
	prefix, postfix := e.commentSyntax()
	if prefix == "" && postfix == "" {
		return
	}
	e.collapseMultiForLineOp()
	before := e.undoSelectionPosition()
	lang := e.commentLanguage()

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

	if lang == "rst" {
		e.toggleRSTComment(startLine, endLine, before)
		return
	}

	// {{! ... }} ends at the first }}, so a line holding an expression like
	// {{name}} needs the long form to keep the comment from closing early.
	if lang == "handlebars" {
		for l := startLine; l <= endLine; l++ {
			if strings.Contains(e.Buf.Lines[l], "}}") {
				prefix, postfix = "{{!--", "--}}"
				break
			}
		}
	}
	delims := e.commentDelimitersPerLine(startLine, endLine, prefix, postfix, lang)
	var cobolFixed []bool
	if lang == "cobol" {
		cobolFixed = cobolFixedFormatLines(e.Buf.Lines)
	}
	fixedAt := func(l int) bool { return l >= 0 && l < len(cobolFixed) && cobolFixed[l] }

	allCommented := true
	for l := startLine; l <= endLine; l++ {
		d := delims[l-startLine]
		if d.skip {
			continue
		}
		_, trimmed := e.commentMarkerStart(e.Buf.Lines[l], fixedAt(l))
		if trimmed == "" {
			continue
		}
		if _, _, ok := commentDelimitersFor(trimmed, d.prefix, d.postfix, lang); !ok {
			allCommented = false
			break
		}
	}

	tabW := e.resolveTabSize()
	commentCol := -1
	if !allCommented {
		for l := startLine; l <= endLine; l++ {
			if delims[l-startLine].skip {
				continue
			}
			line := e.Buf.Lines[l]
			if leadingWhitespace(line) == line {
				continue
			}
			marker, _ := e.commentMarkerStart(line, fixedAt(l))
			indent := bufColToVisualCol(line, marker, tabW)
			if commentCol < 0 || indent < commentCol {
				commentCol = indent
			}
		}
		if commentCol > 0 && commentCol%tabW != 0 {
			// A tab crossing the shallowest indent ends at the next tab stop.
			// Align before the earliest such tab to preserve every indent byte.
			shallowest := commentCol
			for l, line := range e.Buf.Lines[startLine : endLine+1] {
				if delims[l].skip || leadingWhitespace(line) == line {
					continue
				}
				col := visualColToBufCol(line, shallowest, tabW)
				commentCol = min(commentCol, bufColToVisualCol(line, col, tabW))
			}
		}
	}

	selected := e.Selection != nil && e.Selection.Active
	cursorAtStart := false
	if selected {
		start, _ := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		cursorAtStart = start.Line == e.Cursor.Line && start.Col == e.Cursor.Col
	}
	// The selection's start stays before an insertion at its boundary; its
	// end follows it so newly added comment markers remain selected.
	mapColumns := func(line int, transform func(int, bool) int) {
		if line == e.Cursor.Line {
			e.Cursor.Col = transform(e.Cursor.Col, selected && cursorAtStart)
		}
		if selected && line == e.Selection.Anchor.Line {
			e.Selection.Anchor.Col = transform(e.Selection.Anchor.Col, !cursorAtStart)
		}
	}
	removeColumns := func(line, col, count int) {
		mapColumns(line, func(c int, _ bool) int {
			if c > col {
				return max(col, c-count)
			}
			return c
		})
	}
	var cmds []undo.EditCommand

	for l := startLine; l <= endLine; l++ {
		d := delims[l-startLine]
		if d.skip {
			continue
		}
		runes := []rune(e.Buf.Lines[l])
		indent, trimmed := e.commentMarkerStart(e.Buf.Lines[l], fixedAt(l))
		if trimmed == "" {
			continue
		}

		if allCommented {
			removePrefix, removePostfix, _ := commentDelimitersFor(trimmed, d.prefix, d.postfix, lang)
			removePrefixLen := len([]rune(removePrefix))
			removePostfixLen := len([]rune(removePostfix))
			removeLen := removePrefixLen
			if indent+removeLen < len(runes) && runes[indent+removeLen] == ' ' {
				removeLen++
			}
			cmd := &undo.DeleteSelectionCommand{
				StartLine: l, StartCol: indent,
				EndLine: l, EndCol: indent + removeLen,
			}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			removeColumns(l, indent, removeLen)
			if removePostfix != "" {
				tail := []rune(strings.TrimRight(e.Buf.Lines[l], " \t"))
				postStart := len(tail) - removePostfixLen
				postRemoveLen := removePostfixLen
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
					removeColumns(l, postStart, postRemoveLen)
				}
				if removePrefix == "<!--" {
					if unescaped := unescapeXMLCommentText(e.Buf.Lines[l]); unescaped != e.Buf.Lines[l] {
						line := e.Buf.Lines[l]
						mapColumns(l, func(c int, isStart bool) int {
							return unescapeXMLCommentColumn(line, c, isStart)
						})
						replace := &undo.ReplaceLinesCommand{
							Start: l, OldLines: []string{e.Buf.Lines[l]}, NewLines: []string{unescaped},
						}
						replace.Apply(e.Buf)
						cmds = append(cmds, replace)
					}
				}
			}
		} else {
			prefix, postfix := d.prefix, d.postfix
			prefixLen := len([]rune(prefix))
			postfixLen := len([]rune(postfix))
			if prefix == "<!--" {
				if escaped := escapeXMLCommentText(e.Buf.Lines[l]); escaped != e.Buf.Lines[l] {
					mapColumns(l, func(c int, _ bool) int {
						c = max(0, min(c, len(runes)))
						end := len(string(runes[:c]))
						return len([]rune(escapeXMLCommentTextPrefix(string(runes), end)))
					})
					replace := &undo.ReplaceLinesCommand{
						Start: l, OldLines: []string{e.Buf.Lines[l]}, NewLines: []string{escaped},
					}
					replace.Apply(e.Buf)
					cmds = append(cmds, replace)
				}
			}
			col := visualColToBufCol(e.Buf.Lines[l], commentCol, tabW)
			if lang == "cobol" && col < 6 && fixedAt(l) && isCOBOLSequenceArea(e.Buf.Lines[l]) {
				col = 6
			}
			if d.col >= 0 {
				col = d.col
			}
			cmd := &undo.InsertStringCommand{Line: l, Col: col, Text: prefix + " "}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			mapColumns(l, func(c int, isStart bool) int {
				if c > col || (c == col && !isStart) {
					return c + prefixLen + 1
				}
				return c
			})
			if postfix != "" {
				endCol := len([]rune(strings.TrimRight(e.Buf.Lines[l], " \t")))
				add := &undo.InsertStringCommand{Line: l, Col: endCol, Text: " " + postfix}
				add.Apply(e.Buf)
				cmds = append(cmds, add)
				mapColumns(l, func(c int, isStart bool) int {
					if c > endCol || (selected && c == endCol && !isStart) {
						return c + postfixLen + 1
					}
					return c
				})
			}
		}
	}

	e.clampCursor()
	if len(cmds) > 0 && e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds, Before: before, After: e.undoSelectionPosition()})
		e.bufferDirty = true
	}
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
