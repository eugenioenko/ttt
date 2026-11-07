//go:build android || (!darwin && !windows && !linux && !freebsd && !openbsd && !netbsd && !dragonfly)

package trash

import (
	"fmt"
	"runtime"
)

func move(string) error {
	return fmt.Errorf("system Trash is unavailable on %s", runtime.GOOS)
}
