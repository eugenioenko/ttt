package term

import "testing"

func TestMockScreen_SetAndGetCell(t *testing.T) {
	s := NewMockScreen(10, 5)
	c := Cell{Ch: 'A', Style: 1}
	s.SetCell(2, 3, c)
	got, ok := s.Cells[[2]int{2, 3}]
	if !ok || got.Ch != 'A' || got.Style != 1 {
		t.Errorf("expected cell at (2,3) to be {A,1}, got {%c,%d}", got.Ch, got.Style)
	}
}

func TestMockScreen_Clear(t *testing.T) {
	s := NewMockScreen(5, 5)
	s.SetCell(1, 1, Cell{Ch: 'X'})
	s.Clear()
	if len(s.Cells) != 0 {
		t.Error("expected all cells cleared")
	}
}

func TestMockScreen_Size(t *testing.T) {
	s := NewMockScreen(7, 8)
	w, h := s.Size()
	if w != 7 || h != 8 {
		t.Errorf("expected size 7x8, got %dx%d", w, h)
	}
}

func TestParseCursorStyle(t *testing.T) {
	tests := []struct {
		input string
		want  CursorStyle
	}{
		{"", CursorStyleBlinkingBar},
		{"bar", CursorStyleBlinkingBar},
		{"blinkingBar", CursorStyleBlinkingBar},
		{"steadyBar", CursorStyleSteadyBar},
		{"block", CursorStyleBlinkingBlock},
		{"blinkingBlock", CursorStyleBlinkingBlock},
		{"steadyBlock", CursorStyleSteadyBlock},
		{"underline", CursorStyleBlinkingUnderline},
		{"blinkingUnderline", CursorStyleBlinkingUnderline},
		{"steadyUnderline", CursorStyleSteadyUnderline},
		{"unknown", CursorStyleBlinkingBar},
		{"BLOCK", CursorStyleBlinkingBar}, // case sensitive, falls through to default
		{"invalid", CursorStyleBlinkingBar},
	}
	for _, tt := range tests {
		got := ParseCursorStyle(tt.input)
		if got != tt.want {
			t.Errorf("ParseCursorStyle(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCursorStyleConstants(t *testing.T) {
	// Verify the iota-based constants have distinct values
	styles := []CursorStyle{
		CursorStyleBlinkingBar,
		CursorStyleSteadyBar,
		CursorStyleBlinkingBlock,
		CursorStyleSteadyBlock,
		CursorStyleBlinkingUnderline,
		CursorStyleSteadyUnderline,
	}
	seen := make(map[CursorStyle]bool)
	for _, s := range styles {
		if seen[s] {
			t.Errorf("duplicate CursorStyle value: %d", s)
		}
		seen[s] = true
	}
}

func TestCellAttrFlags(t *testing.T) {
	// Verify CellAttr flags are distinct powers of 2
	flags := []CellAttr{CellAttrBold, CellAttrUnderline, CellAttrItalic, CellAttrReverse, CellAttrBlink}
	for i, f := range flags {
		if f == 0 {
			t.Errorf("flag %d should not be zero", i)
		}
		for j := i + 1; j < len(flags); j++ {
			if f&flags[j] != 0 {
				t.Errorf("flags %d and %d overlap: %d & %d = %d", i, j, f, flags[j], f&flags[j])
			}
		}
	}

	// Verify combining flags works correctly
	combined := CellAttrBold | CellAttrUnderline | CellAttrItalic
	if combined&CellAttrBold == 0 {
		t.Error("expected Bold flag to be set in combined")
	}
	if combined&CellAttrUnderline == 0 {
		t.Error("expected Underline flag to be set in combined")
	}
	if combined&CellAttrItalic == 0 {
		t.Error("expected Italic flag to be set in combined")
	}
	if combined&CellAttrReverse != 0 {
		t.Error("expected Reverse flag to NOT be set in combined")
	}
	if combined&CellAttrBlink != 0 {
		t.Error("expected Blink flag to NOT be set in combined")
	}
}

func TestDirectColorZeroValue(t *testing.T) {
	var dc DirectColor
	if dc.Set {
		t.Error("zero-value DirectColor should have Set=false")
	}
	if dc.R != 0 || dc.G != 0 || dc.B != 0 {
		t.Error("zero-value DirectColor should have R=G=B=0")
	}
}

func TestCellDirectMode(t *testing.T) {
	c := Cell{
		Ch:     'X',
		Direct: true,
		Fg:     DirectColor{R: 255, G: 128, B: 0, Set: true},
		Bg:     DirectColor{R: 0, G: 0, B: 0, Set: true},
		Attrs:  CellAttrBold | CellAttrItalic,
	}
	if !c.Direct {
		t.Error("expected Direct to be true")
	}
	if !c.Fg.Set {
		t.Error("expected Fg.Set to be true")
	}
	if c.Fg.R != 255 || c.Fg.G != 128 || c.Fg.B != 0 {
		t.Errorf("unexpected Fg: R=%d G=%d B=%d", c.Fg.R, c.Fg.G, c.Fg.B)
	}
	if c.Attrs&CellAttrBold == 0 {
		t.Error("expected Bold attr")
	}
	if c.Attrs&CellAttrItalic == 0 {
		t.Error("expected Italic attr")
	}
}

func TestBorderSets(t *testing.T) {
	single := SingleBorderSet()
	double := DoubleBorderSet()

	// Verify all fields are set (non-zero runes)
	singleFields := []rune{
		single.Horizontal, single.Vertical,
		single.TopLeft, single.TopRight,
		single.BottomLeft, single.BottomRight,
		single.TopTee, single.BottomTee,
		single.LeftTee, single.RightTee,
	}
	for i, r := range singleFields {
		if r == 0 {
			t.Errorf("SingleBorderSet field %d is zero", i)
		}
	}

	doubleFields := []rune{
		double.Horizontal, double.Vertical,
		double.TopLeft, double.TopRight,
		double.BottomLeft, double.BottomRight,
		double.TopTee, double.BottomTee,
		double.LeftTee, double.RightTee,
	}
	for i, r := range doubleFields {
		if r == 0 {
			t.Errorf("DoubleBorderSet field %d is zero", i)
		}
	}

	// Verify single and double use different characters
	if single.Horizontal == double.Horizontal {
		t.Error("expected single and double Horizontal to differ")
	}
	if single.Vertical == double.Vertical {
		t.Error("expected single and double Vertical to differ")
	}

	// Verify known single border characters
	if single.Horizontal != '─' {
		t.Errorf("expected single Horizontal '─', got %c", single.Horizontal)
	}
	if single.Vertical != '│' {
		t.Errorf("expected single Vertical '│', got %c", single.Vertical)
	}

	// Verify known double border characters
	if double.Horizontal != '═' {
		t.Errorf("expected double Horizontal '═', got %c", double.Horizontal)
	}
	if double.Vertical != '║' {
		t.Errorf("expected double Vertical '║', got %c", double.Vertical)
	}
}

func TestStyleConstants(t *testing.T) {
	// Verify StyleDefault is 0 (iota start)
	if StyleDefault != 0 {
		t.Errorf("expected StyleDefault to be 0, got %d", StyleDefault)
	}

	// Verify a selection of style constants are distinct and non-negative
	styles := map[string]Style{
		"StyleDefault":        StyleDefault,
		"StyleStatusBar":      StyleStatusBar,
		"StyleActiveTab":      StyleActiveTab,
		"StyleInactiveTab":    StyleInactiveTab,
		"StyleSelection":      StyleSelection,
		"StyleSearchMatch":    StyleSearchMatch,
		"StyleLineNumber":     StyleLineNumber,
		"StyleBorder":         StyleBorder,
		"StyleDiffAdded":      StyleDiffAdded,
		"StyleDiffDeleted":    StyleDiffDeleted,
		"StyleDiffModified":   StyleDiffModified,
		"StyleSyntaxComment":  StyleSyntaxComment,
		"StyleSyntaxString":   StyleSyntaxString,
		"StyleSyntaxKeyword":  StyleSyntaxKeyword,
		"StyleSuccess":        StyleSuccess,
		"StyleDanger":         StyleDanger,
		"StyleWarning":        StyleWarning,
		"StyleGutterAdded":    StyleGutterAdded,
		"StyleGutterModified": StyleGutterModified,
		"StyleGutterDeleted":  StyleGutterDeleted,
	}

	seen := make(map[Style]string)
	for name, val := range styles {
		if val < 0 {
			t.Errorf("%s has negative value %d", name, val)
		}
		if prev, ok := seen[val]; ok {
			t.Errorf("%s and %s have the same value %d", name, prev, val)
		}
		seen[val] = name
	}
}

func TestMockScreenImplementsScreen(t *testing.T) {
	// Compile-time check that MockScreen implements Screen
	var _ Screen = (*MockScreen)(nil)
}
