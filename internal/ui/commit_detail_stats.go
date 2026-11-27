package ui

import (
	"strconv"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

func (d *CommitDetailWidget) totalStats() (added, deleted int) {
	for i := range d.Files {
		a, r := d.Files[i].Diff.Stats()
		added += a
		deleted += r
	}
	return added, deleted
}

func fileCountLabel(count int) string {
	if count == 1 {
		return i18n.Format("changes.fileCount", i18n.Args{"count": count})
	}
	return i18n.Format("changes.filesCount", i18n.Args{"count": count})
}

func statsLabels(added, deleted int) (string, string) {
	return "+" + strconv.Itoa(added), "−" + strconv.Itoa(deleted)
}

// summaryText is empty for Current Changes, whose own summary already shows
// the same counts.
func (d *CommitDetailWidget) summaryText() string {
	if d.CurrentChanges || len(d.Files) == 0 {
		return ""
	}
	plus, minus := statsLabels(d.totalStats())
	return fileCountLabel(len(d.Files)) + " · " + plus + " " + minus
}

func (d *CommitDetailWidget) renderSummary(surface Surface, y, viewW int) {
	plus, minus := statsLabels(d.totalStats())
	x := 1
	for _, part := range []struct {
		text  string
		style term.Style
	}{
		{fileCountLabel(len(d.Files)) + " · ", term.StyleMuted},
		{plus, term.StyleSuccess},
		{" ", term.StyleMuted},
		{minus, term.StyleDanger},
	} {
		if x >= viewW-1 {
			return
		}
		d.drawStaticText(surface, x, y, viewW-1-x, part.text, part.style, term.StyleDefault, false)
		x += textwidth.String(part.text)
	}
}

// renderFileStats right-aligns a file's counts on its heading row, only when
// they fit after the visible heading text.
func (d *CommitDetailWidget) renderFileStats(surface Surface, file *CommitDetailFile, textX, y, viewW int, heading string, bg term.Style) {
	added, deleted := file.Diff.Stats()
	if added == 0 && deleted == 0 {
		return
	}
	plus, minus := statsLabels(added, deleted)
	x := viewW - 1 - textwidth.String(plus) - 1 - textwidth.String(minus)
	headingEnd := textX + textwidth.String(heading)
	if !d.IsWrapped() {
		headingEnd -= d.LeftCol
	}
	if x <= headingEnd+1 || x < textX {
		return
	}
	plusStyle, minusStyle := term.StyleSuccess, term.StyleDanger
	if bg == term.StyleSidebarSelected {
		plusStyle, minusStyle = bg, bg
	}
	d.drawStaticText(surface, x, y, textwidth.String(plus), plus, plusStyle, bg, false)
	x += textwidth.String(plus)
	d.drawStaticText(surface, x, y, 1, " ", bg, bg, false)
	d.drawStaticText(surface, x+1, y, textwidth.String(minus), minus, minusStyle, bg, false)
}
