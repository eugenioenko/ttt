package main

import (
	"context"
	"sync/atomic"
	"testing"
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
	var starts atomic.Int32
	p := &terminalPalette{colors: map[string]string{}, status: "pending", start: func() { starts.Add(1) }}
	if got := starts.Load(); got != 0 {
		t.Fatalf("capture started before request: %d", got)
	}
	if got, _ := p.Snapshot(); got != "pending" {
		t.Fatalf("status = %q, want pending", got)
	}
	if got := starts.Load(); got != 0 {
		t.Fatalf("snapshot started capture: %d", got)
	}
	p.Request()
	p.Request()
	if got := starts.Load(); got != 1 {
		t.Fatalf("starts while pending = %d, want 1", got)
	}
	p.mu.Lock()
	p.pending = false
	p.status = "partial"
	p.mu.Unlock()
	p.Request()
	if got := starts.Load(); got != 2 {
		t.Fatalf("starts after finished capture = %d, want 2", got)
	}
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
		if n > terminalPaletteQueryCount {
			value = "new"
		}
		ch <- term.OSCReply{Code: code, Payload: value}
	}
	return ch
}

func TestTerminalPaletteRequestWhilePendingQueuesOneRefresh(t *testing.T) {
	q := &reloadPaletteQuerier{entered: make(chan struct{}), release: make(chan struct{})}
	p := &terminalPalette{colors: map[string]string{}, status: "unavailable"}
	p.start = func() { go captureTerminalPalette(context.Background(), q, p) }
	p.Request()
	<-q.entered
	p.Request() // plugin reload while the previous OSC transaction is in flight
	if status, _ := p.Snapshot(); status != "pending" {
		t.Fatalf("queued status = %q, want pending", status)
	}
	close(q.release)
	deadline := time.After(3 * time.Second)
	for q.calls.Load() < terminalPaletteQueryCount*2 {
		select {
		case <-deadline:
			t.Fatalf("queries = %d, want two serial captures", q.calls.Load())
		case <-time.After(time.Millisecond):
		}
	}
	for {
		status, colors := p.Snapshot()
		if status == "complete" {
			if len(colors) != terminalPaletteQueryCount || colors["10"] != "new" {
				t.Fatalf("refreshed snapshot = %q, %d colors", colors["10"], len(colors))
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("capture did not settle: %s", status)
		case <-time.After(time.Millisecond):
		}
	}
	if got := q.calls.Load(); got != terminalPaletteQueryCount*2 {
		t.Fatalf("queries = %d, want exactly two captures without polling", got)
	}
}

func TestTerminalPaletteCaptureRunsOneSerialBatch(t *testing.T) {
	q := &paletteQuerier{}
	p := &terminalPalette{colors: map[string]string{}, status: "pending"}
	captureTerminalPalette(context.Background(), q, p)
	if got := q.calls.Load(); got != terminalPaletteQueryCount {
		t.Fatalf("queries = %d, want %d", got, terminalPaletteQueryCount)
	}
	status, colors := p.Snapshot()
	if status != "complete" || len(colors) != terminalPaletteQueryCount {
		t.Fatalf("snapshot = %s with %d colors, want complete with %d", status, len(colors), terminalPaletteQueryCount)
	}
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
			p := &terminalPalette{colors: tc.colors, status: tc.want}
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
