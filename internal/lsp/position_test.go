package lsp

import "testing"

func TestUTF16Positions(t *testing.T) {
	for _, text := range []string{"", "ASCII", "café", "中文", "😀", "😀😀", "é😀中𐐀z", "e\u0301\t😀"} {
		t.Run(text, func(t *testing.T) {
			runes := []rune(text)
			offset := 0
			for col := 0; col <= len(runes); col++ {
				if got := RuneToUTF16(text, col); got != offset {
					t.Fatalf("rune %d = UTF-16 %d, want %d", col, got, offset)
				}
				if got := UTF16ToRune(text, offset); got != col {
					t.Fatalf("UTF-16 %d = rune %d, want %d", offset, got, col)
				}
				if col < len(runes) {
					offset++
					if runes[col] > 0xffff {
						offset++
					}
				}
			}
			if UTF16ToRune(text, offset+100) != len(runes) || RuneToUTF16(text, len(runes)+100) != offset {
				t.Fatal("offset past line end was not clamped")
			}
			if RuneToUTF16(text, -1) != 0 || UTF16ToRune(text, -1) != 0 {
				t.Fatal("negative offset was not clamped")
			}
		})
	}
	if UTF16ToRune("😀z", 1) != 0 {
		t.Fatal("split surrogate must clamp to the preceding rune boundary")
	}
	lines := []string{"😀a", "中😀b"}
	r := Range{Start: Position{0, 2}, End: Position{1, 4}}
	want := Range{Start: Position{0, 1}, End: Position{1, 3}}
	if got := RangeToRunes(lines, r); got != want {
		t.Fatalf("range = %+v, want %+v", got, want)
	}
}
