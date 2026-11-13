package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestButtonSpansRenderText(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Spans: []ButtonSpan{
		{Text: "12", Style: term.StyleSuccess},
		{Text: " ✓", Style: term.StyleDanger},
	}})
	s := renderWidget(btn, 0, 0, 20, 1)
	row := []rune(surfaceRowText(s, 0))
	// BoxModel default padding shifts content to x=1.
	if string(row[1:5]) != "12 ✓" {
		t.Errorf("expected spans text at offset 1, got row: %q", string(row))
	}
}

func TestButtonSpansUseSpanStylesWhenEnabled(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Spans: []ButtonSpan{
		{Text: "ok", Style: term.StyleSuccess},
		{Text: "no", Style: term.StyleDanger},
	}})
	s := renderWidget(btn, 0, 0, 20, 1)
	if got := s.cells[0][1].Style; got != term.StyleSuccess {
		t.Errorf("expected first span style Success, got %v", got)
	}
	if got := s.cells[0][3].Style; got != term.StyleDanger {
		t.Errorf("expected second span style Danger, got %v", got)
	}
}

func TestButtonSpansDisabledKeepsMutedStyle(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Spans: []ButtonSpan{
		{Text: "ok", Style: term.StyleSuccess},
	}})
	btn.Disabled = true
	s := renderWidget(btn, 0, 0, 20, 1)
	if got := s.cells[0][1].Style; got != term.StyleMuted {
		t.Errorf("expected muted style when disabled, got %v", got)
	}
}

func TestButtonSpansFocusedKeepsFocusedStyle(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Spans: []ButtonSpan{
		{Text: "ok", Style: term.StyleSuccess},
	}})
	btn.SetFocused(true)
	s := renderWidget(btn, 0, 0, 20, 1)
	if got := s.cells[0][1].Style; got != term.StyleButtonFocused {
		t.Errorf("expected focused style when focused, got %v", got)
	}
}

func TestButtonSpansWidthAndClipping(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Spans: []ButtonSpan{
		{Text: "ab", Style: term.StyleSuccess},
		{Text: "cd", Style: term.StyleDanger},
	}})
	if got := btn.Width(); got != 2+2+2 { // text + left/right padding
		t.Errorf("expected width 6, got %d", got)
	}
	// A 4-column surface: padding 1 each side leaves 2 columns for text.
	s := renderWidget(btn, 0, 0, 4, 1)
	row := surfaceRowText(s, 0)
	if got := row[1:3]; got != "ab" {
		t.Errorf("expected clipped 'ab', got %q", row)
	}
}

func TestButtonSpansBracketVariant(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{
		Variant: ButtonBracket,
		Spans:   []ButtonSpan{{Text: "x", Style: term.StyleSuccess}},
	})
	s := renderWidget(btn, 0, 0, 20, 1)
	row := []rune(surfaceRowText(s, 0))
	if got := string(row[1:4]); got != "[x]" {
		t.Errorf("expected bracketed span, got %q", row)
	}
}

func TestButtonSetSpansDropsLabel(t *testing.T) {
	btn := NewButtonWidget(ButtonConfig{Label: "&Save"})
	btn.SetSpans([]ButtonSpan{{Text: "Save", Style: term.StyleSuccess}})
	if !btn.HasSpans() || btn.label != "" || btn.accelIndex != -1 {
		t.Fatalf("SetSpans should drop the label and accelerator, got label=%q accel=%d spans=%v", btn.label, btn.accelIndex, btn.spans)
	}
	btn.SetLabel("Save")
	if btn.HasSpans() {
		t.Fatal("SetLabel should drop the spans")
	}
}
