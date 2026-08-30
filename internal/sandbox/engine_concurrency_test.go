package sandbox

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// newTestSession builds a real sandbox session for concurrency tests. The V8
// isolate stack is required to exercise the EvalAwait/Dispose lock path, so a
// fake FingerprintProvider is not enough — we use the real fpengine.
func newTestSession(t *testing.T) *Session {
	t.Helper()
	eng := New(fpengine.New(), 2)
	t.Cleanup(eng.Dispose)
	sess, err := eng.CreateSession(api.SessionOptions{
		Browser:  "chrome",
		OS:       "windows",
		Location: "https://example.com/",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	return sess
}

// TestEvalAwaitDisposeNoDeadlock verifies the P0 fix: a long-running EvalAwait
// (blocked inside RunScript) must not deadlock against a concurrent Dispose.
// Before the fix, EvalAwait held s.mu across RunScript while Dispose waited on
// s.mu — a classic lock-then-block deadlock. Now RunScript runs on an unlocked
// goroutine, so Dispose can flip disposed and TerminateExecution. The whole
// exchange must settle well under the guard timeout.
func TestEvalAwaitDisposeNoDeadlock(t *testing.T) {
	sess := newTestSession(t)

	done := make(chan struct{})
	go func() {
		// Infinite JS loop: blocks inside RunScript until TerminateExecution.
		_, _ = sess.EvalAwait(`while(true){}`, 30*time.Second)
		close(done)
	}()

	// Give RunScript time to enter the loop and hold the V8 Locker, then
	// dispose concurrently — the scenario that used to deadlock.
	time.Sleep(50 * time.Millisecond)
	disposed := make(chan struct{})
	go func() {
		sess.Dispose()
		close(disposed)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("EvalAwait did not return after Dispose — deadlock regression (EvalAwait still holding s.mu across RunScript?)")
	}
	select {
	case <-disposed:
	case <-time.After(10 * time.Second):
		t.Fatal("Dispose did not return — deadlock regression")
	}
}

// TestEvalAwaitGraceImmediateReturn guards the grace logic (no unconditional
// idle when no async work is pending): a plain non-Promise eval with zero
// pending timers/XHRs must return near-instantly, not wait for a fixed grace.
// Regression guard for the grace-period fix.
func TestEvalAwaitGraceImmediateReturn(t *testing.T) {
	sess := newTestSession(t)
	defer sess.Dispose()

	// Warm the context so first-run compile cost does not skew the bound.
	if _, err := sess.EvalAwait(`1+1`, time.Second); err != nil {
		t.Fatalf("warmup EvalAwait failed: %v", err)
	}

	const graceBound = 50 * time.Millisecond
	for i := 0; i < 20; i++ {
		start := time.Now()
		got, err := sess.EvalAwait(`"ok"`, 5*time.Second)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("EvalAwait failed: %v", err)
		}
		if got != "ok" {
			t.Fatalf("EvalAwait result = %q, want %q", got, "ok")
		}
		if elapsed > graceBound {
			t.Fatalf("iter %d: EvalAwait took %v, want <%v (grace idle regression)", i, elapsed, graceBound)
		}
	}
}

// TestEvalAwaitConcurrentDisposeStress runs several EvalAwait callers against a
// single session and disposes it mid-flight, repeated across sessions. No run
// may hang or race. This exercises the execWG/channel path under -race.
func TestEvalAwaitConcurrentDisposeStress(t *testing.T) {
	const sessions = 4
	const callers = 4

	for s := 0; s < sessions; s++ {
		sess := newTestSession(t)

		var wg sync.WaitGroup
		// Half the callers run a quick script, half a long loop, so Dispose
		// lands while some are inside RunScript and some are not.
		for c := 0; c < callers; c++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				code := `1+1`
				if idx%2 == 0 {
					code = `var t=0; for(var i=0;i<1e7;i++){t+=i}; t`
				}
				// Errors are expected once Dispose races in; we only care that
				// every call returns (no hang).
				_, _ = sess.EvalAwait(code, 5*time.Second)
			}(c)
		}

		// Dispose while callers are in flight.
		time.Sleep(20 * time.Millisecond)
		sess.Dispose()
		wg.Wait()
	}
}

// TestEvalAwaitDisposedReturnsEarly ensures that once a session is disposed,
// further EvalAwait calls refuse immediately with errDisposed instead of
// touching the closed context.
func TestEvalAwaitDisposedReturnsEarly(t *testing.T) {
	sess := newTestSession(t)
	sess.Dispose()

	start := time.Now()
	_, err := sess.EvalAwait(`1+1`, time.Second)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("EvalAwait on disposed session unexpectedly succeeded")
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("EvalAwait on disposed session took %v, want immediate refusal", elapsed)
	}
}

// TestEvalAwaitPromiseResult sanity-checks that the goroutine refactor still
// returns resolved Promise values correctly (the Promise drain branch).
func TestEvalAwaitPromiseResult(t *testing.T) {
	sess := newTestSession(t)
	defer sess.Dispose()

	got, err := sess.EvalAwait(`new Promise(function(r){setTimeout(function(){r("resolved")}, 10)})`, 2*time.Second)
	if err != nil {
		t.Fatalf("EvalAwait promise failed: %v", err)
	}
	if got != "resolved" {
		t.Fatalf("EvalAwait promise result = %q, want %q", got, "resolved")
	}
}

// TestEvalAwaitTimeoutTerminates verifies the timeout branch: a runaway loop
// must be terminated within roughly the timeout + force-quit bound, not hang.
func TestEvalAwaitTimeoutTerminates(t *testing.T) {
	sess := newTestSession(t)
	defer sess.Dispose()

	start := time.Now()
	_, err := sess.EvalAwait(`while(true){}`, 200*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("EvalAwait infinite loop unexpectedly succeeded")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("EvalAwait timeout took %v, force-quit did not engage", elapsed)
	}
	if fmt.Sprintf("%v", err) == "" {
		t.Fatal("expected non-empty timeout error")
	}
}

// TestEvalAwaitSetIntervalReturnsImmediately guards the setInterval deadlock
// fix (setinterval-deadlock). Before the fix, registering a recurring setInterval pinned
// PendingTimers() > 0 forever, so the non-Promise drain loop never exited and
// EvalAwait hit the 30s timeout via TerminateExecution. A real browser returns
// from eval at once when only a setInterval is registered; BES must match that.
// The eval must return the synchronous value 'hello' well under the 30s guard
// (and under a tight 2s bound that a healthy drain clears in milliseconds).
func TestEvalAwaitSetIntervalReturnsImmediately(t *testing.T) {
	sess := newTestSession(t)
	defer sess.Dispose()

	start := time.Now()
	got, err := sess.EvalAwait(`setInterval(function(){}, 1000); 'hello'`, 30*time.Second)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("EvalAwait setInterval failed: %v", err)
	}
	if got != "hello" {
		t.Fatalf("EvalAwait result = %q, want %q", got, "hello")
	}
	// A healthy drain with only a recurring timer pending returns in
	// milliseconds. 2s leaves headroom for CI jitter while still proving the
	// 30s deadlock is gone.
	if elapsed > 2*time.Second {
		t.Fatalf("EvalAwait with setInterval took %v, want <2s (30s deadlock regression)", elapsed)
	}
}

// TestEvalAwaitSetIntervalWithSettleableWork still drains settle-able work
// alongside an active setInterval: a one-shot setTimeout that pushes a value
// into a global must still fire and be observed, while the recurring setInterval
// does not pin the drain. This guards that excluding setInterval from the
// drain-exit condition did not also drop genuine one-shot settlement.
func TestEvalAwaitSetIntervalWithSettleableWork(t *testing.T) {
	sess := newTestSession(t)
	defer sess.Dispose()

	got, err := sess.EvalAwait(
		`setInterval(function(){}, 1000); setTimeout(function(){ globalThis.__hit = 'fired' }, 20); 'ok'`,
		5*time.Second)
	if err != nil {
		t.Fatalf("EvalAwait failed: %v", err)
	}
	if got != "ok" {
		t.Fatalf("EvalAwait result = %q, want %q", got, "ok")
	}
	// The one-shot setTimeout(20) should have fired during the drain and
	// written the global. Eval returns 'ok' (the sync value), but the side
	// effect must be visible.
	hit, err := sess.EvalAwait(`globalThis.__hit || 'miss'`, time.Second)
	if err != nil {
		t.Fatalf("EvalAwait __hit check failed: %v", err)
	}
	if hit != "fired" {
		t.Fatalf("one-shot setTimeout did not fire during drain; __hit = %q", hit)
	}
}
