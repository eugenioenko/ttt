package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/search"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"

	"github.com/gdamore/tcell/v3"
)

type SearchBatch struct {
	Gen    uint64
	Groups []SearchFileGroup
	Done   bool
	Error  string
}

// SearchMatch is a ripgrep match produced by the search service.
type SearchMatch = search.Match

type SearchFileGroup struct {
	FilePath string
	IconPath string
	RelPath  string
	Matches  []SearchMatch
	Expanded bool
}

type DiffSearchSource struct {
	TabName  string
	IconPath string
	Lines    []string
}

type SearchWidget struct {
	BaseWidget
	Icons        string
	Input        *InputWidget
	Include      *InputWidget
	Exclude      *InputWidget
	ReplaceInput *InputWidget
	Options      SearchOptions
	focusIdx     int
	showFilters  bool
	showReplace  bool
	Groups       []SearchFileGroup
	FlatList     []searchItem
	Selected     int
	lastSelected int
	ScrollTop    int
	scrollbar    Scrollbar
	WorkDirs     []string
	Searching    bool
	Error        string
	DiffSources  func() []DiffSearchSource
	OnOpenMatch  func(path string, line, col int)
	OnPreview    func(filePath string, matches []SearchMatch, replacement string, opts SearchOptions)
	OnClear      func()
	PostBatch    func(batch *SearchBatch)
	Debounce     Debouncer

	OnReplaceRequest func(SearchReplaceRequest)
	OnCancelReplace  func()
	// ChevronCollapsed and ChevronExpanded mirror TreeConfig; zero means the
	// default triangle.
	ChevronCollapsed rune
	ChevronExpanded  rune
	debouncing       bool
	searchGen        uint64
	searchCancel     context.CancelFunc
	resultStartY     int
	renderWidth      int
	filterToggleHit  HitRegion
	replaceActionX   int
	replaceActionW   int
	previewActionX   int
	previewActionW   int
	replaceVersion   uint64
	replacing        bool
}

type SearchReplaceRequest struct {
	Options     search.Options
	Replacement string
	Paths       []string
	Fallback    map[string][]SearchMatch
	AllFiles    bool
	version     uint64
	workDirs    []string
}

type searchItem struct {
	IsFile bool
	Group  int
	Match  int
}

func NewSearchWidget() *SearchWidget {
	s := &SearchWidget{}
	s.Input = NewInputWidget()
	s.Input.Placeholder = "Search"
	s.Include = NewInputWidget()
	s.Include.Placeholder = "files to include"
	s.Exclude = NewInputWidget()
	s.Exclude.Placeholder = "files to exclude"
	s.ReplaceInput = NewInputWidget()
	s.ReplaceInput.Placeholder = "Replace"
	s.Input.OnChange = func(text string) {
		if text == "" {
			s.runSearchSync()
			if s.OnClear != nil {
				s.OnClear()
			}
			return
		}
		s.scheduleSearch()
	}
	onChange := func(string) { s.scheduleSearch() }
	s.Include.OnChange = onChange
	s.Exclude.OnChange = onChange
	s.ReplaceInput.OnChange = func(string) { s.CancelReplacement() }
	s.Input.Actions = []InputAction{
		{Label: "Aa", OnClick: func() {
			s.Options.CaseSensitive = !s.Options.CaseSensitive
			s.syncOptionActions()
			s.runSearchSync()
		}},
		{Label: ".*", OnClick: func() {
			s.Options.UseRegex = !s.Options.UseRegex
			s.syncOptionActions()
			s.runSearchSync()
		}},
	}
	s.ReplaceInput.Actions = []InputAction{
		{Label: "⟳All", OnClick: func() {
			if s.replacing {
				s.CancelReplacement()
				return
			}
			s.ReplaceAllFiles()
		}},
	}
	return s
}

func (s *SearchWidget) syncOptionActions() {
	if len(s.Input.Actions) >= 2 {
		s.Input.Actions[0].Active = s.Options.CaseSensitive
		s.Input.Actions[1].Active = s.Options.UseRegex
	}
}

func (s *SearchWidget) SetReplaceMode(on bool) {
	if s.showReplace != on {
		s.CancelReplacement()
	}
	s.showReplace = on
}

func (s *SearchWidget) IsReplaceMode() bool {
	return s.showReplace
}

func (s *SearchWidget) ToggleReplaceMode() {
	s.SetReplaceMode(!s.showReplace)
}

func (s *SearchWidget) Refresh() {
	s.runSearchSync()
}

func (s *SearchWidget) visibleInputs() []*InputWidget {
	inputs := []*InputWidget{s.Input}
	if s.showReplace {
		inputs = append(inputs, s.ReplaceInput)
	}
	if s.showFilters {
		inputs = append(inputs, s.Include, s.Exclude)
	}
	return inputs
}

func (s *SearchWidget) FocusedInput() *InputWidget {
	inputs := s.visibleInputs()
	if s.focusIdx >= 0 && s.focusIdx < len(inputs) {
		return inputs[s.focusIdx]
	}
	return s.Input
}

func (s *SearchWidget) SetWorkDirs(dirs []string) {
	if !slices.Equal(s.WorkDirs, dirs) {
		s.CancelReplacement()
	}
	s.WorkDirs = slices.Clone(dirs)
}

func (s *SearchWidget) Focusable() bool { return true }

func (s *SearchWidget) CursorPosition() (int, int, bool) {
	r := s.GetRect()
	inp := s.FocusedInput()
	y := s.inputY(inp)
	return inp.CursorX(r.X), r.Y + y, true
}

func (s *SearchWidget) inputY(inp *InputWidget) int {
	if inp == s.Input {
		return 0
	}
	if inp == s.ReplaceInput && s.showReplace {
		return 2
	}
	base := 2
	if s.showReplace {
		base += 2
	}
	if inp == s.Include && s.showFilters {
		return base
	}
	if inp == s.Exclude && s.showFilters {
		return base + 2
	}
	return 0
}

func (s *SearchWidget) stopSearch() {
	if s.searchCancel != nil {
		s.searchCancel()
		s.searchCancel = nil
	}
}

func (s *SearchWidget) cancelSearch() {
	s.CancelReplacement()
	s.Debounce.Stop()
	s.debouncing = false
	s.stopSearch()
	s.searchGen++
	s.Searching = false
}

func (s *SearchWidget) scheduleSearch() {
	s.CancelReplacement()
	s.debouncing = true
	s.Groups = nil
	s.FlatList = nil
	s.Selected = 0
	s.ScrollTop = 0
	s.Error = ""
	s.stopSearch()
	s.searchGen++
	gen := s.searchGen
	ctx, cancel := context.WithCancel(context.Background())
	s.searchCancel = cancel
	s.Searching = true

	s.Debounce.Schedule(func() {
		s.debouncing = false
		s.streamSearch(ctx, gen)
	})
}

func (s *SearchWidget) runSearchSync() {
	s.cancelSearch()
	s.Error = ""
	if s.Input.Text == "" {
		s.Groups = nil
		s.FlatList = nil
		s.Selected = 0
		s.ScrollTop = 0
		return
	}
	s.searchGen++
	gen := s.searchGen
	ctx, cancel := context.WithCancel(context.Background())
	s.searchCancel = cancel
	s.Searching = true
	go s.streamSearch(ctx, gen)
}

func (s *SearchWidget) streamSearch(ctx context.Context, gen uint64) {
	// streamSearch runs on its own goroutine, where an unrecovered panic would
	// take down the editor rather than just the search.
	defer func() {
		if r := recover(); r != nil {
			s.sendBatch(&SearchBatch{Gen: gen, Done: true, Error: fmt.Sprintf("search failed: %v", r)})
		}
	}()

	var groups []SearchFileGroup

	if s.DiffSources != nil {
		for _, ds := range s.DiffSources() {
			matches, _ := FindInLines(ds.Lines, s.Input.Text, s.Options)
			if len(matches) > 0 {
				g := SearchFileGroup{
					FilePath: ds.TabName,
					IconPath: ds.IconPath,
					RelPath:  ds.TabName,
					Expanded: true,
				}
				for _, m := range matches {
					lineText := ""
					if m.Line >= 0 && m.Line < len(ds.Lines) {
						lineText = ds.Lines[m.Line]
					}
					g.Matches = append(g.Matches, SearchMatch{
						FilePath: ds.TabName,
						LineNum:  m.Line + 1,
						ColStart: m.Col,
						ColEnd:   m.Col + m.Len,
						LineText: lineText,
					})
				}
				groups = append(groups, g)
			}
		}
	}

	if len(s.WorkDirs) > 0 {
		s.streamFiles(ctx, gen, &groups)
		return
	}

	s.sendBatch(&SearchBatch{Gen: gen, Groups: groups, Done: true})
}

func (s *SearchWidget) sendBatch(batch *SearchBatch) {
	if s.PostBatch != nil {
		s.PostBatch(batch)
	}
}

func (s *SearchWidget) ApplyBatch(batch *SearchBatch) {
	if batch.Gen != s.searchGen {
		return
	}
	s.Groups = batch.Groups
	s.flatten()
	if batch.Done {
		s.Searching = false
		s.Error = batch.Error
	}
}

func (s *SearchWidget) streamFiles(ctx context.Context, gen uint64, groups *[]SearchFileGroup) {
	if _, err := exec.LookPath("rg"); err != nil {
		s.sendBatch(&SearchBatch{Gen: gen, Done: true, Error: "ripgrep (rg) not found"})
		return
	}

	dirs := s.WorkDirs
	args := search.BaseArgs(s.searchOptions(), true)
	args = append(args, dirs...)

	cmd := exec.CommandContext(ctx, "rg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.sendBatch(&SearchBatch{Gen: gen, Done: true, Error: "search failed: " + err.Error()})
		return
	}
	if err := cmd.Start(); err != nil {
		s.sendBatch(&SearchBatch{Gen: gen, Done: true, Error: "search failed: " + err.Error()})
		return
	}

	groupMap := map[string]int{}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	first := true
	lastFlush := time.Now()
	dirty := false
	const flushInterval = 100 * time.Millisecond

	for scanner.Scan() {
		line := scanner.Text()
		var msg search.RgMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msg.Type == "end" {
			if dirty && time.Since(lastFlush) >= flushInterval {
				snapshot := make([]SearchFileGroup, len(*groups))
				copy(snapshot, *groups)
				s.sendBatch(&SearchBatch{Gen: gen, Groups: snapshot})
				lastFlush = time.Now()
				dirty = false
			}
			continue
		}
		if msg.Type != "match" {
			continue
		}

		filePath := msg.Data.Path.Text
		absPath := filePath
		if !filepath.IsAbs(absPath) && len(dirs) > 0 {
			absPath = filepath.Join(dirs[0], absPath)
		}
		relPath := filePath
		for _, d := range dirs {
			if r, err := filepath.Rel(d, absPath); err == nil && !strings.HasPrefix(r, "..") {
				relPath = r
				break
			}
		}

		lineText := msg.Data.Lines.Text
		lineText = strings.TrimRight(lineText, "\n\r")

		for _, sub := range msg.Data.Submatches {
			match := SearchMatch{
				FilePath: absPath,
				LineNum:  msg.Data.LineNumber,
				ColStart: sub.Start,
				ColEnd:   sub.End,
				LineText: lineText,
			}

			idx, ok := groupMap[absPath]
			if !ok {
				idx = len(*groups)
				groupMap[absPath] = idx
				*groups = append(*groups, SearchFileGroup{
					FilePath: absPath,
					IconPath: absPath,
					RelPath:  relPath,
					Expanded: true,
				})
			}
			(*groups)[idx].Matches = append((*groups)[idx].Matches, match)
		}
		dirty = true

		if first {
			snapshot := make([]SearchFileGroup, len(*groups))
			copy(snapshot, *groups)
			s.sendBatch(&SearchBatch{Gen: gen, Groups: snapshot})
			lastFlush = time.Now()
			first = false
			dirty = false
		}
	}

	if scanner.Err() != nil {
		cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return
	}

	snapshot := make([]SearchFileGroup, len(*groups))
	copy(snapshot, *groups)
	s.sendBatch(&SearchBatch{Gen: gen, Groups: snapshot, Done: true, Error: rgFailure(waitErr)})
}

// rgFailure reports a message for a ripgrep run that failed. Exit code 1 means
// "no matches", which is a normal empty result, not a failure.
func rgFailure(err error) string {
	if err == nil {
		return ""
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return ""
	}
	return "search failed: " + err.Error()
}

func (s *SearchWidget) CollapseAll() {
	for i := range s.Groups {
		s.Groups[i].Expanded = false
	}
	s.flatten()
}

func (s *SearchWidget) ExpandAll() {
	for i := range s.Groups {
		s.Groups[i].Expanded = true
	}
	s.flatten()
}

func (s *SearchWidget) flatten() {
	s.FlatList = nil
	for gi, g := range s.Groups {
		s.FlatList = append(s.FlatList, searchItem{IsFile: true, Group: gi})
		if g.Expanded {
			for mi := range g.Matches {
				s.FlatList = append(s.FlatList, searchItem{IsFile: false, Group: gi, Match: mi})
			}
		}
	}
}

func (s *SearchWidget) Render(surface Surface) {
	w, h := surface.Size()
	s.renderWidth = w
	s.filterToggleHit = HitRegion{}
	s.replaceActionX = 0
	s.replaceActionW = 0
	s.previewActionX = 0
	s.previewActionW = 0
	surface.Fill(term.Cell{Ch: ' '})

	if h < 2 {
		return
	}

	s.Input.Render(surface, 0, 0, w)
	y := 1

	if s.showReplace {
		for x := 0; x < w; x++ {
			surface.SetCell(x, y, term.Cell{Ch: '─', Style: term.StyleBorder})
		}
		y++
		s.ReplaceInput.Render(surface, 0, y, w)
		y++
	}

	filterIcon := icons.ChevronDown
	if s.showFilters {
		filterIcon = icons.ArrowLeft
	}
	filterToggle := " " + string(icons.Rune(filterIcon)) + " "
	filterToggleW := textwidth.String(filterToggle)
	ftx := max(w-filterToggleW, 0)
	for x := 0; x < ftx; x++ {
		surface.SetCell(x, y, term.Cell{Ch: '─', Style: term.StyleBorder})
	}
	surface.DrawText(ftx, y, filterToggle, w, term.StyleMuted)
	s.filterToggleHit = HitRegion{X: ftx, Y: y, W: w - ftx}
	y++

	if s.showFilters {
		s.Include.Render(surface, 0, y, w)
		y++
		for x := 0; x < w; x++ {
			surface.SetCell(x, y, term.Cell{Ch: '─', Style: term.StyleBorder})
		}
		y++

		s.Exclude.Render(surface, 0, y, w)
		y++
		for x := 0; x < w; x++ {
			surface.SetCell(x, y, term.Cell{Ch: '─', Style: term.StyleBorder})
		}
		y++
	}

	startY := y
	if s.replacing {
		surface.DrawText(0, startY, "Checking replacements…", w, term.StyleMuted)
		startY++
	}

	if s.Error != "" {
		for i, ch := range s.Error {
			if i < w {
				surface.SetCell(i, startY, term.Cell{Ch: ch, Style: term.StyleMuted})
			}
		}
		return
	}

	if s.Input.Text != "" && len(s.FlatList) == 0 && !s.Searching && !s.debouncing {
		msg := "No results"
		for i, ch := range msg {
			if i < w {
				surface.SetCell(i, startY, term.Cell{Ch: ch, Style: term.StyleMuted})
			}
		}
		return
	}

	if s.debouncing || s.Searching {
		msg := "Searching..."
		for i, ch := range msg {
			if i < w {
				surface.SetCell(i, startY, term.Cell{Ch: ch, Style: term.StyleMuted})
			}
		}
		if len(s.FlatList) == 0 {
			return
		}
		startY++
	}

	if len(s.Groups) > 0 {
		totalMatches := 0
		for _, g := range s.Groups {
			totalMatches += len(g.Matches)
		}
		summary := fmt.Sprintf("%d results in %d files", totalMatches, len(s.Groups))
		for i, ch := range summary {
			if i < w {
				surface.SetCell(i, startY, term.Cell{Ch: ch, Style: term.StyleMuted})
			}
		}
		startY++
		for x := 0; x < w; x++ {
			surface.SetCell(x, startY, term.Cell{Ch: '─', Style: term.StyleBorder})
		}
		startY++
	}

	s.resultStartY = startY
	visibleH := h - startY
	if visibleH <= 0 {
		return
	}

	if s.Selected != s.lastSelected {
		s.lastSelected = s.Selected
		if s.Selected < s.ScrollTop {
			s.ScrollTop = s.Selected
		}
		if s.Selected >= s.ScrollTop+visibleH {
			s.ScrollTop = s.Selected - visibleH + 1
		}
	}

	r := s.GetRect()
	s.scrollbar.X = r.X + w - 1
	s.scrollbar.Y = r.Y + startY
	s.scrollbar.Height = visibleH
	s.scrollbar.TotalItems = len(s.FlatList)
	s.scrollbar.TopItem = s.ScrollTop

	for i := 0; i < visibleH; i++ {
		idx := s.ScrollTop + i
		if idx >= len(s.FlatList) {
			break
		}
		item := s.FlatList[idx]
		y := startY + i

		style := term.StyleDefault
		if idx == s.Selected {
			style = term.StyleSidebarSelected
		}

		for x := 0; x < w; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		}

		if item.IsFile {
			g := s.Groups[item.Group]
			chevron := s.ChevronExpanded
			if chevron == 0 {
				chevron = icons.Rune(icons.ChevronDown)
			}
			if !g.Expanded {
				chevron = s.ChevronCollapsed
				if chevron == 0 {
					chevron = icons.Rune(icons.ChevronRight)
				}
			}
			x := surface.DrawText(0, y, string(chevron), w, style) + 1
			replaceGlyph := string(icons.Rune(icons.Replace))
			replaceW := textwidth.String(replaceGlyph)
			right := w - 1
			if s.showReplace && s.ReplaceInput.Text != "" {
				right = min(right, w-4-replaceW)
			}
			iconPath := g.IconPath
			if iconPath == "" {
				iconPath = g.FilePath
			}
			x = drawFileIcon(surface, x, y, right, fileIconRender{
				mode: s.Icons, path: iconPath, bgStyle: style, selected: idx == s.Selected,
			})
			if x < right {
				x = surface.DrawText(x, y, g.RelPath, right, style)
			}

			countStr := fmt.Sprintf(" (%d)", len(g.Matches))
			mutedStyle := term.StyleMuted
			if idx == s.Selected {
				mutedStyle = style
			}
			if x < right {
				surface.DrawText(x, y, countStr, right, mutedStyle)
			}

			if s.showReplace && s.ReplaceInput.Text != "" {
				bx := w - 1 - replaceW
				if bx >= 0 && bx < w-1 {
					surface.DrawText(bx, y, replaceGlyph, w-1, mutedStyle)
					s.replaceActionX = max(bx-1, 0)
					s.replaceActionW = (bx + replaceW) - s.replaceActionX
				}
				px := bx - 2
				if px >= 0 && px < w-1 {
					surface.SetCell(px, y, term.Cell{Ch: '⊙', Style: mutedStyle})
					s.previewActionX = max(px-1, 0)
					s.previewActionW = (px + 1) - s.previewActionX
				}
			}
		} else {
			m := s.Groups[item.Group].Matches[item.Match]
			x := 2

			lineStr := fmt.Sprintf("%d: ", m.LineNum)
			mutedStyle := term.StyleMuted
			if idx == s.Selected {
				mutedStyle = style
			}
			for _, ch := range lineStr {
				if x >= w {
					break
				}
				surface.SetCell(x, y, term.Cell{Ch: ch, Style: mutedStyle})
				x++
			}

			trimmed := strings.TrimLeft(m.LineText, " \t")
			trimOff := len(m.LineText) - len(trimmed)
			for ci, ch := range trimmed {
				if x >= w {
					break
				}
				cs := style
				origCol := ci + trimOff
				if origCol >= m.ColStart && origCol < m.ColEnd {
					cs = term.StyleSearchMatch
				}
				surface.SetCell(x, y, term.Cell{Ch: ch, Style: cs})
				x++
			}
		}
	}

	s.scrollbar.Render(surface, w-1, startY)
}

func (s *SearchWidget) toggleRow() int {
	y := 1
	if s.showReplace {
		y += 2
	}
	return y
}

func (s *SearchWidget) HandleEvent(ev tcell.Event) EventResult {
	if newTop, consumed := s.scrollbar.HandleEvent(ev); consumed {
		s.ScrollTop = newTop
		if s.scrollbar.IsDragging() {
			return EventCaptured
		}
		return EventConsumed
	}
	switch tev := ev.(type) {
	case *tcell.EventMouse:
		btn := tev.Buttons()
		if btn&tcell.Button1 != 0 {
			mx, my := tev.Position()
			r := s.GetRect()
			localX := mx - r.X
			localY := my - r.Y

			if localY == 0 {
				s.focusIdx = 0
				s.Input.HandleClick(mx, my)
				return EventConsumed
			}

			if s.showReplace && localY == 2 {
				inputs := s.visibleInputs()
				for i, inp := range inputs {
					if inp == s.ReplaceInput {
						s.focusIdx = i
						break
					}
				}
				s.ReplaceInput.HandleClick(mx, my)
				return EventConsumed
			}

			renderW := s.renderWidth
			if renderW == 0 {
				renderW = r.W
			}
			filterHit := s.filterToggleHit
			if filterHit.W == 0 && renderW > 0 {
				ftW := min(3, renderW)
				filterHit = HitRegion{X: renderW - ftW, Y: s.toggleRow(), W: ftW}
			}
			if filterHit.Contains(localX, localY) {
				s.showFilters = !s.showFilters
				inputs := s.visibleInputs()
				if s.focusIdx >= len(inputs) {
					s.focusIdx = 0
				}
				return EventConsumed
			}

			if s.showFilters {
				inputs := s.visibleInputs()
				for i, inp := range inputs {
					iy := s.inputY(inp)
					if localY == iy {
						s.focusIdx = i
						return EventConsumed
					}
				}
			}

			idx := s.ScrollTop + (localY - s.resultStartY)
			if idx >= 0 && idx < len(s.FlatList) {
				item := s.FlatList[idx]
				if s.showReplace && s.ReplaceInput.Text != "" && item.IsFile {
					repX, repW := s.replaceActionX, s.replaceActionW
					if repW == 0 && renderW >= 3 {
						repX, repW = renderW-3, 2
					}
					prevX, prevW := s.previewActionX, s.previewActionW
					if prevW == 0 && renderW >= 5 {
						prevX, prevW = renderW-5, 2
					}
					if repW > 0 && localX >= repX && localX < repX+repW {
						s.replaceInFile(item.Group)
						return EventConsumed
					}
					if prevW > 0 && localX >= prevX && localX < prevX+prevW {
						s.previewFile(item.Group)
						return EventConsumed
					}
				}
				s.Selected = idx
				s.activateSelected()
			}
			return EventConsumed
		}
		if btn&tcell.WheelUp != 0 {
			s.ScrollTop -= 3
			if s.ScrollTop < 0 {
				s.ScrollTop = 0
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			max := len(s.FlatList) - 5
			if max < 0 {
				max = 0
			}
			s.ScrollTop += 3
			if s.ScrollTop > max {
				s.ScrollTop = max
			}
			return EventConsumed
		}
	case *tcell.EventKey:
		if tev.Modifiers()&tcell.ModAlt != 0 && tev.Key() == tcell.KeyRune {
			switch term.KeyRune(tev) {
			case 'c':
				s.Options.CaseSensitive = !s.Options.CaseSensitive
				s.syncOptionActions()
				s.runSearchSync()
				return EventConsumed
			case 'r':
				s.Options.UseRegex = !s.Options.UseRegex
				s.syncOptionActions()
				s.runSearchSync()
				return EventConsumed
			}
		}
		switch tev.Key() {
		case tcell.KeyEscape:
			if s.replacing {
				s.CancelReplacement()
				return EventConsumed
			}
			return EventIgnored
		case tcell.KeyTab:
			inputs := s.visibleInputs()
			if len(inputs) > 1 {
				s.focusIdx = (s.focusIdx + 1) % len(inputs)
			}
			return EventConsumed
		case tcell.KeyBacktab:
			inputs := s.visibleInputs()
			if len(inputs) > 1 {
				s.focusIdx = (s.focusIdx + len(inputs) - 1) % len(inputs)
			}
			return EventConsumed
		case tcell.KeyEnter:
			if s.focusIdx == 0 && len(s.FlatList) == 0 {
				s.runSearchSync()
			} else {
				s.activateSelected()
			}
			return EventConsumed
		case tcell.KeyUp:
			if s.Selected > 0 {
				s.Selected--
			}
			return EventConsumed
		case tcell.KeyDown:
			if s.Selected < len(s.FlatList)-1 {
				s.Selected++
			}
			return EventConsumed
		default:
			if s.FocusedInput().HandleEvent(ev) == EventConsumed {
				return EventConsumed
			}
		}
	}

	return EventIgnored
}

func (s *SearchWidget) replaceInFile(groupIdx int) {
	if groupIdx < 0 || groupIdx >= len(s.Groups) {
		return
	}
	s.requestReplacement(s.Groups[groupIdx:groupIdx+1], false)
}

func (s *SearchWidget) previewFile(groupIdx int) {
	if groupIdx < 0 || groupIdx >= len(s.Groups) {
		return
	}
	g := s.Groups[groupIdx]
	if s.OnPreview != nil {
		s.OnPreview(g.FilePath, g.Matches, s.ReplaceInput.Text, s.Options)
	}
}

// searchOptions builds the ripgrep scan options for the current search.
func (s *SearchWidget) searchOptions() search.Options {
	return search.Options{
		CaseSensitive: s.Options.CaseSensitive,
		UseRegex:      s.Options.UseRegex,
		Include:       s.Include.Text,
		Exclude:       s.Exclude.Text,
		Query:         s.Input.Text,
	}
}

func (s *SearchWidget) ReplaceAllFiles() {
	s.requestReplacement(s.Groups, true)
}

func (s *SearchWidget) requestReplacement(groups []SearchFileGroup, all bool) {
	if s.OnReplaceRequest == nil || len(groups) == 0 {
		return
	}
	req := SearchReplaceRequest{
		Options: s.searchOptions(), Replacement: s.ReplaceInput.Text, AllFiles: all,
		Fallback: make(map[string][]SearchMatch, len(groups)),
		version:  s.replaceVersion, workDirs: slices.Clone(s.WorkDirs),
	}
	for _, g := range groups {
		req.Paths = append(req.Paths, g.FilePath)
		req.Fallback[g.FilePath] = slices.Clone(g.Matches)
	}
	s.OnReplaceRequest(req)
}

func (s *SearchWidget) ReplacementCurrent(req SearchReplaceRequest) bool {
	return req.version == s.replaceVersion && req.Options == s.searchOptions() &&
		req.Replacement == s.ReplaceInput.Text && slices.Equal(req.workDirs, s.WorkDirs)
}

func (s *SearchWidget) SetReplacementPending(pending bool) {
	s.replacing = pending
	if pending {
		s.ReplaceInput.Actions[0].Label = "Cancel"
	} else {
		s.ReplaceInput.Actions[0].Label = "⟳All"
	}
}

func (s *SearchWidget) CancelReplacement() {
	s.replaceVersion++
	s.SetReplacementPending(false)
	if s.OnCancelReplace != nil {
		s.OnCancelReplace()
	}
}

func (s *SearchWidget) activateSelected() {
	if s.Selected < 0 || s.Selected >= len(s.FlatList) {
		return
	}
	item := s.FlatList[s.Selected]
	if item.IsFile {
		if s.showReplace && s.ReplaceInput.Text != "" {
			s.previewFile(item.Group)
		} else {
			g := &s.Groups[item.Group]
			g.Expanded = !g.Expanded
			s.flatten()
		}
	} else {
		if s.showReplace && s.ReplaceInput.Text != "" {
			s.previewFile(item.Group)
		} else {
			m := s.Groups[item.Group].Matches[item.Match]
			if s.OnOpenMatch != nil {
				s.OnOpenMatch(m.FilePath, m.LineNum, m.ColStart)
			}
		}
	}
}
