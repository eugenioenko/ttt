package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestButtonVariantStyles(t *testing.T) {
	for _, tc := range []struct {
		variant ButtonVariant
		want    term.Style
	}{
		{ButtonLink, term.StyleButton},
		{ButtonFilled, term.StyleButtonFilled},
		{ButtonBracket, term.StyleButton},
		{ButtonInverse, term.StyleButtonInverse},
		{ButtonMuted, term.StyleMuted},
	} {
		b := NewButtonWidget(ButtonConfig{Label: "Save", Variant: tc.variant})
		s := renderWidget(b, 0, 0, 12, 1)
		if got := s.cells[0][0].Style; got != tc.want {
			t.Errorf("variant %d: style %v, want %v", tc.variant, got, tc.want)
		}
		b.SetFocused(true)
		if s = renderWidget(b, 0, 0, 12, 1); s.cells[0][0].Style != term.StyleButtonFocused {
			t.Errorf("variant %d: focused style %v", tc.variant, s.cells[0][0].Style)
		}
		b.SetFocused(false)
		b.Disabled = true
		if s = renderWidget(b, 0, 0, 12, 1); s.cells[0][0].Style != term.StyleMuted {
			t.Errorf("variant %d: disabled style %v", tc.variant, s.cells[0][0].Style)
		}
	}
}

func TestBracketButtonWrapsItsLabel(t *testing.T) {
	b := NewButtonWidget(ButtonConfig{Label: "&Save", Variant: ButtonBracket})
	s := renderWidget(b, 0, 0, 12, 1)
	if got := rowText(s, 0); got != " [Save]" {
		t.Fatalf("row = %q", got)
	}
	if b.Width() != 8 {
		t.Fatalf("width = %d, want the brackets and padding counted", b.Width())
	}
	if !s.cells[0][2].Underline {
		t.Fatal("the accelerator underline should stay on S inside the brackets")
	}
}
