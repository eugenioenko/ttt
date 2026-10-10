package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"

	"github.com/gdamore/tcell/v3"
)

// BenchmarkDiff drives diff views through the whole app like BenchmarkEditor.
// Sub-benchmark names describe the user operation, not the widget behind it,
// so a reimplementation of the diff view is compared scenario by scenario.

const (
	diffBenchFile        = "source.go"
	diffBenchScrollReset = 100
	diffBenchPageReset   = 20
)

var diffBenchLayouts = []struct {
	name       string
	mode       string
	wrap       bool
	scrollOnly bool
}{
	{"split", config.DiffModeSplit, false, false},
	{"unified", config.DiffModeUnified, false, false},
	{"split_wrap", config.DiffModeSplit, true, true},
	{"unified_wrap", config.DiffModeUnified, true, true},
}

// changeLines makes the new side of a diff: a modified line every 40 lines,
// two deleted lines every 97 and three inserted lines every 151.
func changeLines(old []string) []string {
	out := make([]string, 0, len(old)+len(old)/50)
	for i := 0; i < len(old); i++ {
		switch {
		case i%151 == 75:
			out = append(out, old[i], "\t// inserted note", fmt.Sprintf("\tbenchInserted%d := %d", i, i), "\t_ = benchInserted"+fmt.Sprint(i))
		case i%97 == 50 && i+1 < len(old):
			i++
		case i%40 == 20:
			out = append(out, old[i]+" // changed")
		default:
			out = append(out, old[i])
		}
	}
	return out
}

type diffBenchPair struct {
	old, new []string
	fd       diff.FileDiff
}

func newDiffBenchPair(b *testing.B, lines int) diffBenchPair {
	b.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "bench", diffBenchFile))
	if err != nil {
		b.Fatal(err)
	}
	old := strings.Split(strings.TrimSuffix(string(repeatToLines(src, lines)), "\n"), "\n")
	updated := changeLines(old)
	return diffBenchPair{old: old, new: updated, fd: diff.Parse(diff.Generate(old, updated, diffBenchFile))}
}

func setDiffView(h *testHarness, mode, context string, wrap bool) {
	h.app.Settings.Editor.DiffMode = mode
	h.app.Settings.Editor.DiffContext = context
	h.app.Settings.Editor.DiffWordWrap = wrap
	h.app.ApplySettings(*h.app.Settings)
}

func openBenchDiff(h *testHarness, pair diffBenchPair, fullFile bool) {
	h.app.EditorGroup.OpenDiff(diffBenchFile, pair.fd, pair.old, pair.new, fullFile)
	h.redraw()
}

func newDiffBenchHarness(b *testing.B, pair diffBenchPair, mode, context string, wrap, fullFile bool) *testHarness {
	b.Helper()
	h := newBenchHarness(b)
	setDiffView(h, mode, context, wrap)
	openBenchDiff(h, pair, fullFile)
	return h
}

func diffWheel(h *testHarness) {
	h.app.Root.HandleEvent(tcell.NewEventMouse(benchWidth/2, benchHeight/2, tcell.WheelDown, tcell.ModNone))
	h.redraw()
}

func resizeBench(h *testHarness, w int) {
	h.screen.SetSize(w, benchHeight)
	h.app.Root.SetSize(w, benchHeight)
	h.renderer.Clear()
	h.redraw()
}

func diffToTop(h *testHarness) {
	h.pressKey(tcell.KeyHome, tcell.ModCtrl)
}

// gapRow finds the first collapsed-lines row on screen.
func gapRow(h *testHarness) (x, y int, ok bool) {
	for y := 0; y < benchHeight; y++ {
		row := h.screenRow(y)
		if strings.Contains(row, "⋯") && strings.Contains(row, "line") {
			return displayColumnOf(row, "⋯"), y, true
		}
	}
	return 0, 0, false
}

func runBenchGit(b *testing.B, dir string, args ...string) string {
	b.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		b.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func initBenchRepo(b *testing.B, dir string) {
	b.Helper()
	runBenchGit(b, dir, "init", "-q", "-b", "main")
	runBenchGit(b, dir, "config", "user.email", "bench@test.com")
	runBenchGit(b, dir, "config", "user.name", "Bench")
	runBenchGit(b, dir, "config", "commit.gpgsign", "false")
}

func writeBenchLines(b *testing.B, path string, lines []string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		b.Fatal(err)
	}
}

func awaitBenchCommitDetail(b *testing.B, h *testHarness) {
	b.Helper()
	deadline := time.After(30 * time.Second)
	for {
		select {
		case event := <-h.screen.EventQ():
			interrupt, ok := event.(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			result, ok := interrupt.Data().(*app.CommitDetailResult)
			if !ok {
				continue
			}
			h.app.ApplyCommitDetail(result)
			h.redraw()
			return
		case <-deadline:
			b.Fatal("timed out waiting for commit detail")
		}
	}
}

func commitDetailFiles() int {
	if testing.Short() {
		return 20
	}
	return 100
}

func BenchmarkDiff(b *testing.B) {
	medium := newDiffBenchPair(b, mediumLines)
	large := newDiffBenchPair(b, largeLines())
	changes, full := config.DiffContextChanges, config.DiffContextFull

	b.Run("go", func(b *testing.B) {
		for _, layout := range diffBenchLayouts {
			b.Run(layout.name, func(b *testing.B) {
				b.Run("redraw", func(b *testing.B) {
					h := newDiffBenchHarness(b, medium, layout.mode, full, layout.wrap, true)
					b.ReportAllocs()
					for b.Loop() {
						h.redraw()
					}
				})

				b.Run("wheel_scroll", func(b *testing.B) {
					h := newDiffBenchHarness(b, medium, layout.mode, full, layout.wrap, true)
					steps := 0
					b.ReportAllocs()
					for b.Loop() {
						if steps == diffBenchScrollReset {
							b.StopTimer()
							diffToTop(h)
							steps = 0
							b.StartTimer()
						}
						diffWheel(h)
						steps++
					}
				})

				if layout.scrollOnly {
					b.Run("resize", func(b *testing.B) {
						h := newDiffBenchHarness(b, medium, layout.mode, full, layout.wrap, true)
						b.Cleanup(func() { resizeBench(h, benchWidth) })
						narrow := false
						b.ReportAllocs()
						for b.Loop() {
							narrow = !narrow
							if narrow {
								resizeBench(h, benchWidth*3/4)
							} else {
								resizeBench(h, benchWidth)
							}
						}
					})
					return
				}

				b.Run("page_down", func(b *testing.B) {
					h := newDiffBenchHarness(b, medium, layout.mode, full, layout.wrap, true)
					steps := 0
					b.ReportAllocs()
					for b.Loop() {
						if steps == diffBenchPageReset {
							b.StopTimer()
							diffToTop(h)
							steps = 0
							b.StartTimer()
						}
						h.pressKey(tcell.KeyPgDn, tcell.ModNone)
						steps++
					}
				})

				b.Run("open_large", func(b *testing.B) {
					h := newBenchHarness(b)
					setDiffView(h, layout.mode, changes, layout.wrap)
					b.ReportAllocs()
					for b.Loop() {
						openBenchDiff(h, large, false)
						b.StopTimer()
						h.exec("tab.close")
						b.StartTimer()
					}
				})

				b.Run("open_large_full_file", func(b *testing.B) {
					h := newBenchHarness(b)
					setDiffView(h, layout.mode, full, layout.wrap)
					b.ReportAllocs()
					for b.Loop() {
						openBenchDiff(h, large, true)
						b.StopTimer()
						h.exec("tab.close")
						b.StartTimer()
					}
				})

				// The first render at the end highlights every line above it.
				b.Run("jump_to_end_large", func(b *testing.B) {
					h := newBenchHarness(b)
					setDiffView(h, layout.mode, full, layout.wrap)
					b.ReportAllocs()
					for b.Loop() {
						b.StopTimer()
						openBenchDiff(h, large, true)
						b.StartTimer()
						h.pressKey(tcell.KeyEnd, tcell.ModCtrl)
						b.StopTimer()
						h.exec("tab.close")
						b.StartTimer()
					}
				})

				b.Run("expand_gap", func(b *testing.B) {
					h := newBenchHarness(b)
					setDiffView(h, layout.mode, changes, layout.wrap)
					b.ReportAllocs()
					for b.Loop() {
						b.StopTimer()
						openBenchDiff(h, large, false)
						x, y, ok := gapRow(h)
						if !ok {
							b.Fatalf("no collapsed-lines row on screen:\n%s", h.screenText())
						}
						b.StartTimer()
						h.click(x, y)
						b.StopTimer()
						h.exec("tab.close")
						b.StartTimer()
					}
				})
			})
		}

		b.Run("toggle_split_unified", func(b *testing.B) {
			h := newDiffBenchHarness(b, large, config.DiffModeSplit, full, false, true)
			b.ReportAllocs()
			for b.Loop() {
				h.exec("diff.toggleUnified")
			}
		})

		b.Run("switch_diff_and_file_tab", func(b *testing.B) {
			h := newBenchHarness(b)
			setDiffView(h, config.DiffModeSplit, full, false)
			src, err := os.ReadFile(filepath.Join("testdata", "bench", diffBenchFile))
			if err != nil {
				b.Fatal(err)
			}
			openBenchFile(b, h, "file_tab.go", repeatToLines(src, largeLines()))
			filePath := filepath.Join(h.dir, "file_tab.go")
			openBenchDiff(h, large, true)
			diffTab := diffBenchFile + " (diff)"
			onDiff := true
			b.ReportAllocs()
			for b.Loop() {
				target := filePath
				if !onDiff {
					target = diffTab
				}
				if !h.app.EditorGroup.SwitchToTabByPath(target) {
					b.Fatalf("no tab %q", target)
				}
				h.redraw()
				onDiff = !onDiff
			}
		})

		b.Run("open_changes_large", func(b *testing.B) {
			h := newBenchHarness(b)
			setDiffView(h, config.DiffModeSplit, changes, false)
			initBenchRepo(b, h.dir)
			path := filepath.Join(h.dir, diffBenchFile)
			writeBenchLines(b, path, large.old)
			runBenchGit(b, h.dir, "add", "-A")
			runBenchGit(b, h.dir, "commit", "-qm", "base")
			writeBenchLines(b, path, large.new)
			status := git.FileStatus{Status: "M", Path: diffBenchFile}
			b.ReportAllocs()
			for b.Loop() {
				h.app.OpenChangeDiff(h.dir, status, false, false)
				h.redraw()
				b.StopTimer()
				h.exec("tab.close")
				b.StartTimer()
			}
		})
	})

	b.Run("commit_detail", func(b *testing.B) {
		setup := func(b *testing.B) (*testHarness, string) {
			h := newBenchHarness(b)
			setDiffView(h, config.DiffModeSplit, changes, false)
			initBenchRepo(b, h.dir)
			for i := range commitDetailFiles() {
				writeBenchLines(b, filepath.Join(h.dir, fmt.Sprintf("file%03d.go", i)), medium.old[:200])
			}
			runBenchGit(b, h.dir, "add", "-A")
			runBenchGit(b, h.dir, "commit", "-qm", "base")
			for i := range commitDetailFiles() {
				writeBenchLines(b, filepath.Join(h.dir, fmt.Sprintf("file%03d.go", i)), changeLines(medium.old[:200]))
			}
			runBenchGit(b, h.dir, "add", "-A")
			runBenchGit(b, h.dir, "commit", "-qm", "change every file")
			return h, runBenchGit(b, h.dir, "rev-parse", "HEAD")
		}

		b.Run("open_many_files", func(b *testing.B) {
			h, ref := setup(b)
			b.ReportAllocs()
			for b.Loop() {
				h.app.OpenCommitDetail(h.dir, ref, ref[:7])
				awaitBenchCommitDetail(b, h)
				b.StopTimer()
				h.exec("tab.close")
				b.StartTimer()
			}
		})

		b.Run("wheel_scroll", func(b *testing.B) {
			h, ref := setup(b)
			h.app.OpenCommitDetail(h.dir, ref, ref[:7])
			awaitBenchCommitDetail(b, h)
			steps := 0
			b.ReportAllocs()
			for b.Loop() {
				if steps == diffBenchScrollReset {
					b.StopTimer()
					diffToTop(h)
					steps = 0
					b.StartTimer()
				}
				diffWheel(h)
				steps++
			}
		})
	})
}
