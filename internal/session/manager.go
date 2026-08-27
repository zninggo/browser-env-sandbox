// Package session manages sandbox sessions with session-unique isolation.
// Each session gets its own fingerprint, cookie jar, proxy, and TLS profile.
package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/zninggo/bes/internal/fpengine"
	"github.com/zninggo/bes/internal/netlayer"
	"github.com/zninggo/bes/internal/sandbox"
	"github.com/zninggo/bes/pkg/api"
)

// Manager manages multiple isolated sandbox sessions.
type Manager struct {
	mu         sync.RWMutex
	sessions   map[string]*ManagedSession
	fpEngine   *fpengine.Engine
	sbEngine   *sandbox.Engine
	proxyPool  *netlayer.ProxyPool
	idleTimeout time.Duration
}

// ManagedSession wraps a sandbox.Session with session-unique resources.
type ManagedSession struct {
	ID         string
	Sandbox    *sandbox.Session
	Fingerprint *api.Fingerprint
	Cookies    *sandbox.CookieStore
	Proxy      *netlayer.ProxyConfig
	TLSProfile *netlayer.TLSProfile
	CreatedAt  time.Time
	LastActive time.Time
	mu         sync.Mutex
}

// New creates a session manager.
func New(fpEng *fpengine.Engine, sbEng *sandbox.Engine, proxyPool *netlayer.ProxyPool) *Manager {
	m := &Manager{
		sessions:    make(map[string]*ManagedSession),
		fpEngine:    fpEng,
		sbEngine:    sbEng,
		proxyPool:   proxyPool,
		idleTimeout: 30 * time.Minute,
	}
	// Start idle cleanup goroutine
	go m.cleanupIdle()
	return m
}

// Create creates a new isolated session.
func (m *Manager) Create(opts api.SessionOptions) (*ManagedSession, error) {
	// 1. Generate fingerprint (session-unique)
	fp, err := m.fpEngine.Generate(opts.Seed, opts.Browser, opts.OS)
	if err != nil {
		return nil, fmt.Errorf("fingerprint generation failed: %w", err)
	}

	// 2. Create sandbox session
	sbSess, err := m.sbEngine.CreateSession(opts)
	if err != nil {
		return nil, fmt.Errorf("sandbox creation failed: %w", err)
	}

	// 3. Assign proxy (session-unique)
	var proxy *netlayer.ProxyConfig
	if opts.Proxy != "" {
		proxy = &netlayer.ProxyConfig{URL: opts.Proxy}
	} else if m.proxyPool != nil && m.proxyPool.Size() > 0 {
		proxy = m.proxyPool.Get()
	}

	// 4. Get TLS profile matching the fingerprint's browser version
	tlsProfile := netlayer.GetTLSProfile(fp.Browser.Name, fp.Browser.Version)

	// 5. Create managed session
	ms := &ManagedSession{
		ID:          sbSess.ID,
		Sandbox:     sbSess,
		Fingerprint: fp,
		Cookies:     sandbox.NewCookieStore(opts.Cookies),
		Proxy:       proxy,
		TLSProfile:  tlsProfile,
		CreatedAt:   time.Now(),
		LastActive:  time.Now(),
	}

	m.mu.Lock()
	m.sessions[ms.ID] = ms
	m.mu.Unlock()

	return ms, nil
}

// Get retrieves a session by ID.
func (m *Manager) Get(id string) (*ManagedSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ms, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	ms.LastActive = time.Now()
	return ms, nil
}

// List returns all active sessions.
func (m *Manager) List() []*ManagedSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ManagedSession, 0, len(m.sessions))
	for _, ms := range m.sessions {
		result = append(result, ms)
	}
	return result
}

// Close closes and removes a session.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ms, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	ms.Sandbox.Dispose()
	delete(m.sessions, id)
	return nil
}

// CloseAll closes all sessions.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, ms := range m.sessions {
		ms.Sandbox.Dispose()
		delete(m.sessions, id)
	}
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Eval executes JS in a session (thread-safe).
func (ms *ManagedSession) Eval(code string) (string, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.LastActive = time.Now()
	return ms.Sandbox.Eval(code)
}

// cleanupIdle periodically removes idle sessions.
func (m *Manager) cleanupIdle() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for id, ms := range m.sessions {
			if now.Sub(ms.LastActive) > m.idleTimeout {
				ms.Sandbox.Dispose()
				delete(m.sessions, id)
			}
		}
		m.mu.Unlock()
	}
}
