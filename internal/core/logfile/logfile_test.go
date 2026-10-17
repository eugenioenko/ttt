package logfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func openIndexed(t *testing.T, path string) *File {
	t.Helper()
	lf, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lf.Close)
	waitIndexed(t, lf)
	return lf
}

func waitIndexed(t *testing.T, lf *File) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for lf.Indexing() {
		if time.Now().After(deadline) {
			t.Fatal("indexing did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func numbered(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "line %d %s\n", i, strings.Repeat("x", i%50))
	}
	return b.String()
}

func TestReadLinesMatchesSplit(t *testing.T) {
	content := numbered(5000)
	want := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	lf := openIndexed(t, writeFile(t, content))

	if got := lf.Lines(); got != len(want) {
		t.Fatalf("Lines() = %d, want %d", got, len(want))
	}
	for _, start := range []int{0, 1, 255, 256, 257, 1000, 4990} {
		got := lf.ReadLines(start, 20)
		end := min(start+20, len(want))
		if strings.Join(got, "\n") != strings.Join(want[start:end], "\n") {
			t.Errorf("ReadLines(%d, 20) = %q, want %q", start, got, want[start:end])
		}
	}
}

func TestLastLineWithoutNewline(t *testing.T) {
	lf := openIndexed(t, writeFile(t, "a\r\nb\nlast"))
	if got := lf.Lines(); got != 3 {
		t.Fatalf("Lines() = %d, want 3", got)
	}
	if got := lf.ReadLines(0, 10); strings.Join(got, "|") != "a|b|last" {
		t.Errorf("ReadLines = %q", got)
	}
}

func TestEmptyFile(t *testing.T) {
	lf := openIndexed(t, writeFile(t, ""))
	if got := lf.Lines(); got != 1 {
		t.Fatalf("Lines() = %d, want 1", got)
	}
	if got := lf.ReadLines(0, 10); len(got) != 1 || got[0] != "" {
		t.Errorf("ReadLines = %q, want one empty line", got)
	}
}

func TestInvalidUTF8IsReplaced(t *testing.T) {
	lf := openIndexed(t, writeFile(t, "ok\xff\n"))
	if got := lf.ReadLines(0, 1); got[0] != "ok�" {
		t.Errorf("ReadLines = %q", got)
	}
}

func TestRefreshIndexesAppendedLines(t *testing.T) {
	path := writeFile(t, numbered(300)+"partial")
	lf := openIndexed(t, path)
	if got := lf.Lines(); got != 301 {
		t.Fatalf("Lines() = %d, want 301", got)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(" done\nnext\n")
	f.Close()
	lf.Refresh()
	waitIndexed(t, lf)

	if got := lf.Lines(); got != 302 {
		t.Fatalf("Lines() after append = %d, want 302", got)
	}
	if got := lf.ReadLines(300, 5); strings.Join(got, "|") != "partial done|next" {
		t.Errorf("ReadLines = %q", got)
	}
	if lf.TakeReplaced() {
		t.Error("append reported as a replacement")
	}
}

func TestRefreshStartsOverWhenTruncated(t *testing.T) {
	path := writeFile(t, numbered(1000))
	lf := openIndexed(t, path)

	if err := os.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte("fresh\n"), 0o644)
	lf.Refresh()
	waitIndexed(t, lf)

	if !lf.TakeReplaced() {
		t.Error("truncation not reported")
	}
	if got := lf.ReadLines(0, 5); strings.Join(got, "|") != "fresh" {
		t.Errorf("ReadLines = %q", got)
	}
}

func TestRefreshFollowsReplacedFile(t *testing.T) {
	path := writeFile(t, numbered(10))
	lf := openIndexed(t, path)

	next := path + ".new"
	os.WriteFile(next, []byte(numbered(20)), 0o644)
	if err := os.Rename(next, path); err != nil {
		t.Fatal(err)
	}
	lf.Refresh()
	waitIndexed(t, lf)

	if !lf.TakeReplaced() {
		t.Error("replacement not reported")
	}
	if got := lf.Lines(); got != 20 {
		t.Errorf("Lines() = %d, want 20", got)
	}
}

func TestLineLongerThanAChunk(t *testing.T) {
	long := strings.Repeat("y", MaxLineBytes+100)
	lf := openIndexed(t, writeFile(t, "a\n"+long+"\nb\n"))
	if got := lf.Lines(); got != 3 {
		t.Fatalf("Lines() = %d, want 3", got)
	}
	got := lf.ReadLines(0, 3)
	if len(got) != 3 || got[0] != "a" || len(got[1]) != MaxLineBytes || got[2] != "b" {
		t.Errorf("ReadLines lengths = %d, %v", len(got), func() []int {
			var l []int
			for _, s := range got {
				l = append(l, len(s))
			}
			return l
		}())
	}
}

func TestTailAndAppendWhileIndexing(t *testing.T) {
	content := strings.Repeat("ordinary log entry\n", 200000) + "last entry\n"
	path := writeFile(t, content)
	reached, release := make(chan struct{}), make(chan struct{})
	var held atomic.Bool
	lf, err := Open(path, func() {
		if held.CompareAndSwap(false, true) {
			close(reached)
			<-release
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Close()
	defer close(release)
	<-reached
	if !lf.Indexing() {
		t.Fatal("test did not hold the index incomplete")
	}
	rows, err := lf.ReadRows(lf.Tail(2), 2)
	if err != nil || len(rows) != 2 || rows[1].Text != "last entry" {
		t.Fatalf("tail while indexing = %v, %v", rows, err)
	}
	if _, known := lf.LineNumber(rows[0].Offset); known {
		t.Fatal("tail line number should be unknown before the index reaches it")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("appended\n")
	f.Close()
	if err := lf.Refresh(); err != nil {
		t.Fatal(err)
	}
	// Reads never wait for the index callback to return.
	rows, err = lf.ReadRows(lf.Tail(1), 1)
	if err != nil || len(rows) != 1 || rows[0].Text != "appended" {
		t.Fatalf("tail after append = %v, %v", rows, err)
	}
}

func TestCacheStaysBoundedAndHandlesEmptyLines(t *testing.T) {
	lf := openIndexed(t, writeFile(t, "\n\n"+strings.Repeat(strings.Repeat("x", 100)+"\n", 10000)))
	if got := lf.ReadLines(0, 3); len(got) != 3 || got[0] != "" || got[1] != "" {
		t.Fatalf("empty lines = %q", got)
	}
	for i := 0; i < 10000; i += Stride {
		lf.ReadLines(i, 1)
	}
	lf.mu.Lock()
	n := len(lf.cache)
	lf.mu.Unlock()
	if n > cacheChunks {
		t.Fatalf("cache has %d chunks", n)
	}
}
