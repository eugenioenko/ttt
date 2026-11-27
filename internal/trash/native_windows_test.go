package trash

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

func nativeMoveAndLocate(t *testing.T, path string) string {
	t.Helper()
	pattern := filepath.Join(filepath.VolumeName(path)+`\`, "$Recycle.Bin", "*", "$I*")
	before, _ := filepath.Glob(pattern)
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	longName := make([]uint16, 32768)
	if n, err := windows.GetLongPathName(name, &longName[0], uint32(len(longName))); err != nil || n >= uint32(len(longName)) {
		t.Fatalf("canonicalize source: %v", err)
	}
	expected := windows.UTF16ToString(longName)
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
		original := string(utf16.Decode(chars))
		if !strings.EqualFold(original, expected) {
			t.Logf("recovery record %q points to %q; expected %q", info, original, expected)
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
	t.Fatalf("deleted item %q has no matching Recycle Bin recovery record among %v", expected, infos)
	return ""
}
