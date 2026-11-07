//go:build android || (!darwin && !windows && !linux && !freebsd && !openbsd && !netbsd && !dragonfly)

package trash

import (
	"context"
	"fmt"
	"runtime"
)

func move(context.Context, string) error {
	return fmt.Errorf("system Trash is unavailable on %s", runtime.GOOS)
}
