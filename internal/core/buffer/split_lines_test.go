package buffer

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

func TestSplitLinesMatchesScanLines(t *testing.T) {
	for _, in := range []string{"", "a", "a\n", "a\nb", "a\r\nb\r\n", "\n", "\n\n", "a\r", "a\rb", "a\rb\n", "a\r\r\n", "x\n\ny\n", "\r\n"} {
		var want []string
		sc := bufio.NewScanner(strings.NewReader(in))
		for sc.Scan() {
			want = append(want, sc.Text())
		}
		got := SplitLines(in)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}
