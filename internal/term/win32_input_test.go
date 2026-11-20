package term

import (
	"bytes"
	"testing"
)

func TestFixWin32CtrlSlash(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"left ctrl", "\x1b[191;53;0;1;8;1_", "\x1b[191;53;31;1;8;1_"},
		{"right ctrl", "\x1b[191;53;0;1;4;1_", "\x1b[191;53;31;1;4;1_"},
		{"key up", "\x1b[191;53;0;0;8;1_", "\x1b[191;53;31;0;8;1_"},
		{"omitted repeat", "\x1b[191;53;0;1;8_", "\x1b[191;53;31;1;8_"},
		{"surrounded", "a\x1b[191;53;0;1;8;1_b", "a\x1b[191;53;31;1;8;1_b"},
		{"plain slash", "\x1b[191;53;47;1;0;1_", "\x1b[191;53;47;1;0;1_"},
		{"altgr", "\x1b[191;53;0;1;9;1_", "\x1b[191;53;0;1;9;1_"},
		{"other key", "\x1b[188;51;0;1;8;1_", "\x1b[188;51;0;1;8;1_"},
		{"truncated", "\x1b[191;53;0;1;8", "\x1b[191;53;0;1;8"},
		{"other csi", "\x1b[1;5A", "\x1b[1;5A"},
		{"plain text", "hello", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixWin32CtrlSlash([]byte(tt.in))
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Fatalf("fixWin32CtrlSlash(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestOSCReaderFixesWin32CtrlSlash(t *testing.T) {
	tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{[]byte("\x1b[191;53;0;1;8;1_")}}
	reader := NewOSCReader(tty)
	var got []byte
	buf := make([]byte, 64)
	for {
		n, err := reader.Read(buf)
		got = append(got, buf[:n]...)
		if err != nil {
			break
		}
	}
	if want := "\x1b[191;53;31;1;8;1_"; string(got) != want {
		t.Fatalf("read %q, want %q", got, want)
	}
}
