package app

import (
	"reflect"
	"testing"
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
		{"lone cr last line", "one\ntwo\r", false, []string{"one", "two"}},
		{"empty", "", false, []string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := blobLines(tc.content, tc.showTrailing)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("blobLines(%q, %v) = %q, want %q", tc.content, tc.showTrailing, got, tc.want)
			}
		})
	}
}
