package netlayer

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	utls "github.com/refraction-networking/utls"
)

// UTLSClient is a TLS-fingerprinting HTTP client using utls.
// It precisely mimics Chrome's TLS ClientHello (JA3/JA4) at the Go level.
//
// The ClientHello preset tracks the session fingerprint's Chrome version
// (chromeVersion) so the TLS handshake, HTTP/2 frames, and User-Agent all
// tell the same version story. The post-quantum signature algorithms
// (0x0904-0x0906) are injected only for Chrome 140+, matching when real
// Chrome started sending them.
type UTLSClient struct {
	target        string
	chromeVersion int
	proxy         string
	timeout       time.Duration
}

// NewUTLSClient creates a utls-based TLS client.
// target is a Chrome impersonate target like "chrome150"; the numeric part
// selects the closest available utls ClientHello preset. When no version can
// be parsed, the preset defaults to HelloChrome_133 (utls's Chrome_Auto).
func NewUTLSClient(target string) *UTLSClient {
	if target == "" {
		target = "chrome"
	}
	return &UTLSClient{
		target:        target,
		chromeVersion: parseChromeVersion(target),
		timeout:       30 * time.Second,
	}
}

// parseChromeVersion extracts the numeric Chrome version from a target string
// like "chrome150". Returns 0 when no version is present.
func parseChromeVersion(target string) int {
	v := strings.TrimPrefix(strings.ToLower(target), "chrome")
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

// utlsPresetFor maps a Chrome version to the closest available utls preset.
// utls v1.8.2 ships Chrome presets up to 133; newer versions reuse 133
// (ClientHello bytes have been stable across recent Chrome releases apart
// from the PQ signature algorithms, which are injected separately below).
func utlsPresetFor(chromeVersion int) utls.ClientHelloID {
	switch {
	case chromeVersion >= 133:
		return utls.HelloChrome_133
	case chromeVersion >= 120:
		return utls.HelloChrome_131
	default:
		return utls.HelloChrome_133
	}
}

func (c *UTLSClient) SetProxy(proxyURL string)   { c.proxy = proxyURL }
func (c *UTLSClient) SetTimeout(d time.Duration) { c.timeout = d }
func (c *UTLSClient) CheckAvailable() bool       { return true }

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

	preset := utlsPresetFor(c.chromeVersion)
	tlsConn := utls.UClient(conn, &utls.Config{
		ServerName: host,
	}, preset)

	// Build the handshake state with the preset. BuildHandshakeState (with
	// session) sets clientHelloBuildStatus = BuildByUtls, so HandshakeContext
	// will NOT re-run applyPresetByID (which would overwrite our mods).
	// It WILL re-run ApplyConfig + MarshalClientHello, picking up our changes.
	if err := tlsConn.BuildHandshakeState(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("build handshake state: %w", err)
	}

	// Post-build: inject post-quantum signature algorithms (0x0904-0x0906)
	// into the SignatureAlgorithmsExtension. Chrome 140+ includes these, and
	// without them the JA4 sig-alg hash differs from real Chrome. Older
	// Chrome versions must NOT send them (the JA4 would be ahead of its time).
	if c.chromeVersion >= 140 {
		for _, ext := range tlsConn.Extensions {
			if sigExt, ok := ext.(*utls.SignatureAlgorithmsExtension); ok {
				sigExt.SupportedSignatureAlgorithms = append(
					[]utls.SignatureScheme{
						sigSchemeMLDSA65SHA256,
						sigSchemeMLDSA87SHA512,
						sigSchemeSLHDSA192sSHA256,
					},
					sigExt.SupportedSignatureAlgorithms...,
				)
				break
			}
		}
	}

	// HandshakeContext will re-ApplyConfig (writing our modified sig algs to
	// the Hello) and re-MarshalClientHello, then perform the TLS handshake.
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	return tlsConn, nil
}

// Request sends an HTTP request with Chrome-mimicking TLS + HTTP/2 fingerprint.
//
// It first tries requestH2 — a manually-constructed HTTP/2 client that
// controls every frame byte (SETTINGS, WINDOW_UPDATE, HEADERS with PRIORITY,
// pseudo-header order) to match real Chrome's akamai fingerprint exactly.
// It falls back to HTTP/1.1 over utls only when ALPN does not negotiate h2;
// all other errors are returned directly to avoid re-sending the request.
func (c *UTLSClient) Request(method, reqURL string, headers map[string]string, body []byte) (*Response, error) {
	// Primary path: HTTP/2 with Chrome-precise frame fingerprinting.
	resp, err := c.requestH2(context.Background(), method, reqURL, headers, body)
	if err == nil {
		return resp, nil
	}

	// Only fall back to HTTP/1.1 when the server did not negotiate h2 via ALPN.
	// Any other error (timeout, RST_STREAM, network failure) is returned as-is —
	// re-sending on those would silently replay non-idempotent requests (e.g. a
	// POST that already executed on the server).
	if !errors.Is(err, errNoH2ALPN) {
		return nil, err
	}

	// Fallback: HTTP/1.1 over utls (still Chrome TLS fingerprint).
	parsedURL, err := url.Parse(reqURL)
	if err != nil {
		return nil, fmt.Errorf("invalid request URL %q: %w", reqURL, err)
	}
	host := parsedURL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("request URL %q has no host", reqURL)
	}
	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return c.requestHTTP1(method, reqURL, headers, body, host, port)
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
	var setCookies []string
	for _, line := range lines[1:] {
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		val := strings.TrimSpace(line[colonIdx+1:])
		headers[key] = val
		if strings.EqualFold(key, "Set-Cookie") {
			setCookies = append(setCookies, val)
			cookieParts := strings.SplitN(val, ";", 2)
			if len(cookieParts) > 0 {
				eqIdx := strings.Index(cookieParts[0], "=")
				if eqIdx > 0 {
					cookies[strings.TrimSpace(cookieParts[0][:eqIdx])] = strings.TrimSpace(cookieParts[0][eqIdx+1:])
				}
			}
		}
	}

	// Decompress before base64 so JS consumers always see the decoded bytes
	// (netlayer handles Content-Encoding; the sandbox never re-decompresses).
	body := string(bodyBytes)
	if headers["Content-Encoding"] == "gzip" {
		if gr, err := gzip.NewReader(strings.NewReader(body)); err == nil {
			if data, err := io.ReadAll(gr); err == nil {
				body = string(data)
				bodyBytes = data
			}
			gr.Close()
		}
	} else if headers["Content-Encoding"] == "br" {
		if br := brotli.NewReader(strings.NewReader(body)); br != nil {
			if data, err := io.ReadAll(br); err == nil {
				body = string(data)
				bodyBytes = data
			}
		}
	}

	return &Response{
		Status:     status,
		Headers:    headers,
		Body:       body,
		BodyB64:    b64Encode(bodyBytes),
		Cookies:    cookies,
		SetCookies: setCookies,
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
