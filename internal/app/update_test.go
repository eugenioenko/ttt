package app

import (
	"context"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/update"
)

type blockedUpdater struct {
	started chan context.Context
	release chan struct{}
}

func (u blockedUpdater) Latest(ctx context.Context) (update.Release, error) {
	u.started <- ctx
	<-u.release
	return update.Release{Tag: "v2.0.0"}, nil
}

func (u blockedUpdater) Apply(context.Context, update.Release, update.Method) error { return nil }

func TestUpdateLookupDeliveryAndCancellation(t *testing.T) {
	for _, action := range []string{"deliver", "cancel", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			a := buildTestApp(t, config.DefaultSettings())
			t.Cleanup(a.CloseTasks)
			a.Version = "v1.0.0"
			a.UpdateMethod = &update.Method{Kind: update.Binary}
			posted := make(chan func(), 1)
			a.tasks.post = func(fn func()) error { posted <- fn; return nil }
			u := blockedUpdater{started: make(chan context.Context, 1), release: make(chan struct{})}
			a.Updates = u
			a.CheckForUpdates()
			var ctx context.Context
			select {
			case ctx = <-u.started:
			case <-time.After(time.Second):
				t.Fatal("lookup did not start")
			}
			s := a.updateDialog
			if len(s.dialog.Buttons) != 1 {
				t.Fatal("worker changed checking state")
			}
			close(u.release)
			var deliver func()
			select {
			case deliver = <-posted:
			case <-time.After(time.Second):
				t.Fatal("lookup did not post delivery")
			}
			if s.release.Tag != "" {
				t.Fatal("worker mutated dialog outside event loop")
			}
			switch action {
			case "cancel":
				a.closeUpdateDialog()
			case "shutdown":
				a.CloseTasks()
			}
			deliver()
			if action == "deliver" {
				if s.release.Tag != "v2.0.0" {
					t.Fatal("main-thread delivery did not apply release")
				}
			} else if ctx.Err() != context.Canceled || s.release.Tag != "" {
				t.Fatal("canceled lookup applied its result")
			}
		})
	}
}
