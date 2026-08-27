package sandbox

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	v8 "rogchap.com/v8go"
)

// TimerManager manages setTimeout/setInterval/requestAnimationFrame
// within a sandbox session.
//
// Timers are registered in Go (via goroutines) and callbacks execute
// inside the V8 context. This mirrors browser behavior where timers
// run on the main thread's event loop.
type TimerManager struct {
	mu      sync.Mutex
	timers  map[int32]*timerEntry
	nextID  int32
	stopped bool
}

type timerEntry struct {
	id       int32
	interval bool
	stop     chan struct{}
	stopped  bool
}

// NewTimerManager creates a new timer manager.
func NewTimerManager() *TimerManager {
	return &TimerManager{
		timers: make(map[int32]*timerEntry),
	}
}

func (tm *TimerManager) allocID() int32 {
	return atomic.AddInt32(&tm.nextID, 1)
}

// SetTimeoutCallback returns a v8go FunctionCallback for setTimeout.
// The JS callback receives a timer ID (integer).
func (tm *TimerManager) SetTimeoutCallback(iso *v8go.Isolate) v8go.FunctionCallback {
	return func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) < 1 {
			return nil
		}
		cb := info.Args()[0]
		if !cb.IsFunction() {
			return nil
		}
		delay := 0
		if len(info.Args()) > 1 {
			delay = int(info.Args()[1].Int32())
		}

		fn, _ := cb.AsFunction()
		id := tm.scheduleTimer(time.Duration(delay)*time.Millisecond, false, func() {
			fn.Call(info.Context().Global())
			info.Context().PerformMicrotaskCheckpoint()
		})

		v, _ := v8go.NewValue(iso, id)
		return v
	}
}

// SetIntervalCallback returns a v8go FunctionCallback for setInterval.
func (tm *TimerManager) SetIntervalCallback(iso *v8go.Isolate) v8go.FunctionCallback {
	return func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) < 1 {
			return nil
		}
		cb := info.Args()[0]
		if !cb.IsFunction() {
			return nil
		}
		delay := 0
		if len(info.Args()) > 1 {
			delay = int(info.Args()[1].Int32())
		}

		fn, _ := cb.AsFunction()
		id := tm.scheduleTimer(time.Duration(delay)*time.Millisecond, true, func() {
			fn.Call(info.Context().Global())
			info.Context().PerformMicrotaskCheckpoint()
		})

		v, _ := v8go.NewValue(iso, id)
		return v
	}
}

// RAFCallback returns a v8go FunctionCallback for requestAnimationFrame.
// Maps to setTimeout(~16ms) like a browser would.
func (tm *TimerManager) RAFCallback(iso *v8go.Isolate) v8go.FunctionCallback {
	return func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) < 1 {
			return nil
		}
		cb := info.Args()[0]
		if !cb.IsFunction() {
			return nil
		}

		fn, _ := cb.AsFunction()
		id := tm.scheduleTimer(16*time.Millisecond, false, func() {
			fn.Call(info.Context().Global())
			info.Context().PerformMicrotaskCheckpoint()
		})

		v, _ := v8go.NewValue(iso, id)
		return v
	}
}

// ClearTimeoutCallback returns a v8go FunctionCallback for clearTimeout/cancelAnimationFrame.
func (tm *TimerManager) ClearTimeoutCallback(iso *v8go.Isolate) v8go.FunctionCallback {
	return func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) > 0 {
			id := info.Args()[0].Int32()
			tm.cancel(id)
		}
		return nil
	}
}

// scheduleTimer registers a timer and starts a goroutine.
func (tm *TimerManager) scheduleTimer(delay time.Duration, interval bool, cb func()) int32 {
	tm.mu.Lock()
	if tm.stopped {
		tm.mu.Unlock()
		return -1
	}
	id := tm.allocID()
	entry := &timerEntry{
		id:       id,
		interval: interval,
		stop:     make(chan struct{}),
	}
	tm.timers[id] = entry
	tm.mu.Unlock()

	go func() {
		if interval {
			ticker := time.NewTicker(delay)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if entry.stopped {
						return
					}
					cb()
				case <-entry.stop:
					return
				}
			}
		} else {
			select {
			case <-time.After(delay):
				if entry.stopped {
					return
				}
				cb()
				tm.mu.Lock()
				delete(tm.timers, id)
				tm.mu.Unlock()
			case <-entry.stop:
				return
			}
		}
	}()

	return id
}

func (tm *TimerManager) cancel(id int32) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if entry, ok := tm.timers[id]; ok {
		entry.stopped = true
		close(entry.stop)
		delete(tm.timers, id)
	}
}

// Flush waits for all pending timers to complete, up to the given timeout.
func (tm *TimerManager) Flush(timeout time.Duration) error {
	deadline := time.After(timeout)
	for {
		tm.mu.Lock()
		count := len(tm.timers)
		tm.mu.Unlock()
		if count == 0 {
			return nil
		}
		select {
		case <-deadline:
			return fmt.Errorf("flush timed out with %d timers pending", count)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// StopAll cancels all active timers.
func (tm *TimerManager) StopAll() {
	tm.mu.Lock()
	tm.stopped = true
	for _, entry := range tm.timers {
		entry.stopped = true
		close(entry.stop)
	}
	tm.timers = make(map[int32]*timerEntry)
	tm.mu.Unlock()
}
