package jsdebug

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/dap"
)

// Standalone js-debug accepts an inspector port. VS Code resolves a processId
// in its extension host before handing the configuration to the adapter.
func enableNodeInspector(ctx context.Context, pid, port int) error {
	id := strconv.Itoa(pid)
	var name string
	if runtime.GOOS == "linux" {
		path, err := os.Readlink("/proc/" + id + "/exe")
		if err != nil {
			return err
		}
		name = filepath.Base(path)
	} else {
		list, err := dap.Processes(ctx)
		if err != nil {
			return err
		}
		for _, p := range list {
			if p.PID == pid {
				name = filepath.Base(p.Name)
				break
			}
		}
	}
	if name != "node" && name != "nodejs" && !strings.EqualFold(name, "node.exe") {
		return fmt.Errorf("process %d is not a Node executable", pid)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	owns := func() (bool, error) {
		if runtime.GOOS == "windows" {
			b, err := exec.CommandContext(ctx, "netstat", "-ano", "-p", "tcp").Output()
			if err != nil {
				return false, err
			}
			for line := range strings.SplitSeq(string(b), "\n") {
				f := strings.Fields(line)
				if len(f) == 5 && f[4] == id && strings.HasSuffix(f[1], ":"+strconv.Itoa(port)) && f[3] == "LISTENING" {
					return true, nil
				}
			}
			return false, nil
		}
		b, err := exec.CommandContext(ctx, "lsof", "-nP", "-a", "-p", id, "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-Fp").Output()
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
			return false, nil
		}
		return slices.Contains(strings.Fields(string(b)), "p"+id), err
	}
	if ok, err := owns(); ok || err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "node", "-e", `process._debugProcess(Number(process.argv[1]))`, id)
	if msg, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("enable Node inspector: %w: %s", err, msg)
	}
	tick := time.NewTicker(60 * time.Millisecond)
	defer tick.Stop()
	for {
		if ok, err := owns(); ok || err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("process %d did not open inspector port %d: %w", pid, port, ctx.Err())
		case <-tick.C:
		}
	}
}
