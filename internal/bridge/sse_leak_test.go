package bridge

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// TestConsoleBroadcasterCloseAllExitsSubscriber verifies item 5: when a session
// is reclaimed, closeAll closes every subscriber channel so an SSE goroutine
// blocked in `case msg, ok := <-ch: if !ok { return }` sees ok=false and exits.
// Before the fix, cleanupIdle disposed the session but never touched the
// broadcaster, so the SSE goroutine leaked — blocked forever on a 15s keepalive
// ticker against a dead session.
func TestConsoleBroadcasterCloseAllExitsSubscriber(t *testing.T) {
	b := newConsoleBroadcaster()
	ch, unsub := b.subscribe()

	// Simulate an SSE goroutine blocked in its receive select. It exits when the
	// channel closes (ok=false) — the same path streamConsole uses.
	exited := make(chan struct{})
	go func() {
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					close(exited)
					return
				}
			case <-time.After(50 * time.Millisecond):
				// keepalive tick — represents the 15s tick in production; short
				// here so the test doesn't hang when the fix is absent.
			}
		}
	}()

	// Reclaim the session: closeAll must unblock the goroutine immediately.
	b.closeAll()

	select {
	case <-exited:
		// SSE goroutine exited — no leak.
	case <-time.After(2 * time.Second):
		t.Fatal("SSE goroutine did not exit after closeAll — broadcaster leak")
	}

	// unsub must be safe to call after closeAll (idempotent, no double-close
	// panic) — this mirrors a client disconnecting after the session was
	// reclaimed, the two-path concern the reviewer flagged.
	unsub()
}

// TestNetworkBroadcasterCloseAllExitsSubscriber mirrors the console test for the
// network broadcaster — both must reclaim subscribers on session teardown.
func TestNetworkBroadcasterCloseAllExitsSubscriber(t *testing.T) {
	b := newNetworkBroadcaster()
	ch, unsub := b.subscribe()

	exited := make(chan struct{})
	go func() {
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					close(exited)
					return
				}
			case <-time.After(50 * time.Millisecond):
			}
		}
	}()

	b.closeAll()

	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("network SSE goroutine did not exit after closeAll — leak")
	}
	unsub()
}

// TestCloseAllConcurrentUnsubNoPanic verifies the two-path idempotency the
// reviewer required: closeAll (session reclaim path) and unsub (client
// disconnect path) running concurrently must never double-close a channel and
// panic. Both take the broadcaster write lock and check subs membership first.
func TestCloseAllConcurrentUnsubNoPanic(t *testing.T) {
	const rounds = 50
	for r := 0; r < rounds; r++ {
		b := newConsoleBroadcaster()
		ch, unsub := b.subscribe()

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					t.Errorf("double-close panic: %v", rec)
				}
			}()
			b.closeAll()
		}()
		go func() {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					t.Errorf("double-close panic in unsub: %v", rec)
				}
			}()
			unsub()
		}()
		// Drain ch so a buffered close doesn't deadlock the broadcaster; closeAll
		// closes it, and any pending send (none here) is non-blocking anyway.
		go func() { for range ch {} }()
		wg.Wait()
	}
}

// TestCloseSessionExitsSSESubscriber verifies the end-to-end item 5 fix via the
// Service layer: a CloseSession on a session with an open SSE subscriber must
// close the broadcaster so the consuming goroutine exits. This covers the
// client-initiated close path (cleanupIdle is timer-driven and slow to exercise
// deterministically, but shares the same closeAll call).
func TestCloseSessionExitsSSESubscriber(t *testing.T) {
	s := newTestService(t)
	id := mustCreateSession(t, s)

	ch, unsub, err := s.SubscribeConsole(id)
	if err != nil {
		t.Fatalf("SubscribeConsole: %v", err)
	}

	var exited int32
	go func() {
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					atomic.StoreInt32(&exited, 1)
					return
				}
			case <-time.After(50 * time.Millisecond):
			}
		}
	}()

	if err := s.CloseSession(id); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		if atomic.LoadInt32(&exited) == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("SSE goroutine did not exit after CloseSession — leak")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// unsub after CloseSession must not panic (client disconnects after server
	// closed the session) — the idempotent path.
	unsub()
}

// TestListSessionsReleasesLockBeforeFingerprint verifies item 6: ListSessions
// snapshots the entry list under RLock and releases the lock before calling
// GetFingerprint. A CreateSession write lock acquired mid-ListSessions must not
// deadlock — proof the read lock is no longer held across GetFingerprint. Under
// the old code GetFingerprint ran under the held RLock and a CreateSession
// (which needs the write lock) could block all new session creation while N
// fingerprints were read.
func TestListSessionsReleasesLockBeforeFingerprint(t *testing.T) {
	s := newTestService(t)
	// Seed a few sessions so ListSessions has real work.
	for i := 0; i < 4; i++ {
		mustCreateSession(t, s)
	}


	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Hammer ListSessions.
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

	// Concurrently create+close sessions. If ListSessions held the read lock
	// across GetFingerprint, these write-lock acquisitions would serialize
	// behind it and throughput would collapse / potentially deadlock under
	// contention. They must make progress.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			id, _, err := s.CreateSession(api.SessionOptions{Browser: "chrome", OS: "windows"})
			if err != nil {
				t.Errorf("CreateSession blocked/failed under ListSessions: %v", err)
				return
			}
			_ = s.CloseSession(id)
		}
		close(stop)
	}()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ListSessions held the read lock across GetFingerprint — CreateSession deadlocked")
	}
}
