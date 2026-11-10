package term

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v3"
)

const oscReplyMax = 4096

type OSCReply struct {
	Code    string
	Payload string
	Err     error
}

// OSCReader observes only replies for registered color queries. Other input is
// streamed to tcell unchanged; only a candidate reply is buffered, up to oscReplyMax.
type OSCReader struct {
	tty             tcell.Tty
	mu              sync.Mutex
	readMu, writeMu sync.Mutex
	candidate       []byte
	want            map[string]chan OSCReply
	closed          bool
	readFailed      bool
}

func NewOSCReader(tty tcell.Tty) *OSCReader {
	return &OSCReader{tty: tty, want: make(map[string]chan OSCReply)}
}

func (o *OSCReader) Query(code string, timeout time.Duration) <-chan OSCReply {
	out := make(chan OSCReply, 1)
	parts := strings.SplitN(code, ";", 2)
	base := parts[0]
	if (base != "10" && base != "11" && base != "12" && base != "17" && (base != "4" || len(parts) != 2)) || timeout <= 0 {
		out <- OSCReply{Code: code, Err: fmt.Errorf("invalid color query %q or timeout", code)}
		return out
	}
	o.mu.Lock()
	if o.closed || o.readFailed {
		o.mu.Unlock()
		out <- OSCReply{Code: code, Err: io.ErrClosedPipe}
		return out
	}
	if prev := o.want[code]; prev != nil {
		prev <- OSCReply{Code: code, Err: fmt.Errorf("query replaced")}
	}
	o.want[code] = out
	o.mu.Unlock()
	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		<-timer.C
		o.mu.Lock()
		if o.want[code] == out {
			delete(o.want, code)
			out <- OSCReply{Code: code, Err: fmt.Errorf("OSC %s timeout", code)}
		}
		o.mu.Unlock()
	}()
	query := "\x1b]" + base + ";?\x1b\\"
	if base == "4" {
		query = "\x1b]4;" + parts[1] + ";?\x1b\\"
	}
	_, err := io.WriteString(OSCTty{Tty: o.tty, Reader: o}, query)
	if err != nil {
		o.finish(code, out, err)
	}
	return out
}

func (o *OSCReader) finish(code string, ch chan OSCReply, err error) {
	o.mu.Lock()
	if o.want[code] == ch {
		delete(o.want, code)
		ch <- OSCReply{Code: code, Err: err}
	}
	o.mu.Unlock()
}

func (o *OSCReader) Read(dst []byte) (int, error) {
	o.readMu.Lock()
	defer o.readMu.Unlock()
	if len(dst) == 0 {
		return 0, nil
	}
	n, err := o.tty.Read(dst)
	if n > 0 {
		o.observe(dst[:n])
	}
	if err != nil {
		o.mu.Lock()
		o.readFailed = true
		for code, ch := range o.want {
			delete(o.want, code)
			ch <- OSCReply{Code: code, Err: err}
		}
		o.mu.Unlock()
	}
	return n, err
}

// observe never changes or delays the bytes returned to tcell. Its only retained
// state is a bounded candidate OSC response, independent of the input buffer.
func (o *OSCReader) observe(in []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, b := range in {
		if len(o.candidate) == 0 {
			if b == '\x1b' {
				o.candidate = append(o.candidate, b)
			}
			continue
		}
		if len(o.candidate) == 1 {
			if b == ']' {
				o.candidate = append(o.candidate, b)
			} else if b == '\x1b' {
				o.candidate[0] = b
			} else {
				o.candidate = nil
			}
			continue
		}
		o.candidate = append(o.candidate, b)
		if len(o.candidate) > oscReplyMax {
			o.candidate = nil
			continue
		}
		if end, term := oscEnd(o.candidate); end >= 0 {
			o.accept(o.candidate[2:end])
			o.candidate = nil
			_ = term
		} else if !o.isCandidate(o.candidate) {
			o.candidate = nil
		}
	}
}

func (o *OSCReader) isCandidate(b []byte) bool {
	if len(b) < 3 {
		return true
	}
	body := string(b[2:])
	for code := range o.want {
		prefix := code + ";"
		if strings.HasPrefix(prefix, body) || strings.HasPrefix(body, prefix) {
			return true
		}
	}
	return false
}

func (o *OSCReader) accept(seq []byte) bool {
	fields := bytes.SplitN(seq, []byte(";"), 2)
	if len(fields) != 2 {
		return false
	}
	key, payload := string(fields[0]), string(fields[1])
	if key == "4" {
		parts := bytes.SplitN(fields[1], []byte(";"), 2)
		if len(parts) != 2 {
			return false
		}
		key, payload = "4;"+string(parts[0]), string(parts[1])
	}
	if key != "10" && key != "11" && key != "12" && key != "17" && !strings.HasPrefix(key, "4;") {
		return false
	}
	if !strings.Contains(payload, ":") {
		return false
	}
	ch := o.want[key]
	if ch == nil {
		return false
	}
	delete(o.want, key)
	ch <- OSCReply{Code: key, Payload: payload}
	return true
}

func oscEnd(b []byte) (int, int) {
	for i := 2; i < len(b); i++ {
		if b[i] == 7 {
			return i, 1
		}
		if b[i] == '\x1b' && i+1 < len(b) && b[i+1] == '\\' {
			return i, 2
		}
	}
	return -1, 0
}

type OSCTty struct {
	tcell.Tty
	Reader *OSCReader
}

func (t OSCTty) Read(p []byte) (int, error) { return t.Reader.Read(p) }

func (t OSCTty) Write(p []byte) (int, error) {
	t.Reader.writeMu.Lock()
	defer t.Reader.writeMu.Unlock()
	t.Reader.mu.Lock()
	closed := t.Reader.closed
	t.Reader.mu.Unlock()
	if closed {
		return 0, io.ErrClosedPipe
	}
	return t.Tty.Write(p)
}

// Close prevents subsequent palette queries and writes after screen shutdown.
func (o *OSCReader) Close() {
	o.mu.Lock()
	o.closed = true
	for code, ch := range o.want {
		delete(o.want, code)
		ch <- OSCReply{Code: code, Err: io.ErrClosedPipe}
	}
	o.mu.Unlock()
}
