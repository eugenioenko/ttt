package ui

import (
	"errors"
	"os/exec"
	"testing"
)

func TestRgFailureIgnoresNoMatches(t *testing.T) {
	// rg exits 1 when it finds nothing, which is an empty result, not a failure.
	err := exec.Command("sh", "-c", "exit 1").Run()
	if err == nil {
		t.Fatal("expected a non-nil error from exit 1")
	}
	if got := rgFailure(err); got != "" {
		t.Errorf("rgFailure(exit 1) = %q, want empty", got)
	}
}

func TestRgFailureReportsRealFailures(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 2").Run()
	if err == nil {
		t.Fatal("expected a non-nil error from exit 2")
	}
	if got := rgFailure(err); got == "" {
		t.Error("rgFailure(exit 2) = empty, want a message")
	}

	if got := rgFailure(errors.New("boom")); got != "search failed: boom" {
		t.Errorf("rgFailure(boom) = %q", got)
	}
	if got := rgFailure(nil); got != "" {
		t.Errorf("rgFailure(nil) = %q, want empty", got)
	}
}

func TestApplyBatchSurfacesError(t *testing.T) {
	s := NewSearchWidget()
	s.searchGen = 7

	s.ApplyBatch(&SearchBatch{Gen: 7, Done: true, Error: "ripgrep (rg) not found"})
	if s.Error != "ripgrep (rg) not found" {
		t.Fatalf("Error = %q, want the rg message", s.Error)
	}
	if s.Searching {
		t.Error("expected Searching to be cleared")
	}
}

func TestApplyBatchIgnoresStaleGeneration(t *testing.T) {
	s := NewSearchWidget()
	s.searchGen = 8

	s.ApplyBatch(&SearchBatch{Gen: 7, Done: true, Error: "stale"})
	if s.Error != "" {
		t.Errorf("Error = %q, want empty for a stale batch", s.Error)
	}
}

// A partial batch must not wipe a message set by the run that follows it.
func TestApplyBatchKeepsErrorUntilDone(t *testing.T) {
	s := NewSearchWidget()
	s.searchGen = 3
	s.Error = "previous"

	s.ApplyBatch(&SearchBatch{Gen: 3})
	if s.Error != "previous" {
		t.Errorf("Error = %q, want it untouched by a partial batch", s.Error)
	}

	s.ApplyBatch(&SearchBatch{Gen: 3, Done: true})
	if s.Error != "" {
		t.Errorf("Error = %q, want cleared by a successful final batch", s.Error)
	}
}

func TestSearchStartClearsError(t *testing.T) {
	s := NewSearchWidget()
	s.Error = "ripgrep (rg) not found"
	s.Input.Text = ""

	s.runSearchSync()
	if s.Error != "" {
		t.Errorf("Error = %q, want cleared when a new search starts", s.Error)
	}
}
