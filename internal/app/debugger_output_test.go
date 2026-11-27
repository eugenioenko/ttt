package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
)

func TestDebugOutputPreservesFragmentsAcrossEvents(t *testing.T) {
	a := &App{Output: ui.NewOutputWidget()}
	r := &debugRun{}
	a.appendDebugOutput(r, "probe", "stdout", "result")
	a.appendDebugOutput(r, "probe", "stderr", "warning\n")
	a.appendDebugOutput(r, "probe", "stdout", " 42\r\nlast")
	a.flushDebugOutput(r, "probe")
	lines := a.Output.Lines
	if len(lines) != 3 || lines[0].Message != "warning" || lines[1].Message != "result 42" || lines[2].Message != "last" {
		t.Fatalf("unexpected output: %+v", lines)
	}
}
