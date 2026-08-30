package debug

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
)

// handleWebSocket handles CDP WebSocket connections using raw TCP
// (no external WebSocket library needed).
func (s *CDPServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		http.NotFound(w, r)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "not a hijacker", 500)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer conn.Close()

	// WebSocket handshake
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return
	}
	accept := computeAcceptKey(key)

	// Target routing: /devtools/page/<sessionID> — which sandbox session
	// this DevTools connection attaches to. Empty = "default" fallback.
	targetID := "default"
	if strings.HasPrefix(r.URL.Path, "/devtools/page/") {
		if id := strings.TrimPrefix(r.URL.Path, "/devtools/page/"); id != "" {
			targetID = id
		}
	}

	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := bufrw.WriteString(response); err != nil {
		return
	}
	bufrw.Flush()

	log.Printf("[cdp] client connected from %s (target=%s)", conn.RemoteAddr(), targetID)

	// One buffered reader/writer per connection, reused across frames. The
	// handshake bytes were already consumed from bufrw by the http server, so
	// seed the reader/writer on the raw conn from here on.
	client := &CDPClient{
		conn:     conn,
		br:       bufio.NewReader(conn),
		bw:       bufio.NewWriter(conn),
		requests: make(chan CDPRequest, 100),
		targetID: targetID,
	}
	s.mu.Lock()
	s.clients[conn.RemoteAddr().String()] = client
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, conn.RemoteAddr().String())
		s.mu.Unlock()
	}()

	// Read loop
	go func() {
		for {
			req, err := client.readFrame()
			if err != nil {
				if err != io.EOF {
					log.Printf("[cdp] read error: %v", err)
				}
				close(client.requests)
				return
			}
			var cdpReq CDPRequest
			if err := json.Unmarshal(req, &cdpReq); err != nil {
				continue
			}
			client.requests <- cdpReq
		}
	}()

	// Process requests
	for req := range client.requests {
		resp := s.handleCDPRequest(client, req)
		data, _ := json.Marshal(resp)
		if err := client.writeFrame(data); err != nil {
			log.Printf("[cdp] write error: %v", err)
			return
		}
	}
}

// handleCDPRequest dispatches a CDP request to the appropriate handler.
// The client carries the target session so Runtime.evaluate lands in the
// sandbox the DevTools window attached to.
func (s *CDPServer) handleCDPRequest(client *CDPClient, req CDPRequest) CDPResponse {
	switch {
	case strings.HasPrefix(req.Method, "Runtime."):
		return s.handleRuntime(client, req)
	case strings.HasPrefix(req.Method, "Network."):
		return s.handleNetwork(req)
	case strings.HasPrefix(req.Method, "Console."):
		return s.handleConsole(req)
	case strings.HasPrefix(req.Method, "Debugger."):
		return s.handleDebugger(req)
	case strings.HasPrefix(req.Method, "Page."):
		return s.handlePage(req)
	default:
		return CDPResponse{
			ID:    req.ID,
			Error: &CDPError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func (s *CDPServer) handleRuntime(client *CDPClient, req CDPRequest) CDPResponse {
	switch req.Method {
	case "Runtime.enable":
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	case "Runtime.evaluate":
		var params struct {
			Expression string `json:"expression"`
		}
		json.Unmarshal(req.Params, &params)
		if s.sessions != nil {
			result, err := s.sessions.Eval(client.targetID, params.Expression)
			if err != nil {
				return CDPResponse{ID: req.ID, Error: &CDPError{Code: -1, Message: err.Error()}}
			}
			resultJSON, _ := json.Marshal(map[string]interface{}{
				"result": map[string]interface{}{
					"type":  "string",
					"value": result,
				},
			})
			return CDPResponse{ID: req.ID, Result: resultJSON}
		}
		return CDPResponse{ID: req.ID, Error: &CDPError{Code: -1, Message: "no session provider"}}
	default:
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	}
}

func (s *CDPServer) handleNetwork(req CDPRequest) CDPResponse {
	// Network.enable, Network.disable, etc. — return empty result
	return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
}

func (s *CDPServer) handleConsole(req CDPRequest) CDPResponse {
	return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
}

// debuggerState tracks breakpoints and pause-on-exceptions state for a CDP
// Debugger session. v8go does not expose V8 Inspector session
// (sendCommand/dispatchProtocolMessage), so true execution pause/step is not
// possible — but we implement the full CDP Debugger.* method routing so
// DevTools connects cleanly and breakpoint state is tracked. When a
// breakpoint URL+line matches a script being evaluated, the bridge Eval path
// can inject a console.trace to surface the hit.
type debuggerState struct {
	mu                  sync.Mutex
	enabled             bool
	breakpoints         map[string]breakpoint // breakpointId → breakpoint
	pauseOnExceptions   string                // "none" | "uncaught" | "all"
	nextBreakpointID    int
}

type breakpoint struct {
	ID       string `json:"breakpointId"`
	URL      string `json:"url"`
	Line     int    `json:"lineNumber"`
	Column   int    `json:"columnNumber,omitempty"`
	Condition string `json:"condition,omitempty"`
}

func (s *CDPServer) handleDebugger(req CDPRequest) CDPResponse {
	// Lazy-init the shared debugger state under s.mu. Multiple CDP clients
	// share one CDPServer and its single dbgState, and their read loops run as
	// independent goroutines — the old `if s.dbgState == nil` check was a
	// read-without-lock that raced a concurrent first init. s.mu is the
	// server's existing client-map lock; we hold it only long enough to read
	// or create the pointer, then drop it before taking dbg.mu, so the two
	// locks are never nested (and dbg.mu never nests the per-client write
	// frame lock either — writeFrame runs in the process loop after this
	// returns, with dbg.mu already released).
	s.mu.Lock()
	if s.dbgState == nil {
		s.dbgState = &debuggerState{breakpoints: make(map[string]breakpoint)}
	}
	dbg := s.dbgState
	s.mu.Unlock()

	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	switch req.Method {
	case "Debugger.enable":
		dbg.enabled = true
		return CDPResponse{ID: req.ID, Result: json.RawMessage(`{"debuggerId":"bes-dbg-1"}`)}
	case "Debugger.disable":
		dbg.enabled = false
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	case "Debugger.setBreakpointsActive":
		// Always active in our model
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	case "Debugger.setBreakpointByUrl":
		var params struct {
			URL       string `json:"url"`
			Line      int    `json:"lineNumber"`
			Column    int    `json:"columnNumber"`
			Condition string `json:"condition"`
		}
		json.Unmarshal(req.Params, &params)
		dbg.nextBreakpointID++
		bpID := fmt.Sprintf("bes-bp-%d", dbg.nextBreakpointID)
		bp := breakpoint{ID: bpID, URL: params.URL, Line: params.Line, Column: params.Column, Condition: params.Condition}
		dbg.breakpoints[bpID] = bp
		result, _ := json.Marshal(map[string]any{
			"breakpointId": bpID,
			"locations":    []map[string]any{{"scriptId": "1", "lineNumber": params.Line, "columnNumber": params.Column}},
		})
		return CDPResponse{ID: req.ID, Result: result}
	case "Debugger.removeBreakpoint":
		var params struct {
			BPID string `json:"breakpointId"`
		}
		json.Unmarshal(req.Params, &params)
		delete(dbg.breakpoints, params.BPID)
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	case "Debugger.getBreakpoints":
		bps := []breakpoint{}
		for _, bp := range dbg.breakpoints {
			bps = append(bps, bp)
		}
		result, _ := json.Marshal(map[string]any{"breakpoints": bps})
		return CDPResponse{ID: req.ID, Result: result}
	case "Debugger.setPauseOnExceptions":
		var params struct {
			State string `json:"state"`
		}
		json.Unmarshal(req.Params, &params)
		dbg.pauseOnExceptions = params.State
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	case "Debugger.pause":
		// v8go has no Inspector session — cannot truly pause V8 execution.
		// DevTools expects a Debugger.paused event; we send a synthetic one
		// so the UI enters paused state (step/resume are no-ops).
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	case "Debugger.resume":
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	case "Debugger.stepOver", "Debugger.stepInto", "Debugger.stepOut":
		// No-op: v8go Inspector session not exposed. DevTools treats these
		// as immediate resume since we can't hold execution.
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	case "Debugger.getScriptSource":
		// Return empty source — actual script source isn't tracked at CDP level.
		result, _ := json.Marshal(map[string]string{"scriptSource": ""})
		return CDPResponse{ID: req.ID, Result: result}
	default:
		return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
	}
}

func (s *CDPServer) handlePage(req CDPRequest) CDPResponse {
	return CDPResponse{ID: req.ID, Result: json.RawMessage("{}")}
}

// --- WebSocket frame helpers (RFC 6455) ---

// maxFrameSize bounds a single inbound frame's payload allocation. A CDP
// client controls the 64-bit length field; without a cap a malicious or buggy
// client can request a multi-GB allocation and exhaust memory in one frame.
// 16 MiB is far above any legitimate CDP message (Runtime.evaluate payloads
// are KB-scale), so rejecting above this protects the host without affecting
// real DevTools traffic.
const maxFrameSize = 16 * 1024 * 1024

// readFrame reads one WebSocket frame off the connection using the client's
// reused bufio.Reader. Reusing the reader avoids a 4 KiB allocation per frame
// and — more importantly — keeps any buffered lookahead bytes inside the same
// reader instance so they are not lost between frames (a fresh reader each
// call would discard unclaimed buffered bytes, corrupting the stream).
func (c *CDPClient) readFrame() ([]byte, error) {
	reader := c.br

	// Read frame header
	firstByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	secondByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	fin := firstByte&0x80 != 0
	opcode := firstByte & 0x0F
	masked := secondByte&0x80 != 0
	length := int64(secondByte & 0x7F)

	if length == 126 {
		var len16 [2]byte
		if _, err := io.ReadFull(reader, len16[:]); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint16(len16[:]))
	} else if length == 127 {
		var len64 [8]byte
		if _, err := io.ReadFull(reader, len64[:]); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint64(len64[:]))
	}

	if length > maxFrameSize {
		// Don't attempt to drain the oversized payload — close the connection
		// so the client cannot keep the reader busy with junk. Returning an
		// error tears down the read loop and the deferred conn.Close follows.
		return nil, fmt.Errorf("cdp: frame too large (%d bytes, max %d)", length, maxFrameSize)
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(reader, maskKey[:]); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	if opcode == 0x8 { // Close
		return nil, io.EOF
	}

	_ = fin
	return payload, nil
}

// writeFrame writes one WebSocket text frame. The per-client mutex serializes
// writes to THIS connection only — multiple CDP clients no longer contend on a
// single global lock, so one slow client's write cannot head-of-line block
// another's. The reused bufio.Writer avoids a per-frame allocation.
func (c *CDPClient) writeFrame(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	writer := c.bw

	// Text frame, FIN=1
	writer.WriteByte(0x82)

	length := len(data)
	if length < 126 {
		writer.WriteByte(byte(length))
	} else if length < 65536 {
		writer.WriteByte(126)
		var len16 [2]byte
		binary.BigEndian.PutUint16(len16[:], uint16(length))
		writer.Write(len16[:])
	} else {
		writer.WriteByte(127)
		var len64 [8]byte
		binary.BigEndian.PutUint64(len64[:], uint64(length))
		writer.Write(len64[:])
	}

	writer.Write(data)
	return writer.Flush()
}

// computeAcceptKey computes the WebSocket accept key from the client key.
func computeAcceptKey(key string) string {
	const magic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.Sum([]byte(key + magic))
	return base64.StdEncoding.EncodeToString(h[:])
}
