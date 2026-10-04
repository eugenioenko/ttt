package app

import (
	"bytes"
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
	if line < 0 || line >= len(ed.Buf.Lines) {
		return
	}

	if !strings.Contains(text, "\n") {
		cmd := &undo.InsertStringCommand{Line: line, Col: col, Text: text}
		cmd.Apply(ed.Buf)
		ed.Undo.Push(cmd)
	} else {
		runes := []rune(ed.Buf.Lines[line])
		colClamped := col
		if colClamped > len(runes) {
			colClamped = len(runes)
		}
		suffix := string(runes[colClamped:])
		cmd := &undo.PasteCommand{Line: line, Col: colClamped, Text: text, Suffix: suffix}
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
	if startLine < 0 || startLine >= len(ed.Buf.Lines) {
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

	var insertCmd undo.EditCommand
	if !strings.Contains(text, "\n") {
		insertCmd = &undo.InsertStringCommand{Line: startLine, Col: startCol, Text: text}
	} else {
		runes := []rune(ed.Buf.Lines[startLine])
		colClamped := startCol
		if colClamped > len(runes) {
			colClamped = len(runes)
		}
		suffix := string(runes[colClamped:])
		insertCmd = &undo.PasteCommand{Line: startLine, Col: colClamped, Text: text, Suffix: suffix}
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
	allowedRoots []string
	onWrite      func(path string)
}

func NewPluginFilesystemAPI(allowedRoots ...string) *PluginFilesystemAPI {
	resolved := make([]string, 0, len(allowedRoots))
	for _, root := range allowedRoots {
		if abs, err := filepath.Abs(root); err == nil {
			resolved = append(resolved, filepath.Clean(abs))
		}
	}
	return &PluginFilesystemAPI{allowedRoots: resolved}
}

func (f *PluginFilesystemAPI) validatePath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	clean := filepath.Clean(abs)

	if resolved, err := filepath.EvalSymlinks(filepath.Dir(clean)); err == nil {
		clean = filepath.Join(resolved, filepath.Base(clean))
	}

	for _, root := range f.allowedRoots {
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return nil
		}
	}
	return fmt.Errorf("access denied: path %q is outside allowed directories", path)
}

func (f *PluginFilesystemAPI) ReadFile(path string) (string, error) {
	if err := f.validatePath(path); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *PluginFilesystemAPI) WriteFile(path, content string) error {
	if err := f.validatePath(path); err != nil {
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
	if err := f.validatePath(path); err != nil {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func (f *PluginFilesystemAPI) ListDir(path string) ([]plugin.FileEntry, error) {
	if err := f.validatePath(path); err != nil {
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

func NewPluginSystemAPI() *PluginSystemAPI {
	return &PluginSystemAPI{}
}

var dangerousArgPatterns = []string{
	"--upload-pack", "--receive-pack",
	"--exec=", "--config=",
	"core.fsmonitor", "core.sshCommand", "core.pager",
	"diff.external", "merge.tool",
}

func (s *PluginSystemAPI) validateArgs(binary string, args []string) error {
	base := filepath.Base(binary)
	for i, arg := range args {
		if strings.Contains(arg, "=!") {
			return fmt.Errorf("argument %d contains command injection pattern", i)
		}

		if base == "git" {
			if arg == "-c" && i+1 < len(args) && strings.Contains(args[i+1], "=!") {
				return fmt.Errorf("git -c argument contains command injection pattern")
			}
			for _, pattern := range dangerousArgPatterns {
				if strings.Contains(arg, pattern) {
					return fmt.Errorf("argument %d contains blocked pattern %q", i, pattern)
				}
			}
		}
	}
	return nil
}

func (s *PluginSystemAPI) Exec(binary string, args []string, stdin string) (string, string, int, error) {
	if err := s.validateArgs(binary, args); err != nil {
		return "", "", -1, err
	}
	cmd := exec.Command(binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	err := cmd.Run()
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

// PluginNetworkAPI implements plugin.NetworkAPI.
type PluginNetworkAPI struct {
	client *http.Client
}

func NewPluginNetworkAPI() *PluginNetworkAPI {
	return &PluginNetworkAPI{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

var privateNetworks = []net.IPNet{
	{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
	{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},
	{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)},
	{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)},
	{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
}

func (n *PluginNetworkAPI) validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme %q not allowed, only http and https", u.Scheme)
	}

	hostname := u.Hostname()
	if hostname == "localhost" {
		return fmt.Errorf("requests to localhost are not allowed")
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("requests to %s (%s) are not allowed", hostname, ip)
		}
		for _, pn := range privateNetworks {
			if pn.Contains(ip) {
				return fmt.Errorf("requests to private network %s (%s) are not allowed", hostname, ip)
			}
		}
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
