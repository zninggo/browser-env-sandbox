// Package netlayer handles network requests from within the sandbox.
//
// Two modes:
// - Replay: offline, responses come from a pre-recorded session
// - Live: real requests via HTTP client with TLS fingerprint matching
package netlayer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Mode determines how network requests are handled.
type Mode string

const (
	ModeReplay Mode = "replay"
	ModeLive   Mode = "live"
)

// Handler processes XHR/fetch requests from the sandbox.
type Handler struct {
	mode       Mode
	replay     *ReplayStore
	cookieJar  map[string]string
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
		cookieJar: make(map[string]string),
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
	Cookies map[string]string
}

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
	// Inject cookies into headers
	h.mu.Lock()
	if len(h.cookieJar) > 0 {
		cookieParts := make([]string, 0, len(h.cookieJar))
		for k, v := range h.cookieJar {
			cookieParts = append(cookieParts, k+"="+v)
		}
		if headers == nil {
			headers = make(map[string]string)
		}
		headers["Cookie"] = strings.Join(cookieParts, "; ")
	}
	h.mu.Unlock()

	// Use TLSClient (curl-impersonate → curl_cffi → standard HTTP fallback)
	// for TLS fingerprint matching with the UA version.
	if h.tlsClient != nil {
		resp, err := h.tlsClient.Request(method, urlStr, headers, body)
		if err != nil {
			return nil, fmt.Errorf("TLS client request failed: %w", err)
		}

		// Update cookie jar from Set-Cookie
		if resp.Cookies != nil {
			h.mu.Lock()
			for k, v := range resp.Cookies {
				h.cookieJar[k] = v
			}
			h.mu.Unlock()
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

	// Inject cookies
	h.mu.Lock()
	if len(h.cookieJar) > 0 {
		cookieParts := make([]string, 0, len(h.cookieJar))
		for k, v := range h.cookieJar {
			cookieParts = append(cookieParts, k+"="+v)
		}
		req.Header.Set("Cookie", strings.Join(cookieParts, "; "))
	}
	h.mu.Unlock()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract Set-Cookie headers
	cookies := make(map[string]string)
	for _, c := range resp.Cookies() {
		cookies[c.Name] = c.Value
		h.mu.Lock()
		h.cookieJar[c.Name] = c.Value
		h.mu.Unlock()
	}

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	return &Response{
		Status:  resp.StatusCode,
		Headers: respHeaders,
		Body:    string(respBody),
		Cookies: cookies,
	}, nil
}

// GetCookies returns the current cookie jar.
func (h *Handler) GetCookies() map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := make(map[string]string, len(h.cookieJar))
	for k, v := range h.cookieJar {
		cp[k] = v
	}
	return cp
}

// SetCookie sets a cookie in the jar.
func (h *Handler) SetCookie(name, value string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cookieJar[name] = value
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
