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

	"github.com/zninggo/bes/pkg/api"
)

// Engine manages V8 Isolates and creates sandbox sessions.
type Engine struct {
	pool               *IsolatePool
	fpEng              FingerprintProvider
	consoleSinkFactory ConsoleSinkFactory
	netHandlerFactory  NetHandlerFactory
	mu                 sync.Mutex
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

// SetConsoleSinkFactory configures a factory that produces a fresh ConsoleSink
// for each new session. When set, console.* calls emitted from inside the
// sandbox are routed to the per-session sink (instead of printed to stdout),
// which enables per-session streaming (e.g. SSE console push from the bridge).
//
// If never called (the default), console output falls back to fmt.Printf and
// existing behaviour (including the CLI) is unchanged.
func (e *Engine) SetConsoleSinkFactory(f ConsoleSinkFactory) {
	e.consoleSinkFactory = f
}

// NetHandlerFactory creates a NetHandler for a new session. When set, each
// session's XHR/fetch make real HTTP requests through the handler. When nil
// (default), XHR/fetch are stubs — suitable for pure JS execution / signing.
type NetHandlerFactory func(opts api.SessionOptions, cookieStore *CookieStore) NetHandler

// SetNetHandlerFactory configures a factory that produces a NetHandler for
// each new session. This wires the sandbox's XHR/fetch to a real network
// stack (e.g. the netlayer package with curl_cffi TLS fingerprinting).
func (e *Engine) SetNetHandlerFactory(f NetHandlerFactory) {
	e.netHandlerFactory = f
}

// Session is a single sandbox execution context.
type Session struct {
	ID          string
	iso         *v8go.Isolate
	ctx         *v8go.Context
	fp          *api.Fingerprint
	location    string
	cookieStore *CookieStore
	timers      *TimerManager
	netHandler  NetHandler
	consoleSink ConsoleSink
	disposed    bool
	mu          sync.Mutex
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

	// Optional per-session console sink. When a factory is configured (e.g. by
	// the bridge layer), each session gets its own sink so console.* output can
	// be streamed independently. nil falls back to fmt.Printf inside the env.
	var consoleSink ConsoleSink
	if e.consoleSinkFactory != nil {
		consoleSink = e.consoleSinkFactory()
	}

	// Optional per-session net handler. When a factory is configured, each
	// session's XHR/fetch make real HTTP requests. nil = stubs (offline mode).
	var netHandler NetHandler
	if e.netHandlerFactory != nil {
		netHandler = e.netHandlerFactory(opts, cookieStore)
	}

	builder := &EnvBuilder{
		iso:         iso,
		global:      global,
		fp:          fp,
		location:    location,
		cookieStore: cookieStore,
		timerMgr:    timerMgr,
		consoleSink: consoleSink,
		netHandler:  netHandler,
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
		netHandler:  netHandler,
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
		consoleSink: consoleSink,
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

// ConsoleSink returns the per-session console sink, if one was configured via
// Engine.SetConsoleSinkFactory. The bridge layer type-asserts this to its own
// broadcaster implementation in order to subscribe to console events for SSE.
// Returns nil when no factory was set (console output goes to stdout instead).
func (s *Session) ConsoleSink() ConsoleSink {
	return s.consoleSink
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
