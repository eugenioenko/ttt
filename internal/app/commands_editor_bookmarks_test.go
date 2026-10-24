package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
)

func TestNextBookmarkLineWraps(t *testing.T) {
	bookmarks := map[int]ui.Bookmark{2: {}, 7: {}, 11: {}}
	tests := []struct {
		name      string
		current   int
		direction int
		want      int
	}{
		{"next", 2, 1, 7},
		{"next wraps", 11, 1, 2},
		{"previous", 11, -1, 7},
		{"previous wraps", 2, -1, 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextBookmarkLine(bookmarks, tt.current, tt.direction); got != tt.want {
				t.Fatalf("nextBookmarkLine() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNextBookmarkLineEmpty(t *testing.T) {
	if got := nextBookmarkLine(nil, 0, 1); got != -1 {
		t.Fatalf("nextBookmarkLine() = %d, want -1", got)
	}
}
