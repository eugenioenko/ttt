package widgets

const TabDragThreshold = 2

type TabDragState struct {
	pressedIndex int
	targetIndex  int
	sourceID     string
	pressX       int
	active       bool
	dragging     bool
}

func (d *TabDragState) Begin(index int, sourceID string, screenX int) {
	d.pressedIndex = index
	d.targetIndex = index
	d.sourceID = sourceID
	d.pressX = screenX
	d.active = true
	d.dragging = false
}

func (d *TabDragState) SetSourceIndex(index int) {
	d.pressedIndex = index
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
	d.sourceID = ""
	d.pressX = 0
	d.active = false
	d.dragging = false
}

func (d *TabDragState) Active() bool     { return d.active }
func (d *TabDragState) Dragging() bool   { return d.dragging }
func (d *TabDragState) From() int        { return d.pressedIndex }
func (d *TabDragState) Target() int      { return d.targetIndex }
func (d *TabDragState) SourceID() string { return d.sourceID }
