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

// BenchmarkLog opens a log in the built binary, as a person would from the
// command line, so the time and peak RSS include everything up to the first
// frame that shows the log: startup, loading, and the event loop's first pass.
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
	for _, sizeMiB := range sizes {
		b.Run(fmt.Sprintf("%dMiB", sizeMiB), func(b *testing.B) {
			path := logFile(b, dir, sizeMiB)
			var peak float64
			for b.Loop() {
				cmd := exec.Command(binary, "--size", "120x40", "--exec", "wait-for "+logMarker+"; quit", path)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "TTT_CONFIG_DIR="+configDir)
				started := time.Now()
				if out, err := cmd.CombinedOutput(); err != nil {
					b.Fatalf("open %s after %s: %v\n%s", filepath.Base(path), time.Since(started), err, out)
				}
				peak = max(peak, peakRSS(cmd.ProcessState))
			}
			b.ReportMetric(peak, "peak-RSS-B")
		})
	}
}
