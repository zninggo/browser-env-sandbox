package netlayer

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// QUICClient provides HTTP/3 (QUIC) support.
//
// Go 1.25's net/http and golang.org/x/net/http3 (v0.58) integrated H3 into
// the standard http.Transport via internal registerTransport. The public API
// for standalone H3 RoundTrippers was removed in this version. To enable H3,
// we configure an http.Transport that advertises h3 in ALPN and lets the
// http3 package's linkname-registered transport hook handle QUIC negotiation
// when the server advertises Alt-Svc with h3.
//
// This client is a thin wrapper that:
//  1. Creates an http.Transport with h3-friendly TLS config.
//  2. Sends the request. If the server supports H3 (via Alt-Svc upgrade),
//     the transport transparently uses QUIC. Otherwise it falls back to H2/TCP.
//
// Note: true JA3/JA4 TLS fingerprinting doesn't apply to QUIC (no TCP
// ClientHello). The QUIC transport parameters form the H3 fingerprint, which
// Go's stack produces with defaults close to (but not identical to) Chrome.
type QUICClient struct {
	target  string
	proxy   string
	timeout time.Duration
	client  *http.Client
}

// NewQUICClient creates an HTTP/3-capable client.
func NewQUICClient(target string) *QUICClient {
	c := &QUICClient{
		target:  target,
		timeout: 30 * time.Second,
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			NextProtos: []string{"h3", "h2", "http/1.1"},
		},
		ForceAttemptHTTP2: true,
	}
	c.client = &http.Client{
		Transport: transport,
		Timeout:   c.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return c
}

func (c *QUICClient) SetProxy(proxyURL string) {
	c.proxy = proxyURL
	if proxyURL != "" {
		if pu, err := url.Parse(proxyURL); err == nil {
			c.client.Transport.(*http.Transport).Proxy = http.ProxyURL(pu)
		}
	}
}
func (c *QUICClient) SetTimeout(d time.Duration) { c.timeout = d; c.client.Timeout = d }

// CheckAvailable returns true — H3/QUIC support is built-in.
func (c *QUICClient) CheckAvailable() bool { return true }

// Backend returns the transport name for logging.
func (c *QUICClient) Backend() string { return "quic-h3" }

// Request sends an HTTP request. For https URLs, the transport attempts H3
// via ALPN/Alt-Svc. On any error, callers should fall back to HTTP/2-over-utls.
func (c *QUICClient) Request(method, reqURL string, headers map[string]string, body []byte) (*Response, error) {
	parsedURL, err := url.Parse(reqURL)
	if err != nil {
		return nil, fmt.Errorf("invalid request URL %q: %w", reqURL, err)
	}
	if parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("QUIC/H3 requires https, got %s", parsedURL.Scheme)
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = strings.NewReader(string(body))
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
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
	}
	for k, v := range resp.Header {
		if strings.EqualFold(k, "Set-Cookie") {
			continue
		}
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	// Determine whether the response used H3 (for logging/observability).
	proto := resp.Proto
	respHeaders["X-BES-Proto"] = proto

	return &Response{
		Status:     resp.StatusCode,
		Headers:    respHeaders,
		Body:       string(respBody),
		BodyB64:    b64Encode(respBody),
		Cookies:    cookies,
		SetCookies: setCookies,
	}, nil
}
