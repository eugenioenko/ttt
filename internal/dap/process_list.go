package dap

import (
	"context"
	"encoding/csv"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type Process struct {
	PID  int
	Name string
}

func Processes(ctx context.Context) ([]Process, error) {
	var processes []Process
	if runtime.GOOS == "windows" {
		data, err := exec.CommandContext(ctx, "tasklist", "/fo", "csv", "/nh").Output()
		if err != nil {
			return nil, err
		}
		rows, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if len(row) >= 2 {
				if pid, err := strconv.Atoi(row[1]); err == nil && pid > 0 {
					processes = append(processes, Process{pid, row[0]})
				}
			}
		}
	} else {
		data, err := exec.CommandContext(ctx, "ps", "-A", "-o", "pid=,comm=").Output()
		if err != nil {
			return nil, err
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			pid, err := strconv.Atoi(fields[0])
			if err == nil && pid > 0 {
				processes = append(processes, Process{pid, strings.Join(fields[1:], " ")})
			}
		}
	}
	return processes, nil
}
