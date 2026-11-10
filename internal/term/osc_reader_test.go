package term

import (
	"bytes"
	"io"
	"testing"
	"time"
)

type oscTestTty struct {
	*mockTty
	reads [][]byte
}

func (t *oscTestTty) Read(p []byte) (int, error) {
	if len(t.reads) == 0 {
		return 0, io.EOF
	}
	b := t.reads[0]
	n := copy(p, b)
	if n == len(b) {
		t.reads = t.reads[1:]
	} else {
		t.reads[0] = b[n:]
	}
	return n, nil
}

func TestOSCReaderRemovesOnlyRequestedColorReplies(t *testing.T) {
	tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{
		[]byte("x\x1b]10;rgb:aa/bb/cc\x07\x1b]52;c;clipboard\x07y\x1b]11;rgb:00/11/22\x1b\\z"),
	}}
	reader := NewOSCReader(tty)
	fg := reader.Query("10", time.Second)
	bg := reader.Query("11", time.Second)
	var got []byte
	buf := make([]byte, 64)
	for {
		n, err := reader.Read(buf)
		got = append(got, buf[:n]...)
		if err != nil {
			break
		}
	}
	wantBytes := "x\x1b]10;rgb:aa/bb/cc\x07\x1b]52;c;clipboard\x07y\x1b]11;rgb:00/11/22\x1b\\z"
	if !bytes.Equal(got, []byte(wantBytes)) {
		t.Fatalf("passthrough = %q, want %q", got, wantBytes)
	}
	for _, result := range []struct {
		ch      <-chan OSCReply
		payload string
	}{{fg, "rgb:aa/bb/cc"}, {bg, "rgb:00/11/22"}} {
		r := <-result.ch
		if r.Err != nil || r.Payload != result.payload {
			t.Fatalf("reply = %+v, want %q", r, result.payload)
		}
	}
}

func TestOSCReaderPreservesFragmentedUnrelatedOSC(t *testing.T) {
	tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{[]byte("a\x1b]0;ti"), []byte("tle\x07b")}}
	reader := NewOSCReader(tty)
	buf := make([]byte, 32)
	var got []byte
	for len(got) < len("a\x1b]0;title\x07b") {
		n, err := reader.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, buf[:n]...)
	}
	if want := "a\x1b]0;title\x07b"; string(got) != want {
		t.Fatalf("read = %q, want %q", got, want)
	}
}

func TestOSCReaderBoundsAndStreamsLongOSC52(t *testing.T) {
	input := append([]byte("\x1b]52;c;"), bytes.Repeat([]byte("x"), 10000)...)
	input = append(input, 7)
	tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{input}}
	reader := NewOSCReader(tty)
	reader.Query("10", time.Second)
	buf := make([]byte, 2048)
	var got []byte
	for len(got) < len(input) {
		n, err := reader.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, buf[:n]...)
		if len(reader.candidate) > oscReplyMax {
			t.Fatalf("candidate grew to %d", len(reader.candidate))
		}
	}
	if !bytes.Equal(got, input) {
		t.Fatal("OSC 52 was not preserved byte-for-byte")
	}
}

func TestOSCReaderCapturesAcrossEveryReplyBoundary(t *testing.T) {
	sequence := []byte("\x1b]10;rgb:ab/cd/ef\x1b\\")
	for split := 1; split < len(sequence); split++ {
		t.Run(string(rune(split)), func(t *testing.T) {
			tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{sequence[:split], sequence[split:]}}
			r := NewOSCReader(tty)
			result := r.Query("10", time.Second)
			buf := make([]byte, 128)
			var got []byte
			for len(got) < len(sequence) {
				n, err := r.Read(buf)
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, buf[:n]...)
			}
			if !bytes.Equal(got, sequence) {
				t.Fatalf("bytes changed: %q", got)
			}
			if reply := <-result; reply.Err != nil || reply.Payload != "rgb:ab/cd/ef" {
				t.Fatalf("reply: %+v", reply)
			}
		})
	}
}

func TestOSCReaderInvalidLateAndTimedOutRepliesRemainInput(t *testing.T) {
	testCases := []string{"\x1b]10;not-a-color\x07", "\x1b]10;rgb:11/22/33\x07"}
	for _, input := range testCases {
		tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{[]byte(input)}}
		r := NewOSCReader(tty)
		if input == testCases[0] {
			ch := r.Query("10", time.Second)
			buf := make([]byte, 128)
			n, err := r.Read(buf)
			if err != nil || string(buf[:n]) != input {
				t.Fatalf("invalid payload altered: %q %v", buf[:n], err)
			}
			<-ch
		} else {
			ch := r.Query("11", 5*time.Millisecond)
			if reply := <-ch; reply.Err == nil {
				t.Fatal("expected timeout")
			}
			buf := make([]byte, 128)
			n, err := r.Read(buf)
			if err != nil || string(buf[:n]) != input {
				t.Fatalf("late reply altered: %q %v", buf[:n], err)
			}
		}
	}
}

func TestOSCReaderSplitEscapeDoesNotDelayInput(t *testing.T) {
	tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{[]byte("\x1b"), []byte("]10;rgb:01/02/03\x07")}}
	reader := NewOSCReader(tty)
	reader.Query("10", time.Second)
	buf := make([]byte, 64)
	n, err := reader.Read(buf)
	if err != nil || string(buf[:n]) != "\x1b" {
		t.Fatalf("lone ESC read = %q, %v", buf[:n], err)
	}
	n, err = reader.Read(buf)
	if err != nil || string(buf[:n]) != "]10;rgb:01/02/03\x07" {
		t.Fatalf("remaining input = %q, %v", buf[:n], err)
	}
}

func TestOSCReaderDuplicateQueryAndEOFCompletePending(t *testing.T) {
	tty := &oscTestTty{mockTty: &mockTty{}}
	reader := NewOSCReader(tty)
	first := reader.Query("10", time.Second)
	second := reader.Query("10", time.Second)
	if r := <-first; r.Err == nil {
		t.Fatal("replaced query was not completed")
	}
	_, err := reader.Read(make([]byte, 8))
	if err != io.EOF {
		t.Fatalf("Read error = %v, want EOF", err)
	}
	if r := <-second; r.Err != io.EOF {
		t.Fatalf("pending query error = %v, want EOF", r.Err)
	}
	if r := <-reader.Query("11", time.Second); r.Err != io.ErrClosedPipe {
		t.Fatalf("query after close = %v", r.Err)
	}
}

func TestOSCTtySerializesWritesAndRejectsWritesAfterClose(t *testing.T) {
	tty := &oscTestTty{mockTty: &mockTty{}}
	reader := NewOSCReader(tty)
	ttyWrapper := OSCTty{Tty: tty, Reader: reader}
	query := reader.Query("10", time.Second)
	if _, err := ttyWrapper.Write([]byte("screen-output")); err != nil {
		t.Fatal(err)
	}
	if got, want := tty.buf.String(), "\x1b]10;?\x1b\\screen-output"; got != want {
		t.Fatalf("serialized writes = %q, want %q", got, want)
	}
	reader.Close()
	if _, err := ttyWrapper.Write([]byte("late")); err != io.ErrClosedPipe {
		t.Fatalf("write after close = %v, want ErrClosedPipe", err)
	}
	if reply := <-query; reply.Err != io.ErrClosedPipe {
		t.Fatalf("pending query after close = %v", reply.Err)
	}
}
