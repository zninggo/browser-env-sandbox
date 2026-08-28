package netlayer

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// UTLSClient is a TLS-fingerprinting HTTP client using utls.
// It precisely mimics Chrome's TLS ClientHello (JA3/JA4) at the Go level.
//
// Key advantage over curl_cffi: utls reproduces the exact ClientHello bytes
// that real Chrome sends. curl_cffi wraps libcurl+openssl which has subtle
// differences that can be detected at the TLS layer.
type UTLSClient struct {
	target  string
	proxy   string
	timeout time.Duration
}

// NewUTLSClient creates a utls-based TLS client.
func NewUTLSClient(target string) *UTLSClient {
	if target == "" {
		target = "chrome"
	}
	return &UTLSClient{
		target:  target,
		timeout: 30 * time.Second,
	}
}

func (c *UTLSClient) SetProxy(proxyURL string) { c.proxy = proxyURL }
func (c *UTLSClient) SetTimeout(d time.Duration) { c.timeout = d }
func (c *UTLSClient) CheckAvailable() bool { return true }

// dialUTLS establishes a TCP connection and wraps it with a utls TLS handshake
// using Chrome's ClientHello fingerprint. The returned net.Conn is a *utls.UConn.
func (c *UTLSClient) dialUTLS(ctx context.Context, host, port string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: c.timeout}

	var conn net.Conn
	var err error
	if c.proxy != "" {
		conn, err = c.dialViaProxy(c.proxy, host, port, dialer)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	}
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	tlsConn := utls.UClient(conn, &utls.Config{
		ServerName: host,
	}, utls.HelloChrome_131)

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	return tlsConn, nil
}

// Request sends an HTTP request with Chrome-mimicking TLS fingerprint.
// It uses http.Transport with a custom DialTLSContext that performs the utls
// handshake. HTTP/2 is disabled because http.Transport checks for *tls.Conn
// to enable h2, and utls.UConn is not *tls.Conn. HTTP/1.1 over utls is still
// sufficient to pass JA3/JA4 TLS fingerprint checks.
func (c *UTLSClient) Request(method, reqURL string, headers map[string]string, body []byte) (*Response, error) {
	parsedURL, err := url.Parse(reqURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	// Create transport with utls dial + Chrome-aligned HTTP/2 SETTINGS.
	// Since utls.UConn is not *crypto/tls.Conn, http.Transport won't enable
	// HTTP/2 automatically. We use http2.Transport directly for h2 support.
	// The SETTINGS frame values below match what real Chrome sends, so the
	// HTTP/2 fingerprint (Akamai-style) lines up with the TLS fingerprint.
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			return c.dialUTLS(ctx, host, port)
		},
		// Chrome's SETTINGS_HEADER_TABLE_SIZE (Chrome uses 65536, not the
		// HTTP/2 spec default of 4096 that Go sends).
		MaxDecoderHeaderTableSize: 65536,
		MaxEncoderHeaderTableSize: 65536,
		// Chrome's SETTINGS_MAX_HEADER_LIST_SIZE.
		MaxHeaderListSize: 262144,
	}

	// Build request
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = strings.NewReader(string(body))
	}
	httpReq, err := http.NewRequestWithContext(context.Background(), method, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set headers
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}
	// Defaults matching Chrome
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	}
	if httpReq.Header.Get("Accept") == "" {
		httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}
	if httpReq.Header.Get("Accept-Language") == "" {
		httpReq.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	}
	if httpReq.Header.Get("Accept-Encoding") == "" {
		httpReq.Header.Set("Accept-Encoding", "gzip, deflate, br")
	}

	// Execute via HTTP/2 transport (Chrome uses h2 for HTTPS)
	resp, err := transport.RoundTrip(httpReq)
	if err != nil {
		// Fallback to HTTP/1.1 over utls
		return c.requestHTTP1(method, reqURL, headers, body, host, port)
	}
	defer resp.Body.Close()

	// Read and decompress body
	respBody := c.readBody(resp)

	// Parse headers and cookies
	respHeaders := make(map[string]string)
	cookies := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
		if strings.EqualFold(k, "Set-Cookie") {
			for _, cookie := range v {
				cookieParts := strings.SplitN(cookie, ";", 2)
				if len(cookieParts) > 0 {
					eqIdx := strings.Index(cookieParts[0], "=")
					if eqIdx > 0 {
						cookies[strings.TrimSpace(cookieParts[0][:eqIdx])] = strings.TrimSpace(cookieParts[0][eqIdx+1:])
					}
				}
			}
		}
	}

	return &Response{
		Status:  resp.StatusCode,
		Headers: respHeaders,
		Body:    respBody,
		Cookies: cookies,
	}, nil
}

// readBody decompresses the response body based on Content-Encoding.
func (c *UTLSClient) readBody(resp *http.Response) string {
	encoding := resp.Header.Get("Content-Encoding")
	var reader io.Reader = resp.Body

	switch encoding {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			data, _ := io.ReadAll(resp.Body)
			return string(data)
		}
		reader = gr
		defer gr.Close()
	case "br":
		reader = brotli.NewReader(resp.Body)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return ""
	}
	return string(data)
}

// requestHTTP1 is the HTTP/1.1 fallback when HTTP/2 fails. It sends a raw
// HTTP/1.1 request over a fresh utls TLS connection.
func (c *UTLSClient) requestHTTP1(method, reqURL string, headers map[string]string, body []byte, host, port string) (*Response, error) {
	parsedURL, _ := url.Parse(reqURL)
	ctx := context.Background()
	conn, err := c.dialUTLS(ctx, host, port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	if parsedURL.RawQuery != "" {
		path += "?" + parsedURL.RawQuery
	}

	// Disable ALPN h2 by sending HTTP/1.1 request — but TLS ALPN already
	// negotiated h2, so we must not use the h2-negotiated conn for HTTP/1.1.
	// Instead, re-dial without h2 in ALPN isn't possible with HelloChrome_Auto
	// preset. So we just attempt raw HTTP/1.1 and let the server respond.

	var reqLine string
	reqLine = fmt.Sprintf("%s %s HTTP/1.1\r\nHost: %s\r\n", method, path, parsedURL.Host)
	for k, v := range headers {
		reqLine += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	if _, ok := headers["User-Agent"]; !ok {
		reqLine += "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36\r\n"
	}
	if _, ok := headers["Accept"]; !ok {
		reqLine += "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n"
	}
	if _, ok := headers["Accept-Language"]; !ok {
		reqLine += "Accept-Language: zh-CN,zh;q=0.9\r\n"
	}
	if _, ok := headers["Accept-Encoding"]; !ok {
		reqLine += "Accept-Encoding: gzip, deflate, br\r\n"
	}
	reqLine += "Connection: close\r\n\r\n"
	if len(body) > 0 {
		reqLine += string(body)
	}

	if _, err := conn.Write([]byte(reqLine)); err != nil {
		return nil, err
	}

	raw, err := io.ReadAll(conn)
	if err != nil {
		return nil, err
	}

	return parseHTTP1Response(raw)
}

// parseHTTP1Response parses a raw HTTP/1.1 response into a Response.
func parseHTTP1Response(raw []byte) (*Response, error) {
	idx := strings.Index(string(raw), "\r\n\r\n")
	if idx < 0 {
		return &Response{Status: 0, Body: string(raw)}, nil
	}

	headerStr := string(raw[:idx])
	bodyBytes := raw[idx+4:]

	lines := strings.Split(headerStr, "\r\n")
	status := 0
	if len(lines) > 0 {
		parts := strings.SplitN(lines[0], " ", 3)
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &status)
		}
	}

	headers := make(map[string]string)
	cookies := make(map[string]string)
	for _, line := range lines[1:] {
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		val := strings.TrimSpace(line[colonIdx+1:])
		headers[key] = val
		if strings.EqualFold(key, "Set-Cookie") {
			cookieParts := strings.SplitN(val, ";", 2)
			if len(cookieParts) > 0 {
				eqIdx := strings.Index(cookieParts[0], "=")
				if eqIdx > 0 {
					cookies[strings.TrimSpace(cookieParts[0][:eqIdx])] = strings.TrimSpace(cookieParts[0][eqIdx+1:])
				}
			}
		}
	}

	body := string(bodyBytes)
	if headers["Content-Encoding"] == "gzip" {
		if gr, err := gzip.NewReader(strings.NewReader(body)); err == nil {
			if data, err := io.ReadAll(gr); err == nil {
				body = string(data)
			}
			gr.Close()
		}
	} else if headers["Content-Encoding"] == "br" {
		if br := brotli.NewReader(strings.NewReader(body)); br != nil {
			if data, err := io.ReadAll(br); err == nil {
				body = string(data)
			}
		}
	}

	return &Response{
		Status:  status,
		Headers: headers,
		Body:    body,
		Cookies: cookies,
	}, nil
}

// dialViaProxy connects through an HTTP CONNECT proxy.
func (c *UTLSClient) dialViaProxy(proxyURL, host, port string, dialer *net.Dialer) (net.Conn, error) {
	proxy, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	proxyHost := proxy.Hostname()
	proxyPort := proxy.Port()
	if proxyPort == "" {
		proxyPort = "8080"
	}

	conn, err := dialer.Dial("tcp", net.JoinHostPort(proxyHost, proxyPort))
	if err != nil {
		return nil, err
	}

	connectReq := fmt.Sprintf("CONNECT %s:%s HTTP/1.1\r\nHost: %s:%s\r\n\r\n", host, port, host, port)
	_, err = conn.Write([]byte(connectReq))
	if err != nil {
		conn.Close()
		return nil, err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if !strings.Contains(string(buf[:n]), "200") {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT failed: %s", strings.TrimSpace(string(buf[:n])))
	}

	return conn, nil
}
