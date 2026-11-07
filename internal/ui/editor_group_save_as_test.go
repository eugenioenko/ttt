package ui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/undo"
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

func TestSaveAsRejectsDirtyTargetWithoutChanges(t *testing.T) {
	g, source, target := newSaveAsGroup(t, "source  ", false)
	g.SwitchToTabByPath(target)
	g.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
	targetBuf := g.ActiveBuffer()
	g.SwitchToTabByPath(source)
	g.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'Y'})
	buf, version := g.ActiveBuffer(), g.ActiveBuffer().Version()
	g.InsertFinalNewline, g.TrimTrailingWhitespace = true, true
	if g.SaveAs(target) {
		t.Fatal("SaveAs overwrote a dirty open target")
	}
	if g.ActiveFilePath() != source || g.ActiveBuffer() != buf || len(g.tabs) != 2 {
		t.Fatal("rejected SaveAs changed the active document or tabs")
	}
	if strings.Join(buf.Lines, "\n") != "Ysource  " || buf.Version() != version || !buf.Dirty || buf.InsertFinalNewline || buf.TrimTrailingWhitespace {
		t.Fatal("rejected SaveAs changed the source buffer or save settings")
	}
	if targetBuf.Lines[0] != "Xtarget" || !targetBuf.Dirty {
		t.Fatal("rejected SaveAs changed the target buffer")
	}
	for path, want := range map[string]string{source: "source  ", target: "target"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatalf("disk content = %q, %v; want %q", data, err, want)
		}
	}
	g.Undo()
	if buf.Lines[0] != "source  " || buf.Dirty {
		t.Fatal("rejected SaveAs changed the undo history or save point")
	}
}

func TestSaveAsReplacesCleanOpenTarget(t *testing.T) {
	for _, kind := range []string{"target after source", "target before source", "pinned duplicates", "failed write"} {
		t.Run(kind, func(t *testing.T) {
			g, source, target := newSaveAsGroup(t, "source", kind != "target after source")
			if kind == "pinned duplicates" {
				for i := 1; i <= 2; i++ {
					if err := g.OpenFile(filepath.ToSlash(filepath.Dir(target)) + "/" + strings.Repeat("./", i) + "target.go"); err != nil {
						t.Fatal(err)
					}
					g.activeTab().FilePath = target
					g.CommitActiveTab()
				}
				g.SwitchToTabByPath(target)
				g.TogglePinTab()
				g.SwitchToTabByPath(source)
			}
			g.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
			buf, stack := g.ActiveBuffer(), g.Editor.Undo
			if kind == "failed write" {
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
			if len(g.tabs) != 1 || g.ActiveFilePath() != target || g.ActiveBuffer() != buf || g.Editor.Undo != stack || g.pinnedCount != 0 {
				t.Fatal("SaveAs did not retain the source as the single target document")
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

func TestSaveAsIgnoresUnrelatedMissingParent(t *testing.T) {
	for _, kind := range []string{"new destination", "open destination"} {
		t.Run(kind, func(t *testing.T) {
			g, source, target := newSaveAsGroup(t, "source", false)
			stale := filepath.Join(t.TempDir(), "deleted", "other.go")
			if err := g.OpenFile(stale); err != nil {
				t.Fatal(err)
			}
			g.CommitActiveTab()
			g.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
			buf := g.ActiveBuffer()
			g.SwitchToTabByPath(source)
			if kind == "new destination" {
				target = filepath.Join(filepath.Dir(source), "new.go")
			}
			if !g.SaveAs(target) {
				t.Fatal("unrelated stale tab blocked SaveAs")
			}
			if g.BufferForPath(stale) != buf || !buf.Dirty || buf.Lines[0] != "X" {
				t.Fatal("SaveAs changed the unrelated stale document")
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != "source" {
				t.Fatalf("destination = %q, %v; want source", data, err)
			}
		})
	}
}

func TestSameSaveTarget(t *testing.T) {
	dir := t.TempDir()
	target, other := filepath.Join(dir, "target"), filepath.Join(dir, "other")
	for _, path := range []string{target, other} {
		if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"parent", "nested", "case"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	upper := filepath.Join(dir, "TARGET")
	_, err := os.Stat(upper)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	insensitive := err == nil
	caseTarget := filepath.Join(dir, "case", "target")
	if err := os.WriteFile(caseTarget, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	upper = filepath.Join(dir, "case", "TARGET")
	var caseLinkErr error
	if !insensitive {
		caseLinkErr = os.Link(caseTarget, upper)
	}
	if err := os.WriteFile(filepath.Join(dir, "OTHER"), []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	hardLinkErr := os.Link(target, alias)
	link, dangling, loop := filepath.Join(dir, "link"), filepath.Join(dir, "dangling"), filepath.Join(dir, "loop")
	fileLinkErr := os.Symlink(target, link)
	danglingErr := os.Symlink(filepath.Join(dir, "missing"), dangling)
	loopErr := os.Symlink(loop, loop)
	parentLink, nestedLink := filepath.Join(dir, "parent-link"), filepath.Join(dir, "nested-link")
	parentLinkErr := os.Symlink(dir, parentLink)
	nestedLinkErr := os.Symlink(filepath.Join(dir, "nested"), nestedLink)
	for _, tc := range []struct {
		name, a, b    string
		want, wantErr bool
		setupErr      error
	}{
		{"same", target, target, true, false, nil},
		{"dot", target, filepath.ToSlash(dir) + "/./target", true, false, nil},
		{"case alias or hard link", caseTarget, upper, insensitive, false, caseLinkErr},
		{"case-distinct files", other, filepath.Join(dir, "OTHER"), insensitive, false, nil},
		{"deleted case alias", filepath.Join(dir, "missing"), filepath.Join(dir, "MISSING"), insensitive, false, nil},
		{"hard link", target, alias, false, false, hardLinkErr},
		{"file symlink", target, link, true, false, fileLinkErr},
		{"parent symlink", target, filepath.Join(parentLink, "target"), true, false, parentLinkErr},
		{"deleted parent alias", filepath.Join(dir, "missing"), filepath.Join(parentLink, "missing"), true, false, parentLinkErr},
		{"link/..", target, nestedLink + "/../target", true, false, nestedLinkErr},
		{"deleted link/..", filepath.Join(dir, "missing"), nestedLink + "/../missing", true, false, nestedLinkErr},
		{"case parent", filepath.Join(dir, "parent", "123"), filepath.Join(dir, "PARENT", "123"), true, false, nil},
		{"empty case", filepath.Join(dir, "parent", "target"), filepath.Join(dir, "parent", "TARGET"), false, true, nil},
		{"missing parent", target, filepath.Join(dir, "gone", "target"), false, false, nil},
		{"invalid parent", target, filepath.Join(target, "child"), false, false, nil},
		{"dangling symlink", target, dangling, false, true, danglingErr},
		{"missing parent versus dangling symlink", filepath.Join(dir, "gone", "target"), dangling, false, false, danglingErr},
		{"symlink loop", target, loop, false, true, loopErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupErr != nil {
				t.Skipf("filesystem capability unavailable: %v", tc.setupErr)
			}
			if tc.name == "case parent" && !insensitive {
				t.Skip("parent path is case-sensitive")
			}
			for _, paths := range [][2]string{{tc.a, tc.b}, {tc.b, tc.a}} {
				same, err := sameSaveTarget(paths[0], paths[1])
				if (err != nil) != tc.wantErr || (!tc.wantErr && same != tc.want) {
					t.Fatalf("sameSaveTarget(%q, %q) = %v, %v; want match=%v, error=%v", paths[0], paths[1], same, err, tc.want, tc.wantErr)
				}
			}
		})
	}
}
