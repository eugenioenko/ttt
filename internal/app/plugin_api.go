package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
)

const maxResponseBody = 10 * 1024 * 1024 // 10 MB

// PluginEditorAPI implements plugin.EditorAPI using the App's editor state.
type PluginEditorAPI struct {
	eg *ui.EditorGroupWidget
}

func NewPluginEditorAPI(app *App) *PluginEditorAPI {
	return &PluginEditorAPI{eg: app.EditorGroup}
}

func (e *PluginEditorAPI) BufferText() string {
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return ""
	}
	return strings.Join(buf.Lines, "\n")
}

func (e *PluginEditorAPI) BufferLines() []string {
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return nil
	}
	result := make([]string, len(buf.Lines))
	copy(result, buf.Lines)
	return result
}

func (e *PluginEditorAPI) CurrentLine() string {
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return ""
	}
	line, _ := e.eg.ActiveCursor()
	if line >= 0 && line < len(buf.Lines) {
		return buf.Lines[line]
	}
	return ""
}

func (e *PluginEditorAPI) GetLine(n int) string {
	buf := e.eg.ActiveBuffer()
	if buf == nil || n < 0 || n >= len(buf.Lines) {
		return ""
	}
	return buf.Lines[n]
}

func (e *PluginEditorAPI) LineCount() int {
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return 0
	}
	return len(buf.Lines)
}

func (e *PluginEditorAPI) CursorPos() (int, int) {
	return e.eg.ActiveCursor()
}

func (e *PluginEditorAPI) Selection() (bool, int, int, int, int) {
	ed := e.eg.Editor
	if ed == nil || ed.Selection == nil || !ed.Selection.Active {
		return false, 0, 0, 0, 0
	}
	line, col := e.eg.ActiveCursor()
	start, end := ed.Selection.Range(line, col)
	return true, start.Line, start.Col, end.Line, end.Col
}

func (e *PluginEditorAPI) SelectionText() string {
	ed := e.eg.Editor
	if ed == nil || ed.Selection == nil || !ed.Selection.Active || ed.Buf == nil {
		return ""
	}
	line, col := e.eg.ActiveCursor()
	return ed.Selection.Text(ed.Buf.Lines, line, col)
}

func (e *PluginEditorAPI) FilePath() string {
	return e.eg.ActiveFilePath()
}

func (e *PluginEditorAPI) FileName() string {
	return e.eg.ActiveFileName()
}

func (e *PluginEditorAPI) Language() string {
	ed := e.eg.Editor
	if ed == nil || ed.Highlighter == nil {
		return ""
	}
	return ed.Highlighter.Language()
}

func (e *PluginEditorAPI) Viewport() (int, int, int) {
	ed := e.eg.Editor
	if ed == nil || ed.Viewport == nil {
		return 0, 0, 0
	}
	top := ed.Viewport.TopLine
	height := ed.Viewport.Height
	bottom := top + height - 1
	buf := e.eg.ActiveBuffer()
	if buf != nil && bottom >= len(buf.Lines) {
		bottom = len(buf.Lines) - 1
	}
	return top, bottom, height
}

func (e *PluginEditorAPI) ScrollTo(line int) {
	ed := e.eg.Editor
	if ed == nil || ed.Viewport == nil {
		return
	}
	if line < 0 {
		line = 0
	}
	ed.Viewport.TopLine = line
}

func (e *PluginEditorAPI) ScrollBy(delta int) {
	ed := e.eg.Editor
	if ed == nil || ed.Viewport == nil {
		return
	}
	newTop := ed.Viewport.TopLine + delta
	if newTop < 0 {
		newTop = 0
	}
	ed.Viewport.TopLine = newTop
}

func (e *PluginEditorAPI) SetLine(line int, text string) {
	ed := e.eg.Editor
	if ed == nil || ed.Buf == nil || ed.Undo == nil {
		return
	}
	if line < 0 || line >= len(ed.Buf.Lines) {
		return
	}
	runes := []rune(ed.Buf.Lines[line])
	e.Replace(line, 0, line, len(runes), text)
}

func (e *PluginEditorAPI) Insert(line, col int, text string) {
	ed := e.eg.Editor
	if ed == nil || ed.Buf == nil || ed.Undo == nil {
		return
	}
	if line < 0 || line >= len(ed.Buf.Lines) || col < 0 {
		return
	}
	runes := []rune(ed.Buf.Lines[line])
	if col > len(runes) {
		col = len(runes)
	}

	if !strings.Contains(text, "\n") {
		cmd := &undo.InsertStringCommand{Line: line, Col: col, Text: text}
		cmd.Apply(ed.Buf)
		ed.Undo.Push(cmd)
	} else {
		cmd := &undo.PasteCommand{Line: line, Col: col, Text: text, Suffix: string(runes[col:])}
		cmd.Apply(ed.Buf)
		ed.Undo.Push(cmd)
	}
	ed.FlushOnChange()
}

func (e *PluginEditorAPI) Replace(startLine, startCol, endLine, endCol int, text string) {
	ed := e.eg.Editor
	if ed == nil || ed.Buf == nil || ed.Undo == nil {
		return
	}
	if startLine < 0 || startLine >= len(ed.Buf.Lines) || endLine < startLine || startCol < 0 || endCol < 0 {
		return
	}
	if startLine == endLine && endCol < startCol {
		return
	}

	delCmd := &undo.DeleteSelectionCommand{
		StartLine: startLine, StartCol: startCol,
		EndLine: endLine, EndCol: endCol,
	}
	if delCmd.ComputeDeleted(ed.Buf) == text {
		return
	}
	delCmd.Apply(ed.Buf)

	if text == "" {
		ed.Undo.Push(delCmd)
		ed.FlushOnChange()
		return
	}

	col := delCmd.StartCol
	var insertCmd undo.EditCommand
	if !strings.Contains(text, "\n") {
		insertCmd = &undo.InsertStringCommand{Line: startLine, Col: col, Text: text}
	} else {
		suffix := string([]rune(ed.Buf.Lines[startLine])[col:])
		insertCmd = &undo.PasteCommand{Line: startLine, Col: col, Text: text, Suffix: suffix}
	}
	insertCmd.Apply(ed.Buf)

	batch := &undo.BatchCommand{Commands: []undo.EditCommand{delCmd, insertCmd}}
	ed.Undo.Push(batch)
	ed.FlushOnChange()
}

func (e *PluginEditorAPI) SetCursor(line, col int) {
	ed := e.eg.Editor
	if ed == nil || ed.Cursor == nil {
		return
	}
	ed.Cursor.Line = line
	ed.Cursor.Col = col
	ed.EnsureCursorVisible()
}

func (e *PluginEditorAPI) SetSelection(startLine, startCol, endLine, endCol int) {
	ed := e.eg.Editor
	if ed == nil || ed.Selection == nil || ed.Cursor == nil {
		return
	}
	ed.Selection.Start(startLine, startCol)
	ed.Cursor.Line = endLine
	ed.Cursor.Col = endCol
}

func (e *PluginEditorAPI) ClearSelection() {
	ed := e.eg.Editor
	if ed == nil || ed.Selection == nil {
		return
	}
	ed.Selection.Clear()
}

func (e *PluginEditorAPI) BeginUndoGroup() {
	ed := e.eg.Editor
	if ed == nil || ed.Undo == nil {
		return
	}
	ed.Undo.BeginTransaction()
}

func (e *PluginEditorAPI) EndUndoGroup() {
	ed := e.eg.Editor
	if ed == nil || ed.Undo == nil {
		return
	}
	ed.Undo.EndTransaction()
}

func (e *PluginEditorAPI) AddCursor(line, col int) {
	e.eg.AddCursor(line, col)
}

func (e *PluginEditorAPI) GetCursors() []plugin.CursorPosition {
	states := e.eg.GetCursors()
	if states == nil {
		line, col := e.eg.ActiveCursor()
		return []plugin.CursorPosition{{Line: line, Col: col}}
	}
	result := make([]plugin.CursorPosition, len(states))
	for i, cs := range states {
		result[i] = plugin.CursorPosition{Line: cs.Line, Col: cs.Col}
	}
	return result
}

func (e *PluginEditorAPI) ClearCursors() {
	e.eg.CollapseMultiCursor()
}

func (e *PluginEditorAPI) SetSearch(pattern string, useRegex bool) {
	if !e.eg.IsEditorActive() {
		return
	}
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return
	}
	opts := ui.SearchOptions{UseRegex: useRegex, CaseSensitive: true}
	matches, _ := ui.FindInLines(buf.Lines, pattern, opts)
	e.eg.SetSearch(pattern, matches)
}

func (e *PluginEditorAPI) ClearSearch() {
	e.eg.ClearSearch()
}

// PluginFilesystemAPI implements plugin.FilesystemAPI with path restrictions.
type PluginFilesystemAPI struct {
	read    fsRoots
	write   fsRoots
	vars    plugin.FsVars
	onWrite func(path string)
}

type fsRoots struct {
	dirs []string
	all  bool
}

const pluginRPCDialTimeout = 10 * time.Second

func dialPluginRPC(vars plugin.FsVars) func(context.Context, string) (io.ReadWriteCloser, error) {
	return func(ctx context.Context, endpoint string) (io.ReadWriteCloser, error) {
		ctx, cancel := context.WithTimeout(ctx, pluginRPCDialTimeout)
		defer cancel()
		var d net.Dialer
		if path, ok := strings.CutPrefix(endpoint, plugin.UnixEndpointPrefix); ok {
			return d.DialContext(ctx, "unix", vars.Expand(path))
		}
		return d.DialContext(ctx, "tcp", endpoint)
	}
}

func NewPluginFilesystemAPI(allowedRoots ...string) *PluginFilesystemAPI {
	roots := newFsRoots(allowedRoots, false)
	return &PluginFilesystemAPI{read: roots, write: roots}
}

func NewPluginFilesystemAPIFor(p *plugin.Plugin, workspace []string) *PluginFilesystemAPI {
	vars := plugin.FsVars{Workspace: workspace, Config: config.UserConfigDir()}
	if home, err := os.UserHomeDir(); err == nil {
		vars.Home = home
	}
	roots := func(access plugin.FsAccess) fsRoots {
		dirs, all := access.Resolve(vars)
		if p.Dir != "" {
			dirs = append(dirs, p.Dir)
		}
		return newFsRoots(dirs, all)
	}
	return &PluginFilesystemAPI{read: roots(p.Granted.FsRead), write: roots(p.Granted.FsWrite), vars: vars}
}

func newFsRoots(dirs []string, all bool) fsRoots {
	resolved := make([]string, 0, len(dirs))
	for _, root := range dirs {
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		abs = filepath.Clean(abs)
		resolved = append(resolved, abs)
		// check resolves symlinks in the requested path, so a root under a symlink (macOS /tmp and
		// /var) must also be allowed in resolved form, or the workspace denies its own files.
		if real, err := filepath.EvalSymlinks(abs); err == nil && real != abs {
			resolved = append(resolved, real)
		}
	}
	return fsRoots{dirs: resolved, all: all}
}

func (r fsRoots) check(path string) error {
	if r.all {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	clean := filepath.Clean(abs)

	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = resolved
	} else if info, err := os.Lstat(clean); err == nil && info.Mode()&os.ModeSymlink != 0 {
		// A dangling symlink: writing through it would create its target, wherever that is.
		return fmt.Errorf("access denied: path %q is a dangling symlink", path)
	} else if resolved, err := filepath.EvalSymlinks(filepath.Dir(clean)); err == nil {
		clean = filepath.Join(resolved, filepath.Base(clean))
	}

	for _, root := range r.dirs {
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return nil
		}
	}
	return fmt.Errorf("access denied: path %q is outside allowed directories", path)
}

func (f *PluginFilesystemAPI) ReadFile(path string) (string, error) {
	path = f.vars.Expand(path)
	if err := f.read.check(path); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *PluginFilesystemAPI) WriteFile(path, content string) error {
	path = f.vars.Expand(path)
	if err := f.write.check(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	if f.onWrite != nil {
		f.onWrite(path)
	}
	return nil
}

func (f *PluginFilesystemAPI) FileExists(path string) bool {
	path = f.vars.Expand(path)
	if err := f.read.check(path); err != nil {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func (f *PluginFilesystemAPI) ListDir(path string) ([]plugin.FileEntry, error) {
	path = f.vars.Expand(path)
	if err := f.read.check(path); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]plugin.FileEntry, len(entries))
	for i, e := range entries {
		result[i] = plugin.FileEntry{Name: e.Name(), IsDir: e.IsDir()}
	}
	return result, nil
}

// PluginSystemAPI implements plugin.SystemAPI.
type PluginSystemAPI struct {
	onExec func()
}

// PluginTerminalAPI adapts the App's terminal tabs to the plugin
// `ttt.terminal` module (terminal.control / terminal.read).
type PluginTerminalAPI struct {
	app *App
}

func NewPluginTerminalAPI(a *App) *PluginTerminalAPI {
	return &PluginTerminalAPI{app: a}
}

func (t *PluginTerminalAPI) TerminalSpawn(shell string, args, env []string, cwd, label string) (string, error) {
	return t.app.SpawnTerminalSpec(TerminalSpec{Shell: shell, Args: args, Env: env, Cwd: cwd, Label: label})
}

func (t *PluginTerminalAPI) TerminalSend(id, text string) error {
	return t.app.SendToTerminal(id, text)
}

func (t *PluginTerminalAPI) TerminalTail(id string, maxBytes int) (string, error) {
	return t.app.TerminalTail(id, maxBytes)
}

func (t *PluginTerminalAPI) TerminalScreen(id string) (string, error) {
	return t.app.TerminalScreen(id)
}

func (t *PluginTerminalAPI) TerminalFocus(id string) error {
	return t.app.FocusTerminal(id)
}

func (t *PluginTerminalAPI) TerminalClose(id string) error {
	for _, info := range t.app.ListTerminals() {
		if info.ID == id {
			t.app.CloseTerminal(id)
			return nil
		}
	}
	return fmt.Errorf("terminal %q not found", id)
}

func (t *PluginTerminalAPI) TerminalList() []plugin.TerminalInfo {
	return t.app.ListTerminals()
}

func NewPluginSystemAPI() *PluginSystemAPI {
	return &PluginSystemAPI{}
}

var dangerousGitArgPatterns = []string{
	"--upload-pack", "--receive-pack",
	"--exec=", "--config=", "--config-env",
	"core.fsmonitor", "core.sshCommand", "core.pager",
	"diff.external", "merge.tool",
}

// validateArgs guards git only: git runs a config value starting with "!"
// (an alias or helper set through -c) as a shell command. Other binaries get
// their arguments as plain argv with no shell, so "=!" is ordinary text there.
func (s *PluginSystemAPI) validateArgs(binary string, args []string) error {
	if !plugin.IsGitBinary(binary) {
		return nil
	}
	for i, arg := range args {
		if strings.Contains(arg, "=!") {
			return fmt.Errorf("argument %d contains command injection pattern", i)
		}
		for _, pattern := range dangerousGitArgPatterns {
			if strings.Contains(arg, pattern) {
				return fmt.Errorf("argument %d contains blocked pattern %q", i, pattern)
			}
		}
	}
	return nil
}

// pluginExecWaitDelay bounds how long a grandchild can hold the output pipes
// open, idle, after the child exits or is killed, and so delay the result.
const pluginExecWaitDelay = time.Second

func (s *PluginSystemAPI) Exec(ctx context.Context, binary string, args []string, opts plugin.ExecOptions) (string, string, int, error) {
	if err := s.validateArgs(binary, args); err != nil {
		return "", "", -1, err
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.WaitDelay = pluginExecWaitDelay
	var stdout, stderr bytes.Buffer
	outW, errW := io.Writer(&stdout), io.Writer(&stderr)
	if opts.Stdout != nil {
		outW = opts.Stdout
	}
	if opts.Stderr != nil {
		errW = opts.Stderr
	}
	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	err := runPiped(cmd, outW, errW)
	if s.onExec != nil {
		s.onExec()
	}
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil
		}
	}
	return stdout.String(), stderr.String(), exitCode, err
}

func (s *PluginSystemAPI) Env(name string) string {
	return os.Getenv(name)
}

// PluginNetworkAPI implements plugin.NetworkAPI. Where a plugin may connect
// is its network.http grant's business; this layer only enforces the scheme
// and holds redirects to the grant.
type PluginNetworkAPI struct {
	client *http.Client
	// allow is the plugin's network.http check. The plugin host applies it
	// to the URL a plugin passes; redirects are checked here.
	allow func(*url.URL) error
}

func NewPluginNetworkAPI() *PluginNetworkAPI {
	n := &PluginNetworkAPI{}
	n.client = &http.Client{Timeout: 30 * time.Second, CheckRedirect: n.checkRedirect}
	return n
}

// ForPlugin returns a copy whose redirects are also held to allow.
func (n *PluginNetworkAPI) ForPlugin(allow func(*url.URL) error) plugin.NetworkAPI {
	c := &PluginNetworkAPI{allow: allow}
	c.client = &http.Client{Timeout: n.client.Timeout, Transport: n.client.Transport, CheckRedirect: c.checkRedirect}
	return c
}

// checkRedirect holds every redirect to the checks the first URL passed;
// without it a listed host could send a plugin anywhere.
func (n *PluginNetworkAPI) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	if err := n.validateURL(req.URL.String()); err != nil {
		return err
	}
	if n.allow != nil {
		return n.allow(req.URL)
	}
	return nil
}

func (n *PluginNetworkAPI) validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme %q not allowed, only http and https", u.Scheme)
	}
	return nil
}

func (n *PluginNetworkAPI) Get(url string, headers map[string]string) (int, string, map[string]string, error) {
	if err := n.validateURL(url); err != nil {
		return 0, "", nil, err
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, "", nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, "", nil, err
	}
	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}
	return resp.StatusCode, string(body), respHeaders, nil
}

func (n *PluginNetworkAPI) Post(url string, headers map[string]string, body string) (int, string, map[string]string, error) {
	if err := n.validateURL(url); err != nil {
		return 0, "", nil, err
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return 0, "", nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, "", nil, err
	}
	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}
	return resp.StatusCode, string(respBody), respHeaders, nil
}

type PluginSettingsAPI struct {
	app *App
}

func NewPluginSettingsAPI(app *App) *PluginSettingsAPI {
	return &PluginSettingsAPI{app: app}
}

func (s *PluginSettingsAPI) Get(key string) (any, bool) {
	m := s.settingsMap()
	if m == nil {
		return nil, false
	}
	return getPath(m, strings.Split(key, "."))
}

func (s *PluginSettingsAPI) Set(key string, value any) error {
	m := s.settingsMap()
	if m == nil {
		m = make(map[string]any)
	}
	setPath(m, strings.Split(key, "."), value)

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, s.app.Settings); err != nil {
		return err
	}
	return config.SaveSettings(*s.app.Settings)
}

func (s *PluginSettingsAPI) settingsMap() map[string]any {
	data, err := json.Marshal(s.app.Settings)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func getPath(m map[string]any, parts []string) (any, bool) {
	for i, part := range parts {
		val, ok := m[part]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return val, true
		}
		child, ok := val.(map[string]any)
		if !ok {
			return nil, false
		}
		m = child
	}
	return nil, false
}

func setPath(m map[string]any, parts []string, value any) {
	for i, part := range parts {
		if i == len(parts)-1 {
			if value == nil {
				delete(m, part)
			} else {
				m[part] = value
			}
			return
		}
		child, ok := m[part].(map[string]any)
		if !ok {
			child = make(map[string]any)
			m[part] = child
		}
		m = child
	}
}
