package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/sandbox"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// V8Executor is the default executor: the pure-V8 sandbox. It is fast (pooled
// isolates), hostless (zero Node/process pollution) and answers fingerprint
// points from the profile's replayed values. It is always Available.
type V8Executor struct {
	engine *sandbox.Engine
}

// NewV8Executor builds a V8 executor around a sandbox engine pool. poolSize
// follows sandbox.New semantics (<= 0 → 4).
func NewV8Executor(fp sandbox.FingerprintProvider, poolSize int) *V8Executor {
	return &V8Executor{engine: sandbox.New(fp, poolSize)}
}

// Engine exposes the underlying sandbox engine (for bridge-level reuse).
func (e *V8Executor) Engine() *sandbox.Engine { return e.engine }

// Name implements Executor.
func (e *V8Executor) Name() string { return "v8" }

// Available implements Executor — the V8 executor is always available.
func (e *V8Executor) Available() bool { return true }

// NewSession implements Executor: creates a sandbox session driven by the
// snapshot's fingerprint via sandbox.Engine.CreateSessionWithFingerprint —
// the SAME fingerprint object the Chrome executor injects, which is what
// guarantees cross-executor identity consistency.
func (e *V8Executor) NewSession(ctx context.Context, profile *ProfileSnapshot, opts SessionOptions) (Session, error) {
	if profile == nil || profile.Fingerprint == nil {
		return nil, fmt.Errorf("executor: profile required")
	}
	sess, err := e.engine.CreateSessionWithFingerprint(api.SessionOptions{
		Seed:     profile.Seed,
		Browser:  profile.Browser,
		OS:       profile.OS,
		Timezone: profile.Timezone,
		Location: opts.Location,
	}, profile.Fingerprint)
	if err != nil {
		return nil, err
	}
	return &v8Session{sess: sess}, nil
}

// Dispose implements Executor — releases the isolate pool.
func (e *V8Executor) Dispose() { e.engine.Dispose() }

// v8Session adapts sandbox.Session to the executor Session interface.
type v8Session struct {
	sess *sandbox.Session
}

func (s *v8Session) Executor() string { return "v8" }

func (s *v8Session) Eval(ctx context.Context, code string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	// The sandbox evaluates with its own timeout; honor a shorter caller
	// deadline by bounding the wait via ctx.
	type res struct {
		val string
		err error
	}
	ch := make(chan res, 1)
	go func() {
		val, err := s.sess.EvalAwait(code, timeout)
		ch <- res{val, err}
	}()
	select {
	case r := <-ch:
		return r.val, r.err
	case <-ctx.Done():
		// EvalAwait's own timeout still bounds the goroutine; the session
		// stays usable for the next eval.
		return "", ctx.Err()
	}
}

func (s *v8Session) Dispose() { s.sess.Dispose() }
