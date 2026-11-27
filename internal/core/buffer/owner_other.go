//go:build !unix

package buffer

import "os"

func keepOwner(*os.File, os.FileInfo) error { return nil }
