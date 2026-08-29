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
	"strings"
	"sync"
	"time"

	"github.com/zninggo/v8go"

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
	GenerateWithTimezone(seed uint64, browser, os, timezone string) (*api.Fingerprint, error)
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
// The fingerprint is passed so the network stack (TLS preset, UA) can be
// aligned with the session's browser version.
type NetHandlerFactory func(opts api.SessionOptions, fp *api.Fingerprint, cookieStore *CookieStore) NetHandler

// SetNetHandlerFactory configures a factory that produces a NetHandler for
// each new session. This wires the sandbox's XHR/fetch to a real network
// stack (e.g. the netlayer package with utls TLS fingerprinting).
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
	proxyURL    string
	cookieStore *CookieStore
	timers      *TimerManager
	netHandler  NetHandler
	consoleSink ConsoleSink
	workers     *workerRegistry
	websockets  *wsRegistry
	disposed    bool
	mu          sync.Mutex
	execWG      sync.WaitGroup // tracks in-flight EvalWithTimeout goroutines so Dispose can wait them out
	pool        *IsolatePool   // Bug 1 fix: return Isolate on Dispose
}

// Disposal / concurrency constants for EvalWithTimeout + Dispose.
//
// EvalWithTimeout runs RunScript on an unlocked goroutine (so Dispose can take
// s.mu and set disposed without deadlocking against a blocked script). Two
// bounded waits prevent unbounded hangs when a native callback cannot be
// interrupted by TerminateExecution:
//   - evalForceQuitTimeout bounds how long EvalWithTimeout waits for the
//     goroutine to exit after requesting termination.
//   - disposeExecWaitTimeout bounds how long Dispose waits for in-flight
//     goroutines before force-closing the context.
const (
	evalForceQuitTimeout   = 2 * time.Second
	disposeExecWaitTimeout = 5 * time.Second
)

// errDisposed is returned by V8 entry points when the session has already been
// disposed (or is being disposed). Dispose sets s.disposed under s.mu before
// tearing down, so a non-nil result means the context is gone or going.
var errDisposed = fmt.Errorf("sandbox: session already disposed")

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
	// BodyB64 is the raw response body base64-encoded (std encoding). Binary
	// payloads must travel via this field: Body is a Go string that cannot
	// hold non-UTF-8 bytes losslessly across the JSON serialization boundary
	// into V8. When BodyB64 is non-empty, JS consumers decode it with
	// __besB64ToUint8Array; Body is then derived (latin1 semantics: each byte
	// maps to the same char code) so text()-style accessors keep working.
	BodyB64    string
	Cookies    map[string]string
	// SetCookies holds raw Set-Cookie header values (one per line) so the
	// sandbox can store them with full attributes, scoped to the response host.
	SetCookies []string
}

// CreateSession creates a new sandbox session with the given options.
func (e *Engine) CreateSession(opts api.SessionOptions) (*Session, error) {
	// 1. Generate fingerprint (timezone-constrained when opts.Timezone is set)
	fp, err := e.fpEng.GenerateWithTimezone(opts.Seed, opts.Browser, opts.OS, opts.Timezone)
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
		netHandler = e.netHandlerFactory(opts, fp, cookieStore)
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
		consoleSink: consoleSink,
		opts:        opts,
	}
	postBuilder.Build()

	sess := &Session{
		ID:          generateSessionID(),
		iso:         iso,
		ctx:         ctx,
		fp:          fp,
		location:    location,
		proxyURL:    opts.Proxy,
		cookieStore: cookieStore,
		timers:      timerMgr,
		netHandler:  netHandler,
		consoleSink: consoleSink,
		workers:     newWorkerRegistry(),
		websockets:  newWSRegistry(),
		pool:        e.pool, // Bug 1 fix: return Isolate on Dispose
	}
	// Inject the real Worker constructor (dedicated isolate per worker),
	// overwriting the env_shim stub.
	injectWorkerConstructor(postBuilder, sess)
	injectWebSocketConstructor(postBuilder, sess)

	log.Printf("[sandbox] session %s created: %s @ %s", sess.ID, fp.Browser.Name+"/"+fp.Browser.Version, fp.OS.Name)
	return sess, nil
}

// Eval executes JavaScript in the sandbox.
// If the result is a Promise, it awaits it (flushing timers/microtasks)
// and returns the resolved value instead of "[object Promise]".
func (s *Session) Eval(code string) (string, error) {
	return s.EvalAwait(code, 30*time.Second)
}

// EvalAwait executes JavaScript; a Promise result is awaited by draining
// the timer loop + microtask checkpoints until it settles or the timeout
// elapses. Non-Promise results return immediately.
func (s *Session) EvalAwait(code string, timeout time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disposed {
		return "", errDisposed
	}

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

	// Await Promise results: poll pending timers + microtask checkpoint so
	// setTimeout-driven resolution actually runs, up to the timeout.
	if val.IsPromise() {
		p, err := val.AsPromise()
		if err != nil {
			return "", err
		}
		deadline := time.Now().Add(timeout)
		for {
			s.timers.DrainCallbacks() // Bug 2 fix: execute queued callbacks on Isolate thread
			s.timers.Flush(100 * time.Millisecond)
			s.ctx.PerformMicrotaskCheckpoint()
			switch p.State() {
			case v8go.Fulfilled:
				if pending, perr := s.ctx.RunScript("typeof __besPendingXHR !== 'undefined' ? __besPendingXHR : 0", "xhr-pending-check.js"); perr == nil && pending != nil {
					if pending.String() != "0" {
						// Bug 13 fix: check deadline before continuing
						if time.Now().After(deadline) {
							return "", fmt.Errorf("promise fulfilled but %s XHR still pending past %v timeout", pending.String(), timeout)
						}
						continue
					}
				}
				return p.Result().String(), nil
			case v8go.Rejected:
				return "", fmt.Errorf("promise rejected: %s", p.Result().String())
			}
			if time.Now().After(deadline) {
				return "", fmt.Errorf("promise pending past %v timeout", timeout)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	// Non-Promise: drain pending async XHR/fetch + worker messages so
	// fire-and-forget calls settle before the eval returns. Worker replies
	// arrive via scheduleTimer(0) into callbackQueue; DrainCallbacks executes
	// them on this (isolate) thread. Spin for a short grace period even when
	// no XHR is pending, so worker onmessage callbacks get a chance to run.
	deadline := time.Now().Add(timeout)
	grace := time.Now().Add(500 * time.Millisecond) // minimum spin for worker msgs
	for {
		s.timers.DrainCallbacks() // Bug 2 fix
		s.timers.Flush(100 * time.Millisecond)
		s.ctx.PerformMicrotaskCheckpoint()
		pending, perr := s.ctx.RunScript("typeof __besPendingXHR !== 'undefined' ? __besPendingXHR : 0", "xhr-pending-check.js")
		xhrBusy := perr == nil && pending != nil && pending.String() != "0"
		if !xhrBusy && time.Now().After(grace) {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return val.String(), nil
}

// EvalRaw executes JavaScript and returns the raw v8go Value.
// Bug 15 fix: acquire s.mu to prevent concurrent V8 access.
func (s *Session) EvalRaw(code string) (*v8go.Value, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disposed {
		return nil, errDisposed
	}
	return s.ctx.RunScript(code, "sandbox-eval.js")
}

// LoadScript loads and executes a script file (by content).
// Bug 14 fix: acquire s.mu + check disposed to prevent concurrent V8 access
// and use-after-dispose (Dispose closes s.ctx under s.mu).
func (s *Session) LoadScript(name, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disposed {
		return errDisposed
	}
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
// Bug 14 fix: acquire s.mu + check disposed. The signature stays func() with
// no return value (callers cmd/bes, bes-selftest ignore it), so a disposed
// session silently returns instead of touching the closed context.
func (s *Session) PerformMicrotasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disposed {
		return
	}
	s.ctx.PerformMicrotaskCheckpoint()
}

// DrainWorkerCallbacks runs pending worker→parent message callbacks on the
// parent isolate thread. The worker outbound pump uses scheduleTimer(0) to
// deliver replies, which fires into callbackQueue via a goroutine. This
// method spins on DrainCallbacks (non-blocking) + short sleeps to give those
// goroutines time to land, up to `wait`.
//
// Unlike FlushTimers (which blocks waiting for timer entries), this only
// drains the callback queue — no deadlock with the pump's timer goroutines.
func (s *Session) DrainWorkerCallbacks(wait time.Duration) {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		s.timers.DrainCallbacks()
		s.ctx.PerformMicrotaskCheckpoint()
		time.Sleep(5 * time.Millisecond)
	}
}

// GetFingerprint returns the session's fingerprint.
func (s *Session) GetFingerprint() *api.Fingerprint {
	return s.fp
}

// SwapFingerprint hot-swaps the session's fingerprint without rebuilding the
// V8 context. It generates a new fingerprint from the engine, then overwrites
// the navigator/screen/window/global properties that fingerprint-detection JS
// reads (userAgent, platform, languages, screen dims, WebGL params, canvas
// hash, etc.). Existing cookies, timers, and loaded scripts are preserved.
//
// This is the "snapshot/fingerprint hot-swap" feature: a long-lived session
// can rotate its identity mid-flight (useful for multi-step scraping where
// each step should look like a different visitor).
func (s *Session) SwapFingerprint(eng *Engine, opts api.SessionOptions) (*api.Fingerprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fp, err := eng.fpEng.GenerateWithTimezone(opts.Seed, opts.Browser, opts.OS, opts.Timezone)
	if err != nil {
		return nil, fmt.Errorf("fingerprint generation failed: %w", err)
	}
	s.fp = fp
	s.location = opts.Location
	if s.location == "" {
		s.location = "https://example.com/"
	}

	// Build a JS snippet that overwrites all fingerprint-derived globals.
	// Uses __besSafeDefine (defined in the IIFE below) to gracefully handle
	// non-configurable properties left over from the initial template injection.
	// navigator (primitives)
	navLines := []string{}
	for k, v := range fp.Navigator {
		if k == "webdriver" || k == "toString" {
			continue
		}
		switch val := v.(type) {
		case string:
			navLines = append(navLines, fmt.Sprintf("__besSafeDefine(navigator,%q,%q);", k, val))
		case bool:
			navLines = append(navLines, fmt.Sprintf("__besSafeDefine(navigator,%q,%t);", k, val))
		case int, int32:
			navLines = append(navLines, fmt.Sprintf("__besSafeDefine(navigator,%q,%d);", k, val))
		case float64:
			navLines = append(navLines, fmt.Sprintf("__besSafeDefine(navigator,%q,%v);", k, val))
		}
	}

	// screen
	screenLines := []string{}
	for k, v := range fp.Screen {
		switch val := v.(type) {
		case int, int32:
			screenLines = append(screenLines, fmt.Sprintf("__besSafeDefine(screen,%q,%d);", k, val))
		case float64:
			screenLines = append(screenLines, fmt.Sprintf("__besSafeDefine(screen,%q,%v);", k, val))
		}
	}

	// window dims
	w := fp.Window
	windowLines := []string{
		fmt.Sprintf("innerWidth=%d;innerHeight=%d;outerWidth=%d;outerHeight=%d;devicePixelRatio=%v;",
			w.InnerWidth, w.InnerHeight, w.OuterWidth, w.OuterHeight, w.DevicePixelRatio),
	}

	// languages array
	langArr := "["
	for i, l := range fp.Languages {
		if i > 0 {
			langArr += ","
		}
		langArr += fmt.Sprintf("%q", l)
	}
	langArr += "]"
	langFirst := "en"
	if len(fp.Languages) > 0 {
		langFirst = fp.Languages[0]
	}

	// userAgentData
	uad := fp.Navigator["userAgentData"]
	uadJSON := "{}"
	if uad != nil {
		if m, ok := uad.(map[string]any); ok {
			brandsStr := "[]"
			if brands, ok := m["brands"].([]map[string]any); ok && len(brands) > 0 {
				parts := []string{}
				for _, b := range brands {
					parts = append(parts, fmt.Sprintf(`{"brand":%q,"version":%q}`, b["brand"], b["version"]))
				}
				brandsStr = "[" + strings.Join(parts, ",") + "]"
			}
			platform, _ := m["platform"].(string)
			mobile, _ := m["mobile"].(bool)
			uadJSON = fmt.Sprintf(`{"brands":%s,"mobile":%t,"platform":%q}`, brandsStr, mobile, platform)
		}
	}

	// canvas hash (toDataURL)
	canvasHash := fp.Canvas.ToDataURLHash

	// WebGL params (JSON map of param int → value string)
	webglParams := "{}"
	if len(fp.WebGL.Params) > 0 {
		parts := []string{}
		for k, v := range fp.WebGL.Params {
			parts = append(parts, fmt.Sprintf(`"%d":%q`, k, v))
		}
		webglParams = "{" + strings.Join(parts, ",") + "}"
	}

	js := fmt.Sprintf(`
		(function(){
			// __besSafeDefine: try defineProperty, fall back to direct assignment
			// when the property is non-configurable (some v8go template-set
			// properties can't be redefined). This prevents the swap from
			// throwing on partial failures.
			function __besSafeDefine(obj, prop, val) {
				try {
					Object.defineProperty(obj, prop, {value: val, configurable: true, writable: true});
				} catch(e) {
					try { obj[prop] = val; } catch(e2) {}
				}
			}
			%s
			%s
			%s
			__besSafeDefine(navigator,'language',%q);
			__besSafeDefine(navigator,'languages',%s);
			__besSafeDefine(navigator,'userAgentData',%s);
			try { Object.defineProperty(navigator,'webdriver',{value:false,configurable:false}); } catch(e) {}
			// canvas toDataURL hash
			var _origCreateElement = document.createElement;
			document.createElement = function(tag) {
				var el = _origCreateElement.call(document, tag);
				if (tag === 'canvas' || (typeof tag === 'string' && tag.toLowerCase() === 'canvas')) {
					el.toDataURL = function() { return 'data:image/png;base64,' + %q; };
				}
				return el;
			};
			// WebGL params
			var _webglParams = %s;
			// location
			__besSafeDefine(location,'href',%q);
			__besSafeDefine(document,'URL',%q);
			__besSafeDefine(document,'documentURI',%q);
			// timezone override for the new fingerprint
			try {
				var _tz = %q;
				if (_tz) {
					var _OrigDTF = Intl.DateTimeFormat;
					Intl.DateTimeFormat = function(locale, options) {
						options = options || {};
						if (!options.timeZone) { options.timeZone = _tz; }
						return new _OrigDTF(locale, options);
					};
					Intl.DateTimeFormat.prototype = _OrigDTF.prototype;
				}
			} catch(e) {}
		})();
	`, strings.Join(navLines, "\n"), strings.Join(screenLines, "\n"), strings.Join(windowLines, "\n"),
		langFirst, langArr, uadJSON, canvasHash, webglParams, s.location, s.location, s.location, fp.Timezone)

	if _, err := s.ctx.RunScript(js, "fingerprint-swap.js"); err != nil {
		return fp, fmt.Errorf("fingerprint swap injection failed: %w", err)
	}
	log.Printf("[sandbox] session %s fingerprint swapped: %s @ %s", s.ID, fp.Browser.Name+"/"+fp.Browser.Version, fp.OS.Name)
	return fp, nil
}

// GetCookies returns the current cookie jar as a string.
func (s *Session) GetCookies() string {
	return s.cookieStore.String()
}

// Cookies returns the current cookie jar as a name→value map (for profile
// snapshots).
func (s *Session) Cookies() map[string]string {
	return s.cookieStore.GetAll()
}

// Location returns the session's document URL.
func (s *Session) Location() string {
	return s.location
}

// ProxyURL returns the session's proxy URL (empty when direct).
// The proxy is carried on the net handler options; it is stored here only for
// observability and profile snapshots.
func (s *Session) ProxyURL() string {
	return s.proxyURL
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
//
// Concurrency model (P0 fix for EvalWithTimeout/Dispose deadlock +
// use-after-dispose): the previous implementation held s.mu for the whole
// teardown, including s.ctx.Close(). Because EvalWithTimeout also held s.mu
// while blocked inside RunScript, Dispose could deadlock waiting for the lock —
// and if it did get the lock, an in-flight RunScript goroutine could still be
// touching s.ctx when Close freed it (use-after-dispose).
//
// The new model:
//  1. Take s.mu only long enough to flip s.disposed (idempotent), then release.
//     EvalWithTimeout no longer holds s.mu during RunScript, so this never
//     deadlocks; once disposed is set, new EvalWithTimeout/RunPrecompiled
//     calls refuse early.
//  2. TerminateExecution to break any in-flight pure-JS loop. This does not
//     take the V8 Locker and is safe to call while the execution goroutine
//     holds it.
//  3. Tear down workers/websockets/timers. These touch their own isolates /
//     network conns, not the parent context; StopAll also blocks queued
//     worker→parent callbacks from re-entering the parent V8.
//  4. Wait for in-flight EvalWithTimeout goroutines (s.execWG) to exit before
//     Close, bounded by disposeExecWaitTimeout so a native callback that
//     TerminateExecution cannot interrupt does not hang Dispose. On the normal
//     path (JS termination) the goroutine exits promptly and Close is safe —
//     no use-after-dispose.
//  5. Close the context and return the Isolate to the pool.
//
// Bug 1 fix: return Isolate to pool.
// Bug 14 fix: prevent concurrent V8 access during disposal.
func (s *Session) Dispose() {
	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		return
	}
	s.disposed = true
	s.mu.Unlock()

	// 1. Request any in-flight V8 execution to terminate. Safe to call from
	//    this goroutine: it does not acquire the V8 Locker.
	s.iso.TerminateExecution()

	// 2. Tear down sub-resources. These do not touch the parent context.
	if s.workers != nil {
		s.workers.disposeAll()
	}
	if s.websockets != nil {
		s.websockets.disposeAll()
	}
	s.timers.StopAll()

	// 3. Wait for in-flight EvalWithTimeout goroutines to exit so that Close
	//    cannot race a RunScript still using s.ctx (use-after-dispose). Bounded
	//    so a blocked native callback cannot hang Dispose.
	done := make(chan struct{})
	go func() {
		s.execWG.Wait()
		close(done)
	}()
	timedOut := false
	select {
	case <-done:
	case <-time.After(disposeExecWaitTimeout):
		// A goroutine is still inside RunScript (stuck in a native callback
		// that TerminateExecution cannot interrupt). Calling Close now would
		// free the context out from under it (use-after-free), so we skip both
		// Close and returning the Isolate to the pool: the session is already
		// unusable, and leaking one Isolate is preferable to crashing the
		// process. The goroutine will eventually exit and release the V8
		// Locker, by which point s.ctx is simply never touched again.
		timedOut = true
		log.Printf("[sandbox] session %s dispose: execution goroutine did not exit within %v, skipping context close to avoid use-after-dispose (possible blocked native callback)", s.ID, disposeExecWaitTimeout)
	}

	// 4. No in-flight execution remains on the normal path; safe to Close.
	if !timedOut {
		s.mu.Lock()
		s.ctx.Close()
		s.mu.Unlock()
	}

	// Bug 1 fix: return Isolate to pool for reuse (prevents leak). Skipped when
	// an execution goroutine is still using the Isolate (timedOut), since
	// returning it to the pool would let another session reuse it concurrently.
	if !timedOut && s.pool != nil {
		s.pool.Put(s.iso)
	}
	log.Printf("[sandbox] session %s disposed", s.ID)
}

// close is kept for backward compatibility; Dispose now handles pool return.
func (s *Session) close(pool *IsolatePool) {
	s.Dispose()
}

func generateSessionID() string {
	return fmt.Sprintf("sess-%d", time.Now().UnixNano())
}

// Dispose releases all Isolates in the pool.
func (e *Engine) Dispose() {
	e.pool.DisposeAll()
}
