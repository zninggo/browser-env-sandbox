package executor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Router is the executor switching mechanism: V8 by default, real-execution
// point classes (canvas/webgl/audio/wasm) to Chrome when it is available,
// automatic fallback to V8 whenever Chrome is missing, fails, or times out.
//
// The routing table (see docs/dual-executor.md):
//
//	class      → executor
//	---------------------------
//	js         → v8
//	navigator  → v8
//	screen     → v8
//	fonts      → v8   (a local Chrome would leak HOST fonts under a foreign
//	                    OS profile — font points stay mocked in V8 by design)
//	canvas     → chrome, fallback v8
//	webgl      → chrome, fallback v8
//	audio      → chrome, fallback v8
//	wasm       → chrome, fallback v8
type Router struct {
	defaultExec Executor // V8
	realExec    Executor  // Chrome; may be nil or unavailable

	// OnFallback, when set, is invoked with the class, the failed executor
	// name and the error whenever a real-class point falls back to V8.
	OnFallback func(class PointClass, from string, err error)

	mu sync.Mutex
	// stats counters (per class)
	routed  map[PointClass]int
	fellBack map[PointClass]int
}

// NewRouter builds a router. defaultExec must be non-nil (the V8 executor);
// realExec may be nil (all classes stay on V8).
func NewRouter(defaultExec, realExec Executor) (*Router, error) {
	if defaultExec == nil {
		return nil, fmt.Errorf("executor: router requires a default executor")
	}
	return &Router{
		defaultExec: defaultExec,
		realExec:    realExec,
		routed:      make(map[PointClass]int),
		fellBack:    make(map[PointClass]int),
	}, nil
}

// RealExecutor returns the real-execution backend (nil when none configured).
func (r *Router) RealExecutor() Executor { return r.realExec }

// DefaultExecutor returns the default backend.
func (r *Router) DefaultExecutor() Executor { return r.defaultExec }

// Route decides the executor for a class: real classes go to the Chrome
// executor when it is configured and Available; everything else (and Chrome
// being missing) stays on the default V8 path. This is the pure decision —
// no execution, no fallback — and is safe to call concurrently.
func (r *Router) Route(class PointClass) Executor {
	if RealClass(class) && r.realExec != nil && r.realExec.Available() {
		return r.realExec
	}
	return r.defaultExec
}

// EvalResult reports where a routed evaluation actually ran.
type EvalResult struct {
	Value    string
	Executor string // "chrome" | "v8" | "v8(fallback:chrome)"
	// Fallback is non-empty when the real-class point fell back to V8.
	Fallback string
}

// Eval routes one fingerprint-point evaluation: the class picks the executor,
// a real-class failure falls back to the V8 executor (recording the fallback),
// and the V8 default path never changes — Chrome being absent cannot slow
// down or break the common case.
func (r *Router) Eval(ctx context.Context, class PointClass, profile *ProfileSnapshot, code string, opts SessionOptions, timeout time.Duration) (EvalResult, error) {
	exec := r.Route(class)
	if exec == r.defaultExec {
		r.count(&r.routed, class)
		out, err := evalOn(ctx, exec, profile, code, opts, timeout)
		return EvalResult{Value: out, Executor: exec.Name()}, err
	}

	// Real class with Chrome available: try Chrome, fall back to V8 on any
	// failure (launch error, session error, eval error, timeout).
	r.count(&r.routed, class)
	out, err := evalOn(ctx, exec, profile, code, opts, timeout)
	if err == nil {
		return EvalResult{Value: out, Executor: exec.Name()}, nil
	}

	r.count(&r.fellBack, class)
	if r.OnFallback != nil {
		r.OnFallback(class, exec.Name(), err)
	}
	fbOut, fbErr := evalOn(ctx, r.defaultExec, profile, code, opts, timeout)
	res := EvalResult{
		Value:    fbOut,
		Executor: r.defaultExec.Name() + "(fallback:" + exec.Name() + ")",
		Fallback: err.Error(),
	}
	if fbErr != nil {
		return res, fmt.Errorf("chrome failed (%v); v8 fallback failed: %w", err, fbErr)
	}
	return res, nil
}

// SessionFor opens a long-lived session on the executor a class routes to.
// Unlike Eval, a session-level failure does not silently fall back — the
// caller owns the session lifecycle and prefers an explicit error over
// silently running canvas points on the mock path.
func (r *Router) SessionFor(ctx context.Context, class PointClass, profile *ProfileSnapshot, opts SessionOptions) (Session, error) {
	exec := r.Route(class)
	return exec.NewSession(ctx, profile, opts)
}

// Stats returns routing counters per class (routed total and fallbacks).
func (r *Router) Stats() (routed, fellBack map[PointClass]int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out1 := make(map[PointClass]int, len(r.routed))
	for k, v := range r.routed {
		out1[k] = v
	}
	out2 := make(map[PointClass]int, len(r.fellBack))
	for k, v := range r.fellBack {
		out2[k] = v
	}
	return out1, out2
}

func evalOn(ctx context.Context, exec Executor, profile *ProfileSnapshot, code string, opts SessionOptions, timeout time.Duration) (string, error) {
	sess, err := exec.NewSession(ctx, profile, opts)
	if err != nil {
		return "", err
	}
	defer sess.Dispose()
	return sess.Eval(ctx, code, timeout)
}

func (r *Router) count(m *map[PointClass]int, class PointClass) {
	r.mu.Lock()
	(*m)[class]++
	r.mu.Unlock()
}
