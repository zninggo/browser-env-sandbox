package sandbox

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zninggo/v8go"
)

// TimerManager manages setTimeout/setInterval/requestAnimationFrame
// within a sandbox session.
//
// Timers are registered in Go (via goroutines) but callbacks are NOT executed
// directly in the goroutine — V8 Isolates are not thread-safe. Instead,
// callbacks are queued into a channel and drained on the Isolate thread
// (during EvalAwait's flush loop via DrainCallbacks).
type TimerManager struct {
	mu             sync.Mutex
	timers         map[int32]*timerEntry
	nextID         int32
	stopped        bool
	callbackQueue  chan func()
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
		timers:        make(map[int32]*timerEntry),
		callbackQueue: make(chan func(), 256),
	}
}

func (tm *TimerManager) allocID() int32 {
	return atomic.AddInt32(&tm.nextID, 1)
}

// DrainCallbacks executes all queued callbacks on the current (Isolate) thread.
// Called by EvalAwait between timer flushes and microtask checkpoints.
func (tm *TimerManager) DrainCallbacks() {
	for {
		select {
		case cb := <-tm.callbackQueue:
			func() {
				defer func() { recover() }()
				cb()
			}()
		default:
			return
		}
	}
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
// The goroutine does NOT call the callback directly — it queues it into
// callbackQueue for DrainCallbacks to execute on the Isolate thread.
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
					// Queue callback for Isolate-thread execution (Bug 2 fix)
					select {
					case tm.callbackQueue <- cb:
					default:
						// queue full, skip this tick
					}
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
				// Queue callback for Isolate-thread execution (Bug 2 fix)
				select {
				case tm.callbackQueue <- cb:
				default:
					// queue full, drop callback
				}
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

// PendingCallbacks returns the number of queued callbacks waiting to be
// drained on the Isolate thread. Worker→parent replies and fired timer
// callbacks both land here via scheduleTimer(0). EvalAwait uses this to decide
// whether any async activity still needs draining.
func (tm *TimerManager) PendingCallbacks() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.callbackQueue)
}

// PendingTimers returns the number of registered (not yet fired/cancelled)
// timers, including recurring setInterval timers. EvalAwait does NOT use this
// to gate its drain loop — see PendingOneShotTimers instead.
func (tm *TimerManager) PendingTimers() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.timers)
}

// PendingOneShotTimers returns the number of registered one-shot timers
// (setTimeout / requestAnimationFrame), excluding recurring setInterval
// timers. EvalAwait's drain loop uses this to decide whether any settle-able
// async work is still outstanding: a setInterval never settles (it repeats
// forever), so counting it would pin hasAsync true forever and force the
// 30s timeout. A real browser returns from eval immediately when only a
// setInterval is registered; this method lets BES match that.
func (tm *TimerManager) PendingOneShotTimers() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	n := 0
	for _, e := range tm.timers {
		if !e.interval {
			n++
		}
	}
	return n
}

// Flush waits for all pending one-shot timers to complete, up to the given
// timeout. Recurring setInterval timers are excluded: they never "complete"
// (they repeat forever), so waiting on them would always hit the timeout.
// Excluding them lets a drain loop with an active setInterval return instead
// of idling for the full Flush budget every iteration.
func (tm *TimerManager) Flush(timeout time.Duration) error {
	deadline := time.After(timeout)
	for {
		tm.mu.Lock()
		count := 0
		for _, e := range tm.timers {
			if !e.interval {
				count++
			}
		}
		tm.mu.Unlock()
		if count == 0 {
			return nil
		}
		select {
		case <-deadline:
			return fmt.Errorf("flush timed out with %d one-shot timers pending", count)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// StopAll cancels all active timers.
//
// It also drains the callback queue: callbacks already queued by worker pumps
// and websocket read loops (each a scheduleTimer(0) closure capturing the
// parent V8 context) are dropped rather than executed. Executing them after
// StopAll would mean RunScript on a context Dispose is about to (or already
// has) closed — a use-after-dispose — and the closures hold references to the
// context/isolate that would otherwise keep them from being garbage collected.
// Dropping is safe because a stopped session will never run an EvalAwait drain
// again, so those callbacks have no meaningful audience.
func (tm *TimerManager) StopAll() {
	tm.mu.Lock()
	tm.stopped = true
	for _, entry := range tm.timers {
		entry.stopped = true
		close(entry.stop)
	}
	tm.timers = make(map[int32]*timerEntry)
	// Drain any callbacks already queued (drop without executing). Holding tm.mu
	// here is fine: we only receive from the channel, we do not invoke the
	// callbacks, so there is no re-entry into TimerManager.
	for {
		select {
		case <-tm.callbackQueue:
		default:
			goto drained
		}
	}
drained:
	tm.mu.Unlock()
}
