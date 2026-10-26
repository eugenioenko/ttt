package plugin

import (
	"fmt"
	"testing"
)

type mockEditorAPI struct {
	bufText    string
	bufLines   []string
	curLine    string
	cursorLine int
	cursorCol  int
	selActive  bool
	selSL      int
	selSC      int
	selEL      int
	selEC      int
	selText    string
	filePath   string
	fileName   string
	language   string

	insertedLine int
	insertedCol  int
	insertedText string

	replaceSL   int
	replaceSC   int
	replaceEL   int
	replaceEC   int
	replaceText string

	setCursorLine int
	setCursorCol  int

	setSelSL int
	setSelSC int
	setSelEL int
	setSelEC int

	selCleared   bool
	extraCursors []CursorPosition

	searchPattern string
	searchRegex   bool
	searchCleared bool
	diffHunks     []DiffHunkTarget
	diffHunkAt    int
	wentToHunk    int
}

func (m *mockEditorAPI) BufferText() string { return m.bufText }
func (m *mockEditorAPI) BufferLines() []string {
	result := make([]string, len(m.bufLines))
	copy(result, m.bufLines)
	return result
}
func (m *mockEditorAPI) CurrentLine() string { return m.curLine }
func (m *mockEditorAPI) GetLine(n int) string {
	if n < 0 || n >= len(m.bufLines) {
		return ""
	}
	return m.bufLines[n]
}
func (m *mockEditorAPI) LineCount() int { return len(m.bufLines) }
func (m *mockEditorAPI) CursorPos() (int, int) {
	return m.cursorLine, m.cursorCol
}
func (m *mockEditorAPI) Selection() (bool, int, int, int, int) {
	return m.selActive, m.selSL, m.selSC, m.selEL, m.selEC
}
func (m *mockEditorAPI) SelectionText() string     { return m.selText }
func (m *mockEditorAPI) FilePath() string          { return m.filePath }
func (m *mockEditorAPI) FileName() string          { return m.fileName }
func (m *mockEditorAPI) Language() string          { return m.language }
func (m *mockEditorAPI) Viewport() (int, int, int) { return 0, 24, 25 }
func (m *mockEditorAPI) ScrollTo(line int)         {}
func (m *mockEditorAPI) ScrollBy(delta int)        {}
func (m *mockEditorAPI) SetLine(line int, text string) {
	if line >= 0 && line < len(m.bufLines) {
		m.bufLines[line] = text
	}
}
func (m *mockEditorAPI) Insert(line, col int, text string) {
	m.insertedLine = line
	m.insertedCol = col
	m.insertedText = text
}
func (m *mockEditorAPI) Replace(sl, sc, el, ec int, text string) {
	m.replaceSL = sl
	m.replaceSC = sc
	m.replaceEL = el
	m.replaceEC = ec
	m.replaceText = text
}
func (m *mockEditorAPI) SetCursor(line, col int) {
	m.setCursorLine = line
	m.setCursorCol = col
}
func (m *mockEditorAPI) SetSelection(sl, sc, el, ec int) {
	m.setSelSL = sl
	m.setSelSC = sc
	m.setSelEL = el
	m.setSelEC = ec
}
func (m *mockEditorAPI) ClearSelection() {
	m.selCleared = true
}
func (m *mockEditorAPI) BeginUndoGroup() {}
func (m *mockEditorAPI) EndUndoGroup()   {}
func (m *mockEditorAPI) AddCursor(line, col int) {
	m.extraCursors = append(m.extraCursors, CursorPosition{Line: line, Col: col})
}
func (m *mockEditorAPI) GetCursors() []CursorPosition {
	result := []CursorPosition{{Line: m.cursorLine, Col: m.cursorCol}}
	result = append(result, m.extraCursors...)
	return result
}
func (m *mockEditorAPI) ClearCursors() {
	m.extraCursors = nil
}
func (m *mockEditorAPI) SetSearch(pattern string, useRegex bool) {
	m.searchPattern = pattern
	m.searchRegex = useRegex
}
func (m *mockEditorAPI) ClearSearch() {
	m.searchCleared = true
}

func (m *mockEditorAPI) DiffHunks() []DiffHunkTarget { return m.diffHunks }
func (m *mockEditorAPI) DiffHunkAt() int             { return m.diffHunkAt }
func (m *mockEditorAPI) GoToDiffHunk(index int) error {
	if index < 1 || index > len(m.diffHunks) {
		return fmt.Errorf("hunk %d out of range", index)
	}
	m.wentToHunk = index
	return nil
}

func setupTestPluginWithEditor(perms PermissionSet, editor *mockEditorAPI) (*Plugin, func()) {
	p, cleanup := newTestPluginBase(perms)
	p.Editor = editor
	return p, cleanup
}

func TestEditorReadBufferText(t *testing.T) {
	mock := &mockEditorAPI{bufText: "hello\nworld"}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		_G.result = editor.buffer_text()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	v := p.State.GetGlobal("result")
	if v.String() != "hello\nworld" {
		t.Errorf("expected 'hello\\nworld', got %q", v.String())
	}
}

func TestEditorReadBufferLines(t *testing.T) {
	mock := &mockEditorAPI{bufLines: []string{"line1", "line2", "line3"}}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		local lines = editor.buffer_lines()
		_G.count = #lines
		_G.first = lines[1]
		_G.last = lines[3]
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("count").String() != "3" {
		t.Errorf("expected 3 lines, got %s", p.State.GetGlobal("count").String())
	}
	if p.State.GetGlobal("first").String() != "line1" {
		t.Errorf("expected 'line1', got %q", p.State.GetGlobal("first").String())
	}
}

func TestEditorReadCursor(t *testing.T) {
	mock := &mockEditorAPI{cursorLine: 5, cursorCol: 10}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		local pos = editor.cursor()
		_G.line = pos.line
		_G.col = pos.col
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("line").String() != "6" {
		t.Errorf("expected line 6 (1-based), got %s", p.State.GetGlobal("line").String())
	}
	if p.State.GetGlobal("col").String() != "11" {
		t.Errorf("expected col 11 (1-based), got %s", p.State.GetGlobal("col").String())
	}
}

func TestEditorReadFilePath(t *testing.T) {
	mock := &mockEditorAPI{filePath: "/tmp/test.go", fileName: "test.go", language: "go"}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		_G.path = editor.file_path()
		_G.name = editor.file_name()
		_G.lang = editor.language()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("path").String() != "/tmp/test.go" {
		t.Errorf("expected '/tmp/test.go', got %q", p.State.GetGlobal("path").String())
	}
	if p.State.GetGlobal("name").String() != "test.go" {
		t.Errorf("expected 'test.go', got %q", p.State.GetGlobal("name").String())
	}
	if p.State.GetGlobal("lang").String() != "go" {
		t.Errorf("expected 'go', got %q", p.State.GetGlobal("lang").String())
	}
}

func TestEditorWriteInsert(t *testing.T) {
	mock := &mockEditorAPI{}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorWrite: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		editor.insert(3, 5, "hello")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.insertedLine != 2 || mock.insertedCol != 4 {
		t.Errorf("expected insert at (2,4) (0-based), got (%d,%d)", mock.insertedLine, mock.insertedCol)
	}
	if mock.insertedText != "hello" {
		t.Errorf("expected text 'hello', got %q", mock.insertedText)
	}
}

func TestEditorWriteReplace(t *testing.T) {
	mock := &mockEditorAPI{}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorWrite: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		editor.replace(1, 1, 3, 5, "replaced")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.replaceSL != 0 || mock.replaceSC != 0 || mock.replaceEL != 2 || mock.replaceEC != 4 {
		t.Errorf("unexpected replace range: (%d,%d)-(%d,%d)", mock.replaceSL, mock.replaceSC, mock.replaceEL, mock.replaceEC)
	}
	if mock.replaceText != "replaced" {
		t.Errorf("expected 'replaced', got %q", mock.replaceText)
	}
}

func TestEditorWriteSetCursor(t *testing.T) {
	mock := &mockEditorAPI{}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorWrite: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		editor.set_cursor(10, 20)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.setCursorLine != 9 || mock.setCursorCol != 19 {
		t.Errorf("expected cursor at (9,19), got (%d,%d)", mock.setCursorLine, mock.setCursorCol)
	}
}

func TestEditorReadWithoutPermission(t *testing.T) {
	mock := &mockEditorAPI{bufText: "secret"}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		editor.buffer_text()
	`)
	if err == nil {
		t.Fatal("expected error when editor.read not granted")
	}
}

func TestEditorWriteWithoutPermission(t *testing.T) {
	mock := &mockEditorAPI{}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		editor.insert(1, 1, "hack")
	`)
	if err == nil {
		t.Fatal("expected error when editor.write not granted")
	}
}

func TestEditorSelection(t *testing.T) {
	mock := &mockEditorAPI{
		selActive: true,
		selSL:     1, selSC: 2,
		selEL: 3, selEC: 4,
		selText: "selected text",
	}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		local sel = editor.selection()
		_G.active = sel.active
		_G.sl = sel.start_line
		_G.sc = sel.start_col
		_G.text = editor.selection_text()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("active").String() != "true" {
		t.Error("expected active selection")
	}
	if p.State.GetGlobal("text").String() != "selected text" {
		t.Errorf("expected 'selected text', got %q", p.State.GetGlobal("text").String())
	}
}

func TestEditorSetSearch(t *testing.T) {
	mock := &mockEditorAPI{}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorWrite: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		editor.set_search("hello")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.searchPattern != "hello" {
		t.Errorf("expected pattern 'hello', got %q", mock.searchPattern)
	}
	if mock.searchRegex {
		t.Error("expected useRegex false")
	}
}

func TestEditorSetSearchRegex(t *testing.T) {
	mock := &mockEditorAPI{}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorWrite: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		editor.set_search("\\bfoo\\b", true)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.searchPattern != `\bfoo\b` {
		t.Errorf("expected pattern '\\bfoo\\b', got %q", mock.searchPattern)
	}
	if !mock.searchRegex {
		t.Error("expected useRegex true")
	}
}

func TestEditorClearSearch(t *testing.T) {
	mock := &mockEditorAPI{}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorWrite: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		editor.clear_search()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !mock.searchCleared {
		t.Error("expected ClearSearch to be called")
	}
}

func TestEditorDiffReportsHunksAndPosition(t *testing.T) {
	mock := &mockEditorAPI{
		diffHunks: []DiffHunkTarget{
			{FilePath: "a.go", Kind: "modified"},
			{FilePath: "b.go", Kind: "deleted"},
		},
		diffHunkAt: 2,
	}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		local d = editor.diff()
		assert(d, "expected a diff table")
		assert(#d.hunks == 2, "expected 2 hunks, got " .. #d.hunks)
		assert(d.hunks[1].file_path == "a.go", "hunk 1 path: " .. tostring(d.hunks[1].file_path))
		assert(d.hunks[1].kind == "modified", "hunk 1 kind: " .. tostring(d.hunks[1].kind))
		assert(d.hunks[2].file_path == "b.go", "hunk 2 path: " .. tostring(d.hunks[2].file_path))
		assert(d.hunks[2].kind == "deleted", "hunk 2 kind: " .. tostring(d.hunks[2].kind))
		assert(d.current_hunk == 2, "current_hunk: " .. tostring(d.current_hunk))
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

// Outside a diff the API must say so with nil rather than an empty table, so a
// plugin can tell "no diff here" from "a diff with no changes".
func TestEditorDiffIsNilOutsideADiff(t *testing.T) {
	mock := &mockEditorAPI{}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		assert(editor.diff() == nil, "expected nil outside a diff")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestEditorGoToDiffHunkMovesAndReportsErrors(t *testing.T) {
	mock := &mockEditorAPI{
		diffHunks: []DiffHunkTarget{{FilePath: "a.go", Kind: "added"}},
	}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true, EditorWrite: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		local ok = editor.go_to_diff_hunk(1)
		assert(ok == true, "expected success")
		local _, err = editor.go_to_diff_hunk(9)
		assert(err ~= nil, "expected an error for an out-of-range hunk")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.wentToHunk != 1 {
		t.Errorf("went to hunk %d, want 1", mock.wentToHunk)
	}
}

// Navigation is a write-tier capability, matching the existing cursor calls.
func TestEditorGoToDiffHunkRequiresWritePermission(t *testing.T) {
	mock := &mockEditorAPI{diffHunks: []DiffHunkTarget{{FilePath: "a.go", Kind: "added"}}}
	p, cleanup := setupTestPluginWithEditor(PermissionSet{EditorRead: true}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local editor = require("ttt.editor")
		assert(editor.go_to_diff_hunk == nil, "go_to_diff_hunk must need editor.write")
		assert(editor.diff ~= nil, "diff must need only editor.read")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}
