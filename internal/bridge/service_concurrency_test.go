package bridge

import (
	"sync"
	"testing"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
	"github.com/zninggo/browser-env-sandbox/internal/sandbox"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// newTestService builds a real Service (real engine + real V8) for bridge
// concurrency tests. The sandbox stack is required to exercise the
// acquire/CloseSession lifecycle path.
func newTestService(t *testing.T) *Service {
	t.Helper()
	eng := sandbox.New(fpengine.New(), 2)
	s := NewService(eng, "")
	t.Cleanup(s.Dispose)
	return s
}

func mustCreateSession(t *testing.T, s *Service) string {
	t.Helper()
	id, _, err := s.CreateSession(api.SessionOptions{Browser: "chrome", OS: "windows"})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	return id
}

// TestEvalConcurrentCloseSessionNoCrash verifies the H4 fix: concurrent Eval
// (which acquires the entry, releases the lock, then runs EvalAwait) and
// CloseSession (which marks closing + removes the entry under the lock, then
// Disposes) must not race into a use-after-dispose. The sandbox layer's
// disposed/execWG guard prevents the crash; acquire makes the bridge layer's
// lifecycle explicit so a CloseSession landing mid-Eval lets the in-flight op
// finish against a still-valid pointer while new ops refuse. Under -race this
// must complete without a detected race or a hang.
func TestEvalConcurrentCloseSessionNoCrash(t *testing.T) {
	s := newTestService(t)

	const rounds = 8
	for r := 0; r < rounds; r++ {
		id := mustCreateSession(t, s)

		var wg sync.WaitGroup
		// Several eval callers; some quick, some long, so CloseSession lands
		// while ops are in flight.
		for c := 0; c < 4; c++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				code := `1+1`
				if idx%2 == 0 {
					code = `var t=0; for(var i=0;i<1e6;i++){t+=i}; t`
				}
				// Errors are expected once CloseSession races in.
				_, _ = s.Eval(id, code)
			}(c)
		}

		// Close while evals are in flight.
		time.Sleep(10 * time.Millisecond)
		_ = s.CloseSession(id)
		wg.Wait()

		// After close, new ops must refuse (entry is gone/closing), not touch a
		// disposed session.
		if _, err := s.Eval(id, `1+1`); err == nil {
			t.Fatalf("round %d: Eval on closed session unexpectedly succeeded", r)
		}
	}
}

// TestListSessionsConcurrentSwapFingerprintNoRace verifies the H5 fix at the
// service layer: ListSessions holds s.mu.RLock and calls GetFingerprint (now
// internally locked on session.mu), while SwapFingerprint writes the
// fingerprint. Under -race the cross-lock read of s.fp must be clean.
func TestListSessionsConcurrentSwapFingerprintNoRace(t *testing.T) {
	s := newTestService(t)
	id := mustCreateSession(t, s)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Reader: hammer ListSessions (reads every session's fingerprint).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = s.ListSessions()
			}
		}
	}()

	// Writer: swap the fingerprint repeatedly via the service layer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, _ = s.SwapFingerprint(id, api.SessionOptions{
				Browser: "chrome", OS: "windows", Seed: uint64(i + 1),
			})
		}
		close(stop)
	}()

	wg.Wait()
}
