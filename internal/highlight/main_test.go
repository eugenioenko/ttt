package highlight

import (
	"os"
	"testing"
)

// The per-line time limit includes a grammar's lazy regex compilation, so on
// a slow or race-instrumented runner a first line can stop early. These tests
// check colors, not latency.
func TestMain(m *testing.M) {
	tokenizeOptions.TimeLimit = 0
	os.Exit(m.Run())
}
