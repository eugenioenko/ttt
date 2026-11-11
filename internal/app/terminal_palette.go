package app

import (
	"context"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
)

const terminalPaletteQueryTimeout = 350 * time.Millisecond

var terminalPaletteCodes = [...]string{"10", "11", "12", "17", "4;0", "4;1", "4;2", "4;3", "4;4", "4;5", "4;6", "4;7", "4;8", "4;9", "4;10", "4;11", "4;12", "4;13", "4;14", "4;15"}

const terminalPaletteQueryCount = len(terminalPaletteCodes)

type terminalColorQuerier interface {
	Query(string, time.Duration) <-chan term.OSCReply
}

type TerminalPalette struct {
	mu      sync.RWMutex
	colors  map[string]string
	status  string
	pending bool
	queued  bool
	closed  bool
	osc     terminalColorQuerier
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewTerminalPalette(osc terminalColorQuerier) *TerminalPalette {
	ctx, cancel := context.WithCancel(context.Background())
	return &TerminalPalette{osc: osc, ctx: ctx, cancel: cancel, status: "unavailable"}
}

func (p *TerminalPalette) Snapshot() (string, map[string]string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	colors := make(map[string]string, len(p.colors))
	for k, v := range p.colors {
		colors[k] = v
	}
	return p.status, colors
}

func (p *TerminalPalette) Request() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.osc == nil {
		return
	}
	if p.pending {
		p.queued = true
		return
	}
	p.pending = true
	p.status = "pending"
	p.colors = make(map[string]string)
	go p.capture()
}

func (p *TerminalPalette) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.queued = false
	p.pending = false
	p.cancel()
	p.settle()
}

func (p *TerminalPalette) settle() {
	switch len(p.colors) {
	case 0:
		p.status = "unavailable"
	case terminalPaletteQueryCount:
		p.status = "complete"
	default:
		p.status = "partial"
	}
}

// OSC replies have no request IDs, so queued refreshes must wait for the whole
// serial capture. Close does not wait for an uncancellable TTY write.
func (p *TerminalPalette) capture() {
	for {
		for _, code := range terminalPaletteCodes {
			if p.ctx.Err() != nil {
				return
			}
			select {
			case <-p.ctx.Done():
				return
			case reply := <-p.osc.Query(code, terminalPaletteQueryTimeout):
				p.mu.Lock()
				if p.closed {
					p.mu.Unlock()
					return
				}
				if reply.Err == nil {
					p.colors[code] = reply.Payload
				}
				p.mu.Unlock()
			}
		}
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		if !p.queued {
			p.pending = false
			p.settle()
			p.mu.Unlock()
			return
		}
		p.queued = false
		p.colors = make(map[string]string)
		p.mu.Unlock()
	}
}
