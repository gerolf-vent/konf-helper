package internal

import (
	"sync"
	"time"
)

type Debouncer struct {
	timer *time.Timer
	delay time.Duration
	mu    sync.Mutex
}

func (d *Debouncer) Call(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	if fn != nil {
		d.timer = time.AfterFunc(d.delay, fn)
	}
}

func NewDebouncer(delay time.Duration) *Debouncer {
	return &Debouncer{delay: delay}
}
