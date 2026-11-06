package ui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/term"
)

func newSaveAsGroup(t *testing.T, text string, targetFirst bool) (*EditorGroupWidget, string, string) {
	t.Helper()
	dir := t.TempDir()
	source, target := filepath.Join(dir, "source.go"), filepath.Join(dir, "target.go")
	for path, content := range map[string]string{source: text, target: "target"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.SyntaxHighlight = true
	paths := []string{source, target}
	if targetFirst {
		slices.Reverse(paths)
	}
	for _, path := range paths {
		if err := g.OpenFile(path); err != nil {
			t.Fatal(err)
		}
		g.CommitActiveTab()
	}
	g.SwitchToTabByPath(source)
	return g, source, target
}

func TestSaveAsRejectsAnotherOpenFileBeforeChangingState(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alias string
	}{
		{name: "dirty target"},
		{name: "alternate target path", alias: "dot"},
		{name: "case-insensitive target path", alias: "case"},
		{name: "deleted case-insensitive target path", alias: "missing-case"},
		{name: "symlink to target", alias: "symlink"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, source, target := newSaveAsGroup(t, "source  ", false)
			dir := filepath.Dir(target)
			var errMsg string
			g.OnError = func(msg string) { errMsg = msg }
			g.SwitchToTabByPath(target)
			g.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
			targetBuf := g.ActiveBuffer()
			targetLines := slices.Clone(targetBuf.Lines)
			g.SwitchToTabByPath(source)
			g.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'Y'})
			sourceBuf := g.ActiveBuffer()
			sourceLines, sourceVersion := slices.Clone(sourceBuf.Lines), sourceBuf.Version()
			g.InsertFinalNewline, g.TrimTrailingWhitespace = true, true

			savePath := target
			switch tc.alias {
			case "dot":
				savePath = filepath.ToSlash(dir) + "/./" + filepath.Base(target)
			case "case", "missing-case":
				savePath = filepath.Join(dir, strings.ToUpper(filepath.Base(target)))
				if _, err := os.Stat(savePath); err != nil {
					t.Skip("filesystem is case-sensitive")
				}
				if tc.alias == "missing-case" {
					if err := os.Remove(target); err != nil {
						t.Fatal(err)
					}
				}
			case "symlink":
				savePath = filepath.Join(dir, "target-link.go")
				if err := os.Symlink(target, savePath); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if g.SaveAs(savePath) {
				t.Fatal("SaveAs succeeded for a file open in another tab")
			}
			if !strings.Contains(errMsg, "already open") {
				t.Fatalf("error = %q, want an explanation of the open-file conflict", errMsg)
			}
			if g.ActiveFilePath() != source || g.ActiveBuffer() != sourceBuf || len(g.tabs) != 2 {
				t.Fatal("rejected SaveAs changed the active document or tabs")
			}
			if !slices.Equal(sourceBuf.Lines, sourceLines) || sourceBuf.Version() != sourceVersion || !sourceBuf.Dirty {
				t.Fatal("rejected SaveAs changed the source buffer")
			}
			if sourceBuf.InsertFinalNewline || sourceBuf.TrimTrailingWhitespace {
				t.Fatal("rejected SaveAs changed the source save settings")
			}
			if !slices.Equal(targetBuf.Lines, targetLines) || !targetBuf.Dirty {
				t.Fatal("rejected SaveAs changed the target buffer")
			}
			for path, want := range map[string]string{source: "source  ", target: "target"} {
				data, err := os.ReadFile(path)
				if path == target && tc.alias == "missing-case" && errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil || string(data) != want {
					t.Fatalf("disk content at %s = %q, %v; want %q", path, data, err, want)
				}
			}
			g.Undo()
			if sourceBuf.Lines[0] != "source  " || sourceBuf.Dirty {
				t.Fatal("rejected SaveAs changed the source undo history or save point")
			}
		})
	}
}

func TestSaveAsReplacesCleanOpenTarget(t *testing.T) {
	for _, tc := range []struct {
		name        string
		targetFirst bool
		failWrite   bool
		pinned      bool
		duplicates  int
	}{
		{name: "target after source"},
		{name: "target before source", targetFirst: true},
		{name: "failed write", failWrite: true},
		{name: "pinned target", pinned: true},
		{name: "multiple old target tabs", targetFirst: true, duplicates: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, source, target := newSaveAsGroup(t, "source", tc.targetFirst)
			dir := filepath.Dir(target)
			for i := 0; i < tc.duplicates; i++ {
				if err := g.OpenFile(filepath.ToSlash(dir) + "/" + strings.Repeat("./", i+1) + "target.go"); err != nil {
					t.Fatal(err)
				}
				g.activeTab().FilePath = target
				g.CommitActiveTab()
			}
			if tc.pinned {
				g.SwitchToTabByPath(target)
				g.TogglePinTab()
			}
			g.SwitchToTabByPath(target)
			g.Editor.SetBookmark(0, 'T', term.StyleDefault)
			g.SetDecorations("save-as", target, []Decoration{tint(0, term.StyleDiffDeleted)})
			g.SwitchToTabByPath(source)
			g.Editor.SetBookmark(0, 'S', term.StyleDefault)
			g.SetDecorations("save-as", source, []Decoration{tint(0, term.StyleDiffAdded)})
			g.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
			buf, stack := g.ActiveBuffer(), g.Editor.Undo
			if tc.failWrite {
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
				if g.SaveAs(target) || len(g.tabs) != 2 || g.ActiveBuffer() != buf || g.ActiveFilePath() != source {
					t.Fatal("failed SaveAs removed a tab or changed the source document")
				}
				return
			}
			g.OnFileClose = func(path, _ string) { t.Errorf("SaveAs closed a document that stays open: %s", path) }
			if !g.SaveAs(target) {
				t.Fatal("SaveAs failed for a clean open target")
			}
			if len(g.tabs) != 1 || g.ActiveFilePath() != target || g.ActiveBuffer() != buf || g.Editor.Undo != stack {
				t.Fatal("SaveAs did not retain the source as the single target document")
			}
			if g.pinnedCount != 0 {
				t.Fatal("SaveAs retained the removed target's pin")
			}
			if mark, ok := g.Editor.GetBookmark(0); !ok || mark.Icon != 'S' {
				t.Fatal("SaveAs did not retain the source's bookmarks")
			}
			if g.Editor.DiffOverlay == nil || g.Editor.DiffOverlay.LineBg[0] != term.StyleDiffAdded {
				t.Fatal("SaveAs did not retain the source's decorations")
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != "Xsource" {
				t.Fatalf("target = %q, %v; want Xsource", data, err)
			}
			g.Undo()
			if buf.Lines[0] != "source" || !buf.Dirty {
				t.Fatal("SaveAs lost the source undo history")
			}
		})
	}
}

func TestSaveAsKeepsIndependentOpenFiles(t *testing.T) {
	for _, tc := range []struct {
		name        string
		dirty       bool
		caseVariant bool
	}{
		{name: "clean hard link"},
		{name: "dirty hard link", dirty: true},
		{name: "clean case-sensitive file", caseVariant: true},
		{name: "dirty case-sensitive file", dirty: true, caseVariant: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, source, own := newSaveAsGroup(t, "replacement", false)
			dir := filepath.Dir(own)
			alias := filepath.Join(dir, "alias.go")
			if tc.caseVariant {
				alias = filepath.Join(dir, strings.ToUpper(filepath.Base(own)))
				if _, err := os.Stat(alias); err == nil {
					t.Skip("filesystem is case-insensitive")
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				if err := os.WriteFile(alias, []byte("alias"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Link(own, alias); err != nil {
				t.Skipf("hard links unavailable: %v", err)
			}
			g.SwitchToTabByPath(own)
			if tc.dirty {
				g.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
			}
			ownBuf := g.ActiveBuffer()
			ownLines := slices.Clone(ownBuf.Lines)
			g.SwitchToTabByPath(source)
			if !g.SaveAs(alias) {
				t.Fatal("SaveAs rejected an independent save target")
			}
			if len(g.tabs) != 2 || g.BufferForPath(own) != ownBuf || ownBuf.Dirty != tc.dirty || !slices.Equal(ownBuf.Lines, ownLines) {
				t.Fatal("SaveAs changed an independent open document")
			}
			for path, want := range map[string]string{own: "target", alias: "replacement"} {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != want {
					t.Fatalf("%s = %q, %v; want %q", path, data, err, want)
				}
			}
		})
	}
}
