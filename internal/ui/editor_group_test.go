package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/fileicons"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestEditorGroupTabIconUsesFilePathInsteadOfCustomTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dockerfile")
	group := NewEditorGroupWidget(nil, 4, false, "extended")
	group.TabBar.Icons = config.IconsNerdFont
	group.OpenDiffTab(path+" (diff)", "custom tab title", path, diff.FileDiff{}, nil, nil, false)
	group.syncTabs()

	group.TabBar.SetRect(Rect{W: 80, H: 3})
	grid := makeGrid(80, 3)
	group.TabBar.Render(NewRenderSurface(grid, Rect{W: 80, H: 3}))
	want := []rune(fileicons.ForFile("Dockerfile").Glyph)[0]
	for _, cell := range grid[1] {
		if cell.Ch == want && cell.Style == term.StyleFileIconBlue {
			return
		}
	}
	t.Fatalf("diff-tab icon for underlying path %q missing:\n%+v", path, group.TabBar.Tabs)
}

func TestEditorGroupDiffSearchSourceKeepsFileIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dockerfile")
	group := NewEditorGroupWidget(nil, 4, false, "extended")
	group.OpenDiffTab(path+" (diff)", "custom title", path, diff.FileDiff{}, nil, nil, false)

	sources := group.DiffTabSources()
	if len(sources) != 1 {
		t.Fatalf("diff sources = %d, want 1", len(sources))
	}
	if sources[0].TabName != path+" (diff)" || sources[0].IconPath != path {
		t.Fatalf("diff source = %+v, want display ID and underlying file path", sources[0])
	}
}

func TestEditorGroupShebangHighlighting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy")
	text := "#!/usr/bin/env python3\nprint(\"deploy\")\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"open", "readonly", "buffer", "save-as", "rename", "refresh", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			g := NewEditorGroupWidget(nil, 4, false, "extended")
			g.SyntaxHighlight = mode != "disabled"
			switch mode {
			case "readonly":
				g.OpenFileReadOnly(path, "deploy")
			case "buffer":
				g.OpenBufferReadOnly("deploy", path, []string{"#!/usr/bin/env python3", `print("deploy")`})
			default:
				g.OpenFile(path)
			}
			switch mode {
			case "save-as":
				if !g.SaveAs(filepath.Join(t.TempDir(), "renamed")) {
					t.Fatal("SaveAs failed")
				}
			case "rename":
				g.RenamePath(path, filepath.Join(t.TempDir(), "renamed"))
			case "refresh":
				g.RefreshHighlighters()
			}
			h := g.Editor.Highlighter
			if mode == "disabled" {
				if h != nil {
					t.Fatal("highlighting should remain disabled")
				}
				return
			}
			if h == nil || h.Language() != "Python" {
				t.Fatalf("highlighter = %v, want Python", h)
			}
			for _, span := range h.HighlightLineAt(g.Editor.Buf.Lines, 1) {
				if span.Style == term.StyleSyntaxString {
					return
				}
			}
			t.Fatal("script string is not highlighted")
		})
	}
}

func TestOpenPluginTabReplacementClosesPreviousContent(t *testing.T) {
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	closed := false
	notified := ""
	group.OnContentTabClose = func(id string) { notified = id }
	first := NewCommitDetailWidget("/repo", "ref", "abcdef0", false)
	first.OnClose = func() { closed = true }
	group.OpenPluginTab("commit", "Commit", first)
	group.OpenPluginTab("commit", "Replacement", NewCommitDetailWidget("/repo", "other", "1234567", false))
	if !closed {
		t.Fatal("replacing a content tab did not close its previous incarnation")
	}
	if notified != "commit" {
		t.Fatalf("replacement close notification = %q, want commit", notified)
	}
}

func TestPluginTabKeyIsSeparateFromFilePath(t *testing.T) {
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	group.OpenBufferReadOnly("Lookalike", "plugin:p:a", []string{"text"})
	before := len(group.tabs)

	group.OpenPluginTab("plugin:p:a", "Page", NewCommitDetailWidget("/repo", "ref", "abcdef0", false))
	if len(group.tabs) != before+1 {
		t.Fatalf("OpenPluginTab reused the buffer at its key: tabs = %d, want %d", len(group.tabs), before+1)
	}
	if !group.ClosePluginTab("plugin:p:a") || len(group.tabs) != before {
		t.Fatalf("ClosePluginTab did not close the plugin tab: tabs = %d", len(group.tabs))
	}
	if group.ClosePluginTab("plugin:p:a") || group.ClosePluginTab("") {
		t.Fatal("ClosePluginTab closed a tab that has no plugin key")
	}
	if len(group.tabs) != before || group.tabs[len(group.tabs)-1].FilePath != "plugin:p:a" {
		t.Fatal("the read-only buffer was closed")
	}
}

func TestInitialTabIsVirtual(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	if !g.tabs[0].Virtual {
		t.Fatal("expected initial tab to be virtual")
	}
}

func TestNewFileAlwaysCreatesTab(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	if len(g.tabs) != 1 || g.tabs[0].FilePath != "untitled" {
		t.Fatal("expected initial untitled tab")
	}

	g.tabs[0].Buf.Lines = []string{"some content"}
	g.tabs[0].Buf.Dirty = true

	g.NewFile()
	if len(g.tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(g.tabs))
	}
	if g.tabs[1].FilePath != "untitled-2" {
		t.Errorf("expected 'untitled-2', got %q", g.tabs[1].FilePath)
	}
	if g.active != 1 {
		t.Errorf("expected active tab 1, got %d", g.active)
	}
}

func TestNewFileFromTextFillsTheStartupPlaceholder(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")

	g.NewFileFromText("one\r\ntwo\r\n")
	if len(g.tabs) != 1 || g.tabs[0].FilePath != "untitled" {
		t.Fatalf("tabs = %d, first %q; want the untitled placeholder filled", len(g.tabs), g.tabs[0].FilePath)
	}
	buf := g.tabs[0].Buf
	if !slices.Equal(buf.Lines, []string{"one", "two"}) || buf.LineEnding != "\r\n" || buf.Dirty {
		t.Fatalf("buffer = %q ending %q dirty %v, want [one two] CRLF clean", buf.Lines, buf.LineEnding, buf.Dirty)
	}
	if !g.tabs[0].Virtual {
		t.Fatal("piped text should stay unsaved (virtual)")
	}
}

func TestNewFileFromTextKeepsOpenTabs(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.tabs[0].Buf.Lines = []string{"typed"}
	g.tabs[0].Buf.Dirty = true

	g.NewFileFromText("piped\n")
	if len(g.tabs) != 2 || g.tabs[1].FilePath != "untitled-2" || g.active != 1 {
		t.Fatalf("tabs = %d, active %d; want a new active untitled-2 tab", len(g.tabs), g.active)
	}
	if g.tabs[0].Buf.Lines[0] != "typed" || g.tabs[1].Buf.Lines[0] != "piped" {
		t.Fatalf("tab contents = %q, %q", g.tabs[0].Buf.Lines, g.tabs[1].Buf.Lines)
	}
}

func TestNewFileSequentialNaming(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")

	g.NewFile()
	g.NewFile()
	g.NewFile()

	if len(g.tabs) != 4 {
		t.Fatalf("expected 4 tabs, got %d", len(g.tabs))
	}
	expected := []string{"untitled", "untitled-2", "untitled-3", "untitled-4"}
	for i, want := range expected {
		if g.tabs[i].FilePath != want {
			t.Errorf("tab %d: got %q, want %q", i, g.tabs[i].FilePath, want)
		}
	}
}

func TestNewFileReusesNameAfterClose(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.NewFile() // untitled-2

	g.SwitchTab(0)
	g.CloseTab()

	if g.tabs[0].FilePath != "untitled-2" {
		t.Fatalf("expected remaining tab 'untitled-2', got %q", g.tabs[0].FilePath)
	}

	g.NewFile()
	names := make(map[string]bool)
	for _, tab := range g.tabs {
		names[tab.FilePath] = true
	}
	if !names["untitled"] {
		t.Error("expected 'untitled' to be reused after close")
	}
}

func TestEditorGroupOpenFileNotFound(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	path := "/nonexistent/path/file.txt"
	g.OpenFile(path)

	if errMsg != "" {
		t.Fatalf("unexpected error for new file: %s", errMsg)
	}
	if g.tabs[g.active].FilePath != path {
		t.Fatalf("expected tab with path %q, got %q", path, g.tabs[g.active].FilePath)
	}
	if g.tabs[g.active].Buf.Lines[0] != "" {
		t.Fatal("expected empty buffer for new file")
	}
}

func TestEditorGroupOpenFileSuccess(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("hello\nworld"), 0644)

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.OpenFile(path)

	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if len(g.tabs) != 1 {
		t.Fatalf("expected 1 tab replacing untitled, got %d", len(g.tabs))
	}
	if g.tabs[g.active].FilePath != path {
		t.Fatalf("expected active tab to be %s, got %s", path, g.tabs[g.active].FilePath)
	}
}

func TestEditorGroupOpenFileKeepsTouchedUntitled(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.tabs[0].Buf.Lines = []string{"typed text"}
	g.tabs[0].Buf.Dirty = true

	g.OpenFile(path)

	if len(g.tabs) != 2 {
		t.Fatalf("expected 2 tabs when untitled is dirty, got %d", len(g.tabs))
	}
	if g.tabs[0].FilePath != "untitled" || g.tabs[1].FilePath != path {
		t.Fatalf("expected [untitled, %s], got [%s, %s]", path, g.tabs[0].FilePath, g.tabs[1].FilePath)
	}
}

func TestEditorGroupOpenFileKeepsNewFileTab(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.NewFile()
	g.SwitchTab(0)
	g.CloseTab()

	g.OpenFile(path)

	if len(g.tabs) != 2 {
		t.Fatalf("expected 2 tabs when user created new file, got %d", len(g.tabs))
	}
	if g.tabs[0].FilePath != "untitled-2" || g.tabs[1].FilePath != path {
		t.Fatalf("expected [untitled-2, %s], got [%s, %s]", path, g.tabs[0].FilePath, g.tabs[1].FilePath)
	}
}

func TestEditorGroupOpenFileKeepsPinnedPlaceholder(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.TogglePinTab()

	g.OpenFile(path)

	if len(g.tabs) != 2 {
		t.Fatalf("expected 2 tabs when placeholder is pinned, got %d", len(g.tabs))
	}
	if g.tabs[0].FilePath != "untitled" || g.tabs[1].FilePath != path {
		t.Fatalf("expected [untitled, %s], got [%s, %s]", path, g.tabs[0].FilePath, g.tabs[1].FilePath)
	}
	if g.pinnedCount != 1 {
		t.Fatalf("expected pinnedCount to remain 1, got %d", g.pinnedCount)
	}
	if g.IsActiveTabPinned() {
		t.Fatal("expected newly opened tab to not be pinned")
	}
}

func TestEditorGroupSaveError(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.tabs[0].FilePath = "/nonexistent/dir/file.txt"
	g.tabs[0].Virtual = false
	g.tabs[0].Buf.Lines = []string{"test"}

	ok := g.Save()
	if ok {
		t.Fatal("Save should return false on error")
	}
	if errMsg == "" {
		t.Fatal("expected OnError to be called for save failure")
	}
}

func TestEditorGroupSaveSuccess(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "out.txt")

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.tabs[0].FilePath = path
	g.tabs[0].Virtual = false
	g.tabs[0].Buf.Lines = []string{"saved content"}

	ok := g.Save()
	if !ok {
		t.Fatal("Save should return true on success")
	}
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "saved content" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestEditorGroupSaveAsError(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.SaveAs("/nonexistent/dir/file.txt")

	if errMsg == "" {
		t.Fatal("expected OnError to be called for save-as failure")
	}
	if g.tabs[0].FilePath != "untitled" {
		t.Fatal("FilePath should not change on failed SaveAs")
	}
}

func TestEditorGroupSaveAsSuccess(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "saved.go")

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.tabs[0].Buf.Lines = []string{"package main"}
	g.SaveAs(path)

	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if g.tabs[0].FilePath != path {
		t.Fatalf("expected FilePath to be %s, got %s", path, g.tabs[0].FilePath)
	}
	if g.tabs[0].Virtual {
		t.Fatal("SaveAs should clear Virtual flag")
	}
}

func TestEditorGroupSaveAsNewFileUsesSaveSettings(t *testing.T) {
	for _, tc := range []struct {
		name, editorconfig, want string
	}{
		{"settings", "", "hello\n"},
		{"editorconfig", "root = true\n[*]\ninsert_final_newline = false\n", "hello"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.editorconfig != "" {
				if err := os.WriteFile(filepath.Join(dir, ".editorconfig"), []byte(tc.editorconfig), 0644); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(dir, "new.txt")

			g := NewEditorGroupWidget(nil, 4, false, "extended")
			g.InsertFinalNewline = true
			g.TrimTrailingWhitespace = true
			g.NewFile()
			g.activeTab().Buf.Lines = []string{"hello  "}
			if !g.SaveAs(path) {
				t.Fatal("SaveAs failed")
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tc.want {
				t.Fatalf("saved %q, want %q", data, tc.want)
			}
		})
	}
}

func TestEditorGroupWordWrapMaxBytes(t *testing.T) {
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.txt")
	smallPath := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(largePath, []byte("01234567890123456789"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(smallPath, []byte("12345"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewEditorGroupWidget(nil, 4, false, "compact")
	g.WordWrap = true
	g.WordWrapMaxBytes = 10

	// 1. A file larger than the limit opens with g.Editor.WordWrap == false.
	g.OpenFile(largePath)
	g.CommitActiveTab()
	if g.Editor.WordWrap {
		t.Errorf("large file opened with WordWrap = true, want false")
	}
	if g.ActiveFileWordWrap() {
		t.Errorf("large file ActiveFileWordWrap() = true, want false")
	}

	// 2. A file at or under the limit opens with g.Editor.WordWrap == true.
	g.OpenFile(smallPath)
	g.CommitActiveTab()
	if !g.Editor.WordWrap {
		t.Errorf("small file opened with WordWrap = false, want true")
	}
	if !g.ActiveFileWordWrap() {
		t.Errorf("small file ActiveFileWordWrap() = false, want true")
	}

	// 3. Switching between those two tabs flips g.Editor.WordWrap each time.
	g.SwitchTab(0)
	if g.Editor.WordWrap {
		t.Errorf("switching to large file: WordWrap = true, want false")
	}
	g.SwitchTab(1)
	if !g.Editor.WordWrap {
		t.Errorf("switching back to small file: WordWrap = false, want true")
	}
	g.SwitchTab(0)
	if g.Editor.WordWrap {
		t.Errorf("switching to large file again: WordWrap = true, want false")
	}

	// 4. With g.WordWrapMaxBytes = 0, the large file wraps.
	g.WordWrapMaxBytes = 0
	g.syncTabs()
	if !g.Editor.WordWrap {
		t.Errorf("with WordWrapMaxBytes = 0, large file WordWrap = false, want true")
	}
	if !g.ActiveFileWordWrap() {
		t.Errorf("with WordWrapMaxBytes = 0, ActiveFileWordWrap() = false, want true")
	}

	// 5. With g.WordWrap = false, neither file wraps.
	g.WordWrap = false
	g.syncTabs()
	if g.Editor.WordWrap {
		t.Errorf("with WordWrap = false, large file WordWrap = true, want false")
	}
	if g.ActiveFileWordWrap() {
		t.Errorf("with WordWrap = false, large file ActiveFileWordWrap() = true, want false")
	}
	g.SwitchTab(1)
	if g.Editor.WordWrap {
		t.Errorf("with WordWrap = false, small file WordWrap = true, want false")
	}
	if g.ActiveFileWordWrap() {
		t.Errorf("with WordWrap = false, small file ActiveFileWordWrap() = true, want false")
	}

	// 6. Saving a file that grows past the limit disables wrap immediately.
	g.WordWrap = true
	g.WordWrapMaxBytes = 10
	g.syncTabs()
	if !g.Editor.WordWrap {
		t.Fatalf("expected small file to have WordWrap = true")
	}
	g.Editor.Buf.Lines = []string{"01234567890123456789"}
	if !g.Save() {
		t.Fatalf("Save() failed")
	}
	if g.Editor.WordWrap {
		t.Errorf("after saving large content: WordWrap = true, want false")
	}
}
