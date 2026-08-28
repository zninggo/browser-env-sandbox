package netlayer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// CurlImpersonate integrates with curl-impersonate binary for TLS fingerprint matching.
// It shells out to curl-impersonate wrapper scripts (e.g. curl_chrome116).
type CurlImpersonate struct {
	binaryPath string   // path to curl-impersonate wrapper script directory
	target     string   // e.g. "chrome116"
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
		"-s",  // silent
		"-D", "-",  // dump headers to stdout
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
		"method":           method,
		"url":              url,
		"headers":          headers,
		"impersonate":      c.target,
		"allow_redirects":  false,
		"timeout":          int(c.timeout.Seconds()),
	}
	if len(body) > 0 {
		reqSpec["data"] = string(body)
	}
	if c.proxy != "" {
		reqSpec["proxies"] = map[string]string{"http": c.proxy, "https": c.proxy}
	}

	reqJSON, _ := json.Marshal(reqSpec)

	pythonCode := fmt.Sprintf(`
import json, sys
from curl_cffi import requests
req = json.loads(sys.stdin.read())
try:
    resp = requests.request(**req)
    result = {
        "status": resp.status_code,
        "headers": dict(resp.headers),
        "body": resp.text,
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
		Cookies: result.Cookies,
	}, nil
}

// CheckAvailable checks if curl_cffi is installed.
func (c *CurlCffiClient) CheckAvailable() bool {
	cmd := exec.Command(c.pythonPath, "-c", "from curl_cffi import requests; print('ok')")
	return cmd.Run() == nil
}

// --- Unified client ---

// TLSClient is a unified TLS-fingerprinting HTTP client.
// Priority: utls (pure Go, most accurate) → curl-impersonate → curl_cffi → standard Go HTTP.
type TLSClient struct {
	utls        *UTLSClient
	impersonate *CurlImpersonate
	cffi        *CurlCffiClient
	fallback    *http.Client
	target      string
}

// NewTLSClient creates a unified TLS client with the given target.
func NewTLSClient(target string) *TLSClient {
	tc := &TLSClient{target: target}

	// Try utls first — pure Go, most accurate TLS fingerprint, no subprocess
	tc.utls = NewUTLSClient(target)
	if tc.utls.CheckAvailable() {
		return tc
	}

	// Try curl-impersonate
	tc.impersonate = NewCurlImpersonate("", target)
	if tc.impersonate.CheckAvailable() {
		return tc
	}

	// Try curl_cffi
	tc.cffi = NewCurlCffi("", target)
	if tc.cffi.CheckAvailable() {
		return tc
	}

	// Fallback to standard Go HTTP (no TLS fingerprinting)
	tc.fallback = &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return tc
}

// Request sends an HTTP request using the best available TLS client.
func (tc *TLSClient) Request(method, url string, headers map[string]string, body []byte) (*Response, error) {
	if tc.utls != nil && tc.utls.CheckAvailable() {
		return tc.utls.Request(method, url, headers, body)
	}
	if tc.impersonate != nil && tc.impersonate.CheckAvailable() {
		return tc.impersonate.Request(method, url, headers, body)
	}
	if tc.cffi != nil && tc.cffi.CheckAvailable() {
		return tc.cffi.Request(method, url, headers, body)
	}
	// Fallback: standard Go HTTP
	return tc.fallbackRequest(method, url, headers, body)
}

func (tc *TLSClient) fallbackRequest(method, url string, headers map[string]string, body []byte) (*Response, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = strings.NewReader(string(body))
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := tc.fallback.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	respHeaders := make(map[string]string)
	cookies := make(map[string]string)
	var setCookies []string
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}
	for _, sc := range resp.Header["Set-Cookie"] {
		setCookies = append(setCookies, sc)
		nv := parseSetCookieNameValue(sc)
		if nv[1] != "" || nv[0] != "" {
			cookies[nv[0]] = nv[1]
		}
	}
	return &Response{Status: resp.StatusCode, Headers: respHeaders, Body: string(respBody), Cookies: cookies, SetCookies: setCookies}, nil
}

// SetProxy configures proxy on all backends.
func (tc *TLSClient) SetProxy(proxyURL string) {
	if tc.utls != nil {
		tc.utls.SetProxy(proxyURL)
	}
	if tc.impersonate != nil {
		tc.impersonate.SetProxy(proxyURL)
	}
	if tc.cffi != nil {
		tc.cffi.SetProxy(proxyURL)
	}
}

// Backend returns which TLS backend is in use.
func (tc *TLSClient) Backend() string {
	if tc.utls != nil && tc.utls.CheckAvailable() {
		return "utls"
	}
	if tc.impersonate != nil && tc.impersonate.CheckAvailable() {
		return "curl-impersonate"
	}
	if tc.cffi != nil && tc.cffi.CheckAvailable() {
		return "curl_cffi"
	}
	return "standard-http"
}
