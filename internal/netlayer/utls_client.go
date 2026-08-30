package netlayer

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
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
	// A request with a body MUST carry a body-framing header. Without
	// Content-Length (and with Connection: close, no chunked request encoding),
	// servers that expect a body block waiting for more bytes or misparse the
	// request — the body bytes are indistinguishable from a missing body. Emit
	// Content-Length unless the caller already supplied it (case-insensitive).
	if len(body) > 0 && !headerPresent(headers, "Content-Length") {
		reqLine += fmt.Sprintf("Content-Length: %d\r\n", len(body))
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

	// HTTP/1.1 chunked transfer encoding: the body is a sequence of chunks
	// "<size-hex>\r\n<data>\r\n" terminated by a zero-size chunk. Without
	// de-chunking, the size markers and framing bytes are returned as body,
	// corrupting the payload. Only de-chunk when Transfer-Encoding says so;
	// Content-Length bodies are already correctly framed.
	if isChunked(headers["Transfer-Encoding"]) {
		bodyBytes = dechunk(bodyBytes)
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

// dialViaProxy connects through an HTTP CONNECT, SOCKS5, or SOCKS4 proxy.
//
// The proxy scheme and credentials are parsed from proxyURL:
//   - "http://user:pass@host:port" or "https://..." → HTTP CONNECT, with
//     Proxy-Authorization: Basic when user:pass is present.
//   - "socks5://user:pass@host:port" → SOCKS5 with username/password auth.
//   - "socks5://host:port"           → SOCKS5, no auth.
//   - "socks4://host:port"           → SOCKS4 (no auth; SOCKS4a via hostname).
//
// A scheme-less URL is treated as an HTTP proxy for backward compatibility.
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

	// Username/password may come from the URL's userinfo (user:pass@) — the
	// path real callers use. ProxyConfig.Username/Password is a separate
	// pool-level struct that never reaches this string-based entry point.
	username := ""
	password := ""
	if proxy.User != nil {
		username = proxy.User.Username()
		if p, ok := proxy.User.Password(); ok {
			password = p
		}
	}

	scheme := strings.ToLower(proxy.Scheme)
	switch scheme {
	case "socks5", "socks5h":
		return dialSocks5(dialer, proxyHost, proxyPort, host, port, username, password)
	case "socks4", "socks4a":
		return dialSocks4(dialer, proxyHost, proxyPort, host, port)
	default:
		// "http", "https", or empty → HTTP CONNECT tunnel.
		return dialHTTPConnect(dialer, proxyHost, proxyPort, host, port, username, password)
	}
}

// dialHTTPConnect opens an HTTP CONNECT tunnel to host:port through an HTTP
// proxy and returns the tunneled connection for the caller to layer TLS on.
func dialHTTPConnect(dialer *net.Dialer, proxyHost, proxyPort, host, port, username, password string) (net.Conn, error) {
	conn, err := dialer.Dial("tcp", net.JoinHostPort(proxyHost, proxyPort))
	if err != nil {
		return nil, err
	}

	var connectReq strings.Builder
	fmt.Fprintf(&connectReq, "CONNECT %s:%s HTTP/1.1\r\nHost: %s:%s\r\n", host, port, host, port)
	// Proxy-Authorization (Basic) for authenticated HTTP proxies. Without it,
	// proxies requiring credentials reject the CONNECT with 407.
	if username != "" {
		cred := username + ":" + password
		fmt.Fprintf(&connectReq, "Proxy-Authorization: Basic %s\r\n", base64.StdEncoding.EncodeToString([]byte(cred)))
	}
	connectReq.WriteString("\r\n")

	if _, err := conn.Write([]byte(connectReq.String())); err != nil {
		conn.Close()
		return nil, err
	}

	// Read the full CONNECT response. A single Read is insufficient: the
	// response headers may be split across TCP segments, and a transparent
	// proxy may prepend bytes before the status line. Read line-by-line until
	// the terminating blank line so we consume exactly the response and leave
	// the connection positioned at the start of the tunnel.
	r := bufio.NewReader(conn)
	statusLine, err := r.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read proxy CONNECT response: %w", err)
	}
	statusLine = strings.TrimSpace(statusLine)

	// Parse the status line ("HTTP/1.1 200 Connection established"). Match on
	// the 3-digit status code in the standard position; a bare Contains("200")
	// would false-positive on "HTTP/1.1 502 Bad Gateway: upstream took 200ms".
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		conn.Close()
		return nil, fmt.Errorf("malformed proxy CONNECT status line: %q", statusLine)
	}
	status, err := strconv.Atoi(parts[1])
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("parse proxy CONNECT status code: %w", err)
	}
	if status != 200 {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT failed: %s", statusLine)
	}

	// Drain remaining response headers up to the blank line.
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("read proxy CONNECT headers: %w", err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	// If the bufio.Reader buffered bytes beyond the headers (the first bytes of
	// the tunneled stream), wrap the connection so they are not lost.
	if r.Buffered() > 0 {
		buf := make([]byte, r.Buffered())
		if _, err := io.ReadFull(r, buf); err != nil {
			conn.Close()
			return nil, fmt.Errorf("read buffered tunnel bytes: %w", err)
		}
		return &prefixConn{conn: conn, buf: buf}, nil
	}
	return conn, nil
}

// dialSocks5 connects to host:port through a SOCKS5 proxy (RFC 1928) with
// optional username/password authentication (RFC 1929).
func dialSocks5(dialer *net.Dialer, proxyHost, proxyPort, host, port, username, password string) (net.Conn, error) {
	conn, err := dialer.Dial("tcp", net.JoinHostPort(proxyHost, proxyPort))
	if err != nil {
		return nil, err
	}

	// Greeting: offer no-auth (0x00) and, when credentials are present,
	// username/password auth (0x02). SOCKS5 servers that require auth reject
	// no-auth-only greetings, so the auth method must be offered.
	methods := []byte{0x00}
	if username != "" {
		methods = append(methods, 0x02)
	}
	greeting := append([]byte{0x05, byte(len(methods))}, methods...)
	if _, err := conn.Write(greeting); err != nil {
		conn.Close()
		return nil, err
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 greeting response: %w", err)
	}
	if resp[0] != 0x05 {
		conn.Close()
		return nil, fmt.Errorf("socks5 bad version: %d", resp[0])
	}
	method := resp[1]
	switch method {
	case 0x00: // no auth
	case 0x02: // username/password
		cred := []byte{0x01, byte(len(username))}
		cred = append(cred, []byte(username)...)
		cred = append(cred, byte(len(password)))
		cred = append(cred, []byte(password)...)
		if _, err := conn.Write(cred); err != nil {
			conn.Close()
			return nil, err
		}
		authResp := make([]byte, 2)
		if _, err := io.ReadFull(conn, authResp); err != nil {
			conn.Close()
			return nil, fmt.Errorf("socks5 auth response: %w", err)
		}
		if authResp[1] != 0x00 {
			conn.Close()
			return nil, fmt.Errorf("socks5 auth failed (status %d)", authResp[1])
		}
	default:
		conn.Close()
		return nil, fmt.Errorf("socks5 no acceptable auth method (server chose %d)", method)
	}

	// Connect request: SOCKS5, CONNECT, address by hostname (0x03) so the proxy
	// resolves it (avoids local DNS leaking the target and supports hosts the
	// client cannot resolve).
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, []byte(host)...)
	req = append(req, byte(portToInt(port)>>8), byte(portToInt(port)))
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}

	// Response: 4 fixed bytes + variable bound address.
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect response: %w", err)
	}
	if head[0] != 0x05 {
		conn.Close()
		return nil, fmt.Errorf("socks5 bad response version: %d", head[0])
	}
	if head[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect failed (code %d)", head[1])
	}
	// Drain the bound address so the stream is positioned at the tunnel start.
	switch head[3] {
	case 0x01: // IPv4
		io.ReadFull(conn, make([]byte, 4))
	case 0x03: // domain
		var l [1]byte
		io.ReadFull(conn, l[:])
		io.ReadFull(conn, make([]byte, int(l[0])))
	case 0x04: // IPv6
		io.ReadFull(conn, make([]byte, 16))
	}
	io.ReadFull(conn, make([]byte, 2)) // bound port

	return conn, nil
}

// dialSocks4 connects to host:port through a SOCKS4/SOCKS4a proxy. SOCKS4a
// (used here) sends the hostname so the proxy resolves it, since classic SOCKS4
// only accepts an IP address.
func dialSocks4(dialer *net.Dialer, proxyHost, proxyPort, host, port string) (net.Conn, error) {
	conn, err := dialer.Dial("tcp", net.JoinHostPort(proxyHost, proxyPort))
	if err != nil {
		return nil, err
	}

	// SOCKS4a request: VN=4, CD=1 (CONNECT), port (2 bytes), a null IP
	// (0.0.0.x with x!=0 signals SOCKS4a to read the userid + hostname),
	// empty userid, then the hostname.
	p := portToInt(port)
	req := []byte{
		0x04, 0x01, // VN, CD
		byte(p >> 8), byte(p), // port
		0x00, 0x00, 0x00, 0x01, // null IP (SOCKS4a marker)
		0x00, // empty userid
	}
	req = append(req, []byte(host)...)
	req = append(req, 0x00)
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}

	// Response: 8 bytes. Byte 1 (status) must be 0x5a (granted).
	resp := make([]byte, 8)
	if _, err := io.ReadFull(conn, resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks4 response: %w", err)
	}
	if resp[1] != 0x5a {
		conn.Close()
		return nil, fmt.Errorf("socks4 connect failed (code 0x%02x)", resp[1])
	}
	return conn, nil
}

// portToInt parses a port string to an int, defaulting to 80 when empty.
func portToInt(port string) int {
	n, err := strconv.Atoi(port)
	if err != nil || n <= 0 {
		return 80
	}
	return n
}

// prefixConn wraps a net.Conn with a leading buffer of bytes that were already
// read off the connection (e.g. tunnel bytes a bufio.Reader consumed past the
// HTTP CONNECT response headers). Reads drain the buffer first, then the conn.
type prefixConn struct {
	conn net.Conn
	buf  []byte
}

func (p *prefixConn) Read(b []byte) (int, error) {
	if len(p.buf) > 0 {
		n := copy(b, p.buf)
		p.buf = p.buf[n:]
		return n, nil
	}
	return p.conn.Read(b)
}

func (p *prefixConn) Write(b []byte) (int, error)        { return p.conn.Write(b) }
func (p *prefixConn) Close() error                       { return p.conn.Close() }
func (p *prefixConn) LocalAddr() net.Addr                { return p.conn.LocalAddr() }
func (p *prefixConn) RemoteAddr() net.Addr               { return p.conn.RemoteAddr() }
func (p *prefixConn) SetDeadline(t time.Time) error      { return p.conn.SetDeadline(t) }
func (p *prefixConn) SetReadDeadline(t time.Time) error  { return p.conn.SetReadDeadline(t) }
func (p *prefixConn) SetWriteDeadline(t time.Time) error { return p.conn.SetWriteDeadline(t) }

// headerPresent reports whether a header key exists in the map, compared
// case-insensitively (HTTP header names are case-insensitive).
func headerPresent(headers map[string]string, key string) bool {
	for k := range headers {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

// isChunked reports whether a Transfer-Encoding header value indicates
// chunked encoding. The value may be a comma-separated list of codings
// (e.g. "gzip, chunked"); chunked is the last transfer coding per RFC 7230.
func isChunked(te string) bool {
	for _, part := range strings.Split(te, ",") {
		if strings.EqualFold(strings.TrimSpace(part), "chunked") {
			return true
		}
	}
	return false
}

// dechunk decodes an HTTP/1.1 chunked-transfer-encoded body into the raw
// payload bytes. Each chunk is "<hex-size>[;ext]\r\n<data>\r\n"; the body ends
// with a zero-size chunk. Malformed input returns whatever was decoded so far,
// matching the partial-data tolerance used elsewhere in this parser.
func dechunk(data []byte) []byte {
	var out []byte
	for len(data) > 0 {
		// Read the chunk size line up to CRLF. The size may carry chunk
		// extensions (";name=value") which we discard.
		lineEnd := bytes.Index(data, []byte("\r\n"))
		if lineEnd < 0 {
			break
		}
		sizeLine := string(data[:lineEnd])
		if semi := strings.IndexByte(sizeLine, ';'); semi >= 0 {
			sizeLine = sizeLine[:semi]
		}
		size, err := strconv.ParseInt(strings.TrimSpace(sizeLine), 16, 64)
		if err != nil || size < 0 {
			break
		}
		data = data[lineEnd+2:]
		if size == 0 {
			break // terminating zero chunk
		}
		if int64(len(data)) < size {
			out = append(out, data...) // truncated; keep what's left
			break
		}
		out = append(out, data[:size]...)
		data = data[size:]
		// Consume the trailing CRLF after the chunk data.
		if len(data) >= 2 && data[0] == '\r' && data[1] == '\n' {
			data = data[2:]
		}
	}
	return out
}
