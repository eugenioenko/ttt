package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

// Repro test for BUG-004 from audit/2026-07-12-ux-bug-audit.md (branch audit/bug-hunt).
// Asserts the CORRECT behavior and is skipped while the bug exists —
// remove the t.Skip when fixing, and delete the audit entry.
//
// Lives in e2e (not functional) because the --exec harness cannot
// synthesize tcell.KeyBacktab; only direct event injection reaches
// the KeyBacktab branch in editor_widget_keyboard.go.
func TestBacktabExcludesColZeroSelectionLine(t *testing.T) {
	// BUG-004 fixed: outdent now applies col-0 convention via selectedLineRange()

	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "outdent.txt")
	requireNoError(t, os.WriteFile(f, []byte("    aaa\n    bbb\n    ccc\n"), 0644))

	h.openFile(f)
	h.redraw()

	// Select line0 only: selection ends at line1 col 0, which per the
	// col-0 convention (JoinLines/ToggleLineComment) excludes line1.
	h.pressKey(tcell.KeyDown, tcell.ModShift)
	h.redraw()
	h.pressKey(tcell.KeyBacktab, 0)
	h.redraw()

	lines := h.app.EditorGroup.Editor.Buf.Lines
	if lines[0] != "aaa" {
		t.Errorf("expected line0 outdented to %q, got %q", "aaa", lines[0])
	}
	if lines[1] != "    bbb" {
		t.Errorf("expected line1 untouched (%q), got %q", "    bbb", lines[1])
	}
}
