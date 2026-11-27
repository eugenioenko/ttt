package widgets

import "github.com/gdamore/tcell/v3"

func SetControlModifiers(w Widget, modifiers *tcell.ModMask) {
	switch w := w.(type) {
	case *InputWidget:
		w.ControlModifiers = modifiers
	case *MarkdownWidget:
		w.ControlModifiers = modifiers
	case *SelectWidget:
		w.input.ControlModifiers = modifiers
	}
	if container, ok := w.(ContainerWidget); ok {
		for _, child := range container.WidgetChildren() {
			SetControlModifiers(child, modifiers)
		}
	}
}
