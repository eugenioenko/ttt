//go:build unix

package bench

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const codeMarker = "package"

// codeFile repeats this repository's Go sources to sizeMiB, cut at a line
// boundary, so the file is real highlighted code. Like the logs, it is built
// once in dir and shared by every run that finds it there.
func codeFile(b *testing.B, dir string, sizeMiB int) string {
	b.Helper()
	path := filepath.Join(dir, fmt.Sprintf("code-%dMiB.go", sizeMiB))
	size := sizeMiB * 1024 * 1024
	if info, err := os.Stat(path); err == nil && info.Size() > int64(size-1024*1024) && info.Size() <= int64(size) {
		return path
	}
	var sources []string
	root := filepath.Join("..", "..", "internal")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			sources = append(sources, p)
		}
		return err
	})
	if err != nil {
		b.Fatal(err)
	}
	sort.Strings(sources)
	var seed bytes.Buffer
	for _, p := range sources {
		data, err := os.ReadFile(p)
		if err != nil {
			b.Fatal(err)
		}
		seed.Write(bytes.TrimRight(data, "\n"))
		seed.WriteByte('\n')
	}
	var out bytes.Buffer
	for out.Len()+seed.Len() <= size {
		out.Write(seed.Bytes())
	}
	tail := seed.Bytes()[:size-out.Len()]
	out.Write(tail[:bytes.LastIndexByte(tail, '\n')+1])
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		b.Fatal(err)
	}
	return path
}

// BenchmarkCode opens a large Go file in the built binary with LSP and the
// git gutter off, as the other editors in a comparison run. The _type variant
// then types 20 characters, which re-highlights after each one; _enter
// presses Enter 10 times, which also adds a line and refolds the file.
func BenchmarkCode(b *testing.B) {
	dir := os.Getenv("TTT_BENCH_LOG_DIR")
	if dir == "" {
		dir = b.TempDir()
	}
	binary := buildTTT(b)
	configDir := b.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(`{"lsp":{"enabled":false},"editor":{"gitGutter":false}}`), 0o644); err != nil {
		b.Fatal(err)
	}
	path := codeFile(b, dir, 64)
	scripts := []struct{ name, script string }{
		{"64MiB", "wait-for " + codeMarker + "; quit"},
		{"64MiB_type", "wait-for " + codeMarker + ";" + strings.Repeat(" type x;", 20) + " quit"},
		{"64MiB_enter", "wait-for " + codeMarker + ";" + strings.Repeat(" key enter;", 10) + " quit"},
	}
	for _, s := range scripts {
		b.Run(s.name, func(b *testing.B) {
			var peak float64
			for b.Loop() {
				peak = max(peak, runTTT(b, binary, configDir, path, s.script))
			}
			b.ReportMetric(peak, "peak-RSS-B")
		})
	}
}
