package plugin

import (
	"context"
	"net/url"

	"github.com/eugenioenko/ttt/internal/term"
)

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

// DecorationItem is a line tint, text highlight or note set by a plugin, in
// 0-based lines and rune columns. Without Text it tints whole lines
// StartLine..EndLine; with Text it highlights from StartCol on StartLine to
// EndCol (exclusive) on EndLine. With Note it draws the note under
// StartLine.
type DecorationItem struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Text      bool
	Note      string
	Style     term.Style
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

// TerminalInfo is one terminal tab as a plugin sees it.
type TerminalInfo struct {
	ID    string
	Label string
	Cols  int
	Rows  int
}

// TerminalAPI is the host's terminal surface for plugins. Spawn/send/focus are
// gated by `terminal.control`, read/list by `terminal.read`; every call runs on
// the main thread.
type TerminalAPI interface {
	// TerminalSpawn opens a terminal tab for a program and returns its id. An
	// empty shell inherits the configured terminal program; a named shell uses
	// exactly the args given.
	TerminalSpawn(shell string, args, env []string, cwd, label string) (string, error)
	// TerminalSend writes text to a tab's PTY (a trailing newline is the
	// caller's job).
	TerminalSend(id, text string) error
	// TerminalTail returns the last maxBytes of the tab's raw PTY output
	// (ANSI escapes included; maxBytes <= 0 means the whole tail).
	TerminalTail(id string, maxBytes int) (string, error)
	// TerminalScreen returns the visible screen as plain lines (no ANSI,
	// trailing blanks trimmed).
	TerminalScreen(id string) (string, error)
	// TerminalFocus reveals the panel and selects the tab.
	TerminalFocus(id string) error
	// TerminalClose kills the tab's PTY and removes the tab.
	TerminalClose(id string) error
	TerminalList() []TerminalInfo
}

type ExecOptions struct {
	// Stdin is written to the child's standard input; nothing is written when empty.
	Stdin string
	// Env holds KEY=VALUE entries added over the editor's environment.
	Env []string
}

type SystemAPI interface {
	// Exec runs binary with args and kills the child when ctx is done.
	Exec(ctx context.Context, binary string, args []string, opts ExecOptions) (stdout, stderr string, exitCode int, err error)
	Env(name string) string
}

type NetworkAPI interface {
	Get(url string, headers map[string]string) (status int, body string, respHeaders map[string]string, err error)
	Post(url string, headers map[string]string, body string) (status int, respBody string, respHeaders map[string]string, err error)
}

// HostCheckedNetwork is a NetworkAPI that can hold redirects to a plugin's
// network.http hosts. The manager gives each plugin its own copy.
type HostCheckedNetwork interface {
	NetworkAPI
	ForPlugin(allow func(*url.URL) error) NetworkAPI
}

type SettingsAPI interface {
	Get(key string) (any, bool)
	Set(key string, value any) error
}

type PluginAsyncResult struct {
	Plugin   *Plugin
	Callback func()
}
