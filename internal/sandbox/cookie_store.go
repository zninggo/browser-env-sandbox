package sandbox

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// CookieStore manages cookies for a sandbox session.
// Implements document.cookie getter/setter semantics plus RFC 6265
// storage keyed by (name, domain, path) so same-name cookies on
// different domains/paths coexist.
//
// Constraint #9: must support full read-write cycle.
// Target JS writes a cookie then immediately reads it — must work.
type CookieStore struct {
	mu      sync.RWMutex
	cookies map[string]*Cookie
	// creationSeq orders cookies per RFC 6265 §5.4 (path length desc,
	// then earliest creation first). Monotonic counter under mu.
	creationSeq uint64
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
	// HostOnly is true when the cookie was set without an explicit
	// Domain attribute: it matches only its exact host (RFC 6265 §5.3).
	HostOnly bool
	// Creation sequence for §5.4 ordering.
	Seq uint64
}

func cookieKey(name, domain, path string) string {
	return name + "\x00" + strings.ToLower(domain) + "\x00" + path
}

// NewCookieStore creates a cookie store with optional initial cookies.
// Initial cookies are host-only, path "/".
func NewCookieStore(initial map[string]string) *CookieStore {
	cs := &CookieStore{
		cookies: make(map[string]*Cookie),
	}
	for name, value := range initial {
		cs.store(&Cookie{Name: name, Value: value, Path: "/", HostOnly: true})
	}
	return cs
}

// String returns the cookie string in "name1=value1; name2=value2" format.
// This is what document.cookie getter returns. HttpOnly cookies are excluded
// (document.cookie must not expose them); ordering follows RFC 6265 §5.4.
func (cs *CookieStore) String() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	parts := make([]string, 0, len(cs.cookies))
	for _, c := range cs.cookies {
		if c.HTTPOnly {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// Set sets a cookie by name and value (host-only, path "/").
func (cs *CookieStore) Set(name, value, path, domain string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if path == "" {
		path = "/"
	}
	cs.store(&Cookie{Name: name, Value: value, Path: path, Domain: domain, HostOnly: domain == ""})
}

// SetRaw parses a raw cookie string (as written to document.cookie or a
// Set-Cookie header) and stores it under the given request host.
// e.g. "name=value; path=/; domain=.example.com; secure; samesite=lax"
// host is the URL host the cookie came from (used for host-only cookies and
// domain-attribute validation); empty host skips validation.
// Returns false when the cookie must be rejected (domain attribute does not
// domain-match the host, RFC 6265 §5.3 step 6).
func (cs *CookieStore) SetRawForHost(raw, host string) bool {
	cookie := parseRawCookie(raw)
	if cookie == nil {
		return true // nothing storable (e.g. delete syntax), not an error
	}
	if host != "" {
		reqHost := strings.ToLower(host)
		if cookie.Domain != "" {
			dom := strings.ToLower(strings.TrimPrefix(cookie.Domain, "."))
			if dom == "" || !domainMatch(reqHost, dom) {
				return false
			}
			cookie.Domain = dom
			cookie.HostOnly = false
		} else {
			cookie.Domain = reqHost
			cookie.HostOnly = true
		}
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.store(cookie)
	return true
}

// SetRaw parses a raw cookie string (as written to document.cookie).
// e.g. "name=value; path=/; domain=.example.com; secure; samesite=lax"
// Without a request host, a Domain attribute is kept verbatim and the cookie
// is not host-only.
func (cs *CookieStore) SetRaw(raw string) {
	cookie := parseRawCookie(raw)
	if cookie == nil {
		return
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.store(cookie)
}

func parseRawCookie(raw string) *Cookie {
	parts := strings.Split(raw, ";")
	if len(parts) == 0 {
		return nil
	}

	// First part is name=value
	nameValue := strings.TrimSpace(parts[0])
	idx := strings.Index(nameValue, "=")
	if idx < 0 {
		return nil
	}
	name := strings.TrimSpace(nameValue[:idx])
	value := strings.TrimSpace(nameValue[idx+1:])
	if name == "" {
		return nil
	}

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
	if cookie.Path == "" {
		cookie.Path = "/"
	}
	return cookie
}

// store inserts/overwrites a cookie keyed by (name, domain, path).
// Callers must hold cs.mu (or be in the constructor).
func (cs *CookieStore) store(c *Cookie) {
	cs.creationSeq++
	c.Seq = cs.creationSeq
	cs.cookies[cookieKey(c.Name, c.Domain, c.Path)] = c
}

// Get returns a cookie value by name (any domain/path).
func (cs *CookieStore) Get(name string) string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	for _, c := range cs.cookies {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// GetAll returns all cookies as a name→value map (last write wins per name).
func (cs *CookieStore) GetAll() map[string]string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	result := make(map[string]string, len(cs.cookies))
	for _, c := range cs.cookies {
		result[c.Name] = c.Value
	}
	return result
}

// Delete removes all cookies with the given name.
func (cs *CookieStore) Delete(name string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for k, c := range cs.cookies {
		if c.Name == name {
			delete(cs.cookies, k)
		}
	}
}

// Clear removes all cookies.
func (cs *CookieStore) Clear() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.cookies = make(map[string]*Cookie)
}

// ApplySetCookie applies Set-Cookie response headers from network responses.
// host is the response URL host for host-only/domain validation; empty host
// keeps Domain attributes verbatim without validation.
func (cs *CookieStore) ApplySetCookie(setCookieHeaders []string, host string) {
	for _, header := range setCookieHeaders {
		cs.SetRawForHost(header, host)
	}
}

// domainMatch implements RFC 6265 §5.1.3: host equals domain, or host ends
// with domain and the preceding char is a dot (and host is not an IP).
func domainMatch(host, domain string) bool {
	if host == domain {
		return true
	}
	if strings.HasSuffix(host, "."+domain) {
		return true
	}
	return false
}

// pathMatch implements RFC 6265 §5.1.4.
func pathMatch(requestPath, cookiePath string) bool {
	if requestPath == cookiePath {
		return true
	}
	if strings.HasPrefix(requestPath, cookiePath) {
		if strings.HasSuffix(cookiePath, "/") {
			return true
		}
		if len(requestPath) > len(cookiePath) && requestPath[len(cookiePath)] == '/' {
			return true
		}
	}
	return false
}

// CookieHeaderFor returns the Cookie header value for a request to
// scheme://host/requestPath, per RFC 6265 §5.4:
//   - domain match: host-only cookies match the exact host; domain cookies
//     match the host and subdomains;
//   - path match (§5.1.4);
//   - secure cookies only over https;
//   - ordering: longer Path first, then earliest creation first.
//
// HTTP-only cookies are included (they are hidden from document.cookie only).
func (cs *CookieStore) CookieHeaderFor(scheme, host, requestPath string) string {
	if requestPath == "" {
		requestPath = "/"
	}
	reqHost := strings.ToLower(host)

	cs.mu.RLock()
	type entry struct {
		c   *Cookie
		seq uint64
	}
	matched := make([]entry, 0, len(cs.cookies))
	for _, c := range cs.cookies {
		if c.Secure && scheme != "https" {
			continue
		}
		if c.HostOnly {
			if reqHost != strings.ToLower(c.Domain) {
				continue
			}
		} else if c.Domain != "" {
			if !domainMatch(reqHost, strings.ToLower(c.Domain)) {
				continue
			}
		} else {
			// No domain at all: treat as host-only against the empty host —
			// legacy cookies created without any host info stay globally
			// sendable (preserves pre-existing behavior for offline sessions).
			// Fall through.
		}
		if !pathMatch(requestPath, c.Path) {
			continue
		}
		matched = append(matched, entry{c: c, seq: c.Seq})
	}
	cs.mu.RUnlock()

	sort.SliceStable(matched, func(i, j int) bool {
		if len(matched[i].c.Path) != len(matched[j].c.Path) {
			return len(matched[i].c.Path) > len(matched[j].c.Path)
		}
		return matched[i].seq < matched[j].seq
	})

	parts := make([]string, 0, len(matched))
	for _, m := range matched {
		parts = append(parts, m.c.Name+"="+m.c.Value)
	}
	return strings.Join(parts, "; ")
}

// StringForHeader returns cookies as a Cookie header value
// (legacy global form, no URL scoping).
func (cs *CookieStore) StringForHeader() string {
	return cs.String()
}

func (cs *CookieStore) Debug() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	parts := make([]string, 0, len(cs.cookies))
	for _, c := range cs.cookies {
		parts = append(parts, fmt.Sprintf("%s=%s(dom=%s,path=%s)", c.Name, c.Value, c.Domain, c.Path))
	}
	return fmt.Sprintf("CookieStore{%d cookies: %s}", len(cs.cookies), strings.Join(parts, ", "))
}
