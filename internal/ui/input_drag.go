package ui

import "github.com/gdamore/tcell/v3"

func handleInputDrag(ev tcell.Event, inputs ...*InputWidget) EventResult {
	if _, key := ev.(*tcell.EventKey); key {
		cancelInputDrags(inputs...)
	}
	mouse, ok := ev.(*tcell.EventMouse)
	if ok {
		for _, input := range inputs {
			if input != nil && input.OwnsPointerCapture() {
				return input.HandleMouse(mouse)
			}
		}
	}
	return EventIgnored
}
func cancelInputDrags(inputs ...*InputWidget) bool {
	canceled := false
	for _, input := range inputs {
		if input != nil {
			canceled = input.CancelPointerCapture() || canceled
		}
	}
	return canceled
}

func ownsInputDrag(inputs ...*InputWidget) bool {
	for _, input := range inputs {
		if input != nil && input.OwnsPointerCapture() {
			return true
		}
	}
	return false
}
