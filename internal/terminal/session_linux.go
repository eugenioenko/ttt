package terminal

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// bash can exit on SIGHUP without forwarding it to its jobs, or forward it to
// a job that has forked but not yet exec'd and still runs bash's handler
// (#1114). Hang up every job group left in the shell's session the way bash
// would; jobs that ignore SIGHUP (nohup) survive.
func hangupSessionJobs(sid int) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	stopped := map[int]bool{}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		state, pgrp, session, ok := procJobState(pid)
		if !ok || session != sid || pgrp == sid {
			continue
		}
		stopped[pgrp] = stopped[pgrp] || state == "T" || state == "t"
	}
	for pgrp, isStopped := range stopped {
		_ = syscall.Kill(-pgrp, syscall.SIGHUP)
		// A stopped job only acts on SIGHUP once continued.
		if isStopped {
			_ = syscall.Kill(-pgrp, syscall.SIGCONT)
		}
	}
}

func procJobState(pid int) (state string, pgrp, session int, ok bool) {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", 0, 0, false
	}
	i := strings.LastIndexByte(string(stat), ')')
	if i < 0 {
		return "", 0, 0, false
	}
	// Fields after the command name: state, ppid, pgrp, session.
	fields := strings.Fields(string(stat[i+1:]))
	if len(fields) < 4 {
		return "", 0, 0, false
	}
	pgrp, err1 := strconv.Atoi(fields[2])
	session, err2 := strconv.Atoi(fields[3])
	return fields[0], pgrp, session, err1 == nil && err2 == nil
}
