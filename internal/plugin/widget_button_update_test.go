package plugin

import (
	"strings"
	"testing"
)

func TestPluginButtonLabelUpdatesWithoutLosingFocus(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{})
	defer cleanup()
	button := createButtonWidget(WidgetDesc{Label: "&Approve"}, p)
	button.SetFocused(true)
	updateWidget(button, WidgetDesc{Kind: WidgetButton, Label: "&Approve & merge"}, p)
	if !button.IsFocused() || button.Config.Label != "&Approve & merge" {
		t.Fatal("button update lost focus or retained its old label")
	}
	surface := newMockSurface(25, 1)
	button.Render(surface)
	var text string
	for x := 0; x < 25; x++ {
		text += string(surface.cells[[2]int{x, 0}].Ch)
	}
	if !strings.Contains(text, "Approve & merge") {
		t.Fatalf("button text = %q", text)
	}
}
