package ui

import (
	"sync"
	"time"
)

type Debouncer struct {
	DelayMs  int
	OnFinish func()
	timer    *time.Timer
	mu       sync.Mutex
	gen      uint64
}

func (d *Debouncer) Schedule(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	delay := d.DelayMs
	if delay <= 0 {
		fn()
		return
	}
	d.gen++
	gen := d.gen
	d.timer = time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
		d.mu.Lock()
		if gen != d.gen {
			d.mu.Unlock()
			return
		}
		d.mu.Unlock()
		fn()
		if d.OnFinish != nil {
			d.OnFinish()
		}
	})
}

func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.gen++
}
