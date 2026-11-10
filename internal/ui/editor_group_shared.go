package ui

import (
	"slices"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/selection"
)

func (g *EditorGroupWidget) relatedGroups() []*EditorGroupWidget {
	if g.RelatedGroups != nil {
		return g.RelatedGroups()
	}
	return []*EditorGroupWidget{g}
}

func (g *EditorGroupWidget) HasOtherDocumentView() bool {
	t := g.activeTab()
	return t != nil && g.otherDocumentView(*t)
}

func (g *EditorGroupWidget) otherDocumentView(tab editorTab) bool {
	if tab.Buf == nil {
		return false
	}
	for _, group := range g.relatedGroups() {
		for _, other := range group.tabs {
			if other.Buf == tab.Buf && (group != g || other.ID != tab.ID) {
				return true
			}
		}
	}
	return false
}

func (g *EditorGroupWidget) CloneActiveTabTo(target *EditorGroupWidget) bool {
	return g.cloneTabTo(g.active, target)
}

func (g *EditorGroupWidget) OpenSharedFileFrom(source *EditorGroupWidget, path string) bool {
	for i := range source.tabs {
		if source.tabs[i].FilePath == path && source.tabs[i].Content == nil {
			return source.cloneTabTo(i, g)
		}
	}
	return false
}

func (g *EditorGroupWidget) cloneTabTo(index int, target *EditorGroupWidget) bool {
	if index < 0 || index >= len(g.tabs) || target == g {
		return false
	}
	g.saveMultiState()
	t := &g.tabs[index]
	if t.Buf == nil || t.Content != nil {
		return false
	}
	for i := range target.tabs {
		if target.tabs[i].Buf == t.Buf {
			target.SwitchTab(i)
			return true
		}
	}
	t.Preview, t.Placeholder = false, false
	clone := *t
	cur, vp, sel := *t.Cur, *t.Vp, *t.Sel
	clone.ID = ""
	clone.Cur, clone.Vp, clone.Sel = &cur, &vp, &sel
	clone.Multi, clone.autoClosing = nil, nil
	clone.Highlighter, clone.Diff, clone.Annotations = nil, nil, nil
	clone.Placeholder = false
	clone.Folds = fold.NewState()
	clone.Folds.SetPending(func() []fold.Range { return fold.ComputeIndentRanges(clone.Buf.Lines) })
	target.rebuildHighlighter(&clone)
	clone.rememberView()
	if len(target.tabs) == 1 && target.isUntouchedPlaceholder(target.activeTab()) {
		target.notifyContentTabClose(*target.activeTab())
		target.tabs = nil
	}
	clone.Annotations = target.mergedDecorations(clone.FilePath, clone.Buf, clone.TabSize)
	target.tabs = append(target.tabs, clone)
	target.SwitchTab(len(target.tabs) - 1)
	g.syncTabs()
	return true
}

func (t *editorTab) rememberView() {
	if t.Buf == nil || t.Cur == nil {
		return
	}
	t.viewVersion = t.Buf.Version()
	t.viewText, t.viewAnchorText = "", ""
	if t.Cur.Line >= 0 && t.Cur.Line < len(t.Buf.Lines) {
		t.viewText = t.Buf.Lines[t.Cur.Line]
	}
	if t.Sel != nil && t.Sel.Active && t.Sel.Anchor.Line >= 0 && t.Sel.Anchor.Line < len(t.Buf.Lines) {
		t.viewAnchorText = t.Buf.Lines[t.Sel.Anchor.Line]
	}
}

func remapSharedPosition(pos *selection.Position, change buffer.Change) {
	if change.Removed == change.Added || pos.Line < change.Start {
		return
	}
	if pos.Line >= change.Start+change.Removed {
		pos.Line += change.Added - change.Removed
	} else {
		pos.Line = change.Start + min(pos.Line-change.Start, max(change.Added-1, 0))
	}
}

func remapSharedColumn(col int, before, after string) int {
	old, next := []rune(before), []rune(after)
	prefix := 0
	for prefix < len(old) && prefix < len(next) && old[prefix] == next[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(next)-prefix && old[len(old)-suffix-1] == next[len(next)-suffix-1] {
		suffix++
	}
	if col <= prefix {
		return col
	}
	if col >= len(old)-suffix {
		return max(prefix, col+len(next)-len(old))
	}
	return len(next) - suffix
}

func (g *EditorGroupWidget) syncSharedTab(t *editorTab) {
	if t.Buf == nil || t.Cur == nil || t.viewVersion == t.Buf.Version() {
		return
	}
	pos := selection.Position{Line: t.Cur.Line, Col: t.Cur.Col}
	t.Buf.ChangesSince(t.viewVersion, func(change buffer.Change) {
		remapSharedPosition(&pos, change)
		if t.Sel != nil && t.Sel.Active {
			remapSharedPosition(&t.Sel.Anchor, change)
		}
		view := selection.Position{Line: t.Vp.TopLine}
		remapSharedPosition(&view, change)
		t.Vp.TopLine = max(view.Line, 0)
	})
	pos.Line = max(0, min(pos.Line, len(t.Buf.Lines)-1))
	pos.Col = remapSharedColumn(pos.Col, t.viewText, t.Buf.Lines[pos.Line])
	t.Cur.Line, t.Cur.Col = pos.Line, pos.Col
	g.clampCursor(t)
	if t.Sel != nil && t.Sel.Active {
		t.Sel.Anchor.Line = max(0, min(t.Sel.Anchor.Line, len(t.Buf.Lines)-1))
		t.Sel.Anchor.Col = remapSharedColumn(t.Sel.Anchor.Col, t.viewAnchorText, t.Buf.Lines[t.Sel.Anchor.Line])
		t.Sel.Anchor.Col = max(0, min(t.Sel.Anchor.Col, len([]rune(t.Buf.Lines[t.Sel.Anchor.Line]))))
	}
	if t.Highlighter != nil {
		t.Highlighter.SyncBuffer(t.Buf)
	}
	if t.Folds != nil {
		t.Folds.MarkStale(func() []fold.Range { return fold.ComputeIndentRanges(t.Buf.Lines) })
	}
	t.rememberView()
	if g.activeTab() == t {
		g.Editor.InvalidateBracketColors()
		g.Editor.RefreshSearch()
	}
}

func (g *EditorGroupWidget) SyncDocumentViews() { g.syncDocumentViews(g.activeTab()) }

func (g *EditorGroupWidget) syncDocumentViews(source *editorTab) {
	if source == nil || source.Buf == nil {
		return
	}
	source.rememberView()
	for _, peer := range g.relatedGroups() {
		if peer == g {
			continue
		}
		for i := range peer.tabs {
			t := &peer.tabs[i]
			if t.Buf != source.Buf {
				continue
			}
			identityChanged := t.FilePath != source.FilePath || t.LanguageSelection != source.LanguageSelection
			if t.FilePath != source.FilePath {
				peer.movePathState(t.FilePath, source.FilePath)
			}
			t.FilePath, t.IconPath, t.Virtual = source.FilePath, source.IconPath, source.Virtual
			t.Undo, t.ReadOnly = source.Undo, source.ReadOnly
			t.LanguageSelection, t.DetectedLanguage = source.LanguageSelection, source.DetectedLanguage
			t.TabSize, t.UseTabs = source.TabSize, source.UseTabs
			if identityChanged {
				peer.rebuildHighlighter(t)
			}
			peer.syncSharedTab(t)
		}
		peer.syncTabs()
	}
}

func (g *EditorGroupWidget) DiscardDuplicateViews(source *EditorGroupWidget) {
	for i := len(source.tabs) - 1; i >= 0; i-- {
		for _, keep := range g.tabs {
			if keep.Buf != nil && keep.Buf == source.tabs[i].Buf {
				source.notifyContentTabClose(source.tabs[i])
				source.tabs = slices.Delete(source.tabs, i, i+1)
				if i < source.pinnedCount {
					source.pinnedCount--
				}
				break
			}
		}
	}
	if len(source.tabs) == 0 {
		source.tabs = []editorTab{source.placeholderTab()}
	}
	source.active = min(source.active, len(source.tabs)-1)
	source.syncTabs()
}

func (g *EditorGroupWidget) adoptSaveAsViews(source *editorTab, path string) {
	for _, peer := range g.relatedGroups() {
		if peer == g {
			continue
		}
		adopted := false
		for i := range peer.tabs {
			t := &peer.tabs[i]
			if t.Buf == nil || t.Buf == source.Buf || t.Virtual {
				continue
			}
			if same, err := sameSaveTarget(t.FilePath, path); err == nil && same {
				t.Buf, t.Undo = source.Buf, source.Undo
				adopted = true
				peer.clampCursor(t)
				peer.rebuildHighlighter(t)
				t.rememberView()
			}
		}
		if adopted {
			peer.deduplicateDocumentViews()
		}
	}
}

func (g *EditorGroupWidget) deduplicateDocumentViews() {
	seen := map[*buffer.Buffer]bool{}
	if active := g.activeTab(); active != nil && active.Buf != nil {
		seen[active.Buf] = true
	}
	var remove []int
	for i, tab := range g.tabs {
		if i == g.active || tab.Buf == nil {
			continue
		}
		if seen[tab.Buf] {
			remove = append(remove, i)
		} else {
			seen[tab.Buf] = true
		}
	}
	for _, i := range slices.Backward(remove) {
		g.notifyContentTabClose(g.tabs[i])
		g.tabs = slices.Delete(g.tabs, i, i+1)
		if i < g.pinnedCount {
			g.pinnedCount--
		}
		if i < g.active {
			g.active--
		}
	}
}
