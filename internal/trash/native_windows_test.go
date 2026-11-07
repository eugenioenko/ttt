package trash

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func nativeMoveAndLocate(t *testing.T, path string) string {
	t.Helper()
	pattern := filepath.Join(filepath.VolumeName(path)+`\`, "$Recycle.Bin", "*", "$I*")
	before, _ := filepath.Glob(pattern)
	if err := Move(path); err != nil {
		t.Fatal(err)
	}
	infos, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range infos {
		existed := false
		for _, old := range before {
			existed = existed || info == old
		}
		if existed {
			continue
		}
		data, err := os.ReadFile(info)
		if err != nil || len(data) < 28 || binary.LittleEndian.Uint64(data) != 2 {
			continue
		}
		var chars []uint16
		for i := 28; i+1 < len(data); i += 2 {
			ch := binary.LittleEndian.Uint16(data[i:])
			if ch == 0 {
				break
			}
			chars = append(chars, ch)
		}
		if !strings.EqualFold(string(utf16.Decode(chars)), path) {
			continue
		}
		trashed := filepath.Join(filepath.Dir(info), "$R"+filepath.Base(info)[2:])
		restored := filepath.Join(t.TempDir(), filepath.Base(path))
		t.Cleanup(func() {
			if err := os.Rename(trashed, restored); err != nil {
				t.Errorf("restore test item %q: %v", trashed, err)
				return
			}
			if err := os.Remove(info); err != nil {
				t.Error(err)
			}
		})
		return trashed
	}
	t.Fatal("deleted item has no matching Recycle Bin recovery record")
	return ""
}
