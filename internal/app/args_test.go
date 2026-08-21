package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitLineCol(t *testing.T) {
	cases := []struct {
		arg  string
		path string
		line int
		col  int
		ok   bool
	}{
		{"main.go:42", "main.go", 42, 0, true},
		{"main.go:42:8", "main.go", 42, 8, true},
		{"main.go:42:", "main.go", 42, 0, true},
		{"internal/app/main.go:7", "internal/app/main.go", 7, 0, true},
		// When ok is false the argument is returned untouched; callers ignore path.
		{"main.go", "main.go", 0, 0, false},
		{"main.go:", "main.go:", 0, 0, false},
		// A leading drive letter is not a number, so Windows paths survive.
		{`C:\src\main.go`, `C:\src\main.go`, 0, 0, false},
		{`C:\src\main.go:42`, `C:\src\main.go`, 42, 0, true},
		// Line numbers are 1-based; 0 and negatives are part of the name.
		{"main.go:0", "main.go:0", 0, 0, false},
		{"main.go:-3", "main.go:-3", 0, 0, false},
		// No path left once the numbers are stripped.
		{":42", ":42", 0, 0, false},
		{"42", "42", 0, 0, false},
	}
	for _, c := range cases {
		path, line, col, ok := splitLineCol(c.arg)
		if ok != c.ok || path != c.path || line != c.line || col != c.col {
			t.Errorf("splitLineCol(%q) = (%q, %d, %d, %v), want (%q, %d, %d, %v)",
				c.arg, path, line, col, ok, c.path, c.line, c.col, c.ok)
		}
	}
}

// A `path:line` argument only counts as a position when the stripped path is a
// real file. Anything else keeps behaving the way it did before the suffix was
// understood.
func TestResolveLineColArgRequiresExistingFile(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "main.go")
	if err := os.WriteFile(real, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	target, ok := resolveLineColArg(real + ":42:8")
	if !ok {
		t.Fatalf("resolveLineColArg(%q) returned ok=false", real+":42:8")
	}
	if target.Path != real || target.Line != 42 || target.Col != 8 {
		t.Errorf("got %+v, want path=%q line=42 col=8", target, real)
	}

	if _, ok := resolveLineColArg(filepath.Join(dir, "missing.go") + ":42"); ok {
		t.Error("a suffix on a path that does not exist must not resolve")
	}
	if _, ok := resolveLineColArg(dir + ":42"); ok {
		t.Error("a directory must not resolve as a line target")
	}
}

func TestResolveArgsFileLineCol(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A file whose name genuinely ends in a colon and digits must win over the
	// positional reading, because it exists.
	colonName := filepath.Join(dir, "report:12")
	if err := os.WriteFile(colonName, []byte("x\n"), 0o644); err != nil {
		t.Skipf("filesystem rejects colons in names: %v", err)
	}

	saved := os.Args
	defer func() { os.Args = saved }()
	os.Args = []string{"ttt", file + ":42:8", colonName, filepath.Join(dir, "new.go:9")}

	_, openFiles, _, _ := resolveArgs()
	if len(openFiles) != 3 {
		t.Fatalf("got %d targets, want 3: %+v", len(openFiles), openFiles)
	}
	if openFiles[0].Path != file || openFiles[0].Line != 42 || openFiles[0].Col != 8 {
		t.Errorf("positional suffix: got %+v", openFiles[0])
	}
	if openFiles[1].Path != colonName || openFiles[1].Line != 0 {
		t.Errorf("existing colon-named file: got %+v, want path=%q line=0", openFiles[1], colonName)
	}
	if openFiles[2].Line != 0 {
		t.Errorf("missing file keeps its literal name: got %+v", openFiles[2])
	}
}

func TestResolveArgsIgnoresListenFlag(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()
	os.Args = []string{"ttt", "--listen"}

	_, openFiles, _, _ := resolveArgs()
	if len(openFiles) != 0 {
		t.Errorf("--listen was treated as a file to open: %+v", openFiles)
	}
}
