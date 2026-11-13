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

const (
	oscReplyMax        = 4096
	oscEscapeTimeout   = 20 * time.Millisecond
	oscRecoveryTimeout = 350 * time.Millisecond
)

type oscReadResult struct {
	data []byte
	err  error
}

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
	readResult    chan oscReadResult
	readErr       error
	wake          chan struct{}
	inputDeadline time.Time
	writeGate     chan struct{}
	shutdown      chan struct{}
	shutdownOnce  sync.Once
	candidate     []byte
	candidateCode string
	discarding    bool
	ready         []byte
	want          map[string]chan OSCReply
	closed        bool
	drained       bool
	readFailed    bool
	shuttingDown  atomic.Bool
}

func NewOSCReader(tty tcell.Tty) *OSCReader {
	return &OSCReader{tty: tty, want: make(map[string]chan OSCReply), writeGate: make(chan struct{}, 1), shutdown: make(chan struct{}), wake: make(chan struct{}, 1)}
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
	time.AfterFunc(timeout, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if o.want[code] == out {
			delete(o.want, code)
			out <- OSCReply{Code: code, Err: fmt.Errorf("OSC %s timeout", code)}
			// A timeout can precede the semicolon that identifies a reply's code.
			partialPrefix := len(o.candidate) >= 2 && strings.HasPrefix(string(o.candidate), "\x1b]") && strings.HasPrefix(code+";", string(o.candidate[2:]))
			if o.candidateCode == code || partialPrefix {
				trailingEscape := len(o.candidate) > 0 && o.candidate[len(o.candidate)-1] == '\x1b'
				o.candidate = nil
				if trailingEscape {
					o.candidate = []byte{'\x1b'}
				}
				o.candidateCode = ""
				o.discarding = true
				o.inputDeadline = time.Now().Add(oscRecoveryTimeout)
				o.signalRead()
			}
		}
	})
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
	for {
		o.mu.Lock()
		o.expireInput()
		if len(o.ready) > 0 {
			n := copy(dst, o.ready)
			o.ready = o.ready[n:]
			o.mu.Unlock()
			return n, nil
		}
		deadline, closed, drained := o.inputDeadline, o.closed, o.drained
		o.mu.Unlock()
		if o.readErr != nil {
			err := o.readErr
			o.readErr = nil
			return 0, err
		}
		if closed {
			return 0, io.ErrClosedPipe
		}
		if drained {
			return 0, nil
		}
		// Keep at most one TTY read in flight. A timer may return Escape while that
		// read is still blocked; the next Read must consume its result, not race it.
		if o.readResult == nil {
			result := make(chan oscReadResult, 1)
			o.readResult = result
			go func() {
				buf := make([]byte, 4096)
				n, err := o.tty.Read(buf)
				result <- oscReadResult{data: buf[:n], err: err}
			}()
		}
		var timer *time.Timer
		var timeout <-chan time.Time
		if !deadline.IsZero() {
			timer = time.NewTimer(time.Until(deadline))
			timeout = timer.C
		}
		select {
		case result := <-o.readResult:
			o.readResult = nil
			o.observe(result.data)
			if result.err != nil {
				o.readError(result.err)
				o.readErr = result.err
			} else if len(result.data) == 0 {
				o.mu.Lock()
				o.expireInput()
				if len(o.ready) > 0 {
					n := copy(dst, o.ready)
					o.ready = o.ready[n:]
					o.mu.Unlock()
					return n, nil
				}
				o.mu.Unlock()
				return 0, nil
			}
		case <-o.wake:
		case <-timeout:
		}
		if timer != nil {
			timer.Stop()
		}
	}
}

func (o *OSCReader) signalRead() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

func (o *OSCReader) resetCandidate() {
	o.candidate = nil
	o.candidateCode = ""
	o.discarding = false
	o.inputDeadline = time.Time{}
}

func (o *OSCReader) expireInput() {
	if o.inputDeadline.IsZero() || time.Now().Before(o.inputDeadline) {
		return
	}
	if len(o.candidate) == 1 && o.candidate[0] == '\x1b' {
		o.ready = append(o.ready, '\x1b')
	}
	// Late OSC tails and keys have no distinguishing marker. Bound recovery so
	// a missing terminator cannot consume keyboard input indefinitely.
	o.resetCandidate()
}

func (o *OSCReader) readError(err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.readFailed = true
	if o.candidateCode == "" && !o.discarding {
		o.ready = append(o.ready, o.candidate...)
	}
	o.resetCandidate()
	for code, ch := range o.want {
		delete(o.want, code)
		ch <- OSCReply{Code: code, Err: err}
	}
}

// observe withholds matching color replies from tcell so a fragmented OSC cannot
// be mistaken for keyboard input by tcell's shorter escape-sequence timeout.
func (o *OSCReader) observe(in []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.expireInput()
	for _, b := range in {
		if len(o.want) == 0 && len(o.candidate) == 0 && !o.discarding {
			o.ready = append(o.ready, b)
			continue
		}
		if o.discarding {
			if len(o.candidate) == 1 && o.candidate[0] == '\x1b' && b != '\\' {
				o.resetCandidate()
				o.candidate = []byte{'\x1b'}
				o.inputDeadline = time.Now().Add(oscEscapeTimeout)
			} else {
				if b == '\x1b' {
					o.candidate = []byte{b}
					o.inputDeadline = time.Now().Add(oscEscapeTimeout)
				} else {
					o.candidate = append(o.candidate, b)
					if containsOSCTerminator(o.candidate) || len(o.candidate) > oscReplyMax {
						o.resetCandidate()
					}
				}
				continue
			}
		}
		if len(o.candidate) == 0 {
			if b == '\x1b' {
				o.inputDeadline = time.Now().Add(oscEscapeTimeout)
				o.candidate = append(o.candidate, b)
			} else {
				o.ready = append(o.ready, b)
			}
			continue
		}
		if len(o.candidate) == 1 {
			switch b {
			case ']':
				o.inputDeadline = time.Now().Add(2 * oscRecoveryTimeout)
				o.candidate = append(o.candidate, b)
			case '\x1b':
				o.ready = append(o.ready, o.candidate[0])
				o.candidate[0] = b
				o.inputDeadline = time.Now().Add(oscEscapeTimeout)
			default:
				o.ready = append(o.ready, o.candidate[0], b)
				o.resetCandidate()
			}
			continue
		}
		o.candidate = append(o.candidate, b)
		if len(o.candidate) > oscReplyMax {
			if o.candidateCode == "" {
				o.ready = append(o.ready, o.candidate...)
			}
			o.resetCandidate()
			continue
		}
		if end, _ := oscEnd(o.candidate); end >= 0 {
			if o.candidateCode != "" && o.accept(o.candidate[2:end], o.candidateCode) {
				o.resetCandidate()
				continue
			}
			o.ready = append(o.ready, o.candidate...)
			o.resetCandidate()
		} else if o.candidateCode == "" {
			var possible bool
			o.candidateCode, possible = o.matchingQuery(o.candidate)
			if o.candidateCode == "" && !possible && len(o.candidate) > 2 {
				o.ready = append(o.ready, o.candidate...)
				o.resetCandidate()
			}
		} else if !o.isCandidate(o.candidate) {
			o.ready = append(o.ready, o.candidate...)
			o.resetCandidate()
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

func (t OSCTty) Start() error {
	if err := t.Tty.Start(); err != nil {
		return err
	}
	if t.Reader != nil {
		t.Reader.clearDrained()
	}
	return nil
}

func (t OSCTty) Drain() error {
	t.Reader.Drain()
	return t.Tty.Drain()
}

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

// Drain unblocks active and subsequent reads once terminal shutdown or draining begins.
func (o *OSCReader) Drain() {
	o.mu.Lock()
	o.drained = true
	o.resetCandidate()
	o.signalRead()
	o.mu.Unlock()
}

func (o *OSCReader) clearDrained() {
	o.mu.Lock()
	o.drained = false
	o.mu.Unlock()
}

// Close prevents subsequent palette queries and writes after screen shutdown.
func (o *OSCReader) Close() {
	o.BeginShutdown()
	o.mu.Lock()
	o.closed = true
	o.resetCandidate()
	o.signalRead()
	for code, ch := range o.want {
		delete(o.want, code)
		ch <- OSCReply{Code: code, Err: io.ErrClosedPipe}
	}
	o.mu.Unlock()
}
