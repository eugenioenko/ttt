package e2e

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/update"
	"github.com/gdamore/tcell/v3"
)

type fakeUpdater struct {
	latest func(context.Context) (update.Release, error)
	apply  func(context.Context, update.Release, update.Method) error
}

func (f fakeUpdater) Latest(ctx context.Context) (update.Release, error) { return f.latest(ctx) }
func (f fakeUpdater) Apply(ctx context.Context, r update.Release, m update.Method) error {
	return f.apply(ctx, r, m)
}

func updateHarness(t *testing.T, kind update.Kind) *testHarness {
	h := newTestHarness(t, 100, 35)
	t.Cleanup(h.stop)
	h.app.Version = "v1.0.0"
	h.app.UpdateMethod = &update.Method{Kind: kind, Path: "/ttt"}
	return h
}

func TestUpdateDialogResults(t *testing.T) {
	for _, tc := range []struct {
		name, version, tag, want string
		kind                     update.Kind
	}{
		{"latest", "v1.0.0", "v1.0.0", "ttt v1.0.0 is the latest version.", update.Binary},
		{"newer installed", "v2.0.0", "v1.0.0", "ttt v2.0.0 is the latest version.", update.Binary},
		{"available", "v1.0.0", "v2.0.0", "ttt v2.0.0 is available (you have v1.0.0).", update.Binary},
		{"nix", "v1.0.0", "v2.0.0", "Update ttt through your Nix configuration", update.Nix},
		{"dev", "dev", "v2.0.0", "This is a development build.", update.Binary},
		{"git describe", "v1.0.0-12-gabc", "v1.0.0", "This is a development build.", update.Binary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := updateHarness(t, tc.kind)
			h.app.Version = tc.version
			checks := 0
			h.app.Updates = fakeUpdater{latest: func(context.Context) (update.Release, error) {
				checks++
				h.redraw()
				h.assertContains("Checking for updates…")
				h.exec("update.check")
				return update.Release{Tag: tc.tag, Notes: "# Latest notes\nOnly this release."}, nil
			}}
			h.exec("update.check")
			h.assertContains(tc.want)
			if checks != 1 {
				t.Fatal("duplicate command started another check")
			}
			if tc.name == "available" || tc.name == "nix" || tc.name == "dev" || tc.name == "git describe" {
				h.assertContains("Only this release.")
			}
			h.pressKey(tcell.KeyEscape, 0)
			if h.app.Root.HasOverlay() {
				t.Fatal("Escape left update dialog open")
			}
		})
	}
}

func TestUpdateDialogRetryAndCancel(t *testing.T) {
	h := updateHarness(t, update.Binary)
	checks := 0
	h.app.Updates = fakeUpdater{latest: func(context.Context) (update.Release, error) {
		checks++
		if checks == 1 {
			return update.Release{}, errors.New("HTTP 403: rate limited")
		}
		return update.Release{Tag: "v1.0.0"}, nil
	}}
	h.exec("update.check")
	h.assertContains("HTTP 403: rate limited")
	h.assertContains("Retry")
	h.pressKey(tcell.KeyEnter, 0)
	h.assertContains("ttt v1.0.0 is the latest version.")
	h.pressKey(tcell.KeyEscape, 0)
	h.app.Updates = fakeUpdater{latest: func(ctx context.Context) (update.Release, error) {
		h.pressKey(tcell.KeyEscape, 0)
		if ctx.Err() != context.Canceled {
			t.Fatal("Cancel did not cancel lookup")
		}
		return update.Release{Tag: "v2.0.0"}, nil
	}}
	h.exec("update.check")
	if h.app.Root.HasOverlay() {
		t.Fatal("canceled lookup reopened the dialog")
	}
}

func TestUpdateDialogApplyAndRestart(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failure], func(t *testing.T) {
			h := updateHarness(t, update.Binary)
			h.app.Updates = fakeUpdater{
				latest: func(context.Context) (update.Release, error) { return update.Release{Tag: "v2.0.0"}, nil },
				apply: func(_ context.Context, r update.Release, m update.Method) error {
					if r.Tag != "v2.0.0" || m.Path != "/ttt" {
						t.Fatal("incorrect update target")
					}
					h.redraw()
					h.assertContains("Updating…")
					h.pressKey(tcell.KeyEscape, 0)
					h.exec("editor.quit")
					if !h.app.Root.HasOverlay() || !*h.app.Running {
						t.Fatal("update was interrupted")
					}
					if failure {
						return errors.New("command failed\nlast output line")
					}
					return nil
				},
			}
			h.exec("update.check")
			h.pressKey(tcell.KeyEnter, 0)
			if failure {
				h.assertContains("Update failed.")
				h.assertContains("last output line")
				h.pressKey(tcell.KeyEscape, 0)
				return
			}
			h.assertContains("Updated to v2.0.0.")
			h.pressKey(tcell.KeyEnter, 0)
			if !h.app.RestartRequested || *h.app.Running {
				t.Fatal("Restart did not request shutdown")
			}
		})
	}
}

func TestUpdateRestartUnsavedConfirmation(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "confirm", true: "cancel"}[cancel], func(t *testing.T) {
			h := updateHarness(t, update.Binary)
			h.exec("file.new")
			h.pressRune('x')
			h.app.Updates = fakeUpdater{
				latest: func(context.Context) (update.Release, error) { return update.Release{Tag: "v2.0.0"}, nil },
				apply:  func(context.Context, update.Release, update.Method) error { return nil },
			}
			h.exec("update.check")
			h.pressKey(tcell.KeyEnter, 0)
			h.pressKey(tcell.KeyEnter, 0)
			h.assertContains("Unsaved changes")
			if h.app.RestartRequested || !*h.app.Running {
				t.Fatal("restart bypassed unsaved confirmation")
			}
			if cancel {
				h.pressKey(tcell.KeyEscape, 0)
				h.exec("editor.quit")
				h.exec("editor.quit")
				if h.app.RestartRequested {
					t.Fatal("ordinary Quit retained canceled restart")
				}
			} else {
				h.exec("editor.quit")
				if !h.app.RestartRequested || *h.app.Running {
					t.Fatal("confirmed restart did not quit")
				}
			}
		})
	}
}

func TestUpdateNotesScroll(t *testing.T) {
	h := updateHarness(t, update.Binary)
	h.app.Updates = fakeUpdater{latest: func(context.Context) (update.Release, error) {
		return update.Release{Tag: "v2.0.0", Notes: "# Latest notes\n" + strings.Repeat("- Release detail\n", 40) + "\nFinal release note"}, nil
	}}
	h.exec("update.check")
	h.assertNotContains("Final release note")
	for range 30 {
		h.app.Root.HandleEvent(tcell.NewEventMouse(50, 12, tcell.WheelDown, 0))
	}
	h.redraw()
	h.assertContains("Final release note")
	h.assertContains("Update")
	h.assertContains("Cancel")
	h.screen.SetSize(55, 20)
	h.app.Root.SetSize(55, 20)
	h.redraw()
	h.assertContains("Update")
	h.assertContains("Cancel")
}

func TestUpdateRestartSaveAll(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "save", true: "cancel save"}[cancel], func(t *testing.T) {
			h := updateHarness(t, update.Binary)
			h.app.Settings.Editor.SaveAllOnQuit = true
			h.exec("file.new")
			h.pressRune('x')
			h.app.Updates = fakeUpdater{
				latest: func(context.Context) (update.Release, error) { return update.Release{Tag: "v2.0.0"}, nil },
				apply:  func(context.Context, update.Release, update.Method) error { return nil },
			}
			h.exec("update.check")
			h.pressKey(tcell.KeyEnter, 0)
			h.pressKey(tcell.KeyEnter, 0)
			h.pressRune('s')
			if cancel {
				h.pressKey(tcell.KeyEscape, 0)
				h.exec("editor.quit")
				h.exec("editor.quit")
				if h.app.RestartRequested {
					t.Fatal("canceling Save As retained restart intent")
				}
				return
			}
			path := filepath.Join(h.dir, "saved.txt")
			h.typeText(path)
			h.pressKey(tcell.KeyEnter, 0)
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "x\n" || !h.app.RestartRequested || *h.app.Running {
				t.Fatalf("save/restart: %q, %v, restart=%v", data, err, h.app.RestartRequested)
			}
		})
	}
}
