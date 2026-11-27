//go:build unix

package buffer

import (
	"os"
	"syscall"
)

var chown = (*os.File).Chown

// keepOwner gives f the owner and group in orig, the stat of the file it is
// about to replace.
func keepOwner(f *os.File, orig os.FileInfo) error {
	want, ok := orig.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	cur, err := f.Stat()
	if err != nil {
		return err
	}
	if have, ok := cur.Sys().(*syscall.Stat_t); ok && have.Uid == want.Uid && have.Gid == want.Gid {
		return nil
	}
	return chown(f, int(want.Uid), int(want.Gid))
}
