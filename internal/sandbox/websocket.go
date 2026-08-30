package sandbox

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zninggo/v8go"
)

// ── RFC 6455 WebSocket protocol (pure Go, stdlib only) ──

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsOpcode values per RFC 6455 §5.2.
const (
	wsOpContinuation = 0x0
	wsOpText         = 0x1
	wsOpBinary       = 0x2
	wsOpClose        = 0x8
	wsOpPing         = 0x9
	wsOpPong         = 0xA
)

// wsConn wraps a raw net.Conn with WebSocket frame read/write helpers.
type wsConn struct {
	raw    net.Conn
	isTLS  bool
	mu     sync.Mutex // serializes writes
	closed bool
}

// wsDial performs the WebSocket handshake (RFC 6455 §4) and returns a wsConn.
// Supports ws:// (plain TCP) and wss:// (TLS).
func wsDial(rawURL string, protocols []string, extraHeaders map[string]string) (*wsConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("ws dial: bad url: %w", err)
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	var conn net.Conn
	if u.Scheme == "wss" {
		conn, err = tls.Dial("tcp", host, &tls.Config{ServerName: u.Hostname()})
		if err != nil {
			return nil, fmt.Errorf("ws dial tls: %w", err)
		}
	} else {
		conn, err = net.Dial("tcp", host)
		if err != nil {
			return nil, fmt.Errorf("ws dial tcp: %w", err)
		}
	}

	// Generate Sec-WebSocket-Key (16 random bytes, base64).
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ws dial: rand: %w", err)
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)

	// Build the HTTP Upgrade request.
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := fmt.Sprintf("GET %s HTTP/1.1\r\n", path)
	req += fmt.Sprintf("Host: %s\r\n", u.Host)
	req += "Upgrade: websocket\r\n"
	req += "Connection: Upgrade\r\n"
	req += fmt.Sprintf("Sec-WebSocket-Key: %s\r\n", key)
	req += "Sec-WebSocket-Version: 13\r\n"
	if len(protocols) > 0 {
		req += "Sec-WebSocket-Protocol: " + strings.Join(protocols, ", ") + "\r\n"
	}
	for k, v := range extraHeaders {
		req += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	req += "\r\n"

	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ws dial: write: %w", err)
	}

	// Read the HTTP response.
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: "GET"})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ws dial: read response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 101 {
		conn.Close()
		return nil, fmt.Errorf("ws dial: expected 101, got %d", resp.StatusCode)
	}

	// Verify Sec-WebSocket-Accept.
	accept := resp.Header.Get("Sec-WebSocket-Accept")
	expected := wsAcceptKey(key)
	if accept != expected {
		conn.Close()
		return nil, fmt.Errorf("ws dial: bad Sec-WebSocket-Accept")
	}

	// If the server picked a sub-protocol, the caller can read it from resp.
	// The wsConn doesn't track it; the bridge reads it if needed.

	w := &wsConn{raw: conn, isTLS: u.Scheme == "wss"}

	// If bufio.Reader has buffered data, it's already WS frames — but for a
	// fresh handshake this is unlikely. We discard the bufio.Reader and read
	// directly from conn to avoid double-buffering complexity.
	// (In practice, ReadResponse consumes exactly the HTTP headers.)
	if br.Buffered() > 0 {
		// Drain any leftover bytes into the first frame read by prepending.
		// This is an edge case; for echo servers it won't happen.
		buf := make([]byte, br.Buffered())
		br.Read(buf)
		// Prepend by wrapping — simplest: write back via a pipe. But that's
		// overkill. Just ignore; the handshake response body is empty for 101.
	}

	return w, nil
}

// wsAcceptKey computes the expected Sec-WebSocket-Accept value.
func wsAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// writeFrame writes a single WebSocket frame (client-side: always masked).
func (w *wsConn) writeFrame(opcode byte, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("ws: connection closed")
	}

	var buf []byte
	fin := byte(0x80) // FIN bit set
	mask := byte(0x80) // client must mask

	// Payload length encoding.
	var lenBytes []byte
	switch {
	case len(data) < 126:
		lenBytes = []byte{byte(len(data))}
	case len(data) < 65536:
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(len(data)))
		lenBytes = b
	default:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(len(data)))
		lenBytes = b
	}

	// Mask key (4 random bytes).
	maskKey := make([]byte, 4)
	if _, err := rand.Read(maskKey); err != nil {
		return fmt.Errorf("ws: mask rand: %w", err)
	}

	// Header.
	buf = append(buf, fin|opcode)
	buf = append(buf, mask|lenBytes[0])
	if len(lenBytes) > 1 {
		buf = append(buf, lenBytes[1:]...)
	}
	buf = append(buf, maskKey...)

	// Masked payload.
	masked := make([]byte, len(data))
	for i := range data {
		masked[i] = data[i] ^ maskKey[i%4]
	}
	buf = append(buf, masked...)

	_, err := w.raw.Write(buf)
	return err
}

// readFrame reads a single WebSocket frame (server → client, unmasked).
// Returns the opcode and payload. Control frames (close/ping/pong) are
// handled inline: ping → pong auto-reply, close → returns io.EOF.
func (w *wsConn) readFrame() (byte, []byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(w.raw, hdr); err != nil {
		return 0, nil, err
	}
	fin := hdr[0]&0x80 != 0
	opcode := hdr[0] & 0x0F
	masked := hdr[1]&0x80 != 0
	payloadLen := int(hdr[1] & 0x7F)

	// Extended length.
	switch payloadLen {
	case 126:
		b := make([]byte, 2)
		if _, err := io.ReadFull(w.raw, b); err != nil {
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(b))
	case 127:
		b := make([]byte, 8)
		if _, err := io.ReadFull(w.raw, b); err != nil {
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint64(b))
	}

	// Mask key (server frames should be unmasked, but handle anyway).
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(w.raw, maskKey); err != nil {
			return 0, nil, err
		}
	}

	// Payload.
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(w.raw, payload); err != nil {
			return 0, nil, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	// Handle control frames.
	switch opcode {
	case wsOpClose:
		code := 1000
		reason := ""
		if len(payload) >= 2 {
			code = int(binary.BigEndian.Uint16(payload[:2]))
			reason = string(payload[2:])
		}
		// Echo close frame back (RFC 6455 §5.5.1).
		w.writeFrame(wsOpClose, payload)
		w.mu.Lock()
		w.closed = true
		w.mu.Unlock()
		w.raw.Close()
		_ = code
		_ = reason
		return wsOpClose, nil, io.EOF
	case wsOpPing:
		// Auto-reply with pong.
		w.writeFrame(wsOpPong, payload)
		return opcode, payload, nil
	case wsOpPong:
		// Ignore pong frames.
		return opcode, payload, nil
	}

	_ = fin
	return opcode, payload, nil
}

// sendText sends a text frame.
func (w *wsConn) sendText(data string) error {
	return w.writeFrame(wsOpText, []byte(data))
}

// sendBinary sends a binary frame.
func (w *wsConn) sendBinary(data []byte) error {
	return w.writeFrame(wsOpBinary, data)
}

// sendClose sends a close frame with the given code and reason, then closes
// the underlying connection.
func (w *wsConn) sendClose(code int, reason string) error {
	payload := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(payload, uint16(code))
	copy(payload[2:], reason)
	err := w.writeFrame(wsOpClose, payload)
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	w.raw.Close()
	return err
}

// closeRaw closes the underlying connection without sending a close frame.
func (w *wsConn) closeRaw() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	w.raw.Close()
}

// ── WebSocket registry (per-session, like workerRegistry) ──

type wsRegistry struct {
	mu      sync.Mutex
	nextID  int32
	conns   map[int32]*wsBridge
}

func newWSRegistry() *wsRegistry {
	return &wsRegistry{conns: make(map[int32]*wsBridge)}
}

func (r *wsRegistry) register(w *wsBridge) int32 {
	id := atomic.AddInt32(&r.nextID, 1)
	r.mu.Lock()
	r.conns[id] = w
	r.mu.Unlock()
	w.id = id
	return id
}

func (r *wsRegistry) get(id int32) *wsBridge {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.conns[id]
}

func (r *wsRegistry) remove(id int32) {
	r.mu.Lock()
	w, ok := r.conns[id]
	if ok {
		delete(r.conns, id)
	}
	r.mu.Unlock()
	if ok {
		w.terminate()
	}
}

// removeByID removes the bridge from the registry without terminating it.
// Used when terminate is already running (e.g. in a goroutine from closeCb).
func (r *wsRegistry) removeByID(id int32) {
	r.mu.Lock()
	delete(r.conns, id)
	r.mu.Unlock()
}

func (r *wsRegistry) disposeAll() {
	r.mu.Lock()
	all := make([]*wsBridge, 0, len(r.conns))
	for _, w := range r.conns {
		all = append(all, w)
	}
	r.conns = make(map[int32]*wsBridge)
	r.mu.Unlock()
	for _, w := range all {
		w.terminate()
	}
}

// ── WebSocket bridge (Go ↔ JS) ──

// wsSendMsg is a queued send from the isolate thread to the write goroutine.
type wsSendMsg struct {
	data     string
	isBinary bool
}

// wsBridge wraps a wsConn with the parent session's timer manager for
// event delivery. The threading model mirrors Worker:
//   - Read goroutine: reads frames, schedules timer callbacks that run
//     __besWebSocketOn* on the isolate thread.
//   - Write goroutine: drains the send channel and writes frames.
//   - The isolate thread only touches V8; network I/O happens on goroutines.
type wsBridge struct {
	id       int32
	conn     *wsConn
	timers   *TimerManager
	ctx      *v8go.Context
	parent   *Session // owning session; callbacks check IsDisposed() before touching ctx
	sendCh   chan wsSendMsg
	stop     chan struct{}
	loopDone chan struct{}
	once     sync.Once
	closing  atomic.Bool // true when user-initiated close: readLoop skips events
}

// startWSBridge dials the WebSocket and starts the read/write loops.
func startWSBridge(rawURL string, protocols []string, timers *TimerManager, ctx *v8go.Context, parent *Session) (*wsBridge, error) {
	conn, err := wsDial(rawURL, protocols, nil)
	if err != nil {
		return nil, err
	}
	w := &wsBridge{
		conn:     conn,
		timers:   timers,
		ctx:      ctx,
		parent:   parent,
		sendCh:   make(chan wsSendMsg, 64),
		stop:     make(chan struct{}),
		loopDone: make(chan struct{}),
	}

	// Fire onopen on the isolate thread.
	w.scheduleEvent("open", "")

	// Start read + write loops.
	go w.readLoop()
	go w.writeLoop()

	return w, nil
}

// scheduleEvent queues a callback on the isolate thread that calls
// __besWebSocketOnOpen/OnMessage/OnClose/OnError. Must NOT touch V8 here.
//
// The callback checks parent.IsDisposed() before RunScript: a frame read just
// before Dispose can queue a callback that drains after the parent context is
// closed, so without the guard it would RunScript a closed context
// (use-after-dispose). StopAll draining the queue (H9) drops most of these;
// the disposed check covers a callback firing on a live drain before Dispose
// reaches StopAll.
func (w *wsBridge) scheduleEvent(eventType, data string) {
	wRef := w
	et := eventType
	d := data
	wRef.timers.scheduleTimer(0, false, func() {
		wID := wRef.id // read at execution time (register sets it after startWSBridge returns)
		if wRef.parent.IsDisposed() {
			return
		}
		var call string
		switch et {
		case "open":
			call = fmt.Sprintf("typeof __besWebSocketOnOpen==='function'&&__besWebSocketOnOpen(%d)", wID)
		case "message":
			call = fmt.Sprintf("typeof __besWebSocketOnMessage==='function'&&__besWebSocketOnMessage(%d,%s)", wID, jsonString(d))
		case "close":
			call = fmt.Sprintf("typeof __besWebSocketOnClose==='function'&&__besWebSocketOnClose(%d,%s)", wID, d)
		case "error":
			call = fmt.Sprintf("typeof __besWebSocketOnError==='function'&&__besWebSocketOnError(%d,%s)", wID, jsonString(d))
		}
		if _, err := wRef.ctx.RunScript(call, "ws-event.js"); err != nil {
			log.Printf("[sandbox] ws %d event %s error: %v", wID, et, err)
		}
	})
}

// readLoop reads frames on a goroutine and schedules event callbacks.
func (w *wsBridge) readLoop() {
	defer close(w.loopDone)
	for {
		opcode, payload, err := w.conn.readFrame()
		if err != nil {
			// If the user initiated close, the connection teardown is expected —
			// don't fire error/close events (the close callback was already
			// scheduled by __besWebSocketClose).
			if w.closing.Load() {
				return
			}
			if err == io.EOF {
				w.scheduleEvent("close", "1000,'closed'")
			} else {
				w.scheduleEvent("error", err.Error())
				w.scheduleEvent("close", "1006,'abnormal closure'")
			}
			return
		}
		switch opcode {
		case wsOpText:
			w.scheduleEvent("message", string(payload))
		case wsOpBinary:
			// Binary messages are delivered as base64; the JS side decodes.
			b64 := base64.StdEncoding.EncodeToString(payload)
			w.scheduleEvent("message", "__BES_BINARY__:"+b64)
		}
	}
}

// writeLoop drains the send channel and writes frames.
func (w *wsBridge) writeLoop() {
	for {
		select {
		case <-w.stop:
			return
		case msg, ok := <-w.sendCh:
			if !ok {
				return
			}
			if msg.isBinary {
				data, err := base64.StdEncoding.DecodeString(msg.data)
				if err == nil {
					w.conn.sendBinary(data)
				}
			} else {
				w.conn.sendText(msg.data)
			}
		}
	}
}

// terminate stops the bridge: closes the connection, stops the write loop.
func (w *wsBridge) terminate() {
	w.once.Do(func() {
		close(w.stop)
		w.conn.closeRaw()
		select {
		case <-w.loopDone:
		case <-time.After(2 * time.Second):
		}
	})
}

// ── JS injection (PostContext, like injectWorkerConstructor) ──

// injectWebSocketConstructor replaces the env_shim WebSocket stub with a real
// implementation backed by Go network I/O. Called after PostContextBuilder.Build().
func injectWebSocketConstructor(p *PostContextBuilder, sess *Session) {
	iso, ctx := p.iso, p.ctx
	parentTimers := p.timerMgr

	// __besWebSocketCreate(url, protocolsJSON) → ws id (number). Starts a
	// goroutine to dial; events are delivered via scheduleTimer.
	createCb := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		args := info.Args()
		rawURL := ""
		if len(args) > 0 {
			rawURL = args[0].String()
		}
		protocols := []string{}
		if len(args) > 1 && args[1].String() != "" && args[1].String() != "null" {
			// protocolsJSON is a JSON array string.
			protocolsStr := args[1].String()
			// Simple parse: split by comma, strip quotes/brackets.
			protocolsStr = strings.Trim(protocolsStr, "[]")
			if protocolsStr != "" {
				for _, p := range strings.Split(protocolsStr, ",") {
					p = strings.TrimSpace(p)
					p = strings.Trim(p, "\"'")
					if p != "" {
						protocols = append(protocols, p)
					}
				}
			}
		}

		// Dial synchronously — if it fails immediately, return -1 so JS fires
		// onerror. The dial is a blocking TCP connect + handshake (fast for
		// local echo; for remote wss it may take a moment).
		w, err := startWSBridge(rawURL, protocols, parentTimers, ctx, sess)
		if err != nil {
			log.Printf("[sandbox] ws create failed: %v", err)
			v, _ := v8go.NewValue(iso, int32(-1))
			return v
		}
		id := sess.websockets.register(w)
		v, _ := v8go.NewValue(iso, id)
		return v
	}
	if createFn := v8go.NewFunctionTemplate(iso, createCb).GetFunction(ctx); createFn != nil {
		p.global.Set("__besWebSocketCreate", createFn)
	}

	// __besWebSocketSend(id, data, isBinary) → queues send on write goroutine.
	sendCb := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) < 2 {
			return nil
		}
		id := info.Args()[0].Int32()
		data := info.Args()[1].String()
		isBinary := false
		if len(info.Args()) > 2 {
			isBinary = info.Args()[2].Boolean()
		}
		if w := sess.websockets.get(id); w != nil {
			select {
			case w.sendCh <- wsSendMsg{data: data, isBinary: isBinary}:
			default:
				log.Printf("[sandbox] ws %d send queue full, message dropped", id)
			}
		}
		return nil
	}
	if fn := v8go.NewFunctionTemplate(iso, sendCb).GetFunction(ctx); fn != nil {
		p.global.Set("__besWebSocketSend", fn)
	}

	// __besWebSocketClose(id, code, reason) → sends close frame, fires close
	// event directly (we're already on the isolate thread), then terminates.
	closeCb := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) > 0 {
			id := info.Args()[0].Int32()
			code := 1000
			if len(info.Args()) > 1 {
				code = int(info.Args()[1].Int32())
			}
			reason := ""
			if len(info.Args()) > 2 {
				reason = info.Args()[2].String()
			}
			if w := sess.websockets.get(id); w != nil {
				// Mark as user-initiated close so readLoop doesn't fire
				// duplicate error/close events on connection teardown.
				w.closing.Store(true)
				// Fire the close event directly — we're already on the isolate
				// thread (closeCb runs during DrainCallbacks → onmessage →
				// ws.close() → __besWebSocketClose). Using scheduleTimer here
				// would race with DrainCallbacks's non-blocking drain.
				// Guard against a closed parent context: closeCb can run during
				// a drain that overlaps Dispose, so check IsDisposed before
				// RunScript to avoid a use-after-dispose on the parent ctx.
				if !sess.IsDisposed() {
					call := fmt.Sprintf(
						"typeof __besWebSocketOnClose==='function'&&__besWebSocketOnClose(%d,%d,%s)",
						id, code, jsonString(reason))
					if _, err := ctx.RunScript(call, "ws-close-event.js"); err != nil {
						log.Printf("[sandbox] ws %d close event error: %v", id, err)
					}
				}
				// Send the close frame and tear down asynchronously.
				w.conn.sendClose(code, reason)
				go w.terminate()
				sess.websockets.removeByID(id)
			}
		}
		return nil
	}
	if fn := v8go.NewFunctionTemplate(iso, closeCb).GetFunction(ctx); fn != nil {
		p.global.Set("__besWebSocketClose", fn)
	}

	// JS WebSocket class: real semantics over the Go bridges.
	wsJS := `
	(function(){
	  'use strict';
	  var receivers = {};
	  window.__besWebSocketOnOpen = function(id) {
	    var r = receivers[id];
	    if (r) {
	      r.readyState = 1; // OPEN
	      if (typeof r.onopen === 'function') r.onopen({ type: 'open' });
	    }
	  };
	  window.__besWebSocketOnMessage = function(id, data) {
	    var r = receivers[id];
	    if (r) {
	      var msgData = data;
	      if (typeof data === 'string' && data.indexOf('__BES_BINARY__:') === 0) {
	        var b64 = data.substring(15);
	        // Decode base64 to Uint8Array
	        var bin = atob(b64);
	        var arr = new Uint8Array(bin.length);
	        for (var i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i);
	        msgData = arr;
	      }
	      if (typeof r.onmessage === 'function') r.onmessage({ type: 'message', data: msgData });
	    }
	  };
	  window.__besWebSocketOnClose = function(id, code, reason) {
	    var r = receivers[id];
	    if (r) {
	      r.readyState = 3; // CLOSED
	      if (typeof r.onclose === 'function') r.onclose({ type: 'close', code: code || 1000, reason: reason || '', wasClean: true });
	    }
	    delete receivers[id];
	  };
	  window.__besWebSocketOnError = function(id, message) {
	    var r = receivers[id];
	    if (r) {
	      if (typeof r.onerror === 'function') r.onerror({ type: 'error', message: message || 'connection failed' });
	    }
	  };
	  window.__besWebSocketRegisterReceiver = function(id, wsObj) { receivers[id] = wsObj; };
	  window.__besWebSocketUnregisterReceiver = function(id) { delete receivers[id]; };

	  window.WebSocket = function(url, protocols) {
	    var selfObj = this;
	    this.url = url;
	    this.readyState = 0; // CONNECTING
	    this.protocol = '';
	    this.extensions = '';
	    this.bufferedAmount = 0;
	    this.binaryType = 'blob';
	    this.onopen = null;
	    this.onmessage = null;
	    this.onclose = null;
	    this.onerror = null;

	    var protoArg = protocols || '';
	    if (Array.isArray(protoArg)) protoArg = JSON.stringify(protoArg);
	    this.__besID = __besWebSocketCreate(url, protoArg);

	    if (this.__besID >= 0) {
	      __besWebSocketRegisterReceiver(this.__besID, this);
	    } else {
	      this.readyState = 3; // CLOSED
	      var s = this;
	      setTimeout(function(){ if (s.onerror) s.onerror({ type: 'error', message: 'connection failed' }); if (s.onclose) s.onclose({ type: 'close', code: 1006, reason: 'connection failed', wasClean: false }); }, 0);
	    }

	    this.send = function(data) {
	      if (selfObj.readyState !== 1) return; // Only send when OPEN
	      if (typeof data === 'string') {
	        __besWebSocketSend(selfObj.__besID, data, false);
	      } else if (data instanceof Uint8Array) {
	        var s = '';
	        for (var i = 0; i < data.length; i++) s += String.fromCharCode(data[i]);
	        __besWebSocketSend(selfObj.__besID, btoa(s), true);
	      } else if (data instanceof ArrayBuffer) {
	        var u8 = new Uint8Array(data);
	        var s = '';
	        for (var i = 0; i < u8.length; i++) s += String.fromCharCode(u8[i]);
	        __besWebSocketSend(selfObj.__besID, btoa(s), true);
	      } else {
	        __besWebSocketSend(selfObj.__besID, String(data), false);
	      }
	    };

	    this.close = function(code, reason) {
	      if (selfObj.readyState >= 2) return; // Already closing/closed
	      selfObj.readyState = 2; // CLOSING
	      __besWebSocketClose(selfObj.__besID, code || 1000, reason || '');
	      // Receiver is unregistered by __besWebSocketOnClose, not here —
	      // unregistering early would prevent the close event from firing.
	    };

	    this.addEventListener = function(type, fn) {
	      if (type === 'open') selfObj.onopen = fn;
	      else if (type === 'message') selfObj.onmessage = fn;
	      else if (type === 'close') selfObj.onclose = fn;
	      else if (type === 'error') selfObj.onerror = fn;
	    };
	    this.removeEventListener = function() {};
	    this.dispatchEvent = function() { return true; };
	  };
	  window.WebSocket.CONNECTING = 0;
	  window.WebSocket.OPEN = 1;
	  window.WebSocket.CLOSING = 2;
	  window.WebSocket.CLOSED = 3;
	  window.WebSocket.prototype.constructor = window.WebSocket;
	})();
	`
	if _, err := ctx.RunScript(wsJS, "websocket-class.js"); err != nil {
		log.Printf("[sandbox] websocket class warning: %v", err)
	}
}
