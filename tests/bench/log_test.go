//go:build unix

package bench

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	logLineBytes = 256
	logMarker    = "Processed request"
)

func logFile(b *testing.B, dir string, sizeMiB int) string {
	b.Helper()
	path := filepath.Join(dir, fmt.Sprintf("log-%dMiB.log", sizeMiB))
	size := int64(sizeMiB) * 1024 * 1024
	if info, err := os.Stat(path); err == nil && info.Size() == size {
		return path
	}
	file, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	line := fmt.Sprintf("%-*s\n", logLineBytes-1, "2026-10-12T12:00:00.000Z INFO service=api request_id=0123456789abcdef message=\""+logMarker+"\" status=200 latency_ms=12 path=/api/events payload="+strings.Repeat("abcdefgh", 8))
	if len(line) != logLineBytes {
		b.Fatal("log entry exceeds its fixed byte length")
	}
	block := bytes.Repeat([]byte(line), 1024*1024/logLineBytes)
	for range sizeMiB {
		if _, err := file.Write(block); err != nil {
			b.Fatal(err)
		}
	}
	return path
}

func buildTTT(b *testing.B) string {
	b.Helper()
	binary := filepath.Join(b.TempDir(), "ttt")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/ttt")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("build ttt: %v\n%s", err, out)
	}
	return binary
}

// peakRSS converts rusage's Maxrss, which Linux reports in KiB and macOS in bytes.
func peakRSS(state *os.ProcessState) float64 {
	maxrss := float64(state.SysUsage().(*syscall.Rusage).Maxrss)
	if runtime.GOOS == "darwin" {
		return maxrss
	}
	return maxrss * 1024
}

// logRepo commits the sizeMiB log to a git repository under dir, reused across
// runs, and returns the committed file's path.
func logRepo(b *testing.B, dir string, sizeMiB int) string {
	b.Helper()
	repo := filepath.Join(dir, fmt.Sprintf("repo-%dMiB", sizeMiB))
	path := filepath.Join(repo, "app.log")
	if exec.Command("git", "-C", repo, "rev-parse", "--verify", "HEAD").Run() == nil {
		return path
	}
	if err := os.RemoveAll(repo); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(repo, 0o755); err != nil {
		b.Fatal(err)
	}
	data, err := os.ReadFile(logFile(b, dir, sizeMiB))
	if err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		b.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "app.log"},
		{"-c", "user.name=bench", "-c", "user.email=bench@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "log"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			b.Fatalf("git %s: %v\n%s", args[len(args)-1], err, out)
		}
	}
	return path
}

// runTTT runs the binary on path with the --exec script and returns its peak RSS.
func runTTT(b *testing.B, binary, configDir, path, script string) float64 {
	b.Helper()
	cmd := exec.Command(binary, "--size", "120x40", "--exec", script, path)
	cmd.Dir = filepath.Dir(path)
	cmd.Env = append(os.Environ(), "TTT_CONFIG_DIR="+configDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	started := time.Now()
	out, err := cmd.CombinedOutput()
	b.StopTimer()
	// Background processes ttt started, such as git blame, outlive it and
	// would load the next run.
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err != nil {
		b.Fatalf("open %s after %s: %v\n%s", filepath.Base(path), time.Since(started), err, out)
	}
	b.StartTimer()
	return peakRSS(cmd.ProcessState)
}

// BenchmarkLog opens a log in the built binary, as a person would from the
// command line, so the time and peak RSS include everything up to the first
// frame that shows the log: startup, loading, and the event loop's first pass.
// The _wrap variants start with word wrap on, so that frame also lays out every
// line's wrapped rows. The _edit variant opens a log committed to git, types a
// character, and moves down 20 lines, which runs blame on unsaved contents.
func BenchmarkLog(b *testing.B) {
	// CI shares generated files across base/PR passes; -short only skips
	// the manual 1 GiB case, keeping the smaller files at their real sizes.
	dir := os.Getenv("TTT_BENCH_LOG_DIR")
	if dir == "" {
		dir = b.TempDir()
	}
	sizes := []int{1, 128, 256}
	if !testing.Short() {
		sizes = append(sizes, 1024)
	}
	binary := buildTTT(b)
	configDir := b.TempDir()
	wrapConfigDir := b.TempDir()
	if err := os.WriteFile(filepath.Join(wrapConfigDir, "settings.json"), []byte(`{"editor":{"wordWrap":true}}`), 0o644); err != nil {
		b.Fatal(err)
	}
	openScript := "wait-for " + logMarker + "; quit"
	variants := []struct{ suffix, configDir string }{
		{"", configDir},
		{"_wrap", wrapConfigDir},
	}
	for _, variant := range variants {
		for _, sizeMiB := range sizes {
			b.Run(fmt.Sprintf("%dMiB%s", sizeMiB, variant.suffix), func(b *testing.B) {
				path := logFile(b, dir, sizeMiB)
				var peak float64
				for b.Loop() {
					peak = max(peak, runTTT(b, binary, variant.configDir, path, openScript))
				}
				b.ReportMetric(peak, "peak-RSS-B")
			})
		}
	}
	b.Run("128MiB_edit", func(b *testing.B) {
		path := logRepo(b, dir, 128)
		script := "wait-for " + logMarker + "; type x;" + strings.Repeat(" key down;", 20) + " quit"
		var peak float64
		for b.Loop() {
			peak = max(peak, runTTT(b, binary, configDir, path, script))
		}
		b.ReportMetric(peak, "peak-RSS-B")
	})
}
