package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~", home},
		{"~/Documents", filepath.Join(home, "Documents")},
		{"~/a/b/c", filepath.Join(home, "a", "b", "c")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~user/path", "~user/path"},
		{"", ""},
		{"~/", filepath.Join(home, "")},
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAddFolderTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	ws := &Workspace{}
	ws.AddFolder("~")
	if len(ws.Folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(ws.Folders))
	}
	if ws.Folders[0].Path != home {
		t.Errorf("expected %s, got %s", home, ws.Folders[0].Path)
	}
}

func TestNew(t *testing.T) {
	tmp := t.TempDir()
	dir1 := filepath.Join(tmp, "repo1")
	dir2 := filepath.Join(tmp, "repo2")
	requireNoError(t, os.Mkdir(dir1, 0755))
	requireNoError(t, os.Mkdir(dir2, 0755))

	ws := New([]string{dir1, dir2})
	if len(ws.Folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(ws.Folders))
	}
	if ws.Folders[0].Path != dir1 {
		t.Errorf("expected %s, got %s", dir1, ws.Folders[0].Path)
	}
}

func TestAddFolderDedup(t *testing.T) {
	tmp := t.TempDir()
	dir1 := filepath.Join(tmp, "repo1")
	requireNoError(t, os.Mkdir(dir1, 0755))

	ws := New([]string{dir1})
	ws.AddFolder(dir1)
	if len(ws.Folders) != 1 {
		t.Fatalf("expected 1 folder after dedup, got %d", len(ws.Folders))
	}
}

func TestRemoveFolder(t *testing.T) {
	tmp := t.TempDir()
	dir1 := filepath.Join(tmp, "repo1")
	dir2 := filepath.Join(tmp, "repo2")
	requireNoError(t, os.Mkdir(dir1, 0755))
	requireNoError(t, os.Mkdir(dir2, 0755))

	ws := New([]string{dir1, dir2})
	ws.RemoveFolder(dir1)
	if len(ws.Folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(ws.Folders))
	}
	if ws.Folders[0].Path != dir2 {
		t.Errorf("expected %s, got %s", dir2, ws.Folders[0].Path)
	}
}

func TestRemoveFolderNotFound(t *testing.T) {
	tmp := t.TempDir()
	dir1 := filepath.Join(tmp, "repo1")
	requireNoError(t, os.Mkdir(dir1, 0755))

	ws := New([]string{dir1})
	ws.RemoveFolder("/nonexistent")
	if len(ws.Folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(ws.Folders))
	}
}

func TestPaths(t *testing.T) {
	tmp := t.TempDir()
	dir1 := filepath.Join(tmp, "a")
	dir2 := filepath.Join(tmp, "b")
	requireNoError(t, os.Mkdir(dir1, 0755))
	requireNoError(t, os.Mkdir(dir2, 0755))

	ws := New([]string{dir1, dir2})
	paths := ws.Paths()
	if len(paths) != 2 || paths[0] != dir1 || paths[1] != dir2 {
		t.Errorf("unexpected paths: %v", paths)
	}
}

func TestPrimary(t *testing.T) {
	tmp := t.TempDir()
	dir1 := filepath.Join(tmp, "first")
	dir2 := filepath.Join(tmp, "second")
	requireNoError(t, os.Mkdir(dir1, 0755))
	requireNoError(t, os.Mkdir(dir2, 0755))

	ws := New([]string{dir1, dir2})
	if ws.Primary() != dir1 {
		t.Errorf("expected %s, got %s", dir1, ws.Primary())
	}
}

func TestPrimaryEmpty(t *testing.T) {
	ws := &Workspace{}
	if ws.Primary() != "" {
		t.Errorf("expected empty, got %s", ws.Primary())
	}
}

func TestIsRepo(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	plain := filepath.Join(tmp, "plain")
	requireNoError(t, os.Mkdir(repo, 0755))
	requireNoError(t, os.Mkdir(plain, 0755))
	requireNoError(t, os.Mkdir(filepath.Join(repo, ".git"), 0755))

	ws := New([]string{repo, plain})
	if !ws.Folders[0].IsRepo {
		t.Error("expected repo to be detected as git repo")
	}
	if ws.Folders[1].IsRepo {
		t.Error("expected plain to not be a git repo")
	}
}

func TestFolderForFile(t *testing.T) {
	tmp := t.TempDir()
	dir1 := filepath.Join(tmp, "repo1")
	dir2 := filepath.Join(tmp, "repo2")
	requireNoError(t, os.Mkdir(dir1, 0755))
	requireNoError(t, os.Mkdir(dir2, 0755))

	ws := New([]string{dir1, dir2})

	f := ws.FolderForFile(filepath.Join(dir1, "src", "main.go"))
	if f == nil || f.Path != dir1 {
		t.Errorf("expected folder %s, got %v", dir1, f)
	}

	f = ws.FolderForFile(filepath.Join(dir2, "index.ts"))
	if f == nil || f.Path != dir2 {
		t.Errorf("expected folder %s, got %v", dir2, f)
	}

	f = ws.FolderForFile("/some/other/path")
	if f != nil {
		t.Errorf("expected nil for unmatched path, got %v", f)
	}
}

func TestFolderForFileNestedPaths(t *testing.T) {
	tmp := t.TempDir()
	parent := filepath.Join(tmp, "monorepo")
	child := filepath.Join(parent, "packages", "frontend")
	requireNoError(t, os.MkdirAll(child, 0755))

	ws := New([]string{parent, child})

	f := ws.FolderForFile(filepath.Join(child, "src", "app.tsx"))
	if f == nil || f.Path != child {
		t.Errorf("expected longest match %s, got %v", child, f)
	}

	f = ws.FolderForFile(filepath.Join(parent, "README.md"))
	if f == nil || f.Path != parent {
		t.Errorf("expected %s, got %v", parent, f)
	}
}

func TestSaveAndLoadFile(t *testing.T) {
	tmp := t.TempDir()
	dir1 := filepath.Join(tmp, "repo1")
	dir2 := filepath.Join(tmp, "repo2")
	requireNoError(t, os.Mkdir(dir1, 0755))
	requireNoError(t, os.Mkdir(dir2, 0755))

	ws := New([]string{dir1, dir2})
	wsFile := filepath.Join(tmp, "test.ttt")
	if err := ws.SaveFile(wsFile); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadFile(wsFile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded.Folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(loaded.Folders))
	}
	if loaded.Folders[0].Path != dir1 {
		t.Errorf("expected %s, got %s", dir1, loaded.Folders[0].Path)
	}
	if loaded.Folders[1].Path != dir2 {
		t.Errorf("expected %s, got %s", dir2, loaded.Folders[1].Path)
	}
	if loaded.FilePath != wsFile {
		t.Errorf("expected FilePath %s, got %s", wsFile, loaded.FilePath)
	}
}
