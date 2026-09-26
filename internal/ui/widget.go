package ui

import "github.com/eugenioenko/ttt/internal/widgets"

type Rect = widgets.Rect

type Surface = widgets.Surface

type BoxModel = widgets.BoxModel

type EventResult = widgets.EventResult

type Widget = widgets.Widget

const (
	EventIgnored   = widgets.EventIgnored
	EventConsumed  = widgets.EventConsumed
	EventDismissed = widgets.EventDismissed
	EventCaptured  = widgets.EventCaptured

	DoubleClickMs = 500
)

type CursorProvider interface {
	CursorPosition() (x, y int, visible bool)
}

type dividerXProvider interface {
	DividerScreenX() int
}

type dividerYProvider interface {
	DividerScreenY() int
}

type borderJunctions struct {
	BottomX [2]int
	RightY  int
}

type borderJunctionProvider interface {
	BorderJunctions() borderJunctions
}

// RawKeyConsumer indicates a widget that wants all key events
// sent directly to it, bypassing global key bindings.
// Used by the terminal widget.
type RawKeyConsumer interface {
	WantsRawKeys() bool
}
