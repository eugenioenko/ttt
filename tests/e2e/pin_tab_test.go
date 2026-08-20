package e2e

import (
	"path/filepath"
	"testing"
)

func TestPinTab(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "beta.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "gamma.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.redraw()

	if h.app.EditorGroup.IsActiveTabPinned() {
		t.Fatal("tab should not be pinned initially")
	}

	h.exec("tab.pin")

	if !h.app.EditorGroup.IsActiveTabPinned() {
		t.Fatal("tab should be pinned after toggle")
	}
	h.assertContains("♦")
}

func TestPinTabMovesToFront(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "beta.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "gamma.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.redraw()

	// gamma is tab 3 (last), pin it
	h.exec("tab.pin")

	// After pin, gamma should be at index 0
	path0, _ := h.app.EditorGroup.TabInfo(0)
	if !contains(path0, "gamma") {
		t.Fatalf("pinned gamma should be at tab index 0, got %s", path0)
	}
	if !h.app.EditorGroup.IsActiveTabPinned() {
		t.Fatal("active tab should be pinned")
	}
}

func TestUnpinTab(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.redraw()

	h.exec("tab.pin")
	if !h.app.EditorGroup.IsActiveTabPinned() {
		t.Fatal("tab should be pinned")
	}

	h.exec("tab.pin")
	if h.app.EditorGroup.IsActiveTabPinned() {
		t.Fatal("tab should be unpinned after second toggle")
	}
	h.assertNotContains("♦")
}

func TestPinMultipleTabs(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "beta.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "gamma.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.redraw()

	// Pin gamma (last/active tab) — moves to front
	h.exec("tab.pin")

	// After pin: [gamma(pinned), untitled, alpha, beta]
	// Switch to beta (index 3) and pin it
	h.app.EditorGroup.SwitchTab(3)
	h.redraw()
	h.exec("tab.pin")

	// Verify via TabInfo: first two tabs should be pinned
	path0, _ := h.app.EditorGroup.TabInfo(0)
	path1, _ := h.app.EditorGroup.TabInfo(1)
	if !contains(path0, "gamma") {
		t.Fatalf("tab 0 should be gamma, got %s", path0)
	}
	if !contains(path1, "beta") {
		t.Fatalf("tab 1 should be beta, got %s", path1)
	}
	if !h.app.EditorGroup.IsActiveTabPinned() {
		t.Fatal("active tab (beta) should be pinned")
	}
}

func contains(s, sub string) bool {
	return indexOf(s, sub) >= 0
}

func TestCloseTabDecrementsPin(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "beta.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.redraw()

	h.exec("tab.pin")
	h.exec("tab.close")

	// After closing the pinned tab, the remaining tab should not be pinned
	if h.app.EditorGroup.IsActiveTabPinned() {
		t.Fatal("after closing pinned tab, remaining tab should not be pinned")
	}
}

func TestCloseOtherTabsKeepsPinned(t *testing.T) {
	h := newTestHarness(t, 80, 24)

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "beta.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "gamma.txt"))
	h.app.EditorGroup.CommitActiveTab()
	h.redraw()

	// Pin alpha: switch to it and pin
	h.app.EditorGroup.SwitchTab(0)
	h.redraw()
	h.exec("tab.pin")

	// Switch to gamma (now unpinned, at end)
	h.app.EditorGroup.SwitchTab(2)
	h.redraw()

	// Close others should keep pinned alpha
	h.app.EditorGroup.CloseOtherTabs()
	h.redraw()

	h.assertContains("alpha")
	h.assertContains("gamma")
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
