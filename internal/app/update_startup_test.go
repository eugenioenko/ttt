package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/update"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type recordingUpdater struct {
	calls   atomic.Int32
	release update.Release
	err     error
	applied atomic.Int32
}

func (u *recordingUpdater) Latest(context.Context) (update.Release, error) {
	u.calls.Add(1)
	return u.release, u.err
}

func (u *recordingUpdater) Apply(context.Context, update.Release, update.Method) error {
	u.applied.Add(1)
	return nil
}

func startupTestApp(t *testing.T, settings config.Settings) *App {
	t.Helper()
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, settings)
	t.Cleanup(a.CloseTasks)
	a.Version = "v1.0.0"
	a.UpdateMethod = &update.Method{Kind: update.Binary}
	return a
}

func deliverStartupLookup(t *testing.T, posted <-chan func()) {
	t.Helper()
	select {
	case deliver := <-posted:
		deliver()
	case <-time.After(time.Second):
		t.Fatal("startup lookup did not post delivery")
	}
}

func TestCheckForUpdatesOnStartupDefaultDisabled(t *testing.T) {
	a := startupTestApp(t, config.DefaultSettings())
	u := &recordingUpdater{release: update.Release{Tag: "v2.0.0"}}
	a.Updates = u
	a.CheckForUpdatesOnStartup(time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC))

	if a.updateStartupTask != nil {
		t.Fatal("disabled default created a startup task")
	}
	if a.updateDialog != nil {
		t.Fatal("disabled default opened an update dialog")
	}
}

func TestCheckForUpdatesOnStartupShowsAvailableRelease(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Update.StartupCheck = true
	a := startupTestApp(t, settings)
	u := &recordingUpdater{release: update.Release{Tag: "v2.0.0", Notes: "notes"}}
	a.Updates = u
	posted := make(chan func(), 1)
	a.tasks.post = func(fn func()) error { posted <- fn; return nil }
	now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)

	a.CheckForUpdatesOnStartup(now)
	deliverStartupLookup(t, posted)

	if u.calls.Load() != 1 {
		t.Fatalf("lookup calls = %d, want 1", u.calls.Load())
	}
	if u.applied.Load() != 0 {
		t.Fatal("startup check applied an update")
	}
	if a.updateDialog == nil || a.updateDialog.release.Tag != "v2.0.0" {
		t.Fatal("startup check did not show the available release")
	}
	got := config.LoadState().LastUpdateCheckTime()
	if !got.Equal(now) {
		t.Fatalf("last check = %v, want %v", got, now)
	}
}

func TestCheckForUpdatesOnStartupRespectsDailyFrequency(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Update.StartupCheck = true
	settings.Update.StartupCheckInterval = config.UpdateCheckDaily
	a := startupTestApp(t, settings)
	now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)
	state := a.State
	state.SetLastUpdateCheckTime(now.Add(-time.Hour))
	if err := config.SaveState(state); err != nil {
		t.Fatal(err)
	}
	a.State = state
	u := &recordingUpdater{release: update.Release{Tag: "v2.0.0"}}
	a.Updates = u

	a.CheckForUpdatesOnStartup(now)

	if a.updateStartupTask != nil {
		t.Fatal("daily window created a startup task")
	}
	if a.updateDialog != nil {
		t.Fatal("daily window opened an update dialog")
	}
}

func TestCheckForUpdatesOnStartupEveryLaunchIgnoresRecentCheck(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Update.StartupCheck = true
	settings.Update.StartupCheckInterval = config.UpdateCheckEveryLaunch
	a := startupTestApp(t, settings)
	now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)
	state := a.State
	state.SetLastUpdateCheckTime(now.Add(-time.Minute))
	if err := config.SaveState(state); err != nil {
		t.Fatal(err)
	}
	a.State = state
	u := &recordingUpdater{release: update.Release{Tag: "v1.0.0"}}
	a.Updates = u
	posted := make(chan func(), 1)
	a.tasks.post = func(fn func()) error { posted <- fn; return nil }

	a.CheckForUpdatesOnStartup(now)
	deliverStartupLookup(t, posted)

	if u.calls.Load() != 1 {
		t.Fatalf("every-launch lookup calls = %d, want 1", u.calls.Load())
	}
	if a.updateDialog != nil {
		t.Fatal("already-latest startup check opened a dialog")
	}
}

func TestCheckForUpdatesOnStartupStaysQuietOnError(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Update.StartupCheck = true
	a := startupTestApp(t, settings)
	u := &recordingUpdater{err: errors.New("network down")}
	a.Updates = u
	posted := make(chan func(), 1)
	a.tasks.post = func(fn func()) error { posted <- fn; return nil }
	now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)

	a.CheckForUpdatesOnStartup(now)
	deliverStartupLookup(t, posted)

	if u.calls.Load() != 1 {
		t.Fatalf("lookup calls = %d, want 1", u.calls.Load())
	}
	if a.updateDialog != nil {
		t.Fatal("failed startup check opened a dialog")
	}
	got := config.LoadState().LastUpdateCheckTime()
	if !got.Equal(now) {
		t.Fatalf("failed check was not recorded; last = %v", got)
	}
}

func TestCheckForUpdatesOnStartupSkipsOpenModal(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Update.StartupCheck = true
	a := startupTestApp(t, settings)
	u := &recordingUpdater{release: update.Release{Tag: "v2.0.0"}}
	a.Updates = u
	a.ShowDialog(ui.NewWidgetAdapter(widgets.NewDialogWidget(20)))
	now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)

	a.CheckForUpdatesOnStartup(now)

	if a.updateStartupTask != nil {
		t.Fatal("open modal created a startup task")
	}
	if !config.LoadState().LastUpdateCheckTime().IsZero() {
		t.Fatal("open modal recorded a startup check")
	}
}

type firstBlockedUpdater struct {
	started chan context.Context
	release chan struct{}
	calls   atomic.Int32
	tag     string
	applied atomic.Int32
}

func (u *firstBlockedUpdater) Latest(ctx context.Context) (update.Release, error) {
	n := u.calls.Add(1)
	if n == 1 {
		u.started <- ctx
		<-u.release
	}
	return update.Release{Tag: u.tag}, nil
}

func (u *firstBlockedUpdater) Apply(context.Context, update.Release, update.Method) error {
	u.applied.Add(1)
	return nil
}

func TestCheckForUpdatesOnStartupLeavesManualDialogAlone(t *testing.T) {
	for _, dismiss := range []bool{false, true} {
		name := "open"
		if dismiss {
			name = "dismissed"
		}
		t.Run(name, func(t *testing.T) {
			settings := config.DefaultSettings()
			settings.Update.StartupCheck = true
			a := startupTestApp(t, settings)
			u := &recordingUpdater{release: update.Release{Tag: "v2.0.0"}}
			a.Updates = u
			posted := make(chan func(), 1)
			a.tasks.post = func(fn func()) error { posted <- fn; return nil }
			now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)

			a.CheckForUpdatesOnStartup(now)
			var deliverStartup func()
			select {
			case deliverStartup = <-posted:
			case <-time.After(time.Second):
				t.Fatal("startup lookup did not post delivery")
			}
			a.CheckForUpdates()
			deliverStartupLookup(t, posted)
			manual := a.updateDialog
			if manual == nil || manual.release.Tag != "v2.0.0" {
				t.Fatal("manual check did not show the available release")
			}
			if dismiss {
				a.closeUpdateDialog()
				manual = nil
			}

			deliverStartup()

			if a.updateDialog != manual {
				t.Fatal("startup result replaced or reopened the manual dialog")
			}
			if u.applied.Load() != 0 {
				t.Fatal("startup or manual path applied an update")
			}
			if !config.LoadState().LastUpdateCheckTime().IsZero() {
				t.Fatal("abandoned startup lookup recorded a check time")
			}
		})
	}
}

func TestCheckForUpdatesOnStartupDoesNotRecordWhenResultHitsAModal(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Update.StartupCheck = true
	a := startupTestApp(t, settings)
	u := &firstBlockedUpdater{started: make(chan context.Context, 1), release: make(chan struct{}), tag: "v2.0.0"}
	a.Updates = u
	posted := make(chan func(), 1)
	a.tasks.post = func(fn func()) error { posted <- fn; return nil }
	now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)

	a.CheckForUpdatesOnStartup(now)
	select {
	case <-u.started:
	case <-time.After(time.Second):
		t.Fatal("startup lookup did not start")
	}
	a.ShowDialog(ui.NewWidgetAdapter(widgets.NewDialogWidget(20)))
	close(u.release)
	deliverStartupLookup(t, posted)

	if a.updateDialog != nil {
		t.Fatal("startup check replaced the open modal")
	}
	if !config.LoadState().LastUpdateCheckTime().IsZero() {
		t.Fatal("blocked update was recorded; the next launch should retry")
	}
}
