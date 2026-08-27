// Package sandbox builds a browser environment inside a pure V8 Isolate
// and executes target JavaScript within it.
//
// Key advantage over Node.js vm: v8go Isolates have ZERO Node pollution.
// Buffer, process, require simply don't exist — not "set to undefined" but
// "never existed". This is the fundamental difference.
package sandbox

import (
	"fmt"
	"log"
	"sync"
	"time"

	"rogchap.com/v8go"

	"github.com/xiaoxun/bes/pkg/api"
)

// Engine manages V8 Isolates and creates sandbox sessions.
type Engine struct {
	pool   *IsolatePool
	fpEng  FingerprintProvider
	mu     sync.Mutex
}

// FingerprintProvider generates fingerprints (interface for dependency injection).
type FingerprintProvider interface {
	Generate(seed uint64, browser, os string) (*api.Fingerprint, error)
}

// New creates a sandbox engine with the given fingerprint provider.
func New(fpEng FingerprintProvider, poolSize int) *Engine {
	if poolSize <= 0 {
		poolSize = 4
	}
	return &Engine{
		pool:  NewIsolatePool(poolSize),
		fpEng: fpEng,
	}
}

// Session is a single sandbox execution context.
type Session struct {
	ID           string
	iso          *v8go.Isolate
	ctx          *v8go.Context
	fp           *api.Fingerprint
	location     string
	cookieStore  *CookieStore
	timers       *TimerManager
	netHandler   NetHandler
	disposed     bool
	mu           sync.Mutex
}

// NetHandler handles network requests from within the sandbox.
type NetHandler interface {
	// Request processes an HTTP request and returns the response.
	Request(method, url string, headers map[string]string, body []byte) (*NetResponse, error)
}

// NetResponse is a network response returned to the sandbox.
type NetResponse struct {
	Status  int
	Headers map[string]string
	Body    string
	Cookies map[string]string
}

// CreateSession creates a new sandbox session with the given options.
func (e *Engine) CreateSession(opts api.SessionOptions) (*Session, error) {
	// 1. Generate fingerprint
	fp, err := e.fpEng.Generate(opts.Seed, opts.Browser, opts.OS)
	if err != nil {
		return nil, fmt.Errorf("fingerprint generation failed: %w", err)
	}

	// 2. Get Isolate from pool
	iso := e.pool.Get()

	// 3. Create context with global template
	global := v8go.NewObjectTemplate(iso)

	// 4. Build browser environment and inject into template
	location := opts.Location
	if location == "" {
		location = "https://example.com/"
	}
	cookieStore := NewCookieStore(opts.Cookies)
	timerMgr := NewTimerManager()

	builder := &EnvBuilder{
		iso:         iso,
		global:      global,
		fp:          fp,
		location:    location,
		cookieStore: cookieStore,
		timerMgr:    timerMgr,
	}
	builder.Build()

	// 5. Create context
	ctx := v8go.NewContext(iso, global)

	// 6. Post-context setup (things that need the context, not just template)
	postBuilder := &PostContextBuilder{
		iso:         iso,
		ctx:         ctx,
		global:      ctx.Global(),
		fp:          fp,
		location:    location,
		cookieStore: cookieStore,
		timerMgr:    timerMgr,
	}
	postBuilder.Build()

	sess := &Session{
		ID:          generateSessionID(),
		iso:         iso,
		ctx:         ctx,
		fp:          fp,
		location:    location,
		cookieStore: cookieStore,
		timers:      timerMgr,
	}

	log.Printf("[sandbox] session %s created: %s @ %s", sess.ID, fp.Browser.Name+"/"+fp.Browser.Version, fp.OS.Name)
	return sess, nil
}

// Eval executes JavaScript in the sandbox.
func (s *Session) Eval(code string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, err := s.ctx.RunScript(code, "sandbox-eval.js")
	if err != nil {
		if jsErr, ok := err.(*v8go.JSError); ok {
			return "", fmt.Errorf("JS error: %s\n%s", jsErr.Message, jsErr.StackTrace)
		}
		return "", err
	}
	if val == nil {
		return "", nil
	}
	return val.String(), nil
}

// EvalRaw executes JavaScript and returns the raw v8go Value.
func (s *Session) EvalRaw(code string) (*v8go.Value, error) {
	return s.ctx.RunScript(code, "sandbox-eval.js")
}

// LoadScript loads and executes a script file (by content).
func (s *Session) LoadScript(name, content string) error {
	_, err := s.ctx.RunScript(content, name)
	return err
}

// CallFunction calls a global function by name with string arguments.
func (s *Session) CallFunction(name string, args ...string) (string, error) {
	// Build the call expression
	argStr := ""
	for i, a := range args {
		if i > 0 {
			argStr += ","
		}
		argStr += fmt.Sprintf("%q", a)
	}
	code := fmt.Sprintf("%s(%s)", name, argStr)
	return s.Eval(code)
}

// FlushTimers waits for all pending timers to complete.
func (s *Session) FlushTimers(timeout time.Duration) error {
	return s.timers.Flush(timeout)
}

// PerformMicrotasks runs pending microtasks (Promise callbacks).
func (s *Session) PerformMicrotasks() {
	s.ctx.PerformMicrotaskCheckpoint()
}

// GetFingerprint returns the session's fingerprint.
func (s *Session) GetFingerprint() *api.Fingerprint {
	return s.fp
}

// GetCookies returns the current cookie jar as a string.
func (s *Session) GetCookies() string {
	return s.cookieStore.String()
}

// SetCookie sets a cookie in the sandbox.
func (s *Session) SetCookie(name, value string) {
	s.cookieStore.Set(name, value, "/", "")
}

// Dispose releases all resources.
func (s *Session) Dispose() {
	if s.disposed {
		return
	}
	s.disposed = true
	s.timers.StopAll()
	s.ctx.Close()
	// Note: Isolate is returned to pool by Engine, not disposed here
	log.Printf("[sandbox] session %s disposed", s.ID)
}

// Close returns the Isolate to the pool (called by Engine).
func (s *Session) close(pool *IsolatePool) {
	s.Dispose()
	pool.Put(s.iso)
}

func generateSessionID() string {
	return fmt.Sprintf("sess-%d", time.Now().UnixNano())
}

// Dispose releases all Isolates in the pool.
func (e *Engine) Dispose() {
	e.pool.DisposeAll()
}
