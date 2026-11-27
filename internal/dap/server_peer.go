package dap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Match the accepted connection, since a port selected before the adapter binds it can be reused.
func verifyServerPeer(ctx context.Context, conn net.Conn, pid int, exited <-chan struct{}) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for {
		select {
		case <-exited:
			return errors.New("adapter exited before peer verification")
		default:
		}
		ok, err := ownsServerConnection(ctx, conn, pid)
		if err != nil {
			return fmt.Errorf("verify adapter connection: %w", err)
		}
		if ok {
			select {
			case <-exited:
				return errors.New("adapter exited during peer verification")
			default:
				return nil
			}
		}
		select {
		case <-exited:
			return errors.New("adapter exited before accepting connection")
		case <-ctx.Done():
			return fmt.Errorf("could not verify adapter connection: %w", ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func ownsServerConnection(ctx context.Context, conn net.Conn, pid int) (bool, error) {
	remote, local := conn.RemoteAddr().String(), conn.LocalAddr().String()
	id := strconv.Itoa(pid)
	if runtime.GOOS == "windows" {
		out, err := exec.CommandContext(ctx, "netstat", "-ano", "-p", "tcp").Output()
		if err != nil {
			return false, err
		}
		for line := range strings.SplitSeq(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 5 && fields[0] == "TCP" && fields[1] == remote && fields[2] == local && fields[4] == id {
				return true, nil
			}
		}
		return false, nil
	}
	port := strconv.Itoa(conn.RemoteAddr().(*net.TCPAddr).Port)
	out, err := exec.CommandContext(ctx, "lsof", "-nP", "-a", "-g", id, "-iTCP:"+port, "-Fn").Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for field := range strings.SplitSeq(string(out), "\n") {
		if field == "n"+remote+"->"+local {
			return true, nil
		}
	}
	return false, nil
}
