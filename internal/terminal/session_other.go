//go:build !linux

package terminal

func hangupSessionJobs(int) {}
