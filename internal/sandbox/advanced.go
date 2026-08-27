package sandbox

import (
	"context"
	"fmt"
	"time"

	"rogchap.com/v8go"
)

// EvalWithTimeout executes JavaScript with a timeout.
// If the script runs longer than the timeout, the V8 Isolate's execution
// is terminated, preventing infinite loops from hanging the session.
//
// This uses v8go.Isolate.TerminateExecution() in a goroutine.
func (s *Session) EvalWithTimeout(code string, timeout time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	type result struct {
		val string
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
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
		// Terminate V8 execution
		s.iso.TerminateExecution()
		// Wait for the goroutine to finish (it will get a termination error)
		r := <-resultCh
		if r.err != nil {
			return "", fmt.Errorf("execution terminated after %v: %w", timeout, r.err)
		}
		return "", fmt.Errorf("execution terminated after %v", timeout)
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
