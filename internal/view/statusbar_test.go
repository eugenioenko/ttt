package view

import (
	"testing"
	"time"
)

func TestSetSegment(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSegment(StatusSegment{ID: "branch", Side: "left", Priority: 100, Text: "main"})
	sb.SetSegment(StatusSegment{ID: "pos", Side: "right", Priority: 100, Text: "Ln 1, Col 1"})

	left := sb.LeftSegments()
	if len(left) != 1 || left[0].Text != "main" {
		t.Errorf("expected left segment 'main', got %v", left)
	}
	right := sb.RightSegments()
	if len(right) != 1 || right[0].Text != "Ln 1, Col 1" {
		t.Errorf("expected right segment 'Ln 1, Col 1', got %v", right)
	}
}

func TestSetSegmentUpdate(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSegment(StatusSegment{ID: "pos", Side: "right", Priority: 100, Text: "Ln 1"})
	sb.SetSegment(StatusSegment{ID: "pos", Side: "right", Priority: 100, Text: "Ln 5"})

	right := sb.RightSegments()
	if len(right) != 1 || right[0].Text != "Ln 5" {
		t.Errorf("expected updated text 'Ln 5', got %v", right)
	}
}

func TestRemoveSegment(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSegment(StatusSegment{ID: "a", Side: "left", Priority: 100, Text: "A"})
	sb.SetSegment(StatusSegment{ID: "b", Side: "left", Priority: 200, Text: "B"})
	sb.RemoveSegment("a")

	left := sb.LeftSegments()
	if len(left) != 1 || left[0].ID != "b" {
		t.Errorf("expected only segment 'b', got %v", left)
	}
}

func TestRemoveSegmentsByOwner(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSegment(StatusSegment{ID: "core", Side: "left", Priority: 100, Text: "core"})
	sb.SetSegment(StatusSegment{ID: "probe:a", Side: "left", Priority: 200, Text: "A", Owner: "plugin:probe"})
	sb.SetSegment(StatusSegment{ID: "probe:b", Side: "right", Priority: 200, Text: "B", Owner: "plugin:probe"})
	sb.SetSegment(StatusSegment{ID: "other:a", Side: "left", Priority: 300, Text: "O", Owner: "plugin:other"})
	_ = sb.LeftSegments()

	sb.RemoveSegmentsByOwner("plugin:probe")

	left := sb.LeftSegments()
	if len(left) != 2 || left[0].ID != "core" || left[1].ID != "other:a" {
		t.Errorf("expected core and other:a on the left, got %v", left)
	}
	if right := sb.RightSegments(); len(right) != 0 {
		t.Errorf("expected no right segments, got %v", right)
	}

	sb.RemoveSegmentsByOwner("")
	if left := sb.LeftSegments(); len(left) != 2 {
		t.Errorf("an empty owner must not remove core segments, got %v", left)
	}
}

func TestRemoveSegmentsByPrefix(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSegment(StatusSegment{ID: "core", Side: "left", Priority: 100, Text: "core"})
	sb.SetSegment(StatusSegment{ID: "probe:a", Side: "left", Priority: 200, Text: "A"})
	sb.SetSegment(StatusSegment{ID: "probe:b", Side: "right", Priority: 200, Text: "B"})
	sb.SetSegment(StatusSegment{ID: "other:a", Side: "left", Priority: 300, Text: "O"})
	_ = sb.LeftSegments()

	sb.RemoveSegmentsByPrefix("probe:")

	left := sb.LeftSegments()
	if len(left) != 2 || left[0].ID != "core" || left[1].ID != "other:a" {
		t.Errorf("expected core and other:a on the left, got %v", left)
	}
	if right := sb.RightSegments(); len(right) != 0 {
		t.Errorf("expected no right segments, got %v", right)
	}

	sb.RemoveSegmentsByPrefix("")
	if left := sb.LeftSegments(); len(left) != 2 {
		t.Errorf("an empty prefix must not remove segments, got %v", left)
	}
}

func TestSegmentPrioritySorting(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSegment(StatusSegment{ID: "c", Side: "left", Priority: 300, Text: "C"})
	sb.SetSegment(StatusSegment{ID: "a", Side: "left", Priority: 100, Text: "A"})
	sb.SetSegment(StatusSegment{ID: "b", Side: "left", Priority: 200, Text: "B"})

	left := sb.LeftSegments()
	if len(left) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(left))
	}
	if left[0].ID != "a" || left[1].ID != "b" || left[2].ID != "c" {
		t.Errorf("expected order a,b,c got %s,%s,%s", left[0].ID, left[1].ID, left[2].ID)
	}
}

func TestSetNotification(t *testing.T) {
	sb := NewStatusBar()
	sb.SetNotification("saved", NotifyInfo, 3*time.Second)

	if sb.Notification != "saved" {
		t.Errorf("expected Notification 'saved', got %q", sb.Notification)
	}
	if sb.NotifyLevel != NotifyInfo {
		t.Errorf("expected NotifyLevel NotifyInfo, got %d", sb.NotifyLevel)
	}
	if sb.NotifyExpiry.IsZero() {
		t.Error("expected NotifyExpiry to be set")
	}
	if sb.NotifyAction != nil {
		t.Error("expected NotifyAction to be nil after SetNotification")
	}
	if sb.ActionLabel != "" {
		t.Errorf("expected ActionLabel to be empty, got %q", sb.ActionLabel)
	}
}

func TestSetNotificationClearsAction(t *testing.T) {
	sb := NewStatusBar()
	called := false
	sb.SetNotificationWithAction("error", NotifyError, 5*time.Second, "Retry", func() { called = true })

	sb.SetNotification("info", NotifyInfo, 2*time.Second)

	if sb.NotifyAction != nil {
		t.Error("expected NotifyAction to be nil after SetNotification overwrites action notification")
	}
	if sb.ActionLabel != "" {
		t.Errorf("expected ActionLabel to be empty, got %q", sb.ActionLabel)
	}
	if called {
		t.Error("action should not have been called")
	}
}

func TestSetNotificationWithAction(t *testing.T) {
	sb := NewStatusBar()
	called := false
	action := func() { called = true }
	sb.SetNotificationWithAction("failed", NotifyError, 10*time.Second, "Retry", action)

	if sb.Notification != "failed" {
		t.Errorf("expected Notification 'failed', got %q", sb.Notification)
	}
	if sb.NotifyLevel != NotifyError {
		t.Errorf("expected NotifyLevel NotifyError, got %d", sb.NotifyLevel)
	}
	if sb.ActionLabel != "Retry" {
		t.Errorf("expected ActionLabel 'Retry', got %q", sb.ActionLabel)
	}
	if sb.NotifyAction == nil {
		t.Fatal("expected NotifyAction to be set")
	}

	sb.NotifyAction()
	if !called {
		t.Error("expected action to be called")
	}
}

func TestDismissNotification(t *testing.T) {
	sb := NewStatusBar()
	sb.SetNotificationWithAction("error", NotifyError, 10*time.Second, "Fix", func() {})
	sb.SecondaryLabel = "Details"
	sb.SecondaryAction = func() {}

	sb.DismissNotification()

	if sb.Notification != "" {
		t.Errorf("expected Notification to be empty, got %q", sb.Notification)
	}
	if !sb.NotifyExpiry.IsZero() {
		t.Error("expected NotifyExpiry to be zero after dismiss")
	}
	if sb.NotifyAction != nil {
		t.Error("expected NotifyAction to be nil after dismiss")
	}
	if sb.ActionLabel != "" {
		t.Errorf("expected ActionLabel to be empty, got %q", sb.ActionLabel)
	}
	if sb.SecondaryAction != nil {
		t.Error("expected SecondaryAction to be nil after dismiss")
	}
	if sb.SecondaryLabel != "" {
		t.Errorf("expected SecondaryLabel to be empty, got %q", sb.SecondaryLabel)
	}
}

func TestIsNotificationActive_NoNotification(t *testing.T) {
	sb := NewStatusBar()
	if sb.IsNotificationActive() {
		t.Error("expected no active notification for empty StatusBar")
	}
}

func TestIsNotificationActive_NotExpired(t *testing.T) {
	sb := NewStatusBar()
	sb.SetNotification("hello", NotifyInfo, 10*time.Second)

	if !sb.IsNotificationActive() {
		t.Error("expected notification to be active (not expired)")
	}
}

func TestIsNotificationActive_Expired(t *testing.T) {
	sb := NewStatusBar()
	sb.SetNotification("old", NotifyWarning, -1*time.Second)

	if sb.IsNotificationActive() {
		t.Error("expected notification to be inactive (expired)")
	}
	if sb.Notification != "" {
		t.Errorf("expected Notification to be cleared after expiry check, got %q", sb.Notification)
	}
}

func TestIsNotificationActive_ZeroExpiry(t *testing.T) {
	sb := NewStatusBar()
	sb.Notification = "permanent"
	sb.NotifyExpiry = time.Time{}

	if !sb.IsNotificationActive() {
		t.Error("expected notification with zero expiry to remain active")
	}
}

func TestNotifyLevel_Style(t *testing.T) {
	infoStyle := NotifyInfo.Style()
	warningStyle := NotifyWarning.Style()
	errorStyle := NotifyError.Style()

	if warningStyle == infoStyle {
		t.Error("expected NotifyWarning style to differ from NotifyInfo")
	}
	if errorStyle == infoStyle {
		t.Error("expected NotifyError style to differ from NotifyInfo")
	}
	if errorStyle == warningStyle {
		t.Error("expected NotifyError style to differ from NotifyWarning")
	}
}
