package widgets

type InputDrag struct {
	active       bool
	anchor, x, y int
}

func (d *InputDrag) Begin(x, y, anchor int) { d.active, d.x, d.y, d.anchor = true, x, y, anchor }
func (d *InputDrag) Active() bool           { return d.active }
func (d *InputDrag) Extend(x, y, position int) (int, int, bool) {
	if !d.active || x == d.x && y == d.y {
		return 0, 0, false
	}
	d.x, d.y = x, y
	return d.anchor, position, true
}
func (d *InputDrag) Cancel() bool {
	active := d.active
	d.active = false
	return active
}
