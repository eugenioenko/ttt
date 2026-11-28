package ui

import (
	"slices"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

// projectedBlock records a Result range. Unresolved markers stay in the file
// buffer, so a normal save, a second editor group, and Text see the same file.
type projectedBlock struct{ start, end int }
type mergeProjection struct {
	lines            []string
	blocks           []projectedBlock
	fileVer, paneVer uint64
}

func (m *MergeEditorWidget) project() {
	s := m.session
	s.sync()
	if !s.valid {
		m.View = config.MergeText
		return
	}
	p := m.preview
	old := m.projection
	if old.lines != nil && old.fileVer == s.buf.Version() {
		return
	}
	var lines []string
	ranges := make([]projectedBlock, len(s.blocks))
	at := 0
	for i, b := range s.blocks {
		lines = append(lines, s.buf.Lines[at:b.start]...)
		start := len(lines)
		part := s.buf.Lines[b.start : b.start+b.count]
		if c, ok := gitConflictAt(part, 0, s.markerSize); ok && c.end == len(part)-1 {
			end := c.separator
			if c.base >= 0 {
				end = c.base
			}
			part = part[1:end]
		}
		lines = append(lines, part...)
		ranges[i] = projectedBlock{start, len(lines)}
		at = b.start + b.count
	}
	lines = append(lines, s.buf.Lines[at:]...)
	if len(lines) == 0 {
		lines = []string{""}
	}
	p.Buf.SetLines(slices.Clone(lines))
	p.clampCursor()
	p.clampCursorCol()
	o := NewAnnotationOverlay()
	for i, r := range ranges {
		style := term.StyleDiffAdded
		if s.blocks[i].custom || s.blocks[i].choice == 0 {
			style = term.StyleDiffDeleted
		}
		for j := r.start; j < r.end; j++ {
			o.LineBg[j] = style
		}
	}
	p.SetDiffOverlay(o)
	p.bufferDirty = true
	p.FlushOnChange()
	m.projection = mergeProjection{lines: lines, blocks: ranges, fileVer: s.buf.Version(), paneVer: p.Buf.Version()}
}

// An edit spanning several blocks retains the surviving text of each block.
// Newly typed text belongs to the first touched region. Rewrap touched blocks
// as pending custom resolutions so a partial save preserves those decisions.
func (m *MergeEditorWidget) flushProjection() {
	p := m.preview
	pr := m.projection
	if pr.lines == nil || p.Buf.Version() == pr.paneVer {
		return
	}
	old, newLines := pr.lines, p.Buf.Lines

	// Equal neighboring lines can belong to different conflicts. Bound the
	// splice by the editor's edit journal before comparing their text.
	prefix, suffix, size := len(old), len(old), len(old)
	replayed := p.Buf.ChangesSince(pr.paneVer, func(c buffer.Change) {
		prefix = min(prefix, c.Start)
		suffix = min(suffix, size-c.Start-c.Removed)
		size += c.Added - c.Removed
	})
	if !replayed {
		prefix, suffix = 0, 0
		limit := min(len(old), len(newLines))
		for prefix < limit && old[prefix] == newLines[prefix] {
			prefix++
		}
		for suffix < limit-prefix && old[len(old)-1-suffix] == newLines[len(newLines)-1-suffix] {
			suffix++
		}
	}
	start := prefix
	end, next := len(old)-suffix, len(newLines)-suffix
	if start <= end && start <= next && slices.Equal(old[start:end], newLines[start:next]) {
		m.projection.paneVer = p.Buf.Version()
		return
	}

	s := m.session
	if s.buf.Version() != pr.fileVer {
		// Event routing flushes every edit before another writer can run.
		m.project()
		return
	}
	edits := []undo.EditCommand{}
	raw := 0
	projected := 0
	inserted := false
	var plain []struct {
		start, cut int
		lines      []string
	}
	segment := func(a, z int, block int) {
		lo, hi := max(a, start), min(z, end)
		include := !inserted && start >= a && (start < z || (start == z && start == end && (block >= 0 || z == len(old))))
		if hi <= lo && !include {
			return
		}
		replacement := []string{}
		if include {
			replacement = newLines[start:next]
			inserted = true
		}
		lo = max(a, min(z, lo))
		hi = max(lo, hi)
		if block < 0 {
			plain = append(plain, struct {
				start, cut int
				lines      []string
			}{raw + lo - a, hi - lo, slices.Clone(replacement)})
			return
		}
		b := s.blocks[block]
		text := slices.Clone(old[a:lo])
		text = append(text, replacement...)
		text = append(text, old[hi:z]...)
		c, _ := gitConflictAt(b.original, 0, s.markerSize)
		tail := c.separator
		if c.base >= 0 {
			tail = c.base
		}
		marked := []string{b.original[0]}
		marked = append(marked, text...)
		marked = append(marked, b.original[tail:]...)
		after := mergeRange{start: b.start, count: len(marked), custom: true}
		edits = append(edits, &mergeEdit{session: s, block: block, before: b.mergeRange, after: after, oldLines: slices.Clone(s.buf.Lines[b.start : b.start+b.count]), newLines: marked})
		m.active = block
	}
	for i, r := range pr.blocks {
		segment(projected, r.start, -1)
		segment(r.start, r.end, i)
		projected = r.end
		raw = s.blocks[i].start + s.blocks[i].count
	}
	segment(projected, len(old), -1)
	for _, e := range plain {
		edits = append(edits, &mergePlainEdit{start: e.start, old: slices.Clone(s.buf.Lines[e.start : e.start+e.cut]), new: e.lines})
	}
	// Right-to-left application keeps original offsets valid; Undo reverses it.
	slices.SortStableFunc(edits, func(a, b undo.EditCommand) int { return mergeEditStart(b) - mergeEditStart(a) })
	m.result.ExecCommand(&undo.BatchCommand{Commands: edits})
	m.project()
}

type mergePlainEdit struct {
	start    int
	old, new []string
}

func (c *mergePlainEdit) Apply(b *buffer.Buffer) {
	b.Splice(c.start, len(c.old), c.new...)
	b.Dirty = true
}
func (c *mergePlainEdit) Undo(b *buffer.Buffer) {
	b.Splice(c.start, len(c.new), c.old...)
	b.Dirty = true
}
func mergeEditStart(cmd undo.EditCommand) int {
	if e, ok := cmd.(*mergeEdit); ok {
		return e.before.start
	}
	return cmd.(*mergePlainEdit).start
}
