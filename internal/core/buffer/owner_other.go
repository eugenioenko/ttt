//go:build !unix

package buffer

import "os"

func keepOwner(string, os.FileInfo) error { return nil }
