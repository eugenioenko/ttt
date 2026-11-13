//go:build unix

package buffer

import (
	"os"
	"syscall"
)

var chown = os.Chown

// keepOwner gives the file at name the owner and group in orig, the stat of
// the file it is about to replace.
func keepOwner(name string, orig os.FileInfo) error {
	want, ok := orig.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	cur, err := os.Stat(name)
	if err != nil {
		return err
	}
	if have, ok := cur.Sys().(*syscall.Stat_t); ok && have.Uid == want.Uid && have.Gid == want.Gid {
		return nil
	}
	return chown(name, int(want.Uid), int(want.Gid))
}
