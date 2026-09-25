package diff

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"testing"
)

type cancelAfterChecksContext struct {
	context.Context
	calls    int
	cancelAt int
}

func (c *cancelAfterChecksContext) Err() error {
	c.calls++
	if c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func referenceLCSLength(a, b []string) int {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}
	return dp[len(a)][len(b)]
}

func isSubsequence(sub, of []string) bool {
	j := 0
	for _, s := range of {
		if j < len(sub) && sub[j] == s {
			j++
		}
	}
	return j == len(sub)
}

func TestComputeLCSMatchesReferenceOnRandomInputs(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	alphabet := []string{"a", "b", "c", "d", ""}
	randLines := func() []string {
		lines := make([]string, rng.Intn(40))
		for i := range lines {
			lines[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return lines
	}
	for i := range 5000 {
		a, b := randLines(), randLines()
		got := computeLCS(a, b)
		if len(got) != referenceLCSLength(a, b) || !isSubsequence(got, a) || !isSubsequence(got, b) {
			t.Fatalf("case %d: lcs(%q, %q) = %q, want a common subsequence of length %d", i, a, b, got, referenceLCSLength(a, b))
		}
	}
}

func TestComputeLCSEdgeCases(t *testing.T) {
	tests := []struct {
		a, b []string
		want int
	}{
		{nil, nil, 0},
		{[]string{"x"}, nil, 0},
		{nil, []string{"x"}, 0},
		{[]string{"x", "y"}, []string{"x", "y"}, 2},
		{[]string{"x", "y"}, []string{"p", "q"}, 0},
		{[]string{"x", "y", "z"}, []string{"z", "y", "x"}, 1},
	}
	for _, tt := range tests {
		got := computeLCS(tt.a, tt.b)
		if len(got) != tt.want || !isSubsequence(got, tt.a) || !isSubsequence(got, tt.b) {
			t.Errorf("lcs(%q, %q) = %q, want length %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// Regression for #672: a full LCS table for 20k lines allocated ~3.2 GB.
func TestComputeLCSLargeFileWithScatteredEditsStaysLinear(t *testing.T) {
	const n = 20000
	a := make([]string, n)
	for i := range a {
		a[i] = fmt.Sprintf("row %d", i)
	}
	b := append([]string(nil), a...)
	b[10] = "changed near top"
	b[n/2] = "changed in middle"
	b[n-10] = "changed near bottom"

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got := computeLCS(a, b)
	runtime.ReadMemStats(&after)

	if len(got) != n-3 {
		t.Fatalf("lcs length = %d, want %d", len(got), n-3)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 64<<20 {
		t.Fatalf("allocated %d MB, want under 64 MB", alloc>>20)
	}
}

func TestComputeLCSContextCancelsDuringSearch(t *testing.T) {
	a := make([]string, 4096)
	b := make([]string, 4096)
	for i := range a {
		a[i] = fmt.Sprintf("a%d", i)
		b[i] = fmt.Sprintf("b%d", i)
	}
	ctx := &cancelAfterChecksContext{Context: context.Background(), cancelAt: 3}
	if _, err := computeLCSContext(ctx, a, b); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

func TestComputeLCSContextCancelsWithoutAnyDifference(t *testing.T) {
	lines := make([]string, 4096)
	for i := range lines {
		lines[i] = fmt.Sprintf("row %d", i)
	}
	interned := 2 * len(lines) / cancellationCheckInterval
	tests := []struct {
		name     string
		a, b     []string
		cancelAt int
	}{
		// Only interning runs when one side is empty: cancel on its first check.
		{"one side empty", lines, nil, 2},
		// Identical inputs are consumed by the common-prefix scan: cancel on the
		// first check after interning both sides.
		{"identical", lines, lines, 1 + interned + 1},
	}
	for _, tt := range tests {
		ctx := &cancelAfterChecksContext{Context: context.Background(), cancelAt: tt.cancelAt}
		if _, err := computeLCSContext(ctx, tt.a, tt.b); !errors.Is(err, context.Canceled) {
			t.Errorf("%s: cancellation error = %v", tt.name, err)
		}
	}
}

func TestComputeGutterChangesContextReturnsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	changes, err := ComputeGutterChangesContext(ctx, []string{"a"}, []string{"b"})
	if !errors.Is(err, context.Canceled) || changes != nil {
		t.Fatalf("changes = %v, err = %v; want nil, context.Canceled", changes, err)
	}
}
