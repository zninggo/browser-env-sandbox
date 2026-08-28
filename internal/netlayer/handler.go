// Package netlayer handles network requests from within the sandbox.
//
// Two modes:
// - Replay: offline, responses come from a pre-recorded session
// - Live: real requests via HTTP client with TLS fingerprint matching
package netlayer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/zninggo/bes/internal/sandbox"
)

// Mode determines how network requests are handled.
type Mode string

const (
	ModeReplay Mode = "replay"
	ModeLive   Mode = "live"
)

// Handler processes XHR/fetch requests from the sandbox.
type Handler struct {
	mode   Mode
	replay *ReplayStore
	// cookieJar stores Set-Cookie responses keyed by (name, domain, path)
	// with RFC 6265 semantics; scoped sending happens per-request.
	cookieJar  *sandbox.CookieStore
	proxy      string
	tlsClient  *TLSClient
	mu         sync.Mutex
	recordings []RecordedRequest
	recording  bool
}

// RecordedRequest is a captured request-response pair for replay.
type RecordedRequest struct {
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body"`
	Response RecordedResponse  `json:"response"`
}

// RecordedResponse is the response part of a recording.
type RecordedResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// New creates a network handler with the given mode.
// tlsTarget is the curl-impersonate/curl_cffi target for live mode TLS
// fingerprint matching (e.g. "chrome150"). Empty string defaults to "chrome150".
func New(mode Mode, replayFile, proxy, tlsTarget string) (*Handler, error) {
	h := &Handler{
		mode:      mode,
		cookieJar: sandbox.NewCookieStore(nil),
		proxy:     proxy,
		recording: true,
	}

	if mode == ModeReplay && replayFile != "" {
		store, err := LoadReplay(replayFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load replay: %w", err)
		}
		h.replay = store
	}

	if mode == ModeLive {
		if tlsTarget == "" {
			tlsTarget = "chrome150"
		}
		h.tlsClient = NewTLSClient(tlsTarget)
		if proxy != "" {
			h.tlsClient.SetProxy(proxy)
		}
	}

	return h, nil
}

// Request processes an HTTP request.
// In replay mode: looks up a matching recording.
// In live mode: sends a real HTTP request.
func (h *Handler) Request(method, urlStr string, headers map[string]string, body []byte) (*Response, error) {
	if h.recording {
		h.mu.Lock()
		h.recordings = append(h.recordings, RecordedRequest{
			Method: method, URL: urlStr, Headers: headers, Body: string(body),
		})
		h.mu.Unlock()
	}

	switch h.mode {
	case ModeReplay:
		return h.handleReplay(method, urlStr)
	case ModeLive:
		return h.handleLive(method, urlStr, headers, body)
	default:
		return nil, fmt.Errorf("unknown network mode: %s", h.mode)
	}
}

// Response is a network response returned to the sandbox.
type Response struct {
	Status  int
	Headers map[string]string
	Body    string
	// BodyB64 is the raw response body base64-encoded (std encoding). The
	// Body string field cannot carry arbitrary bytes losslessly: a Go string
	// holding non-UTF-8 data gets mangled on the way through JSON
	// serialization into V8 (invalid bytes → U+FFFD), so binary payloads
	// (images, fonts, WASM, ...) must travel via BodyB64.
	// Consumers MUST prefer BodyB64 when non-empty and fall back to Body
	// only for legacy/replay data.
	BodyB64    string
	Cookies    map[string]string
	// SetCookies holds raw Set-Cookie header values (one per line).
	SetCookies []string
}

// b64Encode is the single helper every backend uses to fill BodyB64.
func b64Encode(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func (h *Handler) handleReplay(method, urlStr string) (*Response, error) {
	if h.replay == nil {
		return nil, fmt.Errorf("no replay store loaded")
	}
	rec := h.replay.Match(method, urlStr)
	if rec == nil {
		return nil, fmt.Errorf("no replay match for %s %s", method, urlStr)
	}
	return &Response{
		Status:  rec.Response.Status,
		Headers: rec.Response.Headers,
		Body:    rec.Response.Body,
	}, nil
}

func (h *Handler) handleLive(method, urlStr string, headers map[string]string, body []byte) (*Response, error) {
	// Inject cookies scoped to this request URL (RFC 6265 §5.4). A manually
	// provided Cookie header wins over the jar.
	if u, err := url.Parse(urlStr); err == nil && u.Hostname() != "" {
		reqPath := u.Path
		if reqPath == "" {
			reqPath = "/"
		}
		if !hasHeader(headers, "Cookie") {
			if cookieStr := h.cookieJar.CookieHeaderFor(strings.ToLower(u.Scheme), u.Hostname(), reqPath); cookieStr != "" {
				if headers == nil {
					headers = make(map[string]string)
				}
				headers["Cookie"] = cookieStr
			}
		}
	}

	// Use TLSClient (curl-impersonate → curl_cffi → standard HTTP fallback)
	// for TLS fingerprint matching with the UA version.
	if h.tlsClient != nil {
		resp, err := h.tlsClient.Request(method, urlStr, headers, body)
		if err != nil {
			return nil, fmt.Errorf("TLS client request failed: %w", err)
		}

		// Update cookie jar from raw Set-Cookie lines, scoped to the
		// response host.
		if len(resp.SetCookies) > 0 {
			if u, err := url.Parse(urlStr); err == nil {
				for _, sc := range resp.SetCookies {
					h.cookieJar.SetRawForHost(sc, u.Hostname())
				}
			}
		}

		return resp, nil
	}

	// Fallback: standard Go HTTP client (no TLS fingerprint spoofing)
	return h.handleLiveFallback(method, urlStr, headers, body)
}

func (h *Handler) handleLiveFallback(method, urlStr string, headers map[string]string, body []byte) (*Response, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Inject cookies scoped to this request URL. A manually provided Cookie
	// header wins over the jar.
	if u, perr := url.Parse(urlStr); perr == nil && u.Hostname() != "" {
		reqPath := u.Path
		if reqPath == "" {
			reqPath = "/"
		}
		if !hasHeader(headers, "Cookie") {
			if cookieStr := h.cookieJar.CookieHeaderFor(strings.ToLower(u.Scheme), u.Hostname(), reqPath); cookieStr != "" {
				req.Header.Set("Cookie", cookieStr)
			}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	respHeaders := make(map[string]string)
	cookies := make(map[string]string)
	var setCookies []string
	for _, sc := range resp.Header["Set-Cookie"] {
		setCookies = append(setCookies, sc)
		nv := parseSetCookieNameValue(sc)
		if nv[0] != "" {
			cookies[nv[0]] = nv[1]
		}
		h.mu.Lock()
		h.cookieJar.SetRawForHost(sc, req.URL.Hostname())
		h.mu.Unlock()
	}

	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	return &Response{
		Status:     resp.StatusCode,
		Headers:    respHeaders,
		Body:       string(respBody),
		BodyB64:    b64Encode(respBody),
		Cookies:    cookies,
		SetCookies: setCookies,
	}, nil
}

// GetCookies returns the current cookie jar (name→value; last write wins per
// name when the same name exists on multiple domains).
func (h *Handler) GetCookies() map[string]string {
	return h.cookieJar.GetAll()
}

// SetCookie sets a cookie in the jar (host-less, path "/").
func (h *Handler) SetCookie(name, value string) {
	h.cookieJar.Set(name, value, "/", "")
}

// hasHeader does a case-insensitive lookup in a header map.
func hasHeader(headers map[string]string, name string) bool {
	for k := range headers {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

// parseSetCookieNameValue extracts "name=value" from a raw Set-Cookie value.
// Returns nil when the pair cannot be parsed.
func parseSetCookieNameValue(sc string) [2]string {
	parts := strings.SplitN(sc, ";", 2)
	if len(parts) == 0 {
		return [2]string{}
	}
	eqIdx := strings.Index(parts[0], "=")
	if eqIdx <= 0 {
		return [2]string{}
	}
	return [2]string{
		strings.TrimSpace(parts[0][:eqIdx]),
		strings.TrimSpace(parts[0][eqIdx+1:]),
	}
}

// TLSBackend returns which TLS backend is in use (for logging).
func (h *Handler) TLSBackend() string {
	if h.tlsClient != nil {
		return h.tlsClient.Backend()
	}
	return "none"
}

// SaveRecording writes all recorded requests to a file.
func (h *Handler) SaveRecording(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	data, err := json.MarshalIndent(h.recordings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// --- Replay Store ---

// ReplayStore holds pre-recorded request-response pairs.
type ReplayStore struct {
	Records []RecordedRequest `json:"records"`
}

// LoadReplay loads a replay file.
func LoadReplay(path string) (*ReplayStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var store ReplayStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// Match finds a recording matching the method and URL.
// Matching is fuzzy: URL query parameters may differ slightly.
func (s *ReplayStore) Match(method, urlStr string) *RecordedRequest {
	for i, rec := range s.Records {
		if rec.Method == method && matchURL(rec.URL, urlStr) {
			return &s.Records[i]
		}
	}
	return nil
}

// matchURL does a fuzzy URL match (ignores query param order).
func matchURL(a, b string) bool {
	// Strip query strings for matching
	aBase := a
	bBase := b
	if idx := strings.Index(a, "?"); idx >= 0 {
		aBase = a[:idx]
	}
	if idx := strings.Index(b, "?"); idx >= 0 {
		bBase = b[:idx]
	}
	return aBase == bBase
}
