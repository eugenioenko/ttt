package main

import (
	"os"
	"strconv"
	"syscall"
	"time"
)

func main() {
	if _, _, err := syscall.Syscall6(syscall.SYS_PRCTL, 0x59616d61, uintptr(os.Getppid()), 0, 0, 0, 0); err != 0 {
		panic(err)
	}
	os.WriteFile("pid", []byte(strconv.Itoa(os.Getpid())), 0600)
	println("TARGET_READY")
	for i := 0; ; i++ {
		value := 40 + 2
		os.WriteFile("heartbeat", []byte(strconv.Itoa(i+value)), 0600)
		time.Sleep(25 * time.Millisecond)
	}
}
