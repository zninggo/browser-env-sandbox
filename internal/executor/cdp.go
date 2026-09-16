package executor

// Minimal Chrome DevTools Protocol client: an RFC 6455 WebSocket (plain TCP,
// client side only) plus the JSON-RPC command/response discipline CDP uses.
// The sandbox package has its own ws implementation for page sockets; this
// one is intentionally separate and self-contained because the requirements
// differ (command correlation, no TLS, localhost-only).

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

// localDebugHosts are the only hosts a CDP connection may be established to.
// The dual-executor constraint is that Chrome is ALWAYS a locally launched,
// isolated instance — a remote/online debugging endpoint must be refused at
// the dial layer, not by convention.
var localDebugHosts = map[string]bool{
	"127.0.0.1": true,
	"localhost": true,
	"[::1]":     true,
	"::1":       true,
}

// validateLocalWSURL parses a DevTools webSocketDebuggerUrl and rejects any
// non-local endpoint. Returns the parsed URL.
func validateLocalWSURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("cdp: bad websocket url: %w", err)
	}
	if u.Scheme != "ws" {
		return nil, fmt.Errorf("cdp: refusing non-ws debugging endpoint %q (scheme %q)", raw, u.Scheme)
	}
	host := u.Hostname()
	if !localDebugHosts[host] {
		return nil, fmt.Errorf("cdp: refusing non-local debugging endpoint %q — only 127.0.0.1/localhost Chrome is allowed", raw)
	}
	return u, nil
}

// cdpFrame opcodes (RFC 6455 §5.2).
const (
	wsOpContinuation = 0x0
	wsOpText         = 0x1
	wsOpBinary       = 0x2
	wsOpClose        = 0x8
	wsOpPing         = 0x9
	wsOpPong         = 0xA
)

// cdpConn is a page-level CDP connection: one WebSocket to one target.
type cdpConn struct {
	raw    net.Conn
	br     *bufio.Reader
	wmu    sync.Mutex // serializes frame writes
	nextID atomic.Int64

	mu      sync.Mutex
	pending map[int64]chan cdpReply
	closed  bool
}

// cdpReply is the correlated answer to an in-flight command.
type cdpReply struct {
	result json.RawMessage
	err    error
}

// cdpDial performs the WebSocket handshake against a local DevTools endpoint
// and starts the read loop.
func cdpDial(ctx context.Context, wsURL string) (*cdpConn, error) {
	u, err := validateLocalWSURL(wsURL)
	if err != nil {
		return nil, err
	}
	d := net.Dialer{}
	raw, err := d.DialContext(ctx, "tcp", u.Host)
	if err != nil {
		return nil, fmt.Errorf("cdp: dial: %w", err)
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		raw.Close()
		return nil, fmt.Errorf("cdp: rand: %w", err)
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)

	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + u.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := raw.Write([]byte(req)); err != nil {
		raw.Close()
		return nil, fmt.Errorf("cdp: handshake write: %w", err)
	}

	br := bufio.NewReader(raw)
	resp, err := httpReadResponse(br)
	if err != nil {
		raw.Close()
		return nil, fmt.Errorf("cdp: handshake read: %w", err)
	}
	if resp.status != "101" {
		raw.Close()
		return nil, fmt.Errorf("cdp: expected 101 Switching Protocols, got %s", resp.status)
	}
	accept := resp.header["Sec-WebSocket-Accept"]
	if accept == "" || accept != cdpAcceptKey(key) {
		raw.Close()
		return nil, fmt.Errorf("cdp: bad Sec-WebSocket-Accept")
	}

	c := &cdpConn{raw: raw, br: br, pending: make(map[int64]chan cdpReply)}
	go c.readLoop()
	return c, nil
}

// httpReadResponse reads just enough of the HTTP 101 response for the
// handshake. A tiny hand-rolled reader instead of net/http.ReadResponse —
// the 101 has no body and we need the connection left in a clean state for
// frame reads.
func httpReadResponse(br *bufio.Reader) (struct {
	status string
	header map[string]string
}, error) {
	var out struct {
		status string
		header map[string]string
	}
	out.header = map[string]string{}
	line, err := br.ReadString('\n')
	if err != nil {
		return out, err
	}
	line = strings.TrimRight(line, "\r\n")
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return out, fmt.Errorf("malformed status line %q", line)
	}
	out.status = parts[1]
	for {
		h, err := br.ReadString('\n')
		if err != nil {
			return out, err
		}
		h = strings.TrimRight(h, "\r\n")
		if h == "" {
			break
		}
		if i := strings.IndexByte(h, ':'); i > 0 {
			k := strings.TrimSpace(h[:i])
			v := strings.TrimSpace(h[i+1:])
			out.header[k] = v
		}
	}
	return out, nil
}

// cdpAcceptKey computes the Sec-WebSocket-Accept value (RFC 6455 §4.2.2).
func cdpAcceptKey(key string) string {
	h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h[:])
}

// call sends a CDP command and waits for its correlated response, bounded by
// ctx. result (when non-nil) receives the "result" member of the response.
func (c *cdpConn) call(ctx context.Context, method string, params, result any) error {
	id := c.nextID.Add(1)
	cmd := struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params,omitempty"`
	}{ID: int(id), Method: method, Params: params}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("cdp: marshal %s: %w", method, err)
	}

	ch := make(chan cdpReply, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("cdp: connection closed")
	}
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.writeFrame(wsOpText, payload); err != nil {
		return fmt.Errorf("cdp: send %s: %w", method, err)
	}

	select {
	case r := <-ch:
		if r.err != nil {
			return fmt.Errorf("cdp: %s: %w", method, r.err)
		}
		if result != nil && len(r.result) > 0 {
			if err := json.Unmarshal(r.result, result); err != nil {
				return fmt.Errorf("cdp: %s: decode result: %w", method, err)
			}
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("cdp: %s: %w", method, ctx.Err())
	}
}

// close idempotently tears the connection down and fails all pending calls.
func (c *cdpConn) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	pending := c.pending
	c.pending = make(map[int64]chan cdpReply)
	c.mu.Unlock()

	_ = c.writeFrame(wsOpClose, nil)
	c.raw.Close()
	for _, ch := range pending {
		ch <- cdpReply{err: fmt.Errorf("cdp: connection closed")}
		close(ch)
	}
}

// readLoop receives frames, reassembles fragmented messages, answers pings,
// and routes responses to their pending call. Events (no id) are dropped —
// none of the commands used here depend on event delivery.
func (c *cdpConn) readLoop() {
	var msg []byte
	var msgOp byte
	for {
		fin, op, payload, err := c.readFrame()
		if err != nil {
			c.failAll(err)
			return
		}
		switch op {
		case wsOpPing:
			_ = c.writeFrame(wsOpPong, payload)
			continue
		case wsOpPong:
			continue
		case wsOpClose:
			c.failAll(io.EOF)
			return
		case wsOpText, wsOpBinary, wsOpContinuation:
			if op != wsOpContinuation {
				msgOp = op
				msg = msg[:0]
			}
			msg = append(msg, payload...)
			if !fin {
				continue
			}
			if msgOp == wsOpText {
				c.dispatch(msg)
			}
			// Binary frames are not part of the CDP message protocol — drop.
		}
	}
}

// dispatch routes one complete JSON message to its pending call.
func (c *cdpConn) dispatch(msg []byte) {
	var env struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &env); err != nil || env.ID == 0 {
		return // event or malformed — dropped
	}
	c.mu.Lock()
	ch, ok := c.pending[int64(env.ID)]
	delete(c.pending, int64(env.ID))
	c.mu.Unlock()
	if !ok {
		return
	}
	if env.Error != nil {
		ch <- cdpReply{err: fmt.Errorf("protocol error %d: %s", env.Error.Code, env.Error.Message)}
	} else {
		ch <- cdpReply{result: env.Result}
	}
}

// failAll unblocks every pending call with err.
func (c *cdpConn) failAll(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	pending := c.pending
	c.pending = make(map[int64]chan cdpReply)
	c.mu.Unlock()
	c.raw.Close()
	for _, ch := range pending {
		ch <- cdpReply{err: fmt.Errorf("cdp: read loop stopped: %w", err)}
		close(ch)
	}
}

// writeFrame writes one client frame (masked, per RFC 6455 §5.3).
func (c *cdpConn) writeFrame(op byte, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var hdr [14]byte
	n := 0
	hdr[0] = 0x80 | op // FIN + opcode
	lenByte := byte(len(payload))
	mask := [4]byte{}
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	switch {
	case len(payload) < 126:
		hdr[1] = 0x80 | lenByte
		n = 2
	case len(payload) <= 0xFFFF:
		hdr[1] = 0x80 | 126
		binary.BigEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		n = 4
	default:
		hdr[1] = 0x80 | 127
		binary.BigEndian.PutUint64(hdr[2:10], uint64(len(payload)))
		n = 10
	}
	copy(hdr[n:], mask[:])
	n += 4

	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	if _, err := c.raw.Write(hdr[:n]); err != nil {
		return err
	}
	if len(masked) > 0 {
		if _, err := c.raw.Write(masked); err != nil {
			return err
		}
	}
	return nil
}

// readFrame reads one frame, unmasking the payload when the server (against
// spec) masks it. Returns fin, opcode, payload.
func (c *cdpConn) readFrame() (bool, byte, []byte, error) {
	var h [2]byte
	if _, err := io.ReadFull(c.br, h[:]); err != nil {
		return false, 0, nil, err
	}
	fin := h[0]&0x80 != 0
	op := h[0] & 0x0F
	masked := h[1]&0x80 != 0
	length := uint64(h[1] & 0x7F)
	switch length {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext[:])
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.br, mask[:]); err != nil {
			return false, 0, nil, err
		}
	}
	if length > 64<<20 { // 64 MiB sanity cap — CDP eval results are far smaller
		return false, 0, nil, fmt.Errorf("cdp: frame too large (%d bytes)", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		return false, 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return fin, op, payload, nil
}

// --- DevTools HTTP endpoints on the same debugging port ----------------------

// devToolsHTTP performs a GET/PUT against the local DevTools HTTP endpoint
// (e.g. /json/list, /json/new). The endpoint is validated local-first.
func devToolsHTTP(ctx context.Context, method, rawURL string) ([]byte, error) {
	u, err := validateLocalHTTPURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cdp: http %s %s: %w", method, u.Path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cdp: http %s %s: status %d", method, u.Path, resp.StatusCode)
	}
	return data, nil
}

// validateLocalHTTPURL is validateLocalWSURL's http:// counterpart.
func validateLocalHTTPURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("cdp: bad http url: %w", err)
	}
	if u.Scheme != "http" {
		return nil, fmt.Errorf("cdp: refusing non-http devtools endpoint %q", raw)
	}
	if !localDebugHosts[u.Hostname()] {
		return nil, fmt.Errorf("cdp: refusing non-local devtools endpoint %q", raw)
	}
	if u.Port() == "" {
		return nil, fmt.Errorf("cdp: devtools endpoint %q has no port", raw)
	}
	return u, nil
}
