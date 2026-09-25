package diff

import "context"

// computeLCSContext returns a longest common subsequence of a and b using
// Myers' linear-space algorithm: O((N+M)D) time and O(N+M) memory, where D is
// the number of differing lines. A full LCS table here costs N*M memory, which
// reached gigabytes for large files with a single change (issue #672).
func computeLCSContext(ctx context.Context, a, b []string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids := make(map[string]int, len(a))
	intern := func(lines []string) []int {
		out := make([]int, len(lines))
		for i, l := range lines {
			id, ok := ids[l]
			if !ok {
				id = len(ids)
				ids[l] = id
			}
			out[i] = id
		}
		return out
	}
	s := &lcsSolver{ctx: ctx, a: intern(a), b: intern(b)}
	if err := s.solve(0, len(a), 0, len(b)); err != nil {
		return nil, err
	}
	lcs := make([]string, len(s.matched))
	for i, ai := range s.matched {
		lcs[i] = a[ai]
	}
	return lcs, nil
}

type lcsSolver struct {
	ctx     context.Context
	a, b    []int
	matched []int // indices into a, in order
	work    int
}

func (s *lcsSolver) checkCanceled() error {
	s.work++
	if s.work%cancellationCheckInterval != 0 {
		return nil
	}
	return s.ctx.Err()
}

func (s *lcsSolver) solve(a0, a1, b0, b1 int) error {
	for a0 < a1 && b0 < b1 && s.a[a0] == s.b[b0] {
		s.matched = append(s.matched, a0)
		a0++
		b0++
	}
	suffix := 0
	for a0 < a1-suffix && b0 < b1-suffix && s.a[a1-suffix-1] == s.b[b1-suffix-1] {
		suffix++
	}
	a1 -= suffix
	b1 -= suffix
	if a0 < a1 && b0 < b1 {
		x, y, err := s.bisect(a0, a1, b0, b1)
		if err != nil {
			return err
		}
		if x >= 0 {
			if err := s.solve(a0, a0+x, b0, b0+y); err != nil {
				return err
			}
			if err := s.solve(a0+x, a1, b0+y, b1); err != nil {
				return err
			}
		}
	}
	for i := range suffix {
		s.matched = append(s.matched, a1+i)
	}
	return nil
}

// bisect finds where the forward and reverse shortest edit paths of
// a[a0:a1] and b[b0:b1] meet, returning the split point relative to a0/b0, or
// x = -1 when the ranges share no line. Both ranges must be non-empty and
// must differ in their first and last lines.
func (s *lcsSolver) bisect(a0, a1, b0, b1 int) (int, int, error) {
	a, b := s.a[a0:a1], s.b[b0:b1]
	n, m := len(a), len(b)
	maxD := (n + m + 1) / 2
	vOff := maxD
	vLen := 2*maxD + 2
	v1 := make([]int, vLen)
	v2 := make([]int, vLen)
	for i := range v1 {
		v1[i], v2[i] = -1, -1
	}
	v1[vOff+1], v2[vOff+1] = 0, 0
	delta := n - m
	front := delta%2 != 0
	k1start, k1end, k2start, k2end := 0, 0, 0, 0
	for d := 0; d < maxD; d++ {
		for k1 := -d + k1start; k1 <= d-k1end; k1 += 2 {
			if err := s.checkCanceled(); err != nil {
				return 0, 0, err
			}
			k1Off := vOff + k1
			var x1 int
			if k1 == -d || (k1 != d && v1[k1Off-1] < v1[k1Off+1]) {
				x1 = v1[k1Off+1]
			} else {
				x1 = v1[k1Off-1] + 1
			}
			y1 := x1 - k1
			for x1 < n && y1 < m && a[x1] == b[y1] {
				x1++
				y1++
			}
			v1[k1Off] = x1
			switch {
			case x1 > n:
				k1end += 2
			case y1 > m:
				k1start += 2
			case front:
				k2Off := vOff + delta - k1
				if k2Off >= 0 && k2Off < vLen && v2[k2Off] != -1 && x1 >= n-v2[k2Off] {
					return x1, y1, nil
				}
			}
		}
		for k2 := -d + k2start; k2 <= d-k2end; k2 += 2 {
			if err := s.checkCanceled(); err != nil {
				return 0, 0, err
			}
			k2Off := vOff + k2
			var x2 int
			if k2 == -d || (k2 != d && v2[k2Off-1] < v2[k2Off+1]) {
				x2 = v2[k2Off+1]
			} else {
				x2 = v2[k2Off-1] + 1
			}
			y2 := x2 - k2
			for x2 < n && y2 < m && a[n-x2-1] == b[m-y2-1] {
				x2++
				y2++
			}
			v2[k2Off] = x2
			switch {
			case x2 > n:
				k2end += 2
			case y2 > m:
				k2start += 2
			case !front:
				k1Off := vOff + delta - k2
				if k1Off >= 0 && k1Off < vLen && v1[k1Off] != -1 {
					x1 := v1[k1Off]
					if x1 >= n-x2 {
						return x1, x1 - (k1Off - vOff), nil
					}
				}
			}
		}
	}
	return -1, -1, nil
}
