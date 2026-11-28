package ui

import (
	"slices"
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/scm"
)

type mergeChoice uint8

const (
	mergeIncoming mergeChoice = 1 << iota
	mergeCurrent
	mergeReverse
	mergeNeither
	mergeManual
)

type mergeRange struct {
	start, count int
	choice       mergeChoice
	custom       bool
	accepted     []string
}
type mergeBlock struct {
	mergeRange
	original []string
	sides    [2][]string
	anchors  [2]int
}
type mergeSession struct {
	buf        *buffer.Buffer
	version    uint64
	markerSize int
	blocks     []mergeBlock
	valid      bool
	labels     [2]string
}

// Input excerpts come from the materialized conflict. Other lines in the
// working copy may already include clean merges and unsaved edits.
func newMergeSession(buf *buffer.Buffer, inputs scm.ConflictInputs) (*mergeSession, [2][]string) {
	var panes [2][]string
	m := &mergeSession{buf: buf, version: buf.Version(), valid: true}
	m.markerSize = inputs.MarkerSize
	if m.markerSize <= 0 {
		m.markerSize = 7
	}
	for i := 0; i < len(buf.Lines); {
		line := buf.Lines[i]
		kind := gitMarker(line, m.markerSize)
		if kind != '<' {
			if kind == '>' || kind == '|' || alternateConflictAt(buf.Lines, i, m.markerSize) {
				return nil, panes
			}
			for side := range panes {
				panes[side] = append(panes[side], line)
			}
			i++
			continue
		}
		c, ok := gitConflictAt(buf.Lines, i, m.markerSize)
		if !ok {
			return nil, panes
		}
		end := c.separator
		if c.base >= 0 {
			end = c.base
		}
		b := mergeBlock{mergeRange: mergeRange{start: i, count: c.end - i + 1}, original: slices.Clone(buf.Lines[i : c.end+1])}
		b.sides = [2][]string{slices.Clone(buf.Lines[c.separator+1 : c.end]), slices.Clone(buf.Lines[i+1 : end])}
		for side := range panes {
			b.anchors[side] = len(panes[side])
			panes[side] = append(panes[side], b.sides[side]...)
			// An empty side needs a real gutter row to accept the deletion.
			if len(b.sides[side]) == 0 {
				panes[side] = append(panes[side], "")
			}
		}
		if len(m.blocks) == 0 {
			m.labels = [2]string{strings.TrimSpace(strings.TrimLeft(buf.Lines[c.end], ">")), strings.TrimSpace(strings.TrimLeft(line, "<"))}
		}
		m.blocks = append(m.blocks, b)
		i = c.end + 1
	}
	if !inputs.Current.Exists || !inputs.Incoming.Exists {
		return nil, panes
	}
	panes[0] = m.sourceLines(0, panes[0], inputs.Incoming.Content)
	panes[1] = m.sourceLines(1, panes[1], inputs.Current.Content)
	return m, panes
}

func (m *mergeSession) sync() {
	if m.version == m.buf.Version() {
		return
	}
	if !m.buf.ChangesSince(m.version, func(c buffer.Change) {
		for i := range m.blocks {
			b := &m.blocks[i]
			end := b.start + b.count
			switch {
			case c.Start+c.Removed <= b.start && c.Start < b.start:
				b.start += c.Added - c.Removed
			case c.Start >= end:
			case c.Start >= b.start && c.Start+c.Removed <= end:
				b.count += c.Added - c.Removed
				b.custom = true
			default:
				// An edit crossing a boundary has no unique owner. Preserve
				// the edited text and require reopening before another choice.
				m.valid = false
			}
		}
	}) {
		m.valid = false
	}
	m.version = m.buf.Version()
	if !m.valid {
		return
	}
	for i := range m.blocks {
		b := &m.blocks[i]
		if b.custom && b.choice != 0 {
			b.custom = !slices.Equal(m.buf.Lines[b.start:b.start+b.count], b.accepted)
		}
	}
}
func (m *mergeSession) remaining() int {
	m.sync()
	n := 0
	for _, b := range m.blocks {
		if b.choice == 0 || b.custom {
			n++
		}
	}
	return n
}
func (m *mergeSession) replacement(i int, choice mergeChoice) []string {
	b := m.blocks[i]
	switch choice {
	case 0:
		return slices.Clone(b.original)
	case mergeManual:
		lines := m.buf.Lines[b.start : b.start+b.count]
		if c, ok := gitConflictAt(lines, 0, m.markerSize); ok && c.end == len(lines)-1 {
			end := c.separator
			if c.base >= 0 {
				end = c.base
			}
			return slices.Clone(lines[1:end])
		}
		return slices.Clone(lines)
	}
	var lines []string
	for k := range 2 {
		side := k
		if choice&mergeReverse != 0 {
			side = 1 - k
		}
		if choice&(1<<side) != 0 {
			lines = append(lines, b.sides[side]...)
		}
	}
	return lines
}

type mergeEdit struct {
	session            *mergeSession
	block              int
	before, after      mergeRange
	oldLines, newLines []string
}

func (c *mergeEdit) replace(buf *buffer.Buffer, remove int, lines []string, state mergeRange) {
	m := c.session
	m.sync()
	buf.Splice(state.start, remove, lines...)
	buf.Dirty = true
	m.blocks[c.block].mergeRange = state
	for j := c.block + 1; j < len(m.blocks); j++ {
		m.blocks[j].start += len(lines) - remove
	}
	m.version = buf.Version()
}
func (c *mergeEdit) Apply(buf *buffer.Buffer) { c.replace(buf, len(c.oldLines), c.newLines, c.after) }
func (c *mergeEdit) Undo(buf *buffer.Buffer)  { c.replace(buf, len(c.newLines), c.oldLines, c.before) }

func (m *mergeSession) edit(i int, choice mergeChoice) undo.EditCommand {
	m.sync()
	if !m.valid || i < 0 || i >= len(m.blocks) {
		return nil
	}
	b := m.blocks[i]
	lines := m.replacement(i, choice)
	if len(lines) == 0 && b.count == len(m.buf.Lines) {
		lines = []string{""}
	}
	after := mergeRange{start: b.start, count: len(lines), choice: choice, accepted: lines}
	return &mergeEdit{session: m, block: i, before: b.mergeRange, after: after, oldLines: slices.Clone(m.buf.Lines[b.start : b.start+b.count]), newLines: lines}
}

func (m *mergeSession) sourceBlock(side, line int) (int, bool) {
	i := sort.Search(len(m.blocks), func(i int) bool { return m.blocks[i].anchors[side] > line }) - 1
	return i, i >= 0 && line < m.blocks[i].anchors[side]+max(1, len(m.blocks[i].sides[side]))
}
