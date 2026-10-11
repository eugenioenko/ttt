package highlight

import (
	"testing"
	"time"
)

func TestSetTimeLimit(t *testing.T) {
	t.Cleanup(func() { SetTimeLimit(0) })
	SetTimeLimit(2 * time.Second)
	if tokenizeOptions.TimeLimit != 2*time.Second {
		t.Fatalf("time limit = %v, want 2s", tokenizeOptions.TimeLimit)
	}
	SetTimeLimit(0)
	if tokenizeOptions.TimeLimit != tokenizeTimeLimit {
		t.Fatalf("time limit after reset = %v, want the default %v", tokenizeOptions.TimeLimit, tokenizeTimeLimit)
	}
}
