package term

import (
	"bytes"
	"io"
	"testing"
	"time"
)

type oscTestTty struct {
	*mockTty
	reads  [][]byte
	delays []time.Duration
}

func (t *oscTestTty) Read(p []byte) (int, error) {
	if len(t.reads) == 0 {
		return 0, io.EOF
	}
	if len(t.delays) > 0 {
		delay := t.delays[0]
		t.delays = t.delays[1:]
		time.Sleep(delay)
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
	wantBytes := "x\x1b]52;c;clipboard\x07yz"
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
			if n, err := r.Read(buf); n != 0 || err != io.EOF {
				t.Fatalf("matching reply reached tcell: n=%d bytes=%q err=%v", n, buf[:n], err)
			}
			if reply := <-result; reply.Err != nil || reply.Payload != "rgb:ab/cd/ef" {
				t.Fatalf("reply: %+v", reply)
			}
		})
	}
}

func TestOSCReaderFiltersInvalidRepliesAndPreservesUnmatchedLateInput(t *testing.T) {
	testCases := []string{"\x1b]10;not-a-color\x07", "\x1b]10;rgb:11/22/33\x07"}
	for _, input := range testCases {
		tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{[]byte(input)}}
		r := NewOSCReader(tty)
		if input == testCases[0] {
			ch := r.Query("10", time.Second)
			buf := make([]byte, 128)
			if n, err := r.Read(buf); n != 0 || err != io.EOF {
				t.Fatalf("invalid color reply reached tcell: %q %v", buf[:n], err)
			}
			if reply := <-ch; reply.Err == nil {
				t.Fatal("invalid color payload should fail the query")
			}
		} else {
			ch := r.Query("11", 5*time.Millisecond)
			if reply := <-ch; reply.Err == nil {
				t.Fatal("expected timeout")
			}
			buf := make([]byte, 128)
			n, err := r.Read(buf)
			if err != nil || string(buf[:n]) != input {
				t.Fatalf("unmatched late input = %q, %v", buf[:n], err)
			}
		}
	}
}

func TestOSCReaderWithholdsSplitColorReply(t *testing.T) {
	tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{[]byte("\x1b"), []byte("]10;rgb:01/02/03\x07")}}
	reader := NewOSCReader(tty)
	reader.Query("10", time.Second)
	buf := make([]byte, 64)
	n, err := reader.Read(buf)
	if n != 0 || err != io.EOF {
		t.Fatalf("fragmented reply reached tcell = %q, %v", buf[:n], err)
	}
	if reply := <-reader.Query("11", time.Second); reply.Err == nil {
		t.Fatal("query after read failure unexpectedly succeeded")
	}
}

func TestOSCReaderWithholdsColorReplyFragmentedBeyondTcellTimeout(t *testing.T) {
	tty := &oscTestTty{mockTty: &mockTty{}, reads: [][]byte{
		[]byte("x\x1b]10;rgb:aa/"), []byte("bb/cc\x07y"),
	}, delays: []time.Duration{0, 150 * time.Millisecond}}
	reader := NewOSCReader(tty)
	replyCh := reader.Query("10", time.Second)
	buf := make([]byte, 64)
	n, err := reader.Read(buf)
	if err != nil || string(buf[:n]) != "x" {
		t.Fatalf("prefix read = %q, %v", buf[:n], err)
	}
	n, err = reader.Read(buf)
	if err != nil || string(buf[:n]) != "y" {
		t.Fatalf("post-reply input = %q, %v", buf[:n], err)
	}
	if reply := <-replyCh; reply.Err != nil || reply.Payload != "rgb:aa/bb/cc" {
		t.Fatalf("fragmented reply = %+v", reply)
	}
}

type streamedReadTty struct {
	*mockTty
	reads chan []byte
}

func (t *streamedReadTty) Read(p []byte) (int, error) {
	data, ok := <-t.reads
	if !ok {
		return 0, io.EOF
	}
	return copy(p, data), nil
}

func TestOSCReaderDiscardsIncompleteReplyOnQueryTimeout(t *testing.T) {
	tty := &streamedReadTty{mockTty: &mockTty{}, reads: make(chan []byte)}
	reader := NewOSCReader(tty)
	reply := reader.Query("10", 25*time.Millisecond)
	readResult := make(chan struct {
		bytes string
		err   error
	}, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := reader.Read(buf)
		readResult <- struct {
			bytes string
			err   error
		}{string(buf[:n]), err}
	}()
	tty.reads <- []byte("\x1b]10;rgb:aa/")
	if result := <-reply; result.Err == nil {
		t.Fatal("expected the incomplete palette query to time out")
	}
	tty.reads <- []byte("bb/cc\x07z")
	result := <-readResult
	if result.err != nil || result.bytes != "z" {
		t.Fatalf("input after timed-out OSC = %q, %v; want only trailing key z", result.bytes, result.err)
	}
	close(tty.reads)
}

func TestOSCReaderDuplicateQueryAndEOFCompletePendingWithoutBlockingCleanupWrites(t *testing.T) {
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
	if _, err := (OSCTty{Tty: tty, Reader: reader}).Write([]byte("reset")); err != nil {
		t.Fatalf("cleanup write after read failure = %v, want nil", err)
	}
	if got, want := tty.buf.String(), "\x1b]10;?\x1b\\\x1b]10;?\x1b\\reset"; got != want {
		t.Fatalf("cleanup writes = %q, want %q", got, want)
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

type blockedPaletteWriteTty struct {
	*mockTty
	started chan struct{}
	release chan struct{}
}

func (t *blockedPaletteWriteTty) Write(p []byte) (int, error) {
	if string(p) == "\x1b]10;?\x1b\\" {
		close(t.started)
		<-t.release
		return len(p), nil
	}
	return t.mockTty.Write(p)
}

func TestOSCTtyShutdownResetBypassesBlockedPaletteWrite(t *testing.T) {
	tty := &blockedPaletteWriteTty{mockTty: &mockTty{}, started: make(chan struct{}), release: make(chan struct{})}
	reader := NewOSCReader(tty)
	queryDone := make(chan struct{})
	go func() {
		reader.Query("10", time.Second)
		close(queryDone)
	}()
	<-tty.started

	reader.BeginShutdown()
	writeDone := make(chan error, 1)
	go func() {
		_, err := (OSCTty{Tty: tty, Reader: reader}).Write([]byte("reset"))
		writeDone <- err
	}()
	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("reset write = %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("reset write waited for blocked palette query")
	}
	if got := tty.buf.String(); got != "reset" {
		t.Fatalf("reset output = %q", got)
	}
	close(tty.release)
	select {
	case <-queryDone:
	case <-time.After(time.Second):
		t.Fatal("palette query did not finish after its write was released")
	}
}
