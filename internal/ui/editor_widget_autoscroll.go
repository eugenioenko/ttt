package ui

import "time"

const editorDragAutoScrollDelay = 50 * time.Millisecond

type EditorDragAutoScrollTick struct {
	Generation uint64
}

func (e *EditorPaneWidget) dragAutoScrollDirection(my int) int {
	r := e.GetRect()
	switch {
	case my < r.Y:
		return -1
	case my >= r.Y+e.Viewport.Height:
		return 1
	}
	return 0
}

func (e *EditorPaneWidget) updateDragAutoScroll(mx, my int) {
	e.dragPointerX, e.dragPointerY = mx, my
	if e.dragAutoScrollDirection(my) == 0 {
		e.cancelDragAutoScroll()
		return
	}
	if e.autoScrollTimer != nil || e.PostDragAutoScrollTick == nil {
		return
	}
	e.autoScrollGeneration++
	generation := e.autoScrollGeneration
	post := e.PostDragAutoScrollTick
	e.autoScrollTimer = time.AfterFunc(editorDragAutoScrollDelay, func() {
		post(generation)
	})
}

func (e *EditorPaneWidget) cancelDragAutoScroll() {
	e.autoScrollGeneration++
	if e.autoScrollTimer != nil {
		e.autoScrollTimer.Stop()
		e.autoScrollTimer = nil
	}
}

func (e *EditorPaneWidget) HandleDragAutoScrollTick(generation uint64) bool {
	if generation != e.autoScrollGeneration {
		return false
	}
	e.autoScrollTimer = nil
	if !e.mouseDown {
		return false
	}
	e.extendDragSelection(e.dragPointerX, e.dragPointerY)
	return true
}

func (e *EditorPaneWidget) extendDragSelection(mx, my int) {
	e.Cursor.Line, e.Cursor.Col = e.mouseToPos(e.GetRect(), mx, my)
	e.scrollViewport()
	e.updateDragAutoScroll(mx, my)
}
