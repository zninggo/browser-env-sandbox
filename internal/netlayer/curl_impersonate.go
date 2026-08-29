package netlayer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CurlImpersonate integrates with curl-impersonate binary for TLS fingerprint matching.
// It shells out to curl-impersonate wrapper scripts (e.g. curl_chrome116).
type CurlImpersonate struct {
	binaryPath string // path to curl-impersonate wrapper script directory
	target     string // e.g. "chrome116"
	proxy      string
	timeout    time.Duration
}

// NewCurlImpersonate creates a curl-impersonate client.
// binaryPath is the directory containing wrapper scripts (e.g. /usr/local/bin).
// target is the impersonate target (e.g. "chrome116", "chrome110", "ff117").
func NewCurlImpersonate(binaryPath, target string) *CurlImpersonate {
	if binaryPath == "" {
		binaryPath = "/usr/local/bin"
	}
	if target == "" {
		target = "chrome150"
	}
	return &CurlImpersonate{
		binaryPath: binaryPath,
		target:     target,
		timeout:    30 * time.Second,
	}
}

// SetProxy configures a proxy for all requests.
func (c *CurlImpersonate) SetProxy(proxyURL string) {
	c.proxy = proxyURL
}

// SetTimeout configures the request timeout.
func (c *CurlImpersonate) SetTimeout(d time.Duration) {
	c.timeout = d
}

// Request sends an HTTP request via curl-impersonate.
func (c *CurlImpersonate) Request(method, url string, headers map[string]string, body []byte) (*Response, error) {
	wrapper := fmt.Sprintf("%s/curl_%s", c.binaryPath, c.target)

	args := []string{
		url,
		"-X", method,
		"-s",      // silent
		"-D", "-", // dump headers to stdout
		"--max-time", fmt.Sprintf("%d", int(c.timeout.Seconds())),
	}

	// Headers
	for k, v := range headers {
		args = append(args, "-H", fmt.Sprintf("%s: %s", k, v))
	}

	// Body
	if len(body) > 0 {
		args = append(args, "--data-raw", string(body))
	}

	// Proxy
	if c.proxy != "" {
		args = append(args, "--proxy", c.proxy)
	}

	// Don't follow redirects (callers handle redirects manually)
	args = append(args, "--location", "--max-redirs", "0")

	cmd := exec.Command(wrapper, args...)
	cmd.Stderr = nil

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// curl-impersonate returns non-zero on some HTTP errors, but still outputs
		// Check if we got output anyway
		if stdout.Len() == 0 {
			return nil, fmt.Errorf("curl-impersonate failed: %w", err)
		}
	}

	return parseCurlOutput(stdout.Bytes())
}

// parseCurlOutput parses curl's raw output (headers + body) into a Response.
func parseCurlOutput(raw []byte) (*Response, error) {
	// Find the double CRLF that separates headers from body
	idx := bytes.Index(raw, []byte("\r\n\r\n"))
	if idx < 0 {
		return &Response{Status: 0, Body: string(raw)}, nil
	}

	headerBytes := raw[:idx]
	bodyBytes := raw[idx+4:]

	lines := strings.Split(string(headerBytes), "\r\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("no headers in response")
	}

	// Parse status line: HTTP/1.1 200 OK
	status := 0
	statusParts := strings.SplitN(lines[0], " ", 3)
	if len(statusParts) >= 2 {
		fmt.Sscanf(statusParts[1], "%d", &status)
	}

	// Parse headers
	headers := make(map[string]string)
	var cookies map[string]string
	var setCookies []string
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		val := strings.TrimSpace(line[colonIdx+1:])
		headers[key] = val

		// Extract Set-Cookie
		if strings.EqualFold(key, "Set-Cookie") {
			setCookies = append(setCookies, val)
			if cookies == nil {
				cookies = make(map[string]string)
			}
			cookieParts := strings.SplitN(val, ";", 2)
			if len(cookieParts) > 0 {
				eqIdx := strings.Index(cookieParts[0], "=")
				if eqIdx > 0 {
					cookies[strings.TrimSpace(cookieParts[0][:eqIdx])] = strings.TrimSpace(cookieParts[0][eqIdx+1:])
				}
			}
		}
	}

	return &Response{
		Status:     status,
		Headers:    headers,
		Body:       string(bodyBytes),
		BodyB64:    b64Encode(bodyBytes),
		Cookies:    cookies,
		SetCookies: setCookies,
	}, nil
}

// CheckAvailable checks if curl-impersonate is installed.
func (c *CurlImpersonate) CheckAvailable() bool {
	wrapper := fmt.Sprintf("%s/curl_%s", c.binaryPath, c.target)
	cmd := exec.Command(wrapper, "--version")
	return cmd.Run() == nil
}

// --- Alternative: curl_cffi via Python subprocess ---

// CurlCffiClient calls curl_cffi Python library via subprocess.
// Use this if curl-impersonate binary isn't available but Python curl_cffi is.
type CurlCffiClient struct {
	pythonPath string
	target     string
	proxy      string
	timeout    time.Duration
}

// NewCurlCffi creates a curl_cffi client.
func NewCurlCffi(pythonPath, target string) *CurlCffiClient {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	if target == "" {
		target = "chrome150"
	}
	return &CurlCffiClient{
		pythonPath: pythonPath,
		target:     target,
		timeout:    30 * time.Second,
	}
}

// SetProxy configures a proxy.
func (c *CurlCffiClient) SetProxy(proxyURL string) {
	c.proxy = proxyURL
}

// Request sends an HTTP request via curl_cffi.
func (c *CurlCffiClient) Request(method, url string, headers map[string]string, body []byte) (*Response, error) {
	// Build JSON request spec
	reqSpec := map[string]interface{}{
		"method":          method,
		"url":             url,
		"headers":         headers,
		"impersonate":     c.target,
		"allow_redirects": false,
		"timeout":         int(c.timeout.Seconds()),
	}
	if len(body) > 0 {
		reqSpec["data"] = string(body)
	}
	if c.proxy != "" {
		reqSpec["proxies"] = map[string]string{"http": c.proxy, "https": c.proxy}
	}

	reqJSON, _ := json.Marshal(reqSpec)

	pythonCode := fmt.Sprintf(`
import json, sys, base64
from curl_cffi import requests
req = json.loads(sys.stdin.read())
try:
    resp = requests.request(**req)
    result = {
        "status": resp.status_code,
        "headers": dict(resp.headers),
        "body": resp.text,
        "body_b64": base64.b64encode(resp.content).decode("ascii"),
        "cookies": dict(resp.cookies),
    }
    print(json.dumps(result))
except Exception as e:
    print(json.dumps({"error": str(e)}))
    sys.exit(1)
`)

	cmd := exec.Command(c.pythonPath, "-c", pythonCode)
	cmd.Stdin = bytes.NewReader(reqJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("curl_cffi failed: %w (stderr: %s)", err, stderr.String())
	}

	var result struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
		BodyB64 string            `json:"body_b64"`
		Cookies map[string]string `json:"cookies"`
		Error   string            `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse curl_cffi output failed: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("curl_cffi error: %s", result.Error)
	}

	return &Response{
		Status:  result.Status,
		Headers: result.Headers,
		Body:    result.Body,
		BodyB64: result.BodyB64,
		Cookies: result.Cookies,
	}, nil
}

// CheckAvailable checks if curl_cffi is installed.
func (c *CurlCffiClient) CheckAvailable() bool {
	cmd := exec.Command(c.pythonPath, "-c", "from curl_cffi import requests; print('ok')")
	return cmd.Run() == nil
}

// --- Unified client ---

// TLSClient is the single TLS-fingerprinting HTTP client used in live mode.
//
// Only the utls backend is selected: utls is a pure-Go reimplementation of
// Chrome's ClientHello that is always available (no subprocess, no shared lib)
// and produces Chrome-accurate JA3/JA4 fingerprints. The former
// curl-impersonate → curl_cffi → standard-HTTP fallback chain was removed —
// those branches were unreachable dead code because utls always reports
// available, so the fallbacks could never be selected.
//
// QUIC (HTTP/3) is a separately-gated optional transport: when EnableQUIC is
// called, https requests try it first only if QUIC actually reports available.
type TLSClient struct {
	utls        *UTLSClient
	quic        *QUICClient
	quicEnabled bool
	target      string
}

// EnableQUIC turns on HTTP/3 (QUIC) as a preferred transport for https URLs.
func (tc *TLSClient) EnableQUIC() {
	tc.quic = NewQUICClient(tc.target)
	tc.quicEnabled = true
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
	// QUIC (HTTP/3) is only tried for https when enabled AND actually available;
	// otherwise it falls through to the TLS-fingerprinted utls HTTP/2 path.
	if tc.quicEnabled && tc.quic != nil && tc.quic.CheckAvailable() && strings.HasPrefix(url, "https://") {
		if resp, err := tc.quic.Request(method, url, headers, body); err == nil {
			return resp, nil
		}
		// QUIC failed — fall through to utls/H2
	}
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
