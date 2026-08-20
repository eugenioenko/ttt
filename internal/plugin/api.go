package plugin

import "github.com/eugenioenko/ttt/internal/term"

// DiagnosticItem is a single editor diagnostic/decoration published by a
// plugin. Coordinates are 0-based (converted from 1-based Lua on the binding
// boundary). Severity mirrors the LSP severities (1=error .. 4=hint). A
// non-zero Style overrides the severity's default squiggle color.
type DiagnosticItem struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Severity  int
	Style     term.Style
	Message   string
	Source    string
}

// BookmarkItem is one entry of a bulk bookmarks.set_all() call.
type BookmarkItem struct {
	Line  int
	Icon  rune
	Style term.Style
}

// ContextMenuEntry is one item contributed by a plugin to the editor's
// right-click context menu. A Separator entry renders as a divider; otherwise
// OnSelect is invoked (on the main thread) when the item is chosen.
type ContextMenuEntry struct {
	Label     string
	Separator bool
	OnSelect  func()
}

type EditorAPI interface {
	BufferText() string
	BufferLines() []string
	CurrentLine() string
	GetLine(n int) string
	LineCount() int
	CursorPos() (line, col int)
	Selection() (active bool, startLine, startCol, endLine, endCol int)
	SelectionText() string
	FilePath() string
	FileName() string
	Language() string

	Viewport() (topLine, bottomLine, height int)
	ScrollTo(line int)
	ScrollBy(delta int)

	Insert(line, col int, text string)
	SetLine(line int, text string)
	Replace(startLine, startCol, endLine, endCol int, text string)
	SetCursor(line, col int)
	SetSelection(startLine, startCol, endLine, endCol int)
	ClearSelection()
	BeginUndoGroup()
	EndUndoGroup()

	AddCursor(line, col int)
	GetCursors() []CursorPosition
	ClearCursors()

	SetSearch(pattern string, useRegex bool)
	ClearSearch()
}

type CursorPosition struct {
	Line int
	Col  int
}

type FileEntry struct {
	Name  string
	IsDir bool
}

type FilesystemAPI interface {
	ReadFile(path string) (string, error)
	WriteFile(path, content string) error
	FileExists(path string) bool
	ListDir(path string) ([]FileEntry, error)
}

type SystemAPI interface {
	// Exec runs binary with args, writing stdin to the child's standard input
	// (nothing is written when stdin is empty).
	Exec(binary string, args []string, stdin string) (stdout, stderr string, exitCode int, err error)
	Env(name string) string
}

type NetworkAPI interface {
	Get(url string, headers map[string]string) (status int, body string, respHeaders map[string]string, err error)
	Post(url string, headers map[string]string, body string) (status int, respBody string, respHeaders map[string]string, err error)
}

type SettingsAPI interface {
	Get(key string) (any, bool)
	Set(key string, value any) error
}

type PluginAsyncResult struct {
	Plugin   *Plugin
	Callback func()
}
