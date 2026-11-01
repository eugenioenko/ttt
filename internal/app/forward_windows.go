package app

import "os"

func forwardDirOK(os.FileInfo) bool {
	return true
}

func forwardProcessDead(string) bool {
	return false
}
