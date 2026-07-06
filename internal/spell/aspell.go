// Package spell provides spell checking by piping text through aspell's
// ispell-compatible pipe protocol.
package spell

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Misspelling is one misspelled word found by aspell.
type Misspelling struct {
	Line        int // 0-based buffer line
	Col         int // rune column of the word start
	Word        string
	Suggestions []string
}

const maxSuggestions = 10

var (
	lookOnce sync.Once
	binPath  string
)

// Available reports whether the aspell binary is present in PATH.
func Available() bool {
	lookOnce.Do(func() { binPath, _ = exec.LookPath("aspell") })
	return binPath != ""
}

// ModeForLanguage maps a highlighter language name to an aspell filter mode.
// ok is false for languages that should not be spell checked (code files).
// An empty language (file with no highlighter) is checked as plain text.
func ModeForLanguage(lang string) (mode string, ok bool) {
	switch lang {
	case "", "plaintext", "reStructuredText", "Org Mode":
		return "", true
	case "markdown":
		return "markdown", true
	case "HTML":
		return "html", true
	case "XML":
		return "sgml", true
	case "TeX":
		return "tex", true
	}
	return "", false
}

// Check pipes lines through aspell and returns the misspellings found.
// lang is an aspell dictionary code like "en_US"; empty follows locale settings.
func Check(ctx context.Context, lines []string, lang, mode string) ([]Misspelling, error) {
	args := []string{"pipe", "--encoding=utf-8"}
	if mode != "" {
		args = append(args, "--mode="+mode)
	}
	if lang != "" {
		args = append(args, "--lang="+lang)
	}
	cmd := exec.CommandContext(ctx, "aspell", args...)
	var in strings.Builder
	in.WriteString("!\n") // terse mode: no output for correct words
	for _, line := range lines {
		in.WriteString("^") // data-line prefix so aspell never parses line content as a command
		in.WriteString(line)
		in.WriteString("\n")
	}
	cmd.Stdin = strings.NewReader(in.String())
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			msg, _, _ := strings.Cut(strings.TrimSpace(string(exitErr.Stderr)), "\n")
			return nil, errors.New(msg)
		}
		return nil, err
	}
	return parsePipeOutput(out), nil
}

// parsePipeOutput parses ispell -a output: a version banner, then results per
// input line terminated by a blank line. Reported offsets are rune positions
// in the sent line, shifted by one for the ^ prefix.
func parsePipeOutput(out []byte) []Misspelling {
	var found []Misspelling
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	first := true
	for sc.Scan() {
		text := sc.Text()
		if first {
			first = false
			if strings.HasPrefix(text, "@(#)") {
				continue
			}
		}
		if text == "" {
			line++
			continue
		}
		if m, ok := parseResult(text); ok {
			m.Line = line
			found = append(found, m)
		}
	}
	return found
}

// parseResult parses "& word count offset: sug1, sug2" and "# word offset".
func parseResult(s string) (Misspelling, bool) {
	if len(s) < 3 || (s[0] != '&' && s[0] != '#') {
		return Misspelling{}, false
	}
	body := s[2:]
	var sugs []string
	if s[0] == '&' {
		head, rest, ok := strings.Cut(body, ": ")
		if !ok {
			return Misspelling{}, false
		}
		body = head
		sugs = strings.Split(rest, ", ")
		if len(sugs) > maxSuggestions {
			sugs = sugs[:maxSuggestions]
		}
	}
	fields := strings.Fields(body)
	wordEnd := len(fields) - 1 // "# word offset"
	if s[0] == '&' {
		wordEnd = len(fields) - 2 // "& word count offset"
	}
	if wordEnd < 1 {
		return Misspelling{}, false
	}
	offset, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil || offset < 1 {
		return Misspelling{}, false
	}
	return Misspelling{
		Col:         offset - 1,
		Word:        strings.Join(fields[:wordEnd], " "),
		Suggestions: sugs,
	}, true
}

// AddToDictionary adds word to the user's personal aspell dictionary.
func AddToDictionary(lang, word string) error {
	args := []string{"pipe", "--encoding=utf-8"}
	if lang != "" {
		args = append(args, "--lang="+lang)
	}
	cmd := exec.Command("aspell", args...)
	cmd.Stdin = strings.NewReader("*" + word + "\n#\n")
	return cmd.Run()
}
