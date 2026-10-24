package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// surfaceHasText checks whether any row in the surface contains the given substring.
func surfaceHasText(s *testSurface, substr string) bool {
	for row := range s.cells {
		text := surfaceRowText(s, row)
		for i := 0; i <= len(text)-len(substr); i++ {
			if text[i:i+len(substr)] == substr {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Construction & Setup
// ---------------------------------------------------------------------------

func TestDialogNewWithWidth(t *testing.T) {
	d := NewDialogWidget(50)
	if d.BoxWidth != 50 {
		t.Fatalf("expected BoxWidth=50, got %d", d.BoxWidth)
	}
	if d.Borders.TopLeft != '╔' {
		t.Fatal("expected DoubleBorderSet by default")
	}
}

func TestDialogNewWithNegativeWidth(t *testing.T) {
	d := NewDialogWidget(-5)
	if d.BoxWidth != 40 {
		t.Fatalf("expected default BoxWidth=40 for negative input, got %d", d.BoxWidth)
	}
}

func TestDialogSetTitle(t *testing.T) {
	d := NewDialogWidget(40)
	d.Title = "My Title"
	if d.Title != "My Title" {
		t.Fatalf("expected title 'My Title', got '%s'", d.Title)
	}
}

func TestDialogSetContentWidget(t *testing.T) {
	d := NewDialogWidget(40)
	content := NewLabelWidget(LabelConfig{Text: "Hello"})
	d.SetContent(content)
	if d.Content != content {
		t.Fatal("SetContent did not set the content widget")
	}
}

func TestDialogWidgetChildrenBoth(t *testing.T) {
	d := NewDialogWidget(40)
	content := NewLabelWidget(LabelConfig{Text: "content"})
	d.SetContent(content)
	d.Buttons = []DialogButton{{Label: "OK"}}
	d.Build()

	children := d.WidgetChildren()
	if len(children) != 2 {
		t.Fatalf("expected 2 children (content + footer), got %d", len(children))
	}
	if children[0] != content {
		t.Fatal("first child should be the content widget")
	}
	if children[1] != d.footer {
		t.Fatal("second child should be the footer")
	}
}

func TestDialogWidgetChildrenFooterOnly(t *testing.T) {
	d := NewDialogWidget(40)
	d.Buttons = []DialogButton{{Label: "OK"}}
	d.Build()

	children := d.WidgetChildren()
	if len(children) != 1 {
		t.Fatalf("expected 1 child (footer only), got %d", len(children))
	}
}

func TestDialogWidgetChildrenEmpty(t *testing.T) {
	d := NewDialogWidget(40)
	d.Build()

	children := d.WidgetChildren()
	if len(children) != 0 {
		t.Fatalf("expected 0 children, got %d", len(children))
	}
}

func TestDialogWidthReturnsBoxWidth(t *testing.T) {
	d := NewDialogWidget(60)
	if d.Width() != 60 {
		t.Fatalf("Width() should return BoxWidth, got %d", d.Width())
	}
}

func TestDialogHeightReturnsZero(t *testing.T) {
	d := NewDialogWidget(40)
	if d.Height() != 0 {
		t.Fatalf("Height() should return 0, got %d", d.Height())
	}
}

func TestDialogBuildFooterPadding(t *testing.T) {
	d := NewDialogWidget(40)
	d.Buttons = []DialogButton{{Label: "OK"}}
	d.Build()

	if d.footer.Box.PaddingLeft != 1 {
		t.Errorf("expected footer PaddingLeft=1, got %d", d.footer.Box.PaddingLeft)
	}
	if d.footer.Box.PaddingRight != 1 {
		t.Errorf("expected footer PaddingRight=1, got %d", d.footer.Box.PaddingRight)
	}
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

func TestDialogRenderTitle(t *testing.T) {
	d := NewDialogWidget(30)
	d.Title = "Confirm"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "Are you sure?"}))
	d.Buttons = []DialogButton{{Label: "OK"}}
	d.Build()

	s := renderWidget(d, 0, 0, 80, 24)

	if !surfaceHasText(s, "Confirm") {
		t.Error("dialog should render the title text")
	}
}

func TestDialogRenderBorderCorners(t *testing.T) {
	d := NewDialogWidget(30)
	d.Title = "Test"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "Body"}))
	d.Build()

	s := renderWidget(d, 0, 0, 80, 24)

	bs := term.DoubleBorderSet()

	// Verify all four corners
	if s.cells[d.boxY][d.boxX].Ch != bs.TopLeft {
		t.Errorf("expected top-left '╔', got '%c'", s.cells[d.boxY][d.boxX].Ch)
	}
	if s.cells[d.boxY][d.boxX+d.boxW-1].Ch != bs.TopRight {
		t.Errorf("expected top-right '╗', got '%c'", s.cells[d.boxY][d.boxX+d.boxW-1].Ch)
	}
	if s.cells[d.boxY+d.boxH-1][d.boxX].Ch != bs.BottomLeft {
		t.Errorf("expected bottom-left '╚', got '%c'", s.cells[d.boxY+d.boxH-1][d.boxX].Ch)
	}
	if s.cells[d.boxY+d.boxH-1][d.boxX+d.boxW-1].Ch != bs.BottomRight {
		t.Errorf("expected bottom-right '╝', got '%c'", s.cells[d.boxY+d.boxH-1][d.boxX+d.boxW-1].Ch)
	}
}

func TestDialogRenderHorizontalBorders(t *testing.T) {
	d := NewDialogWidget(20)
	d.Title = "T"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "B"}))
	d.Build()

	s := renderWidget(d, 0, 0, 80, 24)

	bs := term.DoubleBorderSet()

	// Check horizontal border chars along top edge (between corners)
	topY := d.boxY
	for x := d.boxX + 1; x < d.boxX+d.boxW-1; x++ {
		if s.cells[topY][x].Ch != bs.Horizontal {
			t.Errorf("expected horizontal border '═' at (%d,%d), got '%c'", x, topY, s.cells[topY][x].Ch)
			break
		}
	}
}

func TestDialogRenderVerticalBorders(t *testing.T) {
	d := NewDialogWidget(20)
	d.Title = "T"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "B"}))
	d.Build()

	s := renderWidget(d, 0, 0, 80, 24)

	bs := term.DoubleBorderSet()

	// Check vertical border chars along left edge (between corners)
	leftX := d.boxX
	for y := d.boxY + 1; y < d.boxY+d.boxH-1; y++ {
		if s.cells[y][leftX].Ch != bs.Vertical {
			t.Errorf("expected vertical border '║' at (%d,%d), got '%c'", leftX, y, s.cells[y][leftX].Ch)
			break
		}
	}
}

func TestDialogRenderContentWidget(t *testing.T) {
	d := NewDialogWidget(40)
	d.Title = "Info"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "Hello World"}))
	d.Build()

	s := renderWidget(d, 0, 0, 80, 24)

	if !surfaceHasText(s, "Hello World") {
		t.Error("dialog should render the content widget text")
	}
}

func TestDialogRenderFooterButtons(t *testing.T) {
	d := NewDialogWidget(40)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Buttons = []DialogButton{
		{Label: "Save"},
		{Label: "Cancel"},
	}
	d.Build()

	s := renderWidget(d, 0, 0, 80, 24)

	if !surfaceHasText(s, "Save") {
		t.Error("dialog should render 'Save' button label")
	}
	if !surfaceHasText(s, "Cancel") {
		t.Error("dialog should render 'Cancel' button label")
	}
}

func TestDialogRenderNoTitle(t *testing.T) {
	d := NewDialogWidget(30)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "Content"}))
	d.Build()

	s := renderWidget(d, 0, 0, 80, 24)

	if !surfaceHasText(s, "Content") {
		t.Error("dialog without title should still render content")
	}
}

func TestDialogRenderSmallSurface(t *testing.T) {
	d := NewDialogWidget(30)
	d.Title = "Test"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Build()

	// Surface too small (4x4) should bail out without crashing
	s := renderWidget(d, 0, 0, 4, 4)
	if surfaceHasText(s, "Test") {
		t.Error("dialog should not render on surface <= 4x4")
	}
}

func TestDialogRenderBoxWidthClamped(t *testing.T) {
	d := NewDialogWidget(100)
	d.Title = "Wide"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Build()

	renderWidget(d, 0, 0, 30, 20)

	// Should clamp boxW to sw-4 = 26
	if d.boxW > 26 {
		t.Errorf("boxW should be clamped to %d, got %d", 26, d.boxW)
	}
}

func TestDialogTitleRendersWithBoldAndStyle(t *testing.T) {
	d := NewDialogWidget(30)
	d.Title = "Styled"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Build()

	s := renderWidget(d, 0, 0, 80, 24)

	titleY := d.boxY + 1
	titleStartX := d.boxX + 2 // innerX + 1
	c := s.cells[titleY][titleStartX]
	if c.Ch != 'S' {
		t.Errorf("expected first title char 'S' at (%d,%d), got '%c'", titleStartX, titleY, c.Ch)
	}
	if c.Style != term.StylePaletteItem {
		t.Errorf("title should use StylePaletteItem, got %v", c.Style)
	}
	if !c.Bold {
		t.Error("title should be rendered with Bold=true")
	}
}

func TestDialogRenderFullDialog(t *testing.T) {
	d := NewDialogWidget(40)
	d.Title = "Save Changes"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "You have unsaved changes."}))
	d.Buttons = []DialogButton{
		{Label: "Save"},
		{Label: "Don't Save"},
		{Label: "Cancel"},
	}
	d.Build()

	s := renderWidget(d, 0, 0, 80, 24)

	if !surfaceHasText(s, "Save Changes") {
		t.Error("dialog should render title")
	}
	if !surfaceHasText(s, "You have unsaved changes.") {
		t.Error("dialog should render content")
	}
	if !surfaceHasText(s, "Cancel") {
		t.Error("dialog should render Cancel button")
	}

	// Verify border corners
	bs := term.DoubleBorderSet()
	if s.cells[d.boxY][d.boxX].Ch != bs.TopLeft {
		t.Errorf("expected top-left border '╔', got '%c'", s.cells[d.boxY][d.boxX].Ch)
	}
	if s.cells[d.boxY+d.boxH-1][d.boxX+d.boxW-1].Ch != bs.BottomRight {
		t.Errorf("expected bottom-right border '╝', got '%c'", s.cells[d.boxY+d.boxH-1][d.boxX+d.boxW-1].Ch)
	}
}

// ---------------------------------------------------------------------------
// Keyboard Navigation
// ---------------------------------------------------------------------------

func TestDialogEscapeWithoutOnDismiss(t *testing.T) {
	d := NewDialogWidget(40)
	d.Build()
	// No OnDismiss set — should not panic

	result := d.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone))
	if result != EventConsumed {
		t.Error("Escape should return EventConsumed even without OnDismiss")
	}
}

func TestDialogEnterActivatesFocusedButton(t *testing.T) {
	clicked := ""
	d := NewDialogWidget(40)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Buttons = []DialogButton{
		{Label: "OK", Handler: func() { clicked = "ok" }},
		{Label: "Cancel", Handler: func() { clicked = "cancel" }},
	}
	d.Build()

	// Focus the first button
	d.footer.Children[0].(FocusableWidget).SetFocused(true)

	d.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if clicked != "ok" {
		t.Fatalf("Enter on focused 'OK' button should trigger its handler, got '%s'", clicked)
	}
}

func TestDialogLeftRightNavigateButtons(t *testing.T) {
	d := NewDialogWidget(40)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Buttons = []DialogButton{
		{Label: "A"},
		{Label: "B"},
		{Label: "C"},
	}
	d.Build()

	btn0 := d.footer.Children[0].(FocusableWidget)
	btn1 := d.footer.Children[1].(FocusableWidget)
	btn2 := d.footer.Children[2].(FocusableWidget)

	// Start with first button focused
	btn0.SetFocused(true)

	// Right: A -> B
	d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if btn0.IsFocused() {
		t.Error("A should not be focused after Right")
	}
	if !btn1.IsFocused() {
		t.Error("B should be focused after Right")
	}

	// Right: B -> C
	d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if !btn2.IsFocused() {
		t.Error("C should be focused after second Right")
	}
	if btn1.IsFocused() {
		t.Error("B should not be focused after second Right")
	}

	// Right on last: no wrap, C stays
	d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if !btn2.IsFocused() {
		t.Error("C should remain focused on Right at end (no wrap)")
	}

	// Left: C -> B
	d.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	if !btn1.IsFocused() {
		t.Error("B should be focused after Left from C")
	}
	if btn2.IsFocused() {
		t.Error("C should not be focused after Left")
	}

	// Left: B -> A
	d.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	if !btn0.IsFocused() {
		t.Error("A should be focused after Left from B")
	}

	// Left on first: no wrap, A stays
	d.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	if !btn0.IsFocused() {
		t.Error("A should remain focused on Left at start (no wrap)")
	}
}

func TestDialogEnterOnSecondButton(t *testing.T) {
	clicked := ""
	d := NewDialogWidget(40)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Buttons = []DialogButton{
		{Label: "Save", Handler: func() { clicked = "save" }},
		{Label: "Cancel", Handler: func() { clicked = "cancel" }},
	}
	d.Build()

	d.footer.Children[0].(FocusableWidget).SetFocused(true)
	// Move right to Cancel
	d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))

	d.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if clicked != "cancel" {
		t.Fatalf("Enter should activate focused Cancel button, got '%s'", clicked)
	}
}

func TestDialogSpaceActivatesButton(t *testing.T) {
	clicked := false
	d := NewDialogWidget(40)
	d.Buttons = []DialogButton{
		{Label: "Go", Handler: func() { clicked = true }},
	}
	d.Build()

	d.footer.Children[0].(FocusableWidget).SetFocused(true)

	d.HandleEvent(tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone))
	if !clicked {
		t.Error("Space on focused button should activate it")
	}
}

func TestDialogArrowKeyConsumedResult(t *testing.T) {
	d := NewDialogWidget(40)
	d.Buttons = []DialogButton{{Label: "A"}, {Label: "B"}}
	d.Build()
	d.footer.Children[0].(FocusableWidget).SetFocused(true)

	result := d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if result != EventConsumed {
		t.Error("Right arrow should return EventConsumed")
	}

	result = d.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	if result != EventConsumed {
		t.Error("Left arrow should return EventConsumed")
	}
}

// ---------------------------------------------------------------------------
// Edge Cases: button counts
// ---------------------------------------------------------------------------

func TestDialogSingleButtonNavigation(t *testing.T) {
	clicked := false
	d := NewDialogWidget(40)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "Confirm?"}))
	d.Buttons = []DialogButton{
		{Label: "OK", Handler: func() { clicked = true }},
	}
	d.Build()

	if len(d.footer.Children) != 1 {
		t.Fatalf("expected 1 footer child, got %d", len(d.footer.Children))
	}

	btn := d.footer.Children[0].(FocusableWidget)
	btn.SetFocused(true)

	// Left/Right should be no-ops on single button
	d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if !btn.IsFocused() {
		t.Error("single button should remain focused after Right")
	}
	d.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	if !btn.IsFocused() {
		t.Error("single button should remain focused after Left")
	}

	// Enter triggers the handler
	d.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !clicked {
		t.Error("Enter on single focused button should trigger handler")
	}
}

func TestDialogThreeButtonsNavigateToLast(t *testing.T) {
	clicked := ""
	d := NewDialogWidget(50)
	d.Buttons = []DialogButton{
		{Label: "Yes", Handler: func() { clicked = "yes" }},
		{Label: "No", Handler: func() { clicked = "no" }},
		{Label: "Cancel", Handler: func() { clicked = "cancel" }},
	}
	d.Build()

	if len(d.footer.Children) != 3 {
		t.Fatalf("expected 3 footer children, got %d", len(d.footer.Children))
	}

	// Focus first button, navigate to third
	d.footer.Children[0].(FocusableWidget).SetFocused(true)
	d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))

	if !d.footer.Children[2].(FocusableWidget).IsFocused() {
		t.Error("third button should be focused after two Rights")
	}

	d.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if clicked != "cancel" {
		t.Fatalf("expected 'cancel', got '%s'", clicked)
	}
}

func TestDialogNoButtonsNoFooter(t *testing.T) {
	d := NewDialogWidget(40)
	d.Title = "Info"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "Read only info"}))
	d.Build()

	if d.footer != nil {
		t.Fatal("dialog with no buttons should have no footer")
	}

	// Render should still work
	s := renderWidget(d, 0, 0, 80, 24)
	if !surfaceHasText(s, "Read only info") {
		t.Error("dialog with no buttons should still render content")
	}

	// Arrow keys should not panic (footer is nil, Left/Right skip)
	result := d.HandleEvent(tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone))
	if result != EventConsumed {
		t.Error("dialog should still consume key events even without buttons")
	}

	result = d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if result != EventConsumed {
		t.Error("dialog should still consume Right key even without buttons")
	}
}

// ---------------------------------------------------------------------------
// Mouse Events
// ---------------------------------------------------------------------------

func TestDialogMouseClickOnFooterButton(t *testing.T) {
	clicked := false
	d := NewDialogWidget(40)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Buttons = []DialogButton{
		{Label: "Click Me", Handler: func() { clicked = true }},
	}
	d.Build()

	renderWidget(d, 0, 0, 80, 24)

	// Get the button rect to click inside it
	btn := d.footer.Children[0].(*ButtonWidget)
	r := btn.GetRect()

	if r.W == 0 || r.H == 0 {
		t.Fatal("button rect should be set after render")
	}

	// An immediate mouse-up after the press triggers the button handler.
	d.HandleEvent(mousePress(r.X+1, r.Y))
	if clicked {
		t.Fatal("mouse-down triggered the button handler")
	}
	d.HandleEvent(mouseRelease(r.X+1, r.Y))
	if !clicked {
		t.Error("mouse-up should trigger the button handler")
	}
}

func TestDialogMouseEventDoesNotPanic(t *testing.T) {
	d := NewDialogWidget(40)
	d.Build()
	// No footer, no content — mouse events should not panic
	click := mouseClick(10, 10)
	d.HandleEvent(click)
}

// ---------------------------------------------------------------------------
// Content Height
// ---------------------------------------------------------------------------

func TestDialogContentHeightForWidthUsed(t *testing.T) {
	d := NewDialogWidget(40)
	d.Title = "Wrap"
	d.SetContent(NewParagraphWidget("This is a paragraph that should wrap."))
	d.Buttons = []DialogButton{{Label: "OK"}}
	d.Build()

	renderWidget(d, 0, 0, 80, 24)

	if d.boxW <= 0 || d.boxH <= 0 {
		t.Errorf("dialog should have positive dimensions, got W=%d H=%d", d.boxW, d.boxH)
	}
}

func TestDialogContentHeightFixedHeightWidget(t *testing.T) {
	d := NewDialogWidget(40)
	// Label has Height() = 1
	d.SetContent(NewLabelWidget(LabelConfig{Text: "Fixed"}))

	h := d.contentHeight(36)
	if h != 1 {
		t.Fatalf("contentHeight for a label should be 1, got %d", h)
	}
}

func TestDialogContentHeightNilContent(t *testing.T) {
	d := NewDialogWidget(40)

	h := d.contentHeight(36)
	if h != 1 {
		t.Fatalf("contentHeight with nil content should default to 1, got %d", h)
	}
}

// ---------------------------------------------------------------------------
// Box Dimensions
// ---------------------------------------------------------------------------

func TestDialogBoxDimensionsWithTitleAndButtons(t *testing.T) {
	d := NewDialogWidget(30)
	d.Title = "My Dialog"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "content"}))
	d.Buttons = []DialogButton{{Label: "OK"}}
	d.Build()

	renderWidget(d, 0, 0, 80, 24)

	// boxH = 2 (border top+bottom) + 1 (title) + 1 (gap title-content) + 1 (content) + 1 (gap content-btn) + 1 (button) = 7
	if d.boxH != 7 {
		t.Errorf("expected boxH=7 with title+content+buttons, got %d", d.boxH)
	}

	// boxY is always 2
	if d.boxY != 2 {
		t.Errorf("expected boxY=2, got %d", d.boxY)
	}

	// boxW should be 30 (fits in 80-wide surface)
	if d.boxW != 30 {
		t.Errorf("expected boxW=30, got %d", d.boxW)
	}

	// boxX should be centered: (80-30)/2 = 25
	if d.boxX != 25 {
		t.Errorf("expected boxX=25, got %d", d.boxX)
	}
}

func TestDialogBoxDimensionsNoTitle(t *testing.T) {
	d := NewDialogWidget(30)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "content"}))
	d.Buttons = []DialogButton{{Label: "OK"}}
	d.Build()

	renderWidget(d, 0, 0, 80, 24)

	// boxH = 2 (border) + 0 (no title) + 1 (content) + 1 (gap content-btn) + 1 (button) = 5
	if d.boxH != 5 {
		t.Errorf("expected boxH=5 without title, got %d", d.boxH)
	}
}

func TestDialogBoxDimensionsNoButtons(t *testing.T) {
	d := NewDialogWidget(30)
	d.Title = "Info"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "content"}))
	d.Build()

	renderWidget(d, 0, 0, 80, 24)

	// boxH = 2 (border) + 1 (title) + 1 (gap title-content) + 1 (content) = 5
	if d.boxH != 5 {
		t.Errorf("expected boxH=5 with title but no buttons, got %d", d.boxH)
	}
}

func TestDialogBoxHeightClampedToSurface(t *testing.T) {
	d := NewDialogWidget(30)
	d.Title = "Big Dialog"
	d.SetContent(NewParagraphWidget("Line one. Line two. Line three. Line four. Line five. This is a very long paragraph that should wrap to many lines in a narrow dialog."))
	d.Buttons = []DialogButton{{Label: "OK"}}
	d.Build()

	renderWidget(d, 0, 0, 40, 8)

	// boxH should be clamped: boxY=2, sh=8, so max boxH = 8-2 = 6
	if d.boxH > 6 {
		t.Errorf("boxH should be clamped to sh-boxY=6, got %d", d.boxH)
	}
}

func TestDialogBoxCenteredHorizontally(t *testing.T) {
	d := NewDialogWidget(20)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Build()

	renderWidget(d, 0, 0, 60, 20)

	expected := (60 - 20) / 2
	if d.boxX != expected {
		t.Errorf("expected boxX=%d (centered), got %d", expected, d.boxX)
	}
}

// ---------------------------------------------------------------------------
// Multiple handlers
// ---------------------------------------------------------------------------

func TestDialogMultipleButtonHandlers(t *testing.T) {
	calls := []string{}
	d := NewDialogWidget(40)
	d.Buttons = []DialogButton{
		{Label: "A", Handler: func() { calls = append(calls, "a") }},
		{Label: "B", Handler: func() { calls = append(calls, "b") }},
	}
	d.Build()

	// Activate A
	d.footer.Children[0].(FocusableWidget).SetFocused(true)
	d.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	// Move to B and activate
	d.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	d.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	if len(calls) != 2 || calls[0] != "a" || calls[1] != "b" {
		t.Fatalf("expected [a, b], got %v", calls)
	}
}

// ---------------------------------------------------------------------------
// Button with nil handler
// ---------------------------------------------------------------------------

func TestDialogButtonNilHandlerNoPanic(t *testing.T) {
	d := NewDialogWidget(40)
	d.Buttons = []DialogButton{
		{Label: "No Handler"}, // Handler is nil
	}
	d.Build()

	d.footer.Children[0].(FocusableWidget).SetFocused(true)

	// Should not panic
	d.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
}

// ---------------------------------------------------------------------------
// All events consumed (modal)
// ---------------------------------------------------------------------------

func TestDialogConsumesAllKeyEvents(t *testing.T) {
	d := NewDialogWidget(40)
	d.SetContent(NewLabelWidget(LabelConfig{Text: "body"}))
	d.Buttons = []DialogButton{{Label: "OK"}}
	d.Build()

	keys := []tcell.Key{
		tcell.KeyUp, tcell.KeyDown, tcell.KeyBackspace, tcell.KeyDelete,
	}
	for _, k := range keys {
		result := d.HandleEvent(tcell.NewEventKey(k, "", tcell.ModNone))
		if result != EventConsumed {
			t.Errorf("dialog should consume all key events (modal), but key %v was not consumed", k)
		}
	}
}

// ---------------------------------------------------------------------------
// Gap calculation
// ---------------------------------------------------------------------------

func TestDialogGapsBetweenSections(t *testing.T) {
	// Title + Content + Buttons: 2 gaps
	d := NewDialogWidget(30)
	d.Title = "T"
	d.SetContent(NewLabelWidget(LabelConfig{Text: "C"}))
	d.Buttons = []DialogButton{{Label: "B"}}
	d.Build()

	renderWidget(d, 0, 0, 80, 24)

	// boxH = 2 (border) + 1 (title) + 1 (gap) + 1 (content) + 1 (gap) + 1 (button) = 7
	if d.boxH != 7 {
		t.Errorf("expected boxH=7 with all sections, got %d", d.boxH)
	}

	// Title + Content, no buttons: 1 gap
	d2 := NewDialogWidget(30)
	d2.Title = "T"
	d2.SetContent(NewLabelWidget(LabelConfig{Text: "C"}))
	d2.Build()

	renderWidget(d2, 0, 0, 80, 24)

	// boxH = 2 (border) + 1 (title) + 1 (gap) + 1 (content) = 5
	if d2.boxH != 5 {
		t.Errorf("expected boxH=5 with title+content only, got %d", d2.boxH)
	}

	// Content + Buttons, no title: 1 gap
	d3 := NewDialogWidget(30)
	d3.SetContent(NewLabelWidget(LabelConfig{Text: "C"}))
	d3.Buttons = []DialogButton{{Label: "B"}}
	d3.Build()

	renderWidget(d3, 0, 0, 80, 24)

	// boxH = 2 (border) + 1 (content) + 1 (gap) + 1 (button) = 5
	if d3.boxH != 5 {
		t.Errorf("expected boxH=5 with content+buttons only, got %d", d3.boxH)
	}
}

// ---------------------------------------------------------------------------
// moveFocusButton edge: forward/backward from edges
// ---------------------------------------------------------------------------

func TestDialogMoveFocusButtonForwardFromLast(t *testing.T) {
	d := NewDialogWidget(40)
	d.Buttons = []DialogButton{{Label: "A"}, {Label: "B"}}
	d.Build()

	btn0 := d.footer.Children[0].(FocusableWidget)
	btn1 := d.footer.Children[1].(FocusableWidget)

	btn1.SetFocused(true) // start at last

	d.moveFocusButton(true) // try forward
	// Should be a no-op
	if !btn1.IsFocused() {
		t.Error("B should remain focused (no wrap forward)")
	}
	if btn0.IsFocused() {
		t.Error("A should not gain focus on forward from last")
	}
}

func TestDialogMoveFocusButtonBackwardFromFirst(t *testing.T) {
	d := NewDialogWidget(40)
	d.Buttons = []DialogButton{{Label: "A"}, {Label: "B"}}
	d.Build()

	btn0 := d.footer.Children[0].(FocusableWidget)
	btn1 := d.footer.Children[1].(FocusableWidget)

	btn0.SetFocused(true) // start at first

	d.moveFocusButton(false) // try backward
	// Should be a no-op
	if !btn0.IsFocused() {
		t.Error("A should remain focused (no wrap backward)")
	}
	if btn1.IsFocused() {
		t.Error("B should not gain focus on backward from first")
	}
}
