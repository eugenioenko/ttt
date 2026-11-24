package plugin

import (
	"fmt"
	"testing"
)

type mockFilesystemAPI struct {
	files   map[string]string
	dirs    map[string][]FileEntry
	written map[string]string
}

type closeTrackingFilesystemAPI struct {
	*mockFilesystemAPI
	closeCount int
}

func (m *closeTrackingFilesystemAPI) Close() error {
	m.closeCount++
	return nil
}

func TestPluginFilesystemAPIResourcesClosedOnReplaceAndDestroy(t *testing.T) {
	first := &closeTrackingFilesystemAPI{mockFilesystemAPI: &mockFilesystemAPI{}}
	second := &closeTrackingFilesystemAPI{mockFilesystemAPI: &mockFilesystemAPI{}}
	p := &Plugin{}

	p.SetFilesystemAPI(first)
	p.SetFilesystemAPI(second)
	if first.closeCount != 1 {
		t.Fatalf("replaced filesystem API close count = %d, want 1", first.closeCount)
	}
	p.Destroy()
	if second.closeCount != 1 {
		t.Fatalf("destroyed filesystem API close count = %d, want 1", second.closeCount)
	}
}

func (m *mockFilesystemAPI) ReadFile(path string) (string, error) {
	if content, ok := m.files[path]; ok {
		return content, nil
	}
	return "", fmt.Errorf("file not found: %s", path)
}

func (m *mockFilesystemAPI) WriteFile(path, content string) error {
	if m.written == nil {
		m.written = make(map[string]string)
	}
	m.written[path] = content
	return nil
}

func (m *mockFilesystemAPI) FileExists(path string) bool {
	_, ok := m.files[path]
	return ok
}

func (m *mockFilesystemAPI) ListDir(path string) ([]FileEntry, error) {
	if entries, ok := m.dirs[path]; ok {
		return entries, nil
	}
	return nil, fmt.Errorf("directory not found: %s", path)
}

func setupTestPluginWithFs(perms PermissionSet, fs *mockFilesystemAPI) (*Plugin, func()) {
	p, cleanup := newTestPluginBase(perms)
	p.Filesystem = fs
	return p, cleanup
}

func TestFsRead(t *testing.T) {
	mock := &mockFilesystemAPI{
		files: map[string]string{"/tmp/test.txt": "file content"},
	}
	p, cleanup := setupTestPluginWithFs(PermissionSet{FsRead: FsAccess{Paths: []string{FsWorkspace}}}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local fs = require("ttt.fs")
		_G.content = fs.read("/tmp/test.txt")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("content").String() != "file content" {
		t.Errorf("expected 'file content', got %q", p.State.GetGlobal("content").String())
	}
}

func TestFsReadNotFound(t *testing.T) {
	mock := &mockFilesystemAPI{files: map[string]string{}}
	p, cleanup := setupTestPluginWithFs(PermissionSet{FsRead: FsAccess{Paths: []string{FsWorkspace}}}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local fs = require("ttt.fs")
		local content, err = fs.read("/nonexistent")
		_G.is_nil = (content == nil)
		_G.has_err = (err ~= nil)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("is_nil").String() != "true" {
		t.Error("expected nil content for missing file")
	}
	if p.State.GetGlobal("has_err").String() != "true" {
		t.Error("expected error for missing file")
	}
}

func TestFsWrite(t *testing.T) {
	mock := &mockFilesystemAPI{}
	p, cleanup := setupTestPluginWithFs(PermissionSet{FsWrite: FsAccess{Paths: []string{FsWorkspace}}}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local fs = require("ttt.fs")
		fs.write("/tmp/output.txt", "written data")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.written["/tmp/output.txt"] != "written data" {
		t.Errorf("expected 'written data', got %q", mock.written["/tmp/output.txt"])
	}
}

func TestFsWriteErrorFormat(t *testing.T) {
	p, cleanup := setupTestPluginWithFs(PermissionSet{FsWrite: FsAccess{Paths: []string{FsWorkspace}}}, &mockFilesystemAPI{})
	defer cleanup()

	p.Filesystem = nil

	err := p.State.DoString(`
		local fs = require("ttt.fs")
		local ok, err_msg = fs.write("/tmp/test.txt", "data")
		_G.is_nil = (ok == nil)
		_G.has_err = (err_msg ~= nil)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("is_nil").String() != "true" {
		t.Error("expected nil result on error")
	}
	if p.State.GetGlobal("has_err").String() != "true" {
		t.Error("expected error message")
	}
}

func TestFsExists(t *testing.T) {
	mock := &mockFilesystemAPI{
		files: map[string]string{"/tmp/exists.txt": ""},
	}
	p, cleanup := setupTestPluginWithFs(PermissionSet{FsRead: FsAccess{Paths: []string{FsWorkspace}}}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local fs = require("ttt.fs")
		_G.found = fs.exists("/tmp/exists.txt")
		_G.missing = fs.exists("/tmp/nope.txt")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("found").String() != "true" {
		t.Error("expected found=true")
	}
	if p.State.GetGlobal("missing").String() != "false" {
		t.Error("expected missing=false")
	}
}

func TestFsList(t *testing.T) {
	mock := &mockFilesystemAPI{
		dirs: map[string][]FileEntry{
			"/tmp": {
				{Name: "file.txt", IsDir: false},
				{Name: "subdir", IsDir: true},
			},
		},
	}
	p, cleanup := setupTestPluginWithFs(PermissionSet{FsRead: FsAccess{Paths: []string{FsWorkspace}}}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local fs = require("ttt.fs")
		local entries = fs.list("/tmp")
		_G.count = #entries
		_G.first_name = entries[1].name
		_G.first_dir = entries[1].is_dir
		_G.second_name = entries[2].name
		_G.second_dir = entries[2].is_dir
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("count").String() != "2" {
		t.Errorf("expected 2 entries, got %s", p.State.GetGlobal("count").String())
	}
	if p.State.GetGlobal("first_name").String() != "file.txt" {
		t.Errorf("expected 'file.txt', got %q", p.State.GetGlobal("first_name").String())
	}
	if p.State.GetGlobal("second_dir").String() != "true" {
		t.Error("expected second entry to be a directory")
	}
}

func TestFsReadWithoutPermission(t *testing.T) {
	mock := &mockFilesystemAPI{files: map[string]string{"/tmp/test": "data"}}
	p, cleanup := setupTestPluginWithFs(PermissionSet{}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local fs = require("ttt.fs")
		fs.read("/tmp/test")
	`)
	if err == nil {
		t.Fatal("expected error when fs.read not granted")
	}
}

func TestFsWriteWithoutPermission(t *testing.T) {
	mock := &mockFilesystemAPI{}
	p, cleanup := setupTestPluginWithFs(PermissionSet{FsRead: FsAccess{Paths: []string{FsWorkspace}}}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local fs = require("ttt.fs")
		fs.write("/tmp/test", "data")
	`)
	if err == nil {
		t.Fatal("expected error when fs.write not granted")
	}
}
