package e2e

import "testing"

func requireNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (h *testHarness) openFile(path string) {
	h.t.Helper()
	if err := h.app.EditorGroup.OpenFile(path); err != nil {
		h.t.Fatalf("open %s: %v", path, err)
	}
}
