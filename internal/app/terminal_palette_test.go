package app

import (
	"io"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
)

type paletteQuerier struct{ calls atomic.Int32 }

func (q *paletteQuerier) Query(code string, _ time.Duration) <-chan term.OSCReply {
	q.calls.Add(1)
	ch := make(chan term.OSCReply, 1)
	ch <- term.OSCReply{Code: code, Payload: "rgb:aaaa/bbbb/cccc"}
	return ch
}

func TestTerminalPaletteRequestStartsCaptureAndSnapshotDoesNot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := &paletteQuerier{}
		p := NewTerminalPalette(q)
		defer p.Close()
		if status, _ := p.Snapshot(); status != "unavailable" {
			t.Fatalf("initial status = %s", status)
		}
		synctest.Wait()
		if q.calls.Load() != 0 {
			t.Fatal("snapshot started capture")
		}
		for batch := 1; batch <= 2; batch++ {
			p.Request()
			synctest.Wait()
			status, colors := p.Snapshot()
			if status != "complete" || len(colors) != terminalPaletteQueryCount {
				t.Fatalf("snapshot = %s, %v", status, colors)
			}
			if got := q.calls.Load(); got != int32(batch*terminalPaletteQueryCount) {
				t.Fatalf("queries = %d", got)
			}
		}
	})
}

type reloadPaletteQuerier struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (q *reloadPaletteQuerier) Query(code string, _ time.Duration) <-chan term.OSCReply {
	n := q.calls.Add(1)
	ch := make(chan term.OSCReply, 1)
	if n == 1 {
		close(q.entered)
		go func() { <-q.release; ch <- term.OSCReply{Code: code, Payload: "old"} }()
	} else {
		value := "old"
		if n > int32(terminalPaletteQueryCount) {
			value = "new"
		}
		ch <- term.OSCReply{Code: code, Payload: value}
	}
	return ch
}

func TestTerminalPaletteRequestWhilePendingQueuesOneRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := &reloadPaletteQuerier{entered: make(chan struct{}), release: make(chan struct{})}
		p := NewTerminalPalette(q)
		defer p.Close()
		p.Request()
		<-q.entered
		for range 3 {
			p.Request()
		}
		if status, _ := p.Snapshot(); status != "pending" {
			t.Fatalf("queued status = %s", status)
		}
		close(q.release)
		synctest.Wait()
		status, colors := p.Snapshot()
		if status != "complete" || colors["10"] != "new" {
			t.Fatalf("refreshed snapshot = %s, %v", status, colors)
		}
		if got := q.calls.Load(); got != int32(terminalPaletteQueryCount*2) {
			t.Fatalf("queries = %d, want two serial batches", got)
		}
	})
}

func TestTerminalPaletteSnapshotStates(t *testing.T) {
	cases := []struct {
		name   string
		want   string
		colors map[string]string
	}{
		{"pending", "pending", nil},
		{"pending with early replies", "pending", map[string]string{"10": "rgb:aaaa/bbbb/cccc"}},
		{"unavailable", "unavailable", nil},
		{"partial", "partial", map[string]string{"10": "rgb:aaaa/bbbb/cccc"}},
		{"complete", "complete", makeColors()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &TerminalPalette{colors: tc.colors, status: tc.want}
			got, snapshot := p.Snapshot()
			if got != tc.want {
				t.Fatalf("status = %q, want %q", got, tc.want)
			}
			if tc.name == "partial" {
				snapshot["10"] = "mutated"
				if p.colors["10"] != "rgb:aaaa/bbbb/cccc" {
					t.Fatal("snapshot exposed internal map")
				}
			}
		})
	}
}

func makeColors() map[string]string {
	colors := make(map[string]string, terminalPaletteQueryCount)
	for i := 0; i < terminalPaletteQueryCount; i++ {
		colors["color"+string(rune('a'+i))] = "value"
	}
	return colors
}

type unavailablePaletteQuerier struct{ partial bool }

func (q unavailablePaletteQuerier) Query(code string, _ time.Duration) <-chan term.OSCReply {
	ch := make(chan term.OSCReply, 1)
	reply := term.OSCReply{Err: io.EOF}
	if q.partial && code == "10" {
		reply = term.OSCReply{Payload: "rgb:aa/bb/cc"}
	}
	ch <- reply
	return ch
}

func TestTerminalPaletteSettlesMissingReplies(t *testing.T) {
	for _, partial := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			p := NewTerminalPalette(unavailablePaletteQuerier{partial: partial})
			defer p.Close()
			p.Request()
			synctest.Wait()
			want := "unavailable"
			if partial {
				want = "partial"
			}
			if status, _ := p.Snapshot(); status != want {
				t.Fatalf("status = %s, want %s", status, want)
			}
		})
	}
}

type blockedPaletteQuerier struct {
	entered    chan struct{}
	release    chan struct{}
	calls      atomic.Int32
	blockWrite bool
}

func (q *blockedPaletteQuerier) Query(_ string, _ time.Duration) <-chan term.OSCReply {
	q.calls.Add(1)
	close(q.entered)
	ch := make(chan term.OSCReply, 1)
	if q.blockWrite {
		<-q.release
	}
	return ch
}

func TestTerminalPaletteCloseCancelsCaptureAndQueuedRefresh(t *testing.T) {
	for _, blockWrite := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			q := &blockedPaletteQuerier{entered: make(chan struct{}), release: make(chan struct{}), blockWrite: blockWrite}
			p := NewTerminalPalette(q)
			p.Request()
			<-q.entered
			p.Request()
			p.Close()
			p.Request()
			close(q.release)
			synctest.Wait()
			if q.calls.Load() != 1 {
				t.Fatal("capture continued after Close")
			}
			if status, _ := p.Snapshot(); status != "unavailable" {
				t.Fatalf("closed status = %s", status)
			}
		})
	}
}
