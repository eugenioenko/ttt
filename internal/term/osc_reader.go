package term

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v3"
)

const oscReplyMax = 4096

type OSCReply struct {
	Code    string
	Payload string
	Err     error
}

// OSCReader captures color replies and withholds them from tcell. Non-color input
// is streamed unchanged; a candidate OSC response is bounded by oscReplyMax.
type OSCReader struct {
	tty           tcell.Tty
	mu            sync.Mutex
	readMu        sync.Mutex
	writeGate     chan struct{}
	shutdown      chan struct{}
	shutdownOnce  sync.Once
	candidate     []byte
	candidateCode string
	discarding    bool
	ready         []byte
	want          map[string]chan OSCReply
	closed        bool
	readFailed    bool
	shuttingDown  atomic.Bool
}

func NewOSCReader(tty tcell.Tty) *OSCReader {
	return &OSCReader{tty: tty, want: make(map[string]chan OSCReply), writeGate: make(chan struct{}, 1), shutdown: make(chan struct{})}
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
	if o.closed || o.readFailed || o.shuttingDown.Load() {
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
			if o.candidateCode == code {
				o.candidate = nil
				o.candidateCode = ""
				o.discarding = true
			}
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
	for len(o.ready) == 0 {
		var input [1024]byte
		n, err := o.tty.Read(input[:])
		if n > 0 {
			o.observe(input[:n])
		}
		if err != nil {
			o.mu.Lock()
			o.readFailed = true
			if o.candidateCode == "" && !o.discarding {
				o.ready = append(o.ready, o.candidate...)
			}
			o.candidate = nil
			o.candidateCode = ""
			o.discarding = false
			for code, ch := range o.want {
				delete(o.want, code)
				ch <- OSCReply{Code: code, Err: err}
			}
			o.mu.Unlock()
			if len(o.ready) == 0 {
				return 0, err
			}
		}
	}
	n := copy(dst, o.ready)
	o.ready = o.ready[n:]
	return n, nil
}

// observe withholds matching color replies from tcell so a fragmented OSC cannot
// be mistaken for keyboard input by tcell's shorter escape-sequence timeout.
func (o *OSCReader) observe(in []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, b := range in {
		if o.discarding {
			o.candidate = append(o.candidate, b)
			if containsOSCTerminator(o.candidate) || len(o.candidate) > oscReplyMax {
				o.candidate = nil
				o.discarding = false
			}
			continue
		}
		if len(o.candidate) == 0 {
			if b == '\x1b' {
				o.candidate = append(o.candidate, b)
			} else {
				o.ready = append(o.ready, b)
			}
			continue
		}
		if len(o.candidate) == 1 {
			if b == ']' {
				o.candidate = append(o.candidate, b)
			} else if b == '\x1b' {
				o.ready = append(o.ready, o.candidate[0])
				o.candidate[0] = b
			} else {
				o.ready = append(o.ready, o.candidate[0], b)
				o.candidate = nil
			}
			continue
		}
		o.candidate = append(o.candidate, b)
		if len(o.candidate) > oscReplyMax {
			if o.candidateCode == "" {
				o.ready = append(o.ready, o.candidate...)
			}
			o.candidate = nil
			o.candidateCode = ""
			continue
		}
		if end, _ := oscEnd(o.candidate); end >= 0 {
			if o.candidateCode != "" && o.accept(o.candidate[2:end], o.candidateCode) {
				o.candidate = nil
				o.candidateCode = ""
				continue
			}
			o.ready = append(o.ready, o.candidate...)
			o.candidate = nil
			o.candidateCode = ""
		} else if o.candidateCode == "" {
			var possible bool
			o.candidateCode, possible = o.matchingQuery(o.candidate)
			if o.candidateCode == "" && !possible && len(o.candidate) > 2 {
				o.ready = append(o.ready, o.candidate...)
				o.candidate = nil
			}
		} else if !o.isCandidate(o.candidate) {
			o.ready = append(o.ready, o.candidate...)
			o.candidate = nil
			o.candidateCode = ""
		}
	}
}

func containsOSCTerminator(b []byte) bool {
	for i, c := range b {
		if c == 7 || (c == '\x1b' && i+1 < len(b) && b[i+1] == '\\') {
			return true
		}
	}
	return false
}

func (o *OSCReader) matchingQuery(b []byte) (string, bool) {
	if len(b) < 3 || b[0] != '\x1b' || b[1] != ']' {
		return "", true
	}
	body := string(b[2:])
	for code := range o.want {
		prefix := code + ";"
		if strings.HasPrefix(body, prefix) {
			return code, true
		}
		if strings.HasPrefix(prefix, body) {
			return "", true
		}
	}
	return "", false
}

func (o *OSCReader) isCandidate(b []byte) bool {
	body := string(b[2:])
	prefix := o.candidateCode + ";"
	return strings.HasPrefix(prefix, body) || strings.HasPrefix(body, prefix)
}

func (o *OSCReader) accept(seq []byte, expectedCode string) bool {
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
	if key != expectedCode {
		return false
	}
	ch := o.want[key]
	if ch != nil {
		delete(o.want, key)
		if !strings.Contains(payload, ":") {
			ch <- OSCReply{Code: key, Err: fmt.Errorf("invalid OSC color response")}
		} else {
			ch <- OSCReply{Code: key, Payload: payload}
		}
	}
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
	if t.Reader.shuttingDown.Load() {
		return t.writeDuringShutdown(p)
	}
	select {
	case t.Reader.writeGate <- struct{}{}:
		defer func() { <-t.Reader.writeGate }()
	case <-t.Reader.shutdown:
		return t.writeDuringShutdown(p)
	}
	t.Reader.mu.Lock()
	closed := t.Reader.closed
	t.Reader.mu.Unlock()
	if closed {
		return 0, io.ErrClosedPipe
	}
	return t.Tty.Write(p)
}

func (t OSCTty) writeDuringShutdown(p []byte) (int, error) {
	t.Reader.mu.Lock()
	closed := t.Reader.closed
	t.Reader.mu.Unlock()
	if closed {
		return 0, io.ErrClosedPipe
	}
	return t.Tty.Write(p)
}

// BeginShutdown allows tcell reset writes to proceed without waiting for a
// palette query whose TTY write is blocked.
func (o *OSCReader) BeginShutdown() {
	o.shuttingDown.Store(true)
	o.shutdownOnce.Do(func() { close(o.shutdown) })
}

// Close prevents subsequent palette queries and writes after screen shutdown.
func (o *OSCReader) Close() {
	o.BeginShutdown()
	o.mu.Lock()
	o.closed = true
	for code, ch := range o.want {
		delete(o.want, code)
		ch <- OSCReply{Code: code, Err: io.ErrClosedPipe}
	}
	o.mu.Unlock()
}
