package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"

	"github.com/gdamore/tcell/v3"
)

type paletteMode int

const (
	paletteCommandMode paletteMode = iota
	paletteFileMode
	paletteGoToLineMode
	paletteHelpMode
)

const (
	paletteMinWidth = 40
	paletteMaxWidth = 90
)

type selectDialogLayout struct {
	boxX         int
	boxY         int
	boxW         int
	boxH         int
	visibleItems int
}

type PaletteItem struct {
	Label       string
	Detail      string
	ID          string
	Description string
	kind        paletteItemKind
	topicID     string
	searchText  []string
}

type paletteFile struct {
	Rel string
	Abs string
}

type SelectDialogWidget struct {
	BaseWidget
	Commands          []command.Command
	Items             []PaletteItem
	Input             *InputWidget
	Selected          int
	scrollOffset      int
	inputX            int
	inputY            int
	mode              paletteMode
	files             []paletteFile
	helpTopics        []PaletteItem
	helpCommands      []PaletteItem
	activeHelpTopic   string
	helpQuery         string
	OnExecute         func(id string)
	OnOpenFile        func(path string)
	OnGoToLine        func(line int)
	OnDismiss         func()
	OnSelectionChange func(id string)
	Borders           *term.BorderSet

	boxX         int
	boxY         int
	boxW         int
	boxH         int
	visibleItems int
	showScroll   bool
	scrollbar    Scrollbar
}

func NewSelectDialogWidget(commands []command.Command) *SelectDialogWidget {
	p := &SelectDialogWidget{
		Commands: commands,
	}
	p.helpTopics, p.helpCommands = buildPaletteHelpItems(commands)
	p.Input = NewInputWidget()
	p.Input.Prefix = " "
	p.Input.SetText(">")
	p.Input.OnChange = func(text string) {
		p.filter()
	}
	p.filter()
	return p
}

func (p *SelectDialogWidget) SetFiles(workDirs []string) {
	p.files = nil
	multiRoot := len(workDirs) > 1
	for _, workDir := range workDirs {
		prefix := ""
		if multiRoot {
			prefix = filepath.Base(workDir) + string(filepath.Separator)
		}
		filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if name == ".git" || name == "node_modules" || name == ".cache" || name == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(workDir, path)
			if err != nil {
				return nil
			}
			p.files = append(p.files, paletteFile{Rel: prefix + rel, Abs: path})
			if len(p.files) >= 10000 {
				return filepath.SkipAll
			}
			return nil
		})
	}
}

func (p *SelectDialogWidget) Focusable() bool { return true }

func (p *SelectDialogWidget) CursorPosition() (int, int, bool) {
	return p.Input.CursorX(p.inputX), p.inputY, true
}

func calculatePaletteWidth(screenWidth int) int {
	width := screenWidth * 6 / 10
	width = max(width, paletteMinWidth)
	width = min(width, paletteMaxWidth)
	return min(width, max(screenWidth-4, 1))
}

func (p *SelectDialogWidget) calculateLayout(sw, sh int) selectDialogLayout {
	boxW := calculatePaletteWidth(sw)
	var boxH int
	if p.mode == paletteGoToLineMode {
		boxH = 3
	} else {
		maxItems := 10
		chromeRows := 4
		if p.mode == paletteHelpMode {
			// Help reserves one divider and one detail row beneath the list.
			chromeRows += 2
		}
		itemRows := len(p.Items)
		if p.mode == paletteHelpMode && p.helpQuery != "" && itemRows == 0 {
			itemRows = 1
		}
		boxH = chromeRows + itemRows
		if boxH > maxItems+chromeRows {
			boxH = maxItems + chromeRows
		}
	}
	if boxH > sh-2 {
		boxH = sh - 2
	}

	visibleItems := 0
	if p.mode != paletteGoToLineMode {
		visibleItems = boxH - 4
		if p.mode == paletteHelpMode {
			visibleItems -= 2
		}
	}

	return selectDialogLayout{
		boxX:         (sw - boxW) / 2,
		boxY:         2,
		boxW:         boxW,
		boxH:         boxH,
		visibleItems: visibleItems,
	}
}

func (p *SelectDialogWidget) Render(surface Surface) {
	sw, sh := surface.Size()
	layout := p.calculateLayout(sw, sh)
	boxX := layout.boxX
	boxY := layout.boxY
	boxW := layout.boxW
	boxH := layout.boxH

	p.boxX = boxX
	p.boxY = boxY
	p.boxW = boxW
	p.boxH = boxH

	b := term.DoubleBorderSet()
	if p.Borders != nil {
		b = *p.Borders
	}
	surface.DrawBorder(boxX, boxY, boxW, boxH, b, term.StyleBorder)

	surface.ClearRect(boxX+1, boxY+1, boxW-2, boxH-2, term.StyleDefault)

	p.inputX = boxX + 1
	p.inputY = boxY + 1
	p.Input.Render(surface, p.inputX, p.inputY, boxW-2)

	if p.mode == paletteGoToLineMode {
		p.visibleItems = layout.visibleItems
		p.showScroll = false
		return
	}

	for x := boxX + 1; x < boxX+boxW-1; x++ {
		surface.SetCell(x, boxY+2, term.Cell{Ch: b.Horizontal, Style: term.StyleBorder})
	}

	visibleItems := layout.visibleItems
	p.visibleItems = visibleItems
	p.ensureVisible(visibleItems)
	showScroll := len(p.Items) > visibleItems
	p.showScroll = showScroll
	contentRight := boxX + boxW - 1
	if showScroll {
		contentRight--
	}

	if showScroll {
		p.scrollbar.Height = visibleItems
		p.scrollbar.TotalItems = len(p.Items)
		p.scrollbar.TopItem = p.scrollOffset
		p.scrollbar.X = boxX + boxW - 2
		p.scrollbar.Y = boxY + 3
	}

	for i := 0; i < visibleItems && p.scrollOffset+i < len(p.Items); i++ {
		y := boxY + 3 + i
		idx := p.scrollOffset + i
		item := p.Items[idx]

		style := term.StylePaletteItem
		if idx == p.Selected {
			style = term.StylePaletteSelected
		}

		surface.ClearRect(boxX+1, y, contentRight-boxX-1, 1, style)
		p.renderItemRow(surface, y, contentRight, item, style, idx == p.Selected)
	}
	if p.mode == paletteHelpMode && p.helpQuery != "" && len(p.Items) == 0 && visibleItems > 0 {
		y := boxY + 3
		message := fmt.Sprintf("No help entries match %q", p.helpQuery)
		surface.DrawText(boxX+2, y, message, contentRight-1, term.StyleMuted)
	}

	if showScroll {
		p.scrollbar.Render(surface, p.scrollbar.X, p.scrollbar.Y)
	}

	if p.mode == paletteHelpMode {
		dividerY := boxY + boxH - 3
		for x := boxX + 1; x < boxX+boxW-1; x++ {
			surface.SetCell(x, dividerY, term.Cell{Ch: b.Horizontal, Style: term.StyleBorder})
		}
		detailY := boxY + boxH - 2
		surface.ClearRect(boxX+1, detailY, boxW-2, 1, term.StyleDefault)
		if p.Selected >= 0 && p.Selected < len(p.Items) {
			description := p.Items[p.Selected].Description
			surface.DrawText(boxX+2, detailY, description, boxX+boxW-2, term.StyleMuted)
		} else if p.helpQuery != "" {
			surface.DrawText(boxX+2, detailY, "Try > for all commands", boxX+boxW-2, term.StyleMuted)
		}
	}
}

func (p *SelectDialogWidget) renderItemRow(surface Surface, y, contentRight int, item PaletteItem, style term.Style, selected bool) {
	detail := item.Detail
	detailStyle := term.StyleMuted
	if selected {
		detailStyle = style
	}

	// Help commands reserve room for their derived shortcut. Other modes keep
	// their existing preference for the complete label and add detail only if it
	// fits after the columns DrawText actually consumed.
	if p.mode == paletteHelpMode && item.kind == paletteCommandItem && detail != "" {
		detailW := textwidth.String(detail)
		detailX := contentRight - 1 - detailW
		if detailX > p.boxX+4 {
			surface.DrawText(p.boxX+2, y, item.Label, detailX-2, style)
			surface.DrawText(detailX, y, detail, contentRight-1, detailStyle)
			return
		}
	}

	labelEnd := surface.DrawText(p.boxX+2, y, item.Label, contentRight-1, style)
	if detail == "" {
		return
	}
	availStart := labelEnd + 3
	availW := contentRight - 1 - availStart
	if availW <= 0 {
		return
	}
	detail = truncatePaletteDetail(detail, availW)
	detailX := contentRight - 1 - textwidth.String(detail)
	surface.DrawText(detailX, y, detail, contentRight-1, detailStyle)
}

func truncatePaletteDetail(detail string, availW int) string {
	if textwidth.String(detail) <= availW {
		return detail
	}
	runes := []rune(detail)
	tailW := 0
	i := len(runes)
	for i > 0 {
		width := textwidth.Rune(runes[i-1])
		if tailW+width > availW-1 {
			break
		}
		tailW += width
		i--
	}
	return "…" + string(runes[i:])
}

func (p *SelectDialogWidget) HandleEvent(ev tcell.Event) EventResult {
	if mev, ok := ev.(*tcell.EventMouse); ok {
		btn := mev.Buttons()
		mx, my := mev.Position()

		if p.showScroll {
			if newTop, consumed := p.scrollbar.HandleEvent(ev); consumed {
				p.scrollOffset = newTop
				if p.Selected < p.scrollOffset {
					p.Selected = p.scrollOffset
					p.notifySelectionChange()
				} else if p.visibleItems > 0 && p.Selected >= p.scrollOffset+p.visibleItems {
					p.Selected = p.scrollOffset + p.visibleItems - 1
					if p.Selected >= len(p.Items) {
						p.Selected = len(p.Items) - 1
					}
					p.notifySelectionChange()
				}
				if p.scrollbar.IsDragging() {
					return EventCaptured
				}
				return EventConsumed
			}
			if p.scrollbar.IsDragging() {
				return EventCaptured
			}
		}

		if btn&tcell.WheelUp != 0 {
			if p.Selected > 0 {
				p.Selected--
				p.notifySelectionChange()
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			if p.Selected < len(p.Items)-1 {
				p.Selected++
				p.notifySelectionChange()
			}
			return EventConsumed
		}

		if btn&tcell.Button1 != 0 {
			inBox := mx >= p.boxX && mx < p.boxX+p.boxW && my >= p.boxY && my < p.boxY+p.boxH
			if !inBox {
				if p.OnDismiss != nil {
					p.OnDismiss()
				}
				return EventConsumed
			}

			if my == p.inputY {
				p.Input.HandleClick(mx, my)
				return EventConsumed
			}

			itemsStartY := p.boxY + 3
			if p.visibleItems > 0 && my >= itemsStartY && my < itemsStartY+p.visibleItems {
				clickedIdx := p.scrollOffset + (my - itemsStartY)
				if clickedIdx >= 0 && clickedIdx < len(p.Items) {
					p.Selected = clickedIdx
					p.activateItem(p.Items[p.Selected])
				}
			}
		}

		return EventConsumed
	}

	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventConsumed
	}

	switch kev.Key() {
	case tcell.KeyEscape:
		if p.mode == paletteHelpMode && p.activeHelpTopic != "" {
			p.activeHelpTopic = ""
			p.Input.SetText("?")
			return EventConsumed
		}
		if p.OnDismiss != nil {
			p.OnDismiss()
		}
	case tcell.KeyEnter:
		if p.mode == paletteGoToLineMode {
			if p.OnGoToLine != nil {
				text := strings.TrimPrefix(p.Input.Text, ":")
				if n, err := strconv.Atoi(text); err == nil {
					if n < 1 {
						n = 1
					}
					p.OnGoToLine(n)
				}
			}
		} else if p.Selected >= 0 && p.Selected < len(p.Items) {
			p.activateItem(p.Items[p.Selected])
		}
	case tcell.KeyUp:
		if p.Selected > 0 {
			p.Selected--
		} else if len(p.Items) > 0 {
			p.Selected = len(p.Items) - 1
		}
		p.notifySelectionChange()
	case tcell.KeyDown:
		if p.Selected < len(p.Items)-1 {
			p.Selected++
		} else {
			p.Selected = 0
		}
		p.notifySelectionChange()
	default:
		if kev.Key() == tcell.KeyRune && kev.Str() == "?" && p.mode == paletteCommandMode && p.Input.Text == ">" {
			p.Input.SetText("?")
			return EventConsumed
		}
		p.Input.HandleEvent(ev)
	}

	return EventConsumed
}

func (p *SelectDialogWidget) activateItem(item PaletteItem) {
	if p.mode == paletteHelpMode {
		if item.kind == paletteHelpTopicItem {
			p.activeHelpTopic = item.topicID
			p.Input.SetText("?")
			return
		}
		if p.OnExecute != nil {
			p.OnExecute(item.ID)
		}
		return
	}
	if p.mode == paletteCommandMode {
		if p.OnExecute != nil {
			p.OnExecute(item.ID)
		}
		return
	}
	if p.OnOpenFile != nil {
		p.OnOpenFile(item.ID)
	}
}

func (p *SelectDialogWidget) ensureVisible(visibleItems int) {
	if visibleItems <= 0 {
		return
	}
	if p.Selected < p.scrollOffset {
		p.scrollOffset = p.Selected
	}
	if p.Selected >= p.scrollOffset+visibleItems {
		p.scrollOffset = p.Selected - visibleItems + 1
	}
}

func (p *SelectDialogWidget) notifySelectionChange() {
	if p.OnSelectionChange != nil && p.Selected >= 0 && p.Selected < len(p.Items) {
		p.OnSelectionChange(p.Items[p.Selected].ID)
	}
}

func (p *SelectDialogWidget) filter() {
	text := p.Input.Text
	if strings.HasPrefix(text, ">") {
		p.mode = paletteCommandMode
		p.activeHelpTopic = ""
		query := strings.TrimLeft(text[1:], " ")
		p.filterCommands(query)
	} else if strings.HasPrefix(text, ":") {
		p.mode = paletteGoToLineMode
		p.activeHelpTopic = ""
		p.Items = nil
		p.Selected = 0
		p.scrollOffset = 0
	} else if strings.HasPrefix(text, "?") {
		p.mode = paletteHelpMode
		query := strings.TrimSpace(text[1:])
		p.filterHelp(query)
	} else {
		p.mode = paletteFileMode
		p.activeHelpTopic = ""
		p.filterFiles(text)
	}
	p.notifySelectionChange()
}

func (p *SelectDialogWidget) filterHelp(query string) {
	p.helpQuery = query
	if p.activeHelpTopic != "" {
		topic, ok := paletteHelpTopicByID(p.activeHelpTopic)
		if !ok {
			p.activeHelpTopic = ""
		} else {
			candidates := make([]PaletteItem, 0, len(p.helpCommands))
			for _, item := range p.helpCommands {
				if paletteTopicMatchesCommand(topic, item) {
					candidates = append(candidates, item)
				}
			}
			p.Items = filterPaletteHelpCandidates(query, candidates)
			p.Selected = 0
			p.scrollOffset = 0
			return
		}
	}

	if query == "" {
		p.Items = append(p.Items[:0], p.helpTopics...)
	} else {
		candidates := make([]PaletteItem, 0, len(p.helpTopics)+len(p.helpCommands))
		candidates = append(candidates, p.helpTopics...)
		candidates = append(candidates, p.helpCommands...)
		p.Items = filterPaletteHelpCandidates(query, candidates)
	}
	p.Selected = 0
	p.scrollOffset = 0
}

func filterPaletteHelpCandidates(query string, candidates []PaletteItem) []PaletteItem {
	if query == "" {
		return append([]PaletteItem(nil), candidates...)
	}
	type scored struct {
		item  PaletteItem
		score int
	}
	matches := make([]scored, 0, len(candidates))
	bestScore := 0
	for _, item := range candidates {
		if ok, score := scorePaletteHelpItem(query, item); ok {
			matches = append(matches, scored{item: item, score: score})
			if score > bestScore {
				bestScore = score
			}
		}
	}
	relevanceFloor := bestScore * paletteHelpRelativeNumerator / paletteHelpRelativeDenominator
	if relevanceFloor < paletteHelpScoreFloor {
		relevanceFloor = paletteHelpScoreFloor
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})
	items := make([]PaletteItem, 0, len(matches))
	for _, match := range matches {
		if match.score >= relevanceFloor {
			items = append(items, match.item)
		}
	}
	return items
}

func (p *SelectDialogWidget) filterCommands(query string) {
	p.Items = nil
	if query == "" {
		for _, cmd := range p.Commands {
			p.Items = append(p.Items, PaletteItem{
				Label:  cmd.Title,
				Detail: cmd.Shortcut,
				ID:     cmd.ID,
			})
		}
	} else {
		type scored struct {
			item  PaletteItem
			score int
		}
		var matches []scored
		for _, cmd := range p.Commands {
			bestOk, bestScore := fuzzyMatch(query, cmd.Title)
			for _, kw := range cmd.Keywords {
				if ok, score := fuzzyMatch(query, kw); ok {
					penalized := score / 2
					if !bestOk || penalized > bestScore {
						bestOk = true
						bestScore = penalized
					}
				}
			}
			if bestOk {
				matches = append(matches, scored{
					item: PaletteItem{
						Label:  cmd.Title,
						Detail: cmd.Shortcut,
						ID:     cmd.ID,
					},
					score: bestScore,
				})
			}
		}
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].score > matches[j].score
		})
		for _, m := range matches {
			p.Items = append(p.Items, m.item)
		}
	}
	p.Selected = 0
	p.scrollOffset = 0
}

func fileDetail(f string) string {
	dir := filepath.Dir(f)
	if dir == "." {
		return ""
	}
	return dir
}

func (p *SelectDialogWidget) filterFiles(query string) {
	p.Items = nil
	if query == "" {
		for _, f := range p.files {
			p.Items = append(p.Items, PaletteItem{
				Label:  filepath.Base(f.Rel),
				Detail: fileDetail(f.Rel),
				ID:     f.Abs,
			})
			if len(p.Items) >= 100 {
				break
			}
		}
	} else {
		type scored struct {
			item  PaletteItem
			score int
		}
		var matches []scored
		for _, f := range p.files {
			if ok, score := fuzzyMatch(query, f.Rel); ok {
				matches = append(matches, scored{
					item: PaletteItem{
						Label:  filepath.Base(f.Rel),
						Detail: fileDetail(f.Rel),
						ID:     f.Abs,
					},
					score: score,
				})
			}
		}
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].score > matches[j].score
		})
		for _, m := range matches {
			p.Items = append(p.Items, m.item)
			if len(p.Items) >= 100 {
				break
			}
		}
	}
	p.Selected = 0
	p.scrollOffset = 0
}
