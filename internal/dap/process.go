package dap

import (
	"context"
	"encoding/csv"
	"fmt"
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

const PickProcess = "${command:pickProcess}"

func ProcessID(value any) (int, error) {
	var n int
	switch v := value.(type) {
	case string:
		n, _ = strconv.Atoi(v)
	case float64:
		n = int(v)
		if float64(n) != v {
			n = 0
		}
	case int:
		n = v
	}
	if n < 1 || int64(n) > 2147483647 {
		return 0, fmt.Errorf("invalid processId %v", value)
	}
	return n, nil
}
