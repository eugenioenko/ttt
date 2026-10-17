package logfile

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSearchMatchesRegexpAcrossEntireFile(t *testing.T) {
	lines := []string{"ababa", "", "a a b", "界a界 b", "bad\xff\xfe a", "last"}
	lf := openIndexed(t, writeFile(t, strings.Join(lines, "\r\n")))
	for _, pattern := range []string{"a", "a*", "^", "$", "^|$", "^a$", "^ababa$", "^last$", ".*", "a+|b", `\b[a-z]+\b`, "(?i)A", `(a)(b)?`, "界", `[ab]*?`, "(?m)^a", "a.*b|b"} {
		t.Run(pattern, func(t *testing.T) {
			re := regexp.MustCompile(pattern)
			got, err := lf.Search(context.Background(), re, nil)
			if err != nil {
				t.Fatal(err)
			}
			var want []Match
			off := int64(0)
			for _, line := range lines {
				for _, m := range re.FindAllStringIndex(line, -1) {
					want = append(want, Match{Offset: off, Col: utf8.RuneCountInString(line[:m[0]]), Len: utf8.RuneCountInString(line[m[0]:m[1]])})
				}
				off += int64(len(line) + 2)
			}
			if len(got.Matches) != len(want) {
				t.Fatalf("got %v; want %v", got.Matches, want)
			}
			for i, m := range want {
				if got.Matches[i] != m {
					t.Fatalf("match %d = %+v, want %+v", i, got.Matches[i], m)
				}
			}
		})
	}
}
func TestSearchBeyondDisplayLimitAndCancellation(t *testing.T) {
	text := strings.Repeat("x", MaxLineBytes+100) + "needle\nend\n"
	lf := openIndexed(t, writeFile(t, text))
	result, err := lf.Search(context.Background(), regexp.MustCompile("needle|end"), nil)
	if err != nil || len(result.Matches) != 2 || result.Matches[0].Col != MaxLineBytes+100 || result.Matches[1].Offset != int64(len(text)-4) {
		t.Fatalf("search = %+v, %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lf.Search(ctx, regexp.MustCompile("absent"), nil); err != context.Canceled {
		t.Fatalf("canceled search = %v", err)
	}
}
func TestSearchCapsMatches(t *testing.T) {
	lf := openIndexed(t, writeFile(t, strings.Repeat("a\n", MaxMatches+10)))
	result, err := lf.Search(context.Background(), regexp.MustCompile("a"), nil)
	if err != nil || !result.Limited || len(result.Matches) != MaxMatches {
		t.Fatalf("search = %d matches, limited=%v, err=%v", len(result.Matches), result.Limited, err)
	}
}
func TestCopyAcrossLinesAndCap(t *testing.T) {
	lf := openIndexed(t, writeFile(t, "界abc\r\ndef\nlast"))
	text, limited, err := lf.Copy(Position{Offset: 0, Col: 1}, Position{Offset: 12, Col: 2})
	if err != nil || limited || text != "abc\ndef\nla" {
		t.Fatalf("copy = %q, %v, %v", text, limited, err)
	}
	big := openIndexed(t, writeFile(t, strings.Repeat("界"+strings.Repeat("x", 97)+"\n", 50000)))
	text, limited, err = big.Copy(Position{}, Position{Offset: big.Progress().Size})
	if err != nil || !limited || len(text) > MaxCopyBytes || !utf8.ValidString(text) {
		t.Fatalf("large copy = %d bytes, %v, %v", len(text), limited, err)
	}
}
func TestRefreshReplacesPartialLineWithoutAddingLine(t *testing.T) {
	path := writeFile(t, "partial")
	lf := openIndexed(t, path)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(" more")
	f.Close()
	if err := lf.Refresh(); err != nil {
		t.Fatal(err)
	}
	waitIndexed(t, lf)
	if got := lf.ReadLines(0, 2); len(got) != 1 || got[0] != "partial more" {
		t.Fatalf("append = %q", got)
	}
	if err := os.Truncate(path, 3); err != nil {
		t.Fatal(err)
	}
	if err := lf.Refresh(); err != nil {
		t.Fatal(err)
	}
	waitIndexed(t, lf)
	if !lf.TakeReplaced() || lf.ReadLines(0, 1)[0] != "par" {
		t.Fatal("shrinking an unfinished line did not reset the reader")
	}
}

func TestSearchFromRescansOnlyTheLastIncompleteLine(t *testing.T) {
	for _, tail := range []string{"", "nee", strings.Repeat("x", chunkSize*3) + "nee", "needle\r", "needle"} {
		t.Run(fmt.Sprintf("tail-%d", len(tail)), func(t *testing.T) {
			prefix := "old needle\n"
			path := writeFile(t, prefix+tail)
			lf := openIndexed(t, path)
			oldSize := lf.Progress().Size
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteString("dle\nnew needle\n"); err != nil {
				t.Fatal(err)
			}
			f.Close()
			if err := lf.Refresh(); err != nil {
				t.Fatal(err)
			}
			for _, pattern := range []string{"needle", "needle$", "^needle$"} {
				re := regexp.MustCompile(pattern)
				got, err := lf.SearchFrom(context.Background(), re, oldSize, func(progress SearchResult) {
					if progress.Start != int64(len(prefix)) || progress.Scanned < progress.Start {
						t.Fatalf("invalid incremental range: %+v", progress)
					}
				})
				if err != nil {
					t.Fatal(err)
				}
				full, err := lf.Search(context.Background(), re, nil)
				if err != nil {
					t.Fatal(err)
				}
				var want []Match
				for _, m := range full.Matches {
					if m.Offset >= int64(len(prefix)) {
						want = append(want, m)
					}
				}
				if got.Start != int64(len(prefix)) || !reflect.DeepEqual(got.Matches, want) {
					t.Fatalf("incremental %q = %+v, want %v", pattern, got, want)
				}
			}
		})
	}
}
