package search

import (
	"bytes"
	"io"
	"os"
)

type diskOffsetBase struct {
	bytes int64
	valid bool
}

type DiskOffsets map[string]diskOffsetBase

func (d DiskOffsets) Resolve(path string, offset int64) (int64, bool) {
	base, ok := d[path]
	if !ok {
		base = readDiskOffsetBase(path)
		d[path] = base
	}
	return offset + base.bytes, base.valid
}

func readDiskOffsetBase(path string) diskOffsetBase {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return diskOffsetBase{}
	}
	f, err := os.Open(path)
	if err != nil {
		return diskOffsetBase{}
	}
	defer f.Close()
	var prefix [3]byte
	n, err := f.Read(prefix[:])
	if err != nil && err != io.EOF {
		return diskOffsetBase{}
	}
	if bytes.HasPrefix(prefix[:n], []byte{0xff, 0xfe}) || bytes.HasPrefix(prefix[:n], []byte{0xfe, 0xff}) {
		return diskOffsetBase{}
	}
	// ripgrep removes a UTF-8 BOM and transcodes UTF-16 before reporting offsets.
	if bytes.Equal(prefix[:n], []byte{0xef, 0xbb, 0xbf}) {
		return diskOffsetBase{bytes: 3, valid: true}
	}
	return diskOffsetBase{valid: true}
}
