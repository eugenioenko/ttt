package fold

import "sort"

type Range struct {
	StartLine int
	EndLine   int
}

type State struct {
	ranges    []Range
	collapsed map[int]bool

	dirty          bool
	cachedVisible  []int
	cachedBufToVis map[int]int
}

func NewState() *State {
	return &State{
		collapsed: make(map[int]bool),
		dirty:     true,
	}
}

func (s *State) SetRanges(ranges []Range) {
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].StartLine < ranges[j].StartLine
	})
	keep := make(map[int]bool)
	for _, r := range ranges {
		if s.collapsed[r.StartLine] {
			keep[r.StartLine] = true
		}
	}
	s.ranges = ranges
	s.collapsed = keep
	s.dirty = true
}

func (s *State) Toggle(line int) {
	for _, r := range s.ranges {
		if r.StartLine == line {
			if s.collapsed[line] {
				delete(s.collapsed, line)
			} else {
				s.collapsed[line] = true
			}
			s.dirty = true
			return
		}
	}
	for _, r := range s.ranges {
		if line > r.StartLine && line <= r.EndLine && s.collapsed[r.StartLine] {
			delete(s.collapsed, r.StartLine)
			s.dirty = true
			return
		}
	}
}

func (s *State) Expand(line int) {
	if s.collapsed[line] {
		delete(s.collapsed, line)
		s.dirty = true
	}
}

func (s *State) CollapseAll() {
	for _, r := range s.ranges {
		s.collapsed[r.StartLine] = true
	}
	s.dirty = true
}

func (s *State) ExpandAll() {
	s.collapsed = make(map[int]bool)
	s.dirty = true
}

func (s *State) IsCollapsed(startLine int) bool {
	return s.collapsed[startLine]
}

func (s *State) HasCollapsedFolds() bool {
	return len(s.collapsed) > 0
}

func (s *State) FoldAt(line int) *Range {
	for i := range s.ranges {
		if s.ranges[i].StartLine == line {
			return &s.ranges[i]
		}
	}
	return nil
}

func (s *State) ContainingFold(line int) *Range {
	for i := len(s.ranges) - 1; i >= 0; i-- {
		r := &s.ranges[i]
		if line > r.StartLine && line <= r.EndLine && s.collapsed[r.StartLine] {
			return r
		}
	}
	return nil
}

func (s *State) rebuild(totalLines int) {
	if !s.dirty && s.cachedVisible != nil {
		return
	}
	visible := make([]int, 0, totalLines)
	bufToVis := make(map[int]int, totalLines)

	i := 0
	for i < totalLines {
		bufToVis[i] = len(visible)
		visible = append(visible, i)
		if s.collapsed[i] {
			if r := s.FoldAt(i); r != nil {
				for skip := i + 1; skip <= r.EndLine && skip < totalLines; skip++ {
					bufToVis[skip] = -1
				}
				i = r.EndLine + 1
				continue
			}
		}
		i++
	}

	s.cachedVisible = visible
	s.cachedBufToVis = bufToVis
	s.dirty = false
}

func (s *State) VisibleLines(totalLines int) []int {
	s.rebuild(totalLines)
	return s.cachedVisible
}

func (s *State) BufferToVisible(bufLine int) int {
	if s.cachedBufToVis == nil {
		return bufLine
	}
	if v, ok := s.cachedBufToVis[bufLine]; ok {
		return v
	}
	return -1
}

func (s *State) VisibleToBuffer(visIdx int) int {
	if s.cachedVisible == nil || visIdx < 0 || visIdx >= len(s.cachedVisible) {
		return visIdx
	}
	return s.cachedVisible[visIdx]
}

func (s *State) VisibleLineCount(totalLines int) int {
	s.rebuild(totalLines)
	return len(s.cachedVisible)
}

func (s *State) IsLineHidden(line int) bool {
	return s.ContainingFold(line) != nil
}
