package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
)

// An external grammar configured with the "Shell" alias must comment with "#"
// even though its language ID is not the canonical "shellscript".
func TestCommentSyntaxResolvesShellAliasToHash(t *testing.T) {
	grammar := filepath.Join(t.TempDir(), "custom-shell.json")
	if err := os.WriteFile(grammar, []byte(`{"scopeName": "source.custom-shell", "name": "Custom Shell", "patterns": []}`), 0600); err != nil {
		t.Fatal(err)
	}
	highlight.SetExternalGrammars([]highlight.ExternalGrammar{
		{Path: grammar, Language: "Shell", FileTypes: []string{"csh"}},
	})
	t.Cleanup(func() { highlight.SetExternalGrammars(nil) })

	h := highlight.New("a.csh")
	if h == nil || h.Language() != "Shell" {
		t.Fatalf("a.csh highlighter = %v, want the configured display name Shell", h)
	}
	e := &EditorPaneWidget{Highlighter: h}
	if prefix, postfix := e.commentSyntax(); prefix != "#" || postfix != "" {
		t.Fatalf("comment syntax = %q/%q, want # with no postfix", prefix, postfix)
	}
}

// The bundled x86 assembly grammar claims .asm, .s and related extensions and
// uses ";" comments, so toggling must round-trip on each of them.
func TestToggleLineCommentAssemblyUsesSemicolon(t *testing.T) {
	for _, file := range []string{"a.asm", "a.s", "a.nasm", "a.inc"} {
		t.Run(file, func(t *testing.T) {
			const original = "mov eax, 1"
			e := newCommentEditor(t, file, []string{original}, 0, 0)

			e.ToggleLineComment()
			if got, want := e.Buf.Lines[0], "; mov eax, 1"; got != want {
				t.Fatalf("commented line = %q, want %q", got, want)
			}

			e.ToggleLineComment()
			if got := e.Buf.Lines[0]; got != original {
				t.Fatalf("uncommented line = %q, want %q", got, original)
			}
		})
	}
}

// COBOL's floating comment marker is "*>". Fixed-format code usually starts
// in the indicator or area A/B columns, so the marker keeps the line's
// indentation and round-trips.
func TestToggleLineCommentCOBOLUsesFloatingMarker(t *testing.T) {
	original := []string{"       DISPLAY 'HELLO'.", "       STOP RUN."}
	e := newCommentEditor(t, "a.cob", slices.Clone(original), 1, len([]rune(original[1])))
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"       *> DISPLAY 'HELLO'.", "       *> STOP RUN."}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

// A fixed-format line can carry a six-column sequence number field. The marker
// must land after it, not inside it, or the line is not a comment at all.
func TestToggleLineCommentCOBOLSkipsSequenceNumberArea(t *testing.T) {
	const original = "000100 MOVE WS-A TO WS-B."
	e := newCommentEditor(t, "a.cob", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "000100*>  MOVE WS-A TO WS-B."; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// Handlebars' short comment form is "{{! ... }}", and unwrapping must find it
// again even though the wrapped line now ends in "}}".
func TestToggleLineCommentHandlebarsUsesBraceMarker(t *testing.T) {
	const original = "  <p>Hello</p>"
	e := newCommentEditor(t, "a.hbs", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "  {{! <p>Hello</p> }}"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// "{{! ... }}" closes at the first "}}", so an HTML line holding a Handlebars
// expression must use the long form instead.
func TestToggleLineCommentHandlebarsUsesLongFormForExpressions(t *testing.T) {
	const original = "  <p>{{name}}</p>"
	e := newCommentEditor(t, "a.hbs", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "  {{!-- <p>{{name}}</p> --}}"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// INI lines are commented with ";" by default and must round-trip.
func TestToggleLineCommentINIUsesSemicolon(t *testing.T) {
	const original = "key = value"
	e := newCommentEditor(t, "a.ini", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "; key = value"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// A parser may use "#" instead of ";", so an existing selection commented
// with either marker must uncomment with that line's own marker.
func TestToggleLineCommentINIUncommentsBothMarkers(t *testing.T) {
	original := []string{"; key = value", "# other = 1"}
	e := newCommentEditor(t, "a.ini", slices.Clone(original), 1, len([]rune(original[1])))
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"key = value", "other = 1"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, want)
	}
}

// JSX code lines use // even when the line contains markup later on.
func TestToggleLineCommentJSXCodeUsesSlashSlash(t *testing.T) {
	const original = "const el = <div />;"
	e := newCommentEditor(t, "a.jsx", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "// const el = <div />;"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// A line that starts in JSX markup is wrapped in a comment expression.
func TestToggleLineCommentJSXMarkupUsesExpressionComment(t *testing.T) {
	const original = "  <div>Hello</div>"
	e := newCommentEditor(t, "a.jsx", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "  {/* <div>Hello</div> */}"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// A mixed selection picks the marker per line and restores every line.
func TestToggleLineCommentTSXMixedSelection(t *testing.T) {
	original := []string{"const x = 1;", "<div>", "  hello", "</div>"}
	e := newCommentEditor(t, "a.tsx", slices.Clone(original), 3, len([]rune(original[3])))
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"// const x = 1;", "{/* <div> */}", "{/*   hello */}", "{/* </div> */}"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

// Markdown has no comment syntax of its own, so a line toggle uses an HTML
// comment, which the common parsers pass through.
func TestToggleLineCommentMarkdownUsesHTMLComment(t *testing.T) {
	const original = "# Title"
	e := newCommentEditor(t, "a.md", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<!-- # Title -->"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// Markdown prose often holds runs of hyphens, which are invalid inside an HTML
// comment, so they must be encoded and restored like any other HTML context.
func TestToggleLineCommentMarkdownEscapesHyphenRuns(t *testing.T) {
	const original = "a -- b --- c"
	e := newCommentEditor(t, "a.md", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<!-- a &#45;&#45; b &#45;&#45;&#45; c -->"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// Close tags inside PHP strings and block comments are not real close tags,
// but one inside a // comment does end the block, as PHP itself does.
func TestPHPCodeStarts(t *testing.T) {
	lines := []string{
		"<h1>Hi</h1>",
		"<?php",
		"$s = '?>';",
		"/* ?> */",
		"// comment ?>",
		"<p>html</p>",
	}
	want := []bool{false, false, true, true, true, false}
	got := phpCodeStarts(lines, len(lines)-1)
	if !slices.Equal(got, want) {
		t.Fatalf("phpCodeStarts = %v, want %v", got, want)
	}
}

// PHP code inside <?php ... ?> takes //, unlike the HTML around it.
func TestToggleLineCommentPHPCodeUsesSlashSlash(t *testing.T) {
	const original = "$name = 'x';"
	e := newCommentEditor(t, "a.php", []string{"<?php", original}, 1, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[1], "// $name = 'x';"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[1]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// Content after a closing tag is HTML, so // there would print text instead of
// commenting the markup.
func TestToggleLineCommentPHPHTMLUsesHTMLComment(t *testing.T) {
	const original = "<div>hi</div>"
	e := newCommentEditor(t, "a.php", []string{"<?php", "echo 'hi';", "?>", original}, 3, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[3], "<!-- <div>hi</div> -->"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[3]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// Code on an opening-tag line is commented after the tag so the tag stays
// effective, and the marker round-trips.
func TestToggleLineCommentPHPCommentsAfterOpenTag(t *testing.T) {
	const original = "<?php echo $name; ?>"
	e := newCommentEditor(t, "a.php", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<?php // echo $name; ?>"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// A lone opening tag has no line-comment position; it must be left intact.
func TestToggleLineCommentPHPLeavesLoneOpenTag(t *testing.T) {
	e := newCommentEditor(t, "a.php", []string{"<?php"}, 0, 0)
	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != "<?php" {
		t.Fatalf("line = %q, want it unchanged", got)
	}
}

// A mixed file picks the marker per line and restores every line.
func TestToggleLineCommentPHPMixedSelection(t *testing.T) {
	original := []string{"<?php", "$name = 'x';", "?>", "<div>hi</div>"}
	e := newCommentEditor(t, "a.php", slices.Clone(original), 3, len([]rune(original[3])))
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"<?php", "// $name = 'x';", "// ?>", "<!-- <div>hi</div> -->"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

// A single reStructuredText line is commented with the ".. " marker.
func TestToggleLineCommentRSTSingleLine(t *testing.T) {
	const original = "Some text."
	e := newCommentEditor(t, "a.rst", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], ".. Some text."; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// A multi-line toggle makes one comment block: the marker on the first line
// and the rest indented under it, not a separate comment per line.
func TestToggleLineCommentRSTBlock(t *testing.T) {
	original := []string{"First line.", "Second line."}
	e := newCommentEditor(t, "a.rst", slices.Clone(original), 1, len([]rune(original[1])))
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{".. First line.", "   Second line."}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

// An indented block keeps its indentation and its continuation lines stay
// aligned under the marker's text.
func TestToggleLineCommentRSTIndentedBlock(t *testing.T) {
	original := []string{"  item one", "  item two"}
	e := newCommentEditor(t, "a.rst", slices.Clone(original), 1, len([]rune(original[1])))
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 2)

	e.ToggleLineComment()
	want := []string{"  .. item one", "     item two"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

// An existing comment block is recognized and unwrapped as one comment.
func TestToggleLineCommentRSTUncommentsBlock(t *testing.T) {
	e := newCommentEditor(t, "a.rst", []string{".. Comment.", "   More comment."}, 1, 15)
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"Comment.", "More comment."}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, want)
	}
}

// Lines individually commented with ".. " each carry their own marker.
func TestToggleLineCommentRSTPerLineComments(t *testing.T) {
	e := newCommentEditor(t, "a.rst", []string{".. one", ".. two"}, 1, 6)
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"one", "two"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, want)
	}
}

// SAS statements may contain semicolons, so the block comment is the only
// safe per-line form.
func TestToggleLineCommentSASWrapsWithBlockComment(t *testing.T) {
	const original = "data x; y = 1; run;"
	e := newCommentEditor(t, "a.sas", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "/* data x; y = 1; run; */"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

// A statement commented with "* ... ;" is recognized when unwrapping.
func TestToggleLineCommentSASUncommentsStatement(t *testing.T) {
	e := newCommentEditor(t, "a.sas", []string{"* show the input data;"}, 0, 5)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "show the input data"; got != want {
		t.Fatalf("uncommented line = %q, want %q", got, want)
	}
}

// Every selected line is wrapped separately, keeping the statement on one line.
func TestToggleLineCommentSASWrapsSelection(t *testing.T) {
	e := newCommentEditor(t, "a.sas", []string{"data x;", "run;"}, 1, 4)
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"/* data x; */", "/* run; */"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}
}

func TestToggleLineCommentSvelteUsesMarkupComment(t *testing.T) {
	original := []string{"<div>", "  <p>Hello</p>", "</div>"}
	e := newCommentEditor(t, "a.svelte", slices.Clone(original), 1, 6)

	e.ToggleLineComment()
	want := []string{"<div>", "  <!-- <p>Hello</p> -->", "</div>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentSvelteUsesLineCommentInScript(t *testing.T) {
	original := []string{"<script>", "  let x = 1;", "</script>"}
	e := newCommentEditor(t, "a.svelte", slices.Clone(original), 1, 4)

	e.ToggleLineComment()
	want := []string{"<script>", "  // let x = 1;", "</script>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentSvelteUsesBlockCommentInStyle(t *testing.T) {
	original := []string{"<style>", "  .a { color: red; }", "</style>"}
	e := newCommentEditor(t, "a.svelte", slices.Clone(original), 1, 4)

	e.ToggleLineComment()
	want := []string{"<style>", "  /* .a { color: red; } */", "</style>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentSvelteUsesLineCommentInExpression(t *testing.T) {
	original := []string{"<div>", "  {items.map((item) => (", "    item.name", "  ))}", "</div>"}
	e := newCommentEditor(t, "a.svelte", slices.Clone(original), 2, 6)

	e.ToggleLineComment()
	want := []string{"<div>", "  {items.map((item) => (", "    // item.name", "  ))}", "</div>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentTwigUsesTwigComment(t *testing.T) {
	original := []string{"<div>", "  <p>Hello</p>", "</div>"}
	e := newCommentEditor(t, "a.twig", slices.Clone(original), 1, 6)

	e.ToggleLineComment()
	want := []string{"<div>", "  {# <p>Hello</p> #}", "</div>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentTwigUsesLineCommentInScript(t *testing.T) {
	original := []string{"<script>", "  let x = 1;", "</script>"}
	e := newCommentEditor(t, "a.twig", slices.Clone(original), 1, 4)

	e.ToggleLineComment()
	want := []string{"<script>", "  // let x = 1;", "</script>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentTwigUsesBlockCommentInStyle(t *testing.T) {
	original := []string{"<style>", "  .a { color: red; }", "</style>"}
	e := newCommentEditor(t, "a.twig", slices.Clone(original), 1, 4)

	e.ToggleLineComment()
	want := []string{"<style>", "  /* .a { color: red; } */", "</style>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentTwigUsesHTMLCommentInVerbatim(t *testing.T) {
	original := []string{"{% verbatim %}", "  <p>literal</p>", "{% endverbatim %}"}
	e := newCommentEditor(t, "a.twig", slices.Clone(original), 1, 6)

	e.ToggleLineComment()
	want := []string{"{% verbatim %}", "  <!-- <p>literal</p> -->", "{% endverbatim %}"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentTwigUncommentsHTMLComment(t *testing.T) {
	e := newCommentEditor(t, "a.twig", []string{"  <!-- <p>Hello</p> -->"}, 0, 8)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "  <p>Hello</p>"; got != want {
		t.Fatalf("uncommented line = %q, want %q", got, want)
	}
}

func TestToggleLineCommentVimLegacyUsesDoubleQuote(t *testing.T) {
	original := []string{"\" a header", "let x = 1"}
	e := newCommentEditor(t, "a.vim", slices.Clone(original), 1, 0)

	e.ToggleLineComment()
	want := []string{"\" a header", "\" let x = 1"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentVim9UsesHash(t *testing.T) {
	original := []string{"vim9script", "", "var x = 1"}
	e := newCommentEditor(t, "a.vim", slices.Clone(original), 2, 0)

	e.ToggleLineComment()
	want := []string{"vim9script", "", "# var x = 1"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentVim9UncommentsHash(t *testing.T) {
	e := newCommentEditor(t, "a.vim", []string{"vim9script", "# var x = 1"}, 1, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[1], "var x = 1"; got != want {
		t.Fatalf("uncommented line = %q, want %q", got, want)
	}
}

func TestIsVim9Script(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		want  bool
	}{
		{"first line", []string{"vim9script"}, true},
		{"indented", []string{"  vim9script  "}, true},
		{"after comments", []string{"\" header", "", "vim9script"}, true},
		{"legacy", []string{"\" header", "let x = 1"}, false},
		{"not first command", []string{"let x = 1", "vim9script"}, false},
		{"longer word", []string{"vim9scriptx"}, false},
		{"empty", nil, false},
	} {
		if got := isVim9Script(tc.lines); got != tc.want {
			t.Errorf("%s: isVim9Script(%q) = %v, want %v", tc.name, tc.lines, got, tc.want)
		}
	}
}

func TestToggleLineCommentVueUsesMarkupComment(t *testing.T) {
	original := []string{"<template>", "  <p>Hello</p>", "</template>"}
	e := newCommentEditor(t, "a.vue", slices.Clone(original), 1, 6)

	e.ToggleLineComment()
	want := []string{"<template>", "  <!-- <p>Hello</p> -->", "</template>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentVueUsesLineCommentInScript(t *testing.T) {
	original := []string{"<script setup lang=\"ts\">", "  const x = 1;", "</script>"}
	e := newCommentEditor(t, "a.vue", slices.Clone(original), 1, 4)

	e.ToggleLineComment()
	want := []string{"<script setup lang=\"ts\">", "  // const x = 1;", "</script>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentVueUsesBlockCommentInStyle(t *testing.T) {
	original := []string{"<style scoped>", "  .a { color: red; }", "</style>"}
	e := newCommentEditor(t, "a.vue", slices.Clone(original), 1, 4)

	e.ToggleLineComment()
	want := []string{"<style scoped>", "  /* .a { color: red; } */", "</style>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentVueElementSelectorInStyle(t *testing.T) {
	original := []string{"<style scoped>", "  body { margin: 0; }", "</style>"}
	e := newCommentEditor(t, "a.vue", slices.Clone(original), 1, 4)

	e.ToggleLineComment()
	want := []string{"<style scoped>", "  /* body { margin: 0; } */", "</style>"}
	if !slices.Equal(e.Buf.Lines, want) {
		t.Fatalf("commented lines = %q, want %q", e.Buf.Lines, want)
	}

	e.ToggleLineComment()
	if !slices.Equal(e.Buf.Lines, original) {
		t.Fatalf("uncommented lines = %q, want %q", e.Buf.Lines, original)
	}
}

func TestToggleLineCommentWebAssemblyUsesDoubleSemicolon(t *testing.T) {
	original := "(func (export \"f\"))"
	e := newCommentEditor(t, "a.wat", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], ";; (func (export \"f\"))"; got != want {
		t.Fatalf("commented line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("uncommented line = %q, want %q", got, original)
	}
}

func newCommentEditor(t *testing.T, filename string, lines []string, line, col int) *EditorPaneWidget {
	t.Helper()
	h := highlight.New(filename)
	if h == nil {
		t.Fatalf("no highlighter for %s", filename)
	}
	return newCommentEditorWith(t, h, lines, line, col)
}

func newCommentEditorWith(t *testing.T, h *highlight.Highlighter, lines []string, line, col int) *EditorPaneWidget {
	t.Helper()
	e := NewEditorPaneWidget(
		&buffer.Buffer{Lines: lines},
		&cursor.Cursor{Line: line, Col: col},
		&view.Viewport{Width: 80, Height: 10},
	)
	e.Highlighter = h
	return e
}

// An inner "-->" would end the new HTML comment early, so wrapping must escape
// it and toggling back off must restore the line exactly.
func TestToggleLineCommentEscapesInlineHTMLComments(t *testing.T) {
	const original = "<!-- a -->hello<!-- b -->"
	e := newCommentEditor(t, "a.html", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<!-- <!&#45;&#45; a &#45;&#45;>hello<!&#45;&#45; b &#45;&#45;> -->"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}
	if strings.Contains(e.Buf.Lines[0][len("<!-- "):len(e.Buf.Lines[0])-len(" -->")], "--") {
		t.Fatalf("wrapped content = %q, contains a raw -- run", e.Buf.Lines[0])
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("unwrapped line = %q, want %q", got, original)
	}
}

// A mixed selection comments every line, including lines that already hold a
// comment pair, and the second toggle restores both original lines.
func TestToggleLineCommentEscapesMixedSelection(t *testing.T) {
	original := []string{"<!-- a -->", "<div>hello</div>"}
	e := newCommentEditor(t, "a.html", append([]string(nil), original...), 1, len([]rune(original[1])))
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{"<!-- <!&#45;&#45; a &#45;&#45;> -->", "<!-- <div>hello</div> -->"}
	for i := range want {
		if e.Buf.Lines[i] != want[i] {
			t.Fatalf("wrapped line %d = %q, want %q", i, e.Buf.Lines[i], want[i])
		}
	}

	e.ToggleLineComment()
	for i := range original {
		if e.Buf.Lines[i] != original[i] {
			t.Fatalf("unwrapped line %d = %q, want %q", i, e.Buf.Lines[i], original[i])
		}
	}
}

// OCaml comments nest, so inner closers must not stop an outer wrapper from
// being recognized as commented.
func TestToggleLineCommentRecognizesNestedComments(t *testing.T) {
	e := newCommentEditor(t, "a.ml", []string{"(* outer (* inner *) *)"}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "outer (* inner *)"; got != want {
		t.Fatalf("uncommented line = %q, want %q", got, want)
	}
}

// A delimiter quoted in a string literal is text, not nesting, so a quoted
// opener must not make the wrapper look unbalanced and wrap the line twice.
func TestToggleLineCommentIgnoresDelimitersInStringLiterals(t *testing.T) {
	const original = `let opener = "(*" in opener`
	e := newCommentEditor(t, "a.ml", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], `(* let opener = "(*" in opener *)`; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("unwrapped line = %q, want %q", got, original)
	}
}

// A quote character in a character literal is not a string opener, so the
// wrapper must be recognized as balanced and wrap the line only once.
func TestToggleLineCommentRoundTripsCharLiteralQuote(t *testing.T) {
	const original = `let quote = '"'`
	e := newCommentEditor(t, "a.ml", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], `(* let quote = '"' *)`; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("unwrapped line = %q, want %q", got, original)
	}
}

// A quote inside an OCaml quoted string is text, so the literal must be skipped
// whole and the wrapper recognized as balanced on the next toggle.
func TestToggleLineCommentRoundTripsQuotedString(t *testing.T) {
	const original = `let text = {| " |}`
	e := newCommentEditor(t, "a.ml", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], `(* let text = {| " |} *)`; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("unwrapped line = %q, want %q", got, original)
	}
}

// OCaml quoted-string ids are lowercase letters and underscores, so "{1|" does
// not open a literal and must not hide the wrapper's closer.
func TestToggleLineCommentRejectsDigitLeadingQuotedStringID(t *testing.T) {
	const original = "(* {1| *)"
	if _, ok := skipQuotedString(original, strings.Index(original, "{")); ok {
		t.Fatalf("skipQuotedString(%q) reported a quoted string", original)
	}
	e := newCommentEditor(t, "a.ml", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "{1|"; got != want {
		t.Fatalf("uncommented line = %q, want %q", got, want)
	}
}

// A non-ASCII lowercase id opens a quoted string, so its quotes stay comment
// text and the wrapper is still removed.
func TestToggleLineCommentRecognizesUTF8QuotedStringID(t *testing.T) {
	const original = "(* {é|\"|é} *)"
	if _, ok := skipQuotedString(original, strings.Index(original, "{")); !ok {
		t.Fatalf("skipQuotedString(%q) did not report a quoted string", original)
	}
	e := newCommentEditor(t, "a.ml", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "{é|\"|é}"; got != want {
		t.Fatalf("uncommented line = %q, want %q", got, want)
	}
}

// Greek letters are not quoted-string id characters, so "{α|" does not open a
// literal: the wrapper's closer is seen and removed instead of being swallowed.
func TestToggleLineCommentTreatsGreekDelimiterAsCommentText(t *testing.T) {
	const original = "(* {α| *)"
	if _, ok := skipQuotedString(original, strings.Index(original, "{")); ok {
		t.Fatalf("skipQuotedString(%q) reported a quoted string", original)
	}
	e := newCommentEditor(t, "a.ml", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "{α|"; got != want {
		t.Fatalf("uncommented line = %q, want %q", got, want)
	}
}

func TestSkipQuotedStringRequiresExactCloser(t *testing.T) {
	for _, id := range []string{"", "tag", "é"} {
		t.Run(id, func(t *testing.T) {
			literal := "{" + id + "| ignored |other} |" + id + "}"
			text := "(* " + literal + " *)"
			end, ok := skipQuotedString(text, 3)
			if !ok || end != 3+len(literal) {
				t.Fatalf("skipQuotedString(%q) = (%d, %v), want (%d, true)", text, end, ok, 3+len(literal))
			}
		})
	}
}

// Coq shares OCaml's comment delimiters but has no quoted-string literal, so
// the "{|" inside the wrapper is comment text and the closer still ends the
// line: toggling must remove the wrapper instead of adding a second one.
func TestToggleLineCommentCoqKeepsBracePipeInComment(t *testing.T) {
	// .v is also claimed by other grammars, so select Coq by language ID.
	h := highlight.NewForLanguage("coq")
	if h == nil {
		t.Fatal("no highlighter for coq")
	}
	e := newCommentEditorWith(t, h, []string{"(* {| *)"}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "{|"; got != want {
		t.Fatalf("uncommented line = %q, want %q", got, want)
	}
}

// Pre-existing entity text must survive a wrap/unwrap round trip: unescaping
// has to reverse only what the wrapper itself encoded.
func TestToggleLineCommentPreservesEntityText(t *testing.T) {
	const original = "&amp;lt;!-- &nbsp;"
	e := newCommentEditor(t, "a.html", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<!-- &amp;amp;lt;!&#45;&#45; &amp;nbsp; -->"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("round-tripped line = %q, want %q", got, original)
	}
}

// An unpaired "--" run is invalid inside an XML comment but is not a
// delimiter, so it must be encoded too and restored verbatim on unwrap.
func TestToggleLineCommentEscapesDoubleHyphenRuns(t *testing.T) {
	const original = "a--b --- c - d"
	e := newCommentEditor(t, "a.xml", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "<!-- a&#45;&#45;b &#45;&#45;&#45; c - d -->"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("unwrapped line = %q, want %q", got, original)
	}
}

// A cursor between two adjacent hyphens sits inside one encoded run, so it must
// land after the first &#45; entity rather than inside it.
func TestToggleLineCommentMovesCursorPastEncodedHyphenRun(t *testing.T) {
	const original = "a--b"
	e := newCommentEditor(t, "a.xml", []string{original}, 0, 2)

	e.ToggleLineComment()
	escaped := escapeXMLCommentText(original)
	if got, want := e.Buf.Lines[0], "<!-- "+escaped+" -->"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}
	wantCol := len([]rune("<!-- " + escaped[:strings.Index(escaped, "&#45;")+len("&#45;")]))
	if e.Cursor.Col != wantCol {
		t.Fatalf("cursor col = %d, want %d (just past the first encoded hyphen)", e.Cursor.Col, wantCol)
	}
}

// Batch's REM is a keyword, so an identifier that merely starts with it is code
// and must get wrapped instead of being uncommented.
func TestToggleLineCommentKeepsWordPrefixIdentifiers(t *testing.T) {
	const original = "REMOVE_ME=1"
	e := newCommentEditor(t, "a.bat", []string{original}, 0, 0)

	e.ToggleLineComment()
	if got, want := e.Buf.Lines[0], "REM REMOVE_ME=1"; got != want {
		t.Fatalf("wrapped line = %q, want %q", got, want)
	}

	e.ToggleLineComment()
	if got := e.Buf.Lines[0]; got != original {
		t.Fatalf("unwrapped line = %q, want %q", got, original)
	}
}

func TestToggleLineCommentMatchesWordPrefixCaseInsensitively(t *testing.T) {
	tests := []struct {
		name, line, want string
	}{
		{"lowercase prefix", "rem note", "note"},
		{"prefix alone", "REM", ""},
		{"prefix inside identifier", "REMOTE", "REM REMOTE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newCommentEditor(t, "a.bat", []string{tt.line}, 0, 0)
			e.ToggleLineComment()
			if got := e.Buf.Lines[0]; got != tt.want {
				t.Fatalf("toggled line = %q, want %q", got, tt.want)
			}
		})
	}
}

// A block is commented at its shallowest indentation, as VS Code does, so the
// markers line up and each line keeps its indentation inside the comment.
func TestToggleLineCommentAlignsMarkersAtTheShallowestIndent(t *testing.T) {
	original := []string{
		"function showOutdated() {",
		"    sync(vendorOptions, {",
		"        shouldUpdate: true,",
		"",
		"    });",
		"}",
	}
	e := newCommentEditor(t, "a.js", append([]string(nil), original...), 5, 1)
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 0)

	e.ToggleLineComment()
	want := []string{
		"// function showOutdated() {",
		"//     sync(vendorOptions, {",
		"//         shouldUpdate: true,",
		"",
		"//     });",
		"// }",
	}
	for i := range want {
		if e.Buf.Lines[i] != want[i] {
			t.Fatalf("commented line %d = %q, want %q", i, e.Buf.Lines[i], want[i])
		}
	}

	e.ToggleLineComment()
	for i := range original {
		if e.Buf.Lines[i] != original[i] {
			t.Fatalf("uncommented line %d = %q, want %q", i, e.Buf.Lines[i], original[i])
		}
	}
}

// An indented block is commented at its own shallowest indentation, not at
// column zero.
func TestToggleLineCommentKeepsTheBlockIndentation(t *testing.T) {
	original := []string{"    if ok {", "        run()", "    }"}
	e := newCommentEditor(t, "a.go", append([]string(nil), original...), 2, 5)
	e.Selection = &selection.Selection{}
	e.Selection.Start(0, 4)

	e.ToggleLineComment()
	want := []string{"    // if ok {", "    //     run()", "    // }"}
	for i := range want {
		if e.Buf.Lines[i] != want[i] {
			t.Fatalf("commented line %d = %q, want %q", i, e.Buf.Lines[i], want[i])
		}
	}

	e.ToggleLineComment()
	for i := range original {
		if e.Buf.Lines[i] != original[i] {
			t.Fatalf("uncommented line %d = %q, want %q", i, e.Buf.Lines[i], original[i])
		}
	}
}

func TestToggleLineCommentTracksSelectionEndpoints(t *testing.T) {
	for _, tc := range []struct {
		name, file         string
		lines              []string
		start, end         selection.Position
		wantStart, wantEnd selection.Position
	}{
		{"whole block", "a.js", []string{"function f() {", "    run();", "}"}, selection.Position{Line: 0, Col: 0}, selection.Position{Line: 2, Col: 1}, selection.Position{Line: 0, Col: 0}, selection.Position{Line: 2, Col: 4}},
		{"partial unicode", "a.go", []string{"    界foo()", "        bar()"}, selection.Position{Line: 0, Col: 5}, selection.Position{Line: 1, Col: 11}, selection.Position{Line: 0, Col: 8}, selection.Position{Line: 1, Col: 14}},
		{"next line excluded", "a.go", []string{"foo()", "bar()", "next()"}, selection.Position{Line: 0, Col: 0}, selection.Position{Line: 2, Col: 0}, selection.Position{Line: 0, Col: 0}, selection.Position{Line: 2, Col: 0}},
		{"blank endpoint", "a.go", []string{"", "foo()"}, selection.Position{Line: 0, Col: 0}, selection.Position{Line: 1, Col: 5}, selection.Position{Line: 0, Col: 0}, selection.Position{Line: 1, Col: 8}},
		{"wrapped escaping", "a.xml", []string{"a--b", "<x/>"}, selection.Position{Line: 0, Col: 0}, selection.Position{Line: 1, Col: 4}, selection.Position{Line: 0, Col: 0}, selection.Position{Line: 1, Col: 13}},
		{"partial escaping", "a.xml", []string{"a--b"}, selection.Position{Line: 0, Col: 2}, selection.Position{Line: 0, Col: 4}, selection.Position{Line: 0, Col: 11}, selection.Position{Line: 0, Col: 21}},
	} {
		for _, backward := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/backward=%v", tc.name, backward), func(t *testing.T) {
				anchor, cur := tc.start, tc.end
				if backward {
					anchor, cur = cur, anchor
				}
				e := newCommentEditor(t, tc.file, slices.Clone(tc.lines), cur.Line, cur.Col)
				e.Selection = &selection.Selection{}
				e.Selection.Start(anchor.Line, anchor.Col)
				e.ToggleLineComment()
				start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
				if start != tc.wantStart || end != tc.wantEnd {
					t.Fatalf("selection = %v..%v, want %v..%v", start, end, tc.wantStart, tc.wantEnd)
				}
				if tc.name == "whole block" {
					e.SetRect(Rect{W: 80, H: 10})
					grid := renderGrid(e)
					for x := e.GutterWidth(); x < e.GutterWidth()+2; x++ {
						if cell := grid[0][x]; cell.Ch != '/' || cell.BgStyle != term.StyleSelection {
							t.Fatalf("first comment marker lost selection highlighting: %+v", cell)
						}
					}
				}
				e.ToggleLineComment()
				if !slices.Equal(e.Buf.Lines, tc.lines) || e.Selection.Anchor != anchor || e.Cursor.Line != cur.Line || e.Cursor.Col != cur.Col {
					t.Fatalf("round trip: lines=%q anchor=%v cursor=%d:%d", e.Buf.Lines, e.Selection.Anchor, e.Cursor.Line, e.Cursor.Col)
				}
			})
		}
	}
}

func TestToggleLineCommentAlignsMixedIndentation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		lines, want []string
	}{
		{"equal display indent", []string{"\tfoo()", "    bar()", "\t"}, []string{"\t// foo()", "    // bar()", "\t"}},
		{"nested tabs", []string{"\tfoo()", "\t\tbar()", "        baz()"}, []string{"\t// foo()", "\t// \tbar()", "    //     baz()"}},
		{"inside tab", []string{"  foo()", "\tbar()"}, []string{"//   foo()", "// \tbar()"}},
		{"inside offset tabs", []string{"\t   foo()", "\t  \tbar()", "\t \tbaz()", ""}, []string{"\t //   foo()", "\t //  \tbar()", "\t // \tbaz()", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newCommentEditor(t, "a.go", slices.Clone(tc.lines), len(tc.lines)-1, len([]rune(tc.lines[len(tc.lines)-1])))
			e.TabSize = 4
			e.Undo = &undo.UndoStack{}
			e.Selection = &selection.Selection{}
			e.Selection.Start(0, 0)
			e.ToggleLineComment()
			if !slices.Equal(e.Buf.Lines, tc.want) {
				t.Fatalf("lines = %q, want %q", e.Buf.Lines, tc.want)
			}
			e.Undo.Undo(e.Buf)
			if !slices.Equal(e.Buf.Lines, tc.lines) {
				t.Fatalf("undo = %q, want %q", e.Buf.Lines, tc.lines)
			}
			e.Undo.Redo(e.Buf)
			if !slices.Equal(e.Buf.Lines, tc.want) {
				t.Fatalf("redo = %q, want %q", e.Buf.Lines, tc.want)
			}
			e.ToggleLineComment()
			if !slices.Equal(e.Buf.Lines, tc.lines) {
				t.Fatalf("uncomment = %q, want %q", e.Buf.Lines, tc.lines)
			}
		})
	}
}

func TestToggleLineCommentLeavesCursorBeforeIndent(t *testing.T) {
	e := newCommentEditor(t, "a.go", []string{"    foo()"}, 0, 1)
	e.ToggleLineComment()
	if e.Cursor.Col != 1 {
		t.Fatalf("cursor moved to %d", e.Cursor.Col)
	}
	e.ToggleLineComment()
	if e.Cursor.Col != 1 {
		t.Fatalf("cursor moved to %d", e.Cursor.Col)
	}
}

func TestToggleLineCommentMapsSelectionsInsideXMLEntities(t *testing.T) {
	for _, entity := range []string{"&amp;", "&#45;"} {
		for offset := 1; offset < len(entity); offset++ {
			for _, backward := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/backward=%v", entity, offset, backward), func(t *testing.T) {
					line := "<!-- " + entity + " -->"
					anchor, cur := 5+offset, len(line)
					if backward {
						anchor, cur = cur, anchor
					}
					e := newCommentEditor(t, "a.xml", []string{line}, 0, cur)
					e.Selection = &selection.Selection{}
					e.Selection.Start(0, anchor)
					e.ToggleLineComment()
					start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
					if start.Col != 0 || end.Col != 1 {
						t.Fatalf("decoded selection = %v..%v, want 0..1", start, end)
					}
					if got := e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col); got != unescapeXMLCommentText(entity) {
						t.Fatalf("selected text = %q", got)
					}
				})
			}
		}
	}
}
