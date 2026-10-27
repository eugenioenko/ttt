package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type detailDisclosure struct {
	file string
	gap  int
}

type detailDisclosureRow struct {
	id  detailDisclosure
	row int
}

func (d *CommitDetailWidget) SetFocused(focused bool) { d.focused = focused }

func (d *CommitDetailWidget) disclosure(fileIndex, gap int) detailDisclosure {
	if fileIndex < 0 {
		return detailDisclosure{gap: -1}
	}
	return detailDisclosure{file: commitDetailFileKey(d.Files[fileIndex]), gap: gap}
}

func (d *CommitDetailWidget) disclosureRows() []detailDisclosureRow {
	var controls []detailDisclosureRow
	for i, visual := range d.visualRows {
		row := d.rows[visual.row]
		switch row.kind {
		case commitDetailMessageHeaderRow:
			if len(d.Files) > 0 {
				controls = append(controls, detailDisclosureRow{d.disclosure(-1, -1), i})
			}
		case commitDetailHeadingRow:
			if !visual.continuation {
				controls = append(controls, detailDisclosureRow{d.disclosure(row.fileIndex, -1), i})
			}
		case commitDetailDiffRow:
			v := d.rowView(visual.row)
			if v == nil {
				continue
			}
			if gap, ok := v.embeddedGap(visual.offset); ok {
				controls = append(controls, detailDisclosureRow{d.disclosure(row.fileIndex, gap), i})
			}
		}
	}
	return controls
}

func (v *DiffEditorWidget) embeddedGap(offset int) (int, bool) {
	if !v.IsWrapped() {
		line := offset
		if v.IsUnified() {
			if offset < 0 || offset >= len(v.unifiedRows) {
				return 0, false
			}
			line = v.unifiedRows[offset].sourceLine
		}
		gap, ok := v.gapByLine[line]
		return gap, ok
	}
	p := v.lead()
	layout := p.layout()
	line, _ := layout.rowToTop(offset)
	start, _ := layout.rowOf(line, 0)
	gap, ok := p.DiffOverlay.Gaps[line]
	return gap, ok && start == offset
}

func (d *CommitDetailWidget) reconcileDisclosure(controls []detailDisclosureRow) {
	if !d.keyboardDisclosure {
		return
	}
	for _, c := range controls {
		if c.id == d.selectedDisclosure {
			return
		}
	}
	for _, c := range controls {
		if c.id.file == d.selectedDisclosure.file && c.id.gap == -1 {
			d.selectedDisclosure = c.id
			return
		}
	}
	d.keyboardDisclosure = false
}

func (d *CommitDetailWidget) handleDisclosureKey(event *tcell.EventKey) bool {
	if event.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) != 0 {
		return false
	}
	switch event.Key() {
	case tcell.KeyTab, tcell.KeyBacktab:
		controls := d.disclosureRows()
		if len(controls) == 0 {
			return false
		}
		d.reconcileDisclosure(controls)
		backward := event.Key() == tcell.KeyBacktab || event.Modifiers()&tcell.ModShift != 0
		index := -1
		if d.keyboardDisclosure {
			for i, c := range controls {
				if c.id == d.selectedDisclosure {
					index = i
					break
				}
			}
		}
		if backward {
			if index < 0 {
				index = 0
			}
			index = (index - 1 + len(controls)) % len(controls)
		} else {
			index = (index + 1) % len(controls)
		}
		d.selectedDisclosure = controls[index].id
		d.keyboardDisclosure = true
		row := controls[index].row
		top := d.TopLine
		if row <= d.TopLine {
			d.TopLine = max(row-1, 0)
		} else if row >= d.TopLine+d.viewH {
			d.TopLine = row - d.viewH + 1
		}
		d.clampScroll()
		if d.TopLine != top {
			d.navValid = false
		}
		return true
	case tcell.KeyEscape:
		if d.keyboardDisclosure {
			d.keyboardDisclosure = false
			return true
		}
	case tcell.KeyEnter, tcell.KeyRune:
		if d.keyboardDisclosure && (event.Key() == tcell.KeyEnter || term.KeyRune(event) == ' ') {
			d.activateDisclosure(d.selectedDisclosure)
			return true
		}
	}
	return false
}

func (d *CommitDetailWidget) activateDisclosure(control detailDisclosure) {
	if control.file == "" {
		d.setAllFilesCollapsed(!d.allFilesCollapsed())
		return
	}
	for i, file := range d.Files {
		if commitDetailFileKey(file) == control.file {
			if control.gap < 0 {
				d.toggleFile(i)
			} else {
				d.requestFileContext(i, control.gap)
			}
			return
		}
	}
}

func (d *CommitDetailWidget) disclosureFocused(fileIndex, gap int) bool {
	return d.focused && d.keyboardDisclosure && d.selectedDisclosure == d.disclosure(fileIndex, gap)
}
