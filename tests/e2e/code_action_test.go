package e2e

import "testing"

func TestCodeAction_CommandRegistered(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	cmd, ok := h.reg.Get("editor.codeAction")
	if !ok {
		t.Fatal("editor.codeAction command not registered")
	}
	if cmd.Title != "Code Action" {
		t.Errorf("expected title 'Code Action', got %q", cmd.Title)
	}
}

func TestCodeAction_NoLSP_NoPanic(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	h.exec("editor.codeAction")
	h.redraw()
}
