package widgets

const TabDragThreshold = 2

// TabDragState owns only the pointer gesture. The widget that contains the
// canonical tabs remains responsible for translating the reported from/to
// indices into an actual reorder.
type TabDragState struct {
	pressedIndex int
	targetIndex  int
	pressX       int
	active       bool
	dragging     bool
}

func (d *TabDragState) Begin(index, screenX int) {
	d.pressedIndex = index
	d.targetIndex = index
	d.pressX = screenX
	d.active = true
	d.dragging = false
}

func (d *TabDragState) Update(screenX, targetIndex int) bool {
	if !d.active {
		return false
	}
	distance := screenX - d.pressX
	if distance < 0 {
		distance = -distance
	}
	if !d.dragging && distance >= TabDragThreshold {
		d.dragging = true
	}
	if d.dragging && targetIndex >= 0 {
		d.targetIndex = targetIndex
	}
	return d.dragging
}

func (d *TabDragState) End() (from, to int, dragged bool) {
	if !d.active {
		return 0, 0, false
	}
	from, to, dragged = d.pressedIndex, d.targetIndex, d.dragging
	d.Cancel()
	return from, to, dragged
}

func (d *TabDragState) Cancel() {
	d.pressedIndex = 0
	d.targetIndex = 0
	d.pressX = 0
	d.active = false
	d.dragging = false
}

func (d *TabDragState) Active() bool   { return d.active }
func (d *TabDragState) Dragging() bool { return d.dragging }
func (d *TabDragState) From() int      { return d.pressedIndex }
func (d *TabDragState) Target() int    { return d.targetIndex }
