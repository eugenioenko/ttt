//go:build darwin || (linux && !android) || freebsd || openbsd || netbsd || dragonfly

package trash

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if detail := strings.TrimSpace(string(output)); detail != "" {
			return nil, fmt.Errorf("%s: %w: %s", name, err, detail)
		}
	}
	return output, err
}
