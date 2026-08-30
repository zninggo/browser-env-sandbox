// Package bridge is the JSON-over-HTTP service layer for browser-env-sandbox.
//
// It replaces the originally-planned gRPC bridge with a lightweight JSON-RPC
// over HTTP API (no protoc/codegen required). service.go holds the business
// logic that wraps sandbox.Engine and manages session lifecycles plus the
// per-session event broadcasters used for SSE streaming.
package bridge

import (
	"fmt"
	"sync"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/sandbox"
	"github.com/zninggo/browser-env-sandbox/internal/session"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// ConsoleMessage is a single captured console.* call, ready to stream to a
// client over SSE. Mirrors the ConsoleMessage proto message.
type ConsoleMessage struct {
	Level     string `json:"level"` // "log", "debug", "info", "warn", "error"
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"` // unix milliseconds
}

// NetworkEvent is a single captured network request (fetch/XHR), ready to
// stream over SSE. Mirrors the NetworkEvent proto message.
//
// NOTE: network capture is not yet wired (fetch/XHR are JS stubs pending the
// Phase 4 netlayer). The broadcaster below is in place and the SSE endpoint
// works, but no events are emitted until the netlayer feeds
// Service.PublishNetworkEvent.
type NetworkEvent struct {
	Method       string `json:"method"`
	URL          string `json:"url"`
	Status       int    `json:"status"`
	RequestBody  string `json:"request_body,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
	Timestamp    int64  `json:"timestamp"`
}

// consoleBroadcaster implements sandbox.ConsoleSink and fans captured
// console.* calls out to SSE subscribers. One instance per session.
type consoleBroadcaster struct {
	mu   sync.RWMutex
	subs map[chan ConsoleMessage]struct{}
}

func newConsoleBroadcaster() *consoleBroadcaster {
	return &consoleBroadcaster{subs: make(map[chan ConsoleMessage]struct{})}
}

// Write implements sandbox.ConsoleSink. It is called from V8 function-callback
// threads during script execution, so it MUST be non-blocking: a full
// subscriber buffer drops the message rather than stalling the isolate.
func (b *consoleBroadcaster) Write(level, message string) {
	msg := ConsoleMessage{
		Level:     level,
		Message:   message,
		Timestamp: time.Now().UnixMilli(),
	}
	b.mu.RLock()
	for ch := range b.subs {
		select {
		case ch <- msg:
		default:
			// subscriber can't keep up — drop to protect the V8 thread
		}
	}
	b.mu.RUnlock()
}

func (b *consoleBroadcaster) subscribe() (<-chan ConsoleMessage, func()) {
	ch := make(chan ConsoleMessage, 256)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, unsub
}

// networkBroadcaster fans captured network events out to SSE subscribers.
// One instance per session. Not yet fed by the sandbox (see NetworkEvent note).
type networkBroadcaster struct {
	mu   sync.RWMutex
	subs map[chan NetworkEvent]struct{}
}

func newNetworkBroadcaster() *networkBroadcaster {
	return &networkBroadcaster{subs: make(map[chan NetworkEvent]struct{})}
}

// emit pushes a network event to all subscribers (non-blocking).
func (b *networkBroadcaster) emit(evt NetworkEvent) {
	b.mu.RLock()
	for ch := range b.subs {
		select {
		case ch <- evt:
		default:
		}
	}
	b.mu.RUnlock()
}

func (b *networkBroadcaster) subscribe() (<-chan NetworkEvent, func()) {
	ch := make(chan NetworkEvent, 256)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, unsub
}

// SessionSummary is a lightweight session listing entry.
type SessionSummary struct {
	SessionID string `json:"session_id"`
	Browser   string `json:"browser"`
	UA        string `json:"user_agent"`
}

type sessionEntry struct {
	sess       *sandbox.Session
	console    *consoleBroadcaster
	network    *networkBroadcaster
	lastActive time.Time
	refCount   int  // in-flight operations holding this entry; CloseSession/cleanupIdle mark closing instead of tearing down under a live ref
	closing    bool // set when CloseSession/cleanupIdle has claimed the entry; acquire refuses new ops on a closing entry
}

// Service is the business-logic layer over sandbox.Engine. It owns the session
// registry, the per-session event broadcasters, the profile store, and idle
// session cleanup. The HTTP handlers in server.go are thin wrappers around
// these methods.
type Service struct {
	engine   *sandbox.Engine
	mu       sync.RWMutex
	sessions map[string]*sessionEntry
	profiles *session.ProfileStore

	idleTimeout time.Duration
	done        chan struct{}
}

// NewService wires a console-sink factory into the engine so every session
// routes its console.* output to an independent broadcaster, then returns a
// Service ready to serve. profileDir roots the profile store (empty =
// data/profiles).
func NewService(engine *sandbox.Engine, profileDir string) *Service {
	engine.SetConsoleSinkFactory(func() sandbox.ConsoleSink {
		return newConsoleBroadcaster()
	})
	s := &Service{
		engine:      engine,
		sessions:    make(map[string]*sessionEntry),
		profiles:    session.NewProfileStore(profileDir),
		idleTimeout: 30 * time.Minute,
		done:        make(chan struct{}),
	}
	go s.cleanupIdle()
	return s
}

// cleanupIdle periodically removes sessions idle beyond the timeout, so
// long-running servers don't leak isolates on abandoned sessions.
func (s *Service) cleanupIdle() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			var toDispose []*sandbox.Session
			for id, e := range s.sessions {
				if now.Sub(e.lastActive) > s.idleTimeout {
					e.closing = true
					delete(s.sessions, id)
					toDispose = append(toDispose, e.sess)
				}
			}
			s.mu.Unlock()
			// Dispose outside the lock: Dispose can block on execWG (in-flight
			// EvalAwait goroutines that acquired the entry before closing was
			// set), and Dispose itself is safe to run concurrently with a
			// release() that only touches refCount.
			for _, sess := range toDispose {
				sess.Dispose()
			}
		}
	}
}

// CreateSession creates a new sandbox session and registers it.
func (s *Service) CreateSession(opts api.SessionOptions) (string, *api.Fingerprint, error) {
	sess, err := s.engine.CreateSession(opts)
	if err != nil {
		return "", nil, err
	}
	// The factory above created the per-session broadcaster; pull it back off
	// the session so we (and the SSE handlers) can subscribe to it.
	cb, _ := sess.ConsoleSink().(*consoleBroadcaster)
	if cb == nil {
		// Defensive: factory wasn't set — create an idle broadcaster so SSE
		// still connects even though it will receive nothing.
		cb = newConsoleBroadcaster()
	}
	nb := newNetworkBroadcaster()

	s.mu.Lock()
	s.sessions[sess.ID] = &sessionEntry{sess: sess, console: cb, network: nb, lastActive: time.Now()}
	s.mu.Unlock()
	return sess.ID, sess.GetFingerprint(), nil
}

// touch updates a session's last-active timestamp (called on every op).
func (s *Service) touch(id string) {
	s.mu.Lock()
	if e, ok := s.sessions[id]; ok {
		e.lastActive = time.Now()
	}
	s.mu.Unlock()
}

// acquire pins a session entry for the duration of an operation: it bumps the
// refCount under s.mu so a concurrent CloseSession/cleanupIdle sees a live ref
// and defers the teardown semantics (it still marks closing + removes the map
// entry, but the session itself stays usable for the in-flight op via the
// already-returned pointer). The returned release func drops the ref. A nil
// entry means the session is gone or already closing — callers must refuse.
//
// This closes the TOCTOU window that getEntry had: previously it returned the
// entry pointer under a read lock, released the lock, and the caller then used
// e.sess.XXX() while CloseSession could concurrently Dispose+delete the entry.
// The sandbox layer's disposed/execWG guard already prevents a crash there, but
// acquire makes the bridge layer's lifecycle explicit and race-testable.
func (s *Service) acquire(id string) (*sessionEntry, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.sessions[id]
	if !ok || e.closing {
		return nil, nil
	}
	e.refCount++
	e.lastActive = time.Now()
	return e, func() {
		s.mu.Lock()
		e.refCount--
		s.mu.Unlock()
	}
}

// getEntry returns a peek at an entry without pinning it. Used only for paths
// that read non-V8 state (broadcaster subscription) where the entry pointer
// itself staying valid is enough; those still pair with acquire where a V8 op
// follows. Kept for Subscribe*/Publish which operate on the broadcaster, not
// the V8 context.
func (s *Service) getEntry(id string) (*sessionEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.sessions[id]
	return e, ok
}

// ListSessions returns a summary of all live sessions.
func (s *Service) ListSessions() []SessionSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SessionSummary, 0, len(s.sessions))
	for _, e := range s.sessions {
		fp := e.sess.GetFingerprint()
		ua, _ := fp.Navigator["userAgent"].(string)
		out = append(out, SessionSummary{
			SessionID: e.sess.ID,
			Browser:   fp.Browser.Name + " " + fp.Browser.Version,
			UA:        ua,
		})
	}
	return out
}

// Eval executes JavaScript in a session and returns the stringified result.
// Promise results are awaited (timers + microtasks drained) before returning.
func (s *Service) Eval(id, code string) (string, error) {
	e, release := s.acquire(id)
	if e == nil {
		return "", fmt.Errorf("session not found: %s", id)
	}
	defer release()
	return e.sess.EvalAwait(code, 30*time.Second)
}

// LoadScript loads and executes a named script in a session.
func (s *Service) LoadScript(id, name, content string) error {
	e, release := s.acquire(id)
	if e == nil {
		return fmt.Errorf("session not found: %s", id)
	}
	defer release()
	return e.sess.LoadScript(name, content)
}

// CallFunction calls a global function by name with string arguments.
func (s *Service) CallFunction(id, fn string, args []string) (string, error) {
	e, release := s.acquire(id)
	if e == nil {
		return "", fmt.Errorf("session not found: %s", id)
	}
	defer release()
	return e.sess.CallFunction(fn, args...)
}

// GetFingerprint returns a session's full fingerprint.
func (s *Service) GetFingerprint(id string) (*api.Fingerprint, error) {
	e, release := s.acquire(id)
	if e == nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	defer release()
	return e.sess.GetFingerprint(), nil
}

// GetCookies returns a session's cookie jar as "name1=value1; name2=value2".
func (s *Service) GetCookies(id string) (string, error) {
	e, release := s.acquire(id)
	if e == nil {
		return "", fmt.Errorf("session not found: %s", id)
	}
	defer release()
	return e.sess.GetCookies(), nil
}

// SetCookie sets a cookie in a session.
func (s *Service) SetCookie(id, name, value string) error {
	e, release := s.acquire(id)
	if e == nil {
		return fmt.Errorf("session not found: %s", id)
	}
	defer release()
	e.sess.SetCookie(name, value)
	return nil
}

// SwapFingerprint hot-swaps a session's fingerprint (snapshot/fingerprint
// hot-swap). Returns the new fingerprint.
func (s *Service) SwapFingerprint(id string, opts api.SessionOptions) (*api.Fingerprint, error) {
	e, release := s.acquire(id)
	if e == nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	defer release()
	return e.sess.SwapFingerprint(s.engine, opts)
}

// CloseSession disposes a session and removes it from the registry.
//
// It marks the entry closing and removes it from the map under s.mu, then
// disposes the session outside the lock. In-flight ops that acquired the entry
// before closing was set still hold a live ref and a valid session pointer;
// the sandbox layer's disposed/execWG guard handles their teardown. New ops
// arriving after closing see the entry gone (or closing) and refuse.
func (s *Service) CloseSession(id string) error {
	s.mu.Lock()
	e, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("session not found: %s", id)
	}
	e.closing = true
	delete(s.sessions, id)
	s.mu.Unlock()
	e.sess.Dispose()
	return nil
}

// SubscribeConsole returns a read-only channel of console messages for the
// session plus an unsubscribe function that MUST be called when the consumer
// stops (e.g. on SSE client disconnect) to release the subscription.
func (s *Service) SubscribeConsole(id string) (<-chan ConsoleMessage, func(), error) {
	e, ok := s.getEntry(id)
	if !ok {
		return nil, nil, fmt.Errorf("session not found: %s", id)
	}
	ch, unsub := e.console.subscribe()
	return ch, unsub, nil
}

// SubscribeNetwork returns a read-only channel of network events for the
// session plus an unsubscribe function. See the NetworkEvent note: no events
// are emitted until the netlayer is wired in Phase 4.
func (s *Service) SubscribeNetwork(id string) (<-chan NetworkEvent, func(), error) {
	e, ok := s.getEntry(id)
	if !ok {
		return nil, nil, fmt.Errorf("session not found: %s", id)
	}
	ch, unsub := e.network.subscribe()
	return ch, unsub, nil
}

// PublishNetworkEvent injects a network event into a session's stream. This is
// the hook the Phase 4 netlayer will call when fetch/XHR are wired to Go.
func (s *Service) PublishNetworkEvent(id string, evt NetworkEvent) {
	e, ok := s.getEntry(id)
	if !ok {
		return
	}
	if evt.Timestamp == 0 {
		evt.Timestamp = time.Now().UnixMilli()
	}
	e.network.emit(evt)
}

// ---------- Profile persistence ----------

// SaveProfile snapshots a live session's identity (fingerprint + cookies +
// proxy + location) into the profile store. name overrides the profile name;
// empty defaults to the session ID.
func (s *Service) SaveProfile(id, name string) (*session.Profile, error) {
	e, release := s.acquire(id)
	if e == nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	defer release()
	if name == "" {
		name = id
	}
	proxy := e.sess.ProxyURL()
	fp := e.sess.GetFingerprint()
	p := session.NewFromFingerprint(fp, e.sess.Cookies(), proxy, e.sess.Location(), name)
	p.ID = "profile-" + id // stable per-session snapshot ID (overwrite on re-save)
	if err := s.profiles.Save(p); err != nil {
		return nil, fmt.Errorf("save profile failed: %w", err)
	}
	return p, nil
}

// ListProfiles returns all saved profiles.
func (s *Service) ListProfiles() ([]session.Profile, error) {
	return s.profiles.List()
}

// GetProfile reads one profile from the store.
func (s *Service) GetProfile(id string) (*session.Profile, error) {
	return s.profiles.Load(id)
}

// DeleteProfile removes a profile from the store.
func (s *Service) DeleteProfile(id string) error {
	return s.profiles.Delete(id)
}

// ResumeProfile creates a new session from a stored profile: the profile's
// fingerprint (via its seed), cookies, proxy, and location are all restored.
// Returns the new session ID.
func (s *Service) ResumeProfile(profileID string) (string, *api.Fingerprint, error) {
	p, err := s.profiles.Load(profileID)
	if err != nil {
		return "", nil, fmt.Errorf("load profile failed: %w", err)
	}
	opts := api.SessionOptions{
		Seed:     p.Fingerprint.Seed,
		Location: p.Location,
		Cookies:  p.Cookies,
		Proxy:    p.Proxy,
	}
	sessID, fp, err := s.CreateSession(opts)
	if err != nil {
		return "", nil, fmt.Errorf("resume session failed: %w", err)
	}
	return sessID, fp, nil
}

// Dispose closes all sessions and releases the engine's isolate pool. Intended
// for graceful shutdown.
func (s *Service) Dispose() {
	close(s.done)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, e := range s.sessions {
		e.sess.Dispose()
		delete(s.sessions, id)
	}
	s.engine.Dispose()
}
