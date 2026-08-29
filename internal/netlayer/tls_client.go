package netlayer

import "fmt"

// TLSClient is the single TLS-fingerprinting HTTP client used in live mode.
//
// Only the utls backend is selected: utls is a pure-Go reimplementation of
// Chrome's ClientHello that is always available (no subprocess, no shared lib)
// and produces Chrome-accurate JA3/JA4 fingerprints. The former
// curl-impersonate → curl_cffi → standard-HTTP fallback chain was removed —
// those branches were unreachable dead code because utls always reports
// available, so the fallbacks could never be selected.
type TLSClient struct {
	utls   *UTLSClient
	target string
}

// NewTLSClient creates the unified TLS client. utls is always set up because
// it is embedded and always available.
func NewTLSClient(target string) *TLSClient {
	if target == "" {
		target = "chrome150"
	}
	return &TLSClient{
		utls:   NewUTLSClient(target),
		target: target,
	}
}

// Request sends an HTTP request using the best available TLS client.
func (tc *TLSClient) Request(method, url string, headers map[string]string, body []byte) (*Response, error) {
	if tc.utls != nil {
		return tc.utls.Request(method, url, headers, body)
	}
	return nil, fmt.Errorf("no TLS client available")
}

// SetProxy configures a proxy on the utls backend.
func (tc *TLSClient) SetProxy(proxyURL string) {
	if tc.utls != nil {
		tc.utls.SetProxy(proxyURL)
	}
}

// Backend returns which TLS backend is in use.
func (tc *TLSClient) Backend() string { return "utls" }
