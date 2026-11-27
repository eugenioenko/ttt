package app

import (
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
)

// blobLines must split a blob exactly the way the buffer splits a file so an
// unchanged file diffs as identical rather than off by a trailing line. The
// lone-\r case guards against the two splitters drifting apart again (#893).
func TestBlobLinesSplitsLikeBuffer(t *testing.T) {
	for _, tc := range []struct {
		name         string
		content      string
		showTrailing bool
		want         []string
	}{
		{"lf trailing newline", "one\ntwo\n", false, []string{"one", "two"}},
		{"lf no trailing newline", "one\ntwo", false, []string{"one", "two"}},
		{"crlf", "one\r\ntwo\r\n", false, []string{"one", "two"}},
		{"crlf show trailing", "one\r\ntwo\r\n", true, []string{"one", "two", ""}},
		{"lf trailing blank", "a\n\n", true, []string{"a", "", ""}},
		{"crlf trailing blanks", "a\r\n\r\n\r\n", true, []string{"a", "", "", ""}},
		{"lf newline only", "\n", true, []string{"", ""}},
		{"crlf blank lines only", "\r\n\r\n", true, []string{"", "", ""}},
		{"hidden trailing blank", "a\n\n", false, []string{"a", ""}},
		{"show trailing without final newline", "a", true, []string{"a", ""}},
		{"empty show trailing", "", true, []string{""}},
		{"lone cr last line", "one\ntwo\r", false, []string{"one", "two"}},
		{"empty", "", false, []string{""}},
		{"bom", "\ufeffone\ntwo\n", false, []string{"one", "two"}},
		{"bom show trailing", "\ufeffone\r\ntwo\r\n", true, []string{"one", "two", ""}},
		{"bom only", "\ufeff", true, []string{""}},
		{"interior feff", "one\ufefftwo", false, []string{"one\ufefftwo"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := blobLines(tc.content, tc.showTrailing)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("blobLines(%q, %v) = %q, want %q", tc.content, tc.showTrailing, got, tc.want)
			}
			b := &buffer.Buffer{ShowTrailingNewline: tc.showTrailing}
			b.SetText(tc.content)
			if !reflect.DeepEqual(got, b.Lines) {
				t.Fatalf("blob lines %q differ from loaded buffer %q", got, b.Lines)
			}
		})
	}
}
