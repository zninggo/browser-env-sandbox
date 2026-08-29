package sandbox

import (
	"context"
	"fmt"
	"time"

	"github.com/zninggo/v8go"
)

// EvalWithTimeout executes JavaScript with a timeout.
// If the script runs longer than the timeout, the V8 Isolate's execution
// is terminated, preventing infinite loops from hanging the session.
//
// Concurrency model (P0 fix for EvalWithTimeout/Dispose deadlock +
// use-after-dispose): RunScript runs on a goroutine that does NOT hold s.mu.
// s.mu is acquired only briefly to check disposed and register the in-flight
// execution (s.execWG). This lets Dispose acquire s.mu, set disposed, and
// request termination without deadlocking against a blocked script — which the
// previous "lock-then-block-on-RunScript" design could not do.
//
// V8 serialization is preserved by v8go's C++ v8::Locker (one Isolate, one
// executing thread at a time): if EvalAwait/EvalRaw run concurrently they block
// on the Locker until this goroutine's RunScript returns. Native callbacks
// inside the script may re-enter s.mu because it is not held here.
//
// TerminateExecution interrupts pure-JS loops (verified by v8go's own tests)
// but cannot interrupt a native callback blocked in Go. The force-quit wait
// caps that worst case so neither the caller nor s.mu hangs indefinitely.
func (s *Session) EvalWithTimeout(code string, timeout time.Duration) (string, error) {
	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		return "", errDisposed
	}
	s.execWG.Add(1) // track in-flight execution so Dispose can wait it out
	s.mu.Unlock()

	type result struct {
		val string
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		defer s.execWG.Done()
		// recover guards against a cgo panic propagating from a V8 crash
		// inside RunScript: without it, an unrecovered goroutine panic
		// terminates the whole process. Convert it to an error result so the
		// caller sees a failure and s.execWG still drains via the defer above.
		defer func() {
			if r := recover(); r != nil {
				resultCh <- result{"", fmt.Errorf("sandbox: v8 panic during RunScript: %v", r)}
			}
		}()
		val, err := s.ctx.RunScript(code, "sandbox-eval-timeout.js")
		if err != nil {
			if jsErr, ok := err.(*v8go.JSError); ok {
				resultCh <- result{"", fmt.Errorf("JS error: %s\n%s", jsErr.Message, jsErr.StackTrace)}
				return
			}
			resultCh <- result{"", err}
			return
		}
		if val == nil {
			resultCh <- result{"", nil}
			return
		}
		resultCh <- result{val.String(), nil}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case r := <-resultCh:
		return r.val, r.err
	case <-ctx.Done():
		// Request V8 to terminate. TerminateExecution does not take the V8
		// Locker, so it is safe to call from this goroutine while the
		// execution goroutine holds the Locker inside RunScript.
		s.iso.TerminateExecution()
		// Wait for the goroutine to finish. Under normal JS termination it
		// returns promptly with a termination error. If it is stuck inside a
		// native callback that TerminateExecution cannot interrupt, bail out
		// after evalForceQuitTimeout instead of blocking forever — s.mu is not
		// held here, so this never leaks the lock.
		select {
		case r := <-resultCh:
			if r.err != nil {
				return "", fmt.Errorf("execution terminated after %v: %w", timeout, r.err)
			}
			return "", fmt.Errorf("execution terminated after %v", timeout)
		case <-time.After(evalForceQuitTimeout):
			return "", fmt.Errorf("execution terminated after %v (script did not exit within %v; possible blocked native callback)", timeout, evalForceQuitTimeout)
		}
	}
}

// PrecompileScript compiles a script once for reuse across multiple contexts.
// Uses v8go's CompileUnboundScript + code caching for performance.
type PrecompiledScript struct {
	unbound *v8go.UnboundScript
	cache   *v8go.CompilerCachedData
	name    string
}

// CompileScript precompiles a script in the given Isolate.
func CompileScript(iso *v8go.Isolate, source, name string) (*PrecompiledScript, error) {
	unbound, err := iso.CompileUnboundScript(source, name, v8go.CompileOptions{
		Mode: v8go.CompileModeEager,
	})
	if err != nil {
		return nil, fmt.Errorf("compile failed: %w", err)
	}
	return &PrecompiledScript{
		unbound: unbound,
		cache:   unbound.CreateCodeCache(),
		name:    name,
	}, nil
}

// Run executes a precompiled script in the session's context.
func (s *Session) RunPrecompiled(ps *PrecompiledScript) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disposed {
		return "", errDisposed
	}
	val, err := ps.unbound.Run(s.ctx)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	return val.String(), nil
}

// LoadPrecompiledScripts precompiles and caches common scripts.
// Call this during Isolate pre-warm to speed up subsequent executions.
type ScriptCache struct {
	scripts map[string]*PrecompiledScript
}

// NewScriptCache creates an empty script cache.
func NewScriptCache() *ScriptCache {
	return &ScriptCache{scripts: make(map[string]*PrecompiledScript)}
}

// Compile compiles a script and stores it in the cache.
func (sc *ScriptCache) Compile(iso *v8go.Isolate, name, source string) error {
	ps, err := CompileScript(iso, source, name)
	if err != nil {
		return err
	}
	sc.scripts[name] = ps
	return nil
}

// Get retrieves a precompiled script by name.
func (sc *ScriptCache) Get(name string) *PrecompiledScript {
	return sc.scripts[name]
}

// Names returns all cached script names.
func (sc *ScriptCache) Names() []string {
	names := make([]string, 0, len(sc.scripts))
	for name := range sc.scripts {
		names = append(names, name)
	}
	return names
}
