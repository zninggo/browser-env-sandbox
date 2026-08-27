package sandbox

import (
	"fmt"
	"strings"
	"sync"
)

// CookieStore manages cookies for a sandbox session.
// Implements the document.cookie getter/setter semantics.
//
// Constraint #9: must support full read-write cycle.
// Target JS writes a cookie then immediately reads it — must work.
type CookieStore struct {
	mu      sync.RWMutex
	cookies map[string]*Cookie
}

// Cookie represents a single cookie.
type Cookie struct {
	Name     string
	Value    string
	Path     string
	Domain   string
	Expires  string
	Secure   bool
	HTTPOnly bool
	SameSite string
}

// NewCookieStore creates a cookie store with optional initial cookies.
func NewCookieStore(initial map[string]string) *CookieStore {
	cs := &CookieStore{
		cookies: make(map[string]*Cookie),
	}
	for name, value := range initial {
		cs.cookies[name] = &Cookie{Name: name, Value: value, Path: "/"}
	}
	return cs
}

// String returns the cookie string in "name1=value1; name2=value2" format.
// This is what document.cookie getter returns.
func (cs *CookieStore) String() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	parts := make([]string, 0, len(cs.cookies))
	for _, c := range cs.cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// Set sets a cookie by name and value.
func (cs *CookieStore) Set(name, value, path, domain string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if path == "" {
		path = "/"
	}
	cs.cookies[name] = &Cookie{Name: name, Value: value, Path: path, Domain: domain}
}

// SetRaw parses a raw cookie string (as written to document.cookie).
// e.g. "name=value; path=/; domain=.example.com; secure; samesite=lax"
func (cs *CookieStore) SetRaw(raw string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	parts := strings.Split(raw, ";")
	if len(parts) == 0 {
		return
	}

	// First part is name=value
	nameValue := strings.TrimSpace(parts[0])
	idx := strings.Index(nameValue, "=")
	if idx < 0 {
		return
	}
	name := strings.TrimSpace(nameValue[:idx])
	value := strings.TrimSpace(nameValue[idx+1:])

	cookie := &Cookie{Name: name, Value: value, Path: "/"}

	// Parse attributes
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		lower := strings.ToLower(part)
		switch {
		case strings.HasPrefix(lower, "path="):
			cookie.Path = part[5:]
		case strings.HasPrefix(lower, "domain="):
			cookie.Domain = part[7:]
		case strings.HasPrefix(lower, "expires="):
			cookie.Expires = part[8:]
		case lower == "secure":
			cookie.Secure = true
		case lower == "httponly":
			cookie.HTTPOnly = true
		case strings.HasPrefix(lower, "samesite="):
			cookie.SameSite = part[9:]
		}
	}

	cs.cookies[name] = cookie
}

// Get returns a cookie value by name.
func (cs *CookieStore) Get(name string) string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if c, ok := cs.cookies[name]; ok {
		return c.Value
	}
	return ""
}

// GetAll returns all cookies as a map.
func (cs *CookieStore) GetAll() map[string]string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	result := make(map[string]string, len(cs.cookies))
	for _, c := range cs.cookies {
		result[c.Name] = c.Value
	}
	return result
}

// Delete removes a cookie by name.
func (cs *CookieStore) Delete(name string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.cookies, name)
}

// Clear removes all cookies.
func (cs *CookieStore) Clear() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.cookies = make(map[string]*Cookie)
}

// ApplySetCookie applies Set-Cookie response headers from network responses.
func (cs *CookieStore) ApplySetCookie(setCookieHeaders []string) {
	for _, header := range setCookieHeaders {
		cs.SetRaw(header)
	}
}

// StringForHeader returns cookies as a Cookie header value.
func (cs *CookieStore) StringForHeader() string {
	return cs.String()
}

func (cs *CookieStore) Debug() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	parts := make([]string, 0, len(cs.cookies))
	for _, c := range cs.cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	return fmt.Sprintf("CookieStore{%d cookies: %s}", len(cs.cookies), strings.Join(parts, ", "))
}
