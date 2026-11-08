package update

import (
	"fmt"
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

var describeSuffix = regexp.MustCompile(`-[0-9]+-g[0-9a-f]+(?:-dirty)?$`)

func IsDevelopmentVersion(version string) bool {
	if version == "dev" || describeSuffix.MatchString(version) || strings.HasSuffix(version, "-dirty") {
		return true
	}
	_, err := CompareVersions(version, "0.0.0")
	return err != nil
}

func CompareVersions(a, b string) (int, error) {
	parse := func(s string) ([]string, error) {
		if s == "dev" {
			return nil, nil
		}
		m := versionPattern.FindStringSubmatch(s)
		if m == nil {
			return nil, fmt.Errorf("invalid version %q", s)
		}
		for _, id := range strings.Split(m[4], ".") {
			if numeric(id) && len(id) > 1 && id[0] == '0' {
				return nil, fmt.Errorf("invalid version %q", s)
			}
		}
		return m[1:], nil
	}
	x, err := parse(a)
	if err != nil {
		return 0, err
	}
	y, err := parse(b)
	if err != nil {
		return 0, err
	}
	if x == nil || y == nil {
		return compareInt(len(x), len(y)), nil
	}
	for i := range 3 {
		if c := compareNumber(x[i], y[i]); c != 0 {
			return c, nil
		}
	}
	if x[3] == "" || y[3] == "" {
		return compareInt(len(y[3]), len(x[3])), nil
	}
	xids, yids := strings.Split(x[3], "."), strings.Split(y[3], ".")
	for i := 0; i < min(len(xids), len(yids)); i++ {
		xid, yid := xids[i], yids[i]
		c := strings.Compare(xid, yid)
		if numeric(xid) && numeric(yid) {
			c = compareNumber(xid, yid)
		} else if numeric(xid) != numeric(yid) {
			if numeric(xid) {
				c = -1
			} else {
				c = 1
			}
		}
		if c != 0 {
			return c, nil
		}
	}
	return compareInt(len(xids), len(yids)), nil
}

func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func compareNumber(a, b string) int {
	if c := compareInt(len(a), len(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
