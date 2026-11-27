package search

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestMessageReaderRecordLimit(t *testing.T) {
	for _, size := range []int{MaxRecordBytes - 1, MaxRecordBytes, MaxRecordBytes + 1} {
		for _, newline := range []bool{false, true} {
			t.Run(fmt.Sprintf("bytes=%d/newline=%t", size, newline), func(t *testing.T) {
				prefix, suffix := `{"type":"`, `"}`
				record := prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix
				if newline {
					record += "\n"
				}
				reader := NewMessageReader(strings.NewReader(record))
				msg, err := reader.Next()
				if size > MaxRecordBytes {
					if !errors.Is(err, ErrRecordTooLarge) {
						t.Fatalf("error = %v, want record limit error", err)
					}
					return
				}
				if err != nil || len(msg.Type) != size-len(prefix)-len(suffix) {
					t.Fatalf("type length = %d, error = %v", len(msg.Type), err)
				}
				if _, err := reader.Next(); err != io.EOF {
					t.Fatalf("end of stream error = %v, want EOF", err)
				}
			})
		}
	}
}
