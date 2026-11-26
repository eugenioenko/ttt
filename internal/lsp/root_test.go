package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMarkerFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindRootWalksUpToNearestMarker(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	deep := filepath.Join(project, "src", "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMarkerFile(t, filepath.Join(project, "package.json"))

	if got := FindRoot(deep, []string{"package.json"}); got != project {
		t.Errorf("root = %q, want %q", got, project)
	}
}

func TestFindRootMatchesGlobMarkers(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	writeMarkerFile(t, filepath.Join(project, "App.sln"))

	if got := FindRoot(filepath.Join(project, "src"), []string{"*.sln"}); got != project {
		t.Errorf("root = %q, want %q", got, project)
	}
}

func TestFindRootPrefersEarlierMarkers(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "module")
	writeMarkerFile(t, filepath.Join(root, "go.work"))
	writeMarkerFile(t, filepath.Join(module, "go.mod"))
	src := filepath.Join(module, "src")

	if got := FindRoot(src, []string{"go.work", "go.mod"}); got != root {
		t.Errorf("root = %q, want the go.work directory %q", got, root)
	}
	if got := FindRoot(src, []string{"go.mod", "go.work"}); got != module {
		t.Errorf("root = %q, want the go.mod directory %q", got, module)
	}
}

func TestFindRootReturnsEmptyWithoutMatch(t *testing.T) {
	if got := FindRoot(t.TempDir(), []string{"ttt-root-test-missing-marker"}); got != "" {
		t.Errorf("root = %q, want no root", got)
	}
	if got := FindRoot(t.TempDir(), nil); got != "" {
		t.Errorf("root = %q, want no root", got)
	}
}
