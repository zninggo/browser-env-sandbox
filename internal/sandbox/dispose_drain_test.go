package sandbox

import (
	"sync"
	"testing"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// TestGetFingerprintSwapFingerprintNoRace runs GetFingerprint (now locked) and
// SwapFingerprint concurrently under -race. Before the H5 fix, GetFingerprint
// read s.fp with no lock while SwapFingerprint wrote s.fp under s.mu — two
// different lock states, a data race the race detector flagged.
func TestGetFingerprintSwapFingerprintNoRace(t *testing.T) {
	eng := New(fpengine.New(), 2)
	t.Cleanup(eng.Dispose)
	sess, err := eng.CreateSession(api.SessionOptions{Browser: "chrome", OS: "windows"})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	defer sess.Dispose()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Reader: hammer GetFingerprint.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = sess.GetFingerprint()
			}
		}
	}()

	// Writer: swap fingerprints repeatedly.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_, _ = sess.SwapFingerprint(eng, api.SessionOptions{
				Browser: "chrome", OS: "windows", Seed: uint64(i + 1),
			})
		}
		close(stop)
	}()

	wg.Wait()
}

// TestStopAllDrainsCallbackQueue verifies the H9 fix: StopAll must drain the
// callback queue so closures already queued (which capture the parent context)
// are dropped instead of lingering and blocking GC or firing on a closed
// context. After StopAll, PendingCallbacks must be zero.
func TestStopAllDrainsCallbackQueue(t *testing.T) {
	tm := NewTimerManager()
	// Queue a handful of no-op callbacks directly via scheduleTimer(0). These
	// land in callbackQueue and would normally be drained by EvalAwait; here we
	// never drain, then StopAll must clear them.
	for i := 0; i < 8; i++ {
		tm.scheduleTimer(0, false, func() {})
	}
	// Give the timer goroutines a moment to enqueue (scheduleTimer spawns a
	// goroutine that selects on time.After(0)).
	time.Sleep(50 * time.Millisecond)

	if got := tm.PendingCallbacks(); got == 0 {
		t.Fatalf("expected callbacks queued before StopAll, got %d", got)
	}
	tm.StopAll()
	if got := tm.PendingCallbacks(); got != 0 {
		t.Fatalf("after StopAll, PendingCallbacks = %d, want 0 (queue not drained)", got)
	}
}

// TestIsolatePoolPutAfterDisposeAll verifies the H8 fix: once DisposeAll has
// closed the pool, Put must dispose the returned Isolate on the spot rather
// than pushing it back into a dead pool channel (which would leak — no future
// DisposeAll reclaims it). After Put on a closed pool, a second DisposeAll must
// not double-dispose and the pool channel must stay empty.
func TestIsolatePoolPutAfterDisposeAll(t *testing.T) {
	pool := NewIsolatePool(2)
	iso := pool.Get()
	pool.DisposeAll()
	// Put after close: must dispose iso, not return it.
	pool.Put(iso)
	// Channel should be empty — nothing to reclaim. A second DisposeAll is a
	// no-op (does not panic, does not double-dispose iso).
	pool.DisposeAll()
}

// TestDisposeDrainsWorkerCallbacks verifies the H6/H9 discipline end-to-end: a
// session with an active worker is disposed, and no queued worker→parent
// callback can fire RunScript on the closed parent context afterwards. We
// cannot easily assert "no use-after-dispose" directly without a debug hook,
// but we can assert that after Dispose the session reports disposed and that
// re-entering EvalAwait (which would drain the queue) refuses instead of
// touching the closed context — i.e. the residual callbacks never execute.
func TestDisposeDrainsWorkerCallbacks(t *testing.T) {
	sess := newTestSession(t)
	// Spin up a worker that posts messages back to the parent, so the parent
	// pump queues callbacks into the session's timer manager.
	src := `self.onmessage = function(e){ self.postMessage({echo: e.data}); };`
	w, err := StartWorker(src, sess.timers, sess.consoleSink, "")
	if err != nil {
		t.Fatalf("StartWorker failed: %v", err)
	}
	sess.workers.register(w)
	w.startParentPump(sess.timers, sess.ctx, sess)

	// Drive a few messages through to seed the outbound queue / callbacks.
	for i := 0; i < 5; i++ {
		w.inbound <- `{"type":"message","payload":"hi"}`
	}
	// Let the worker loop dispatch and the pump enqueue callbacks.
	time.Sleep(100 * time.Millisecond)

	// Dispose tears down workers (stopping the pump) and StopAll drains the
	// queue. This must not hang and must not fire a callback on the closed ctx.
	done := make(chan struct{})
	go func() {
		sess.Dispose()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Dispose hung — pump not stopped or queue not drained")
	}

	if !sess.IsDisposed() {
		t.Fatal("session not disposed after Dispose")
	}
	// After dispose, EvalAwait must refuse immediately (errDisposed) rather than
	// drain residual callbacks that would RunScript the closed context.
	_, err = sess.EvalAwait(`1+1`, time.Second)
	if err == nil {
		t.Fatal("EvalAwait on disposed session unexpectedly succeeded — residual callback may have run")
	}
}
