// Package debug implements a CDP (Chrome DevTools Protocol) server
// that allows Chrome DevTools to connect to and debug the sandbox.
package debug

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

// CDPServer exposes a Chrome DevTools Protocol interface for the sandbox.
// It provides /json/version and /json/list HTTP endpoints for discovery
// (chrome://inspect), and a WebSocket endpoint for CDP communication.
type CDPServer struct {
	addr     string
	sessions SessionProvider
	mu       sync.Mutex
	clients  map[string]*CDPClient
	dbgState *debuggerState
}

// SessionProvider provides access to sandbox sessions for CDP operations.
type SessionProvider interface {
	Eval(sessionID, code string) (string, error)
	GetSessions() []SessionInfo
}

// SessionInfo describes a session for CDP discovery.
type SessionInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	WebSocketURL string `json:"webSocketDebuggerUrl"`
}

// NewCDPServer creates a CDP server listening on the given address.
func NewCDPServer(addr string, sessions SessionProvider) *CDPServer {
	if addr == "" {
		addr = ":9223"
	}
	return &CDPServer{
		addr:     addr,
		sessions: sessions,
		clients:  make(map[string]*CDPClient),
	}
}

// Start begins listening for CDP connections.
func (s *CDPServer) Start() error {
	http.HandleFunc("/json/version", s.handleVersion)
	http.HandleFunc("/json/list", s.handleList)
	http.HandleFunc("/json/protocol", s.handleProtocol)
	http.HandleFunc("/", s.handleWebSocket)

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	log.Printf("[cdp] listening on %s", s.addr)
	go http.Serve(ln, nil)
	return nil
}

// /json/version — Chrome discovery endpoint
func (s *CDPServer) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"Browser":         "browser-env-sandbox v0.2.0",
		"Protocol-Version": "1.3",
		"User-Agent":       "Mozilla/5.0 (compatible; BES/0.2; V8/9.0)",
		"V8-Version":      "9.0",
		"WebKit-Version":  "537.36",
		"webSocketDebuggerUrl": cdpWebSocketURL(r, s.addr),
	})
}

// /json/list — Lists available debugging targets (sandbox sessions)
func (s *CDPServer) handleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	wsBase := cdpWebSocketURL(r, s.addr)
	sessions := s.sessions.GetSessions()
	targets := make([]map[string]interface{}, 0, len(sessions))
	for _, sess := range sessions {
		wsURL := fmt.Sprintf("%s/devtools/page/%s", wsBase, sess.ID)
		targets = append(targets, map[string]interface{}{
			"id":                     sess.ID,
			"title":                  sess.Title,
			"url":                    sess.URL,
			"type":                   "page",
			"webSocketDebuggerUrl":   wsURL,
			"devtoolsFrontendUrl":    fmt.Sprintf("devtools://devtools/bundled/inspector.html?ws=%s", strings.TrimPrefix(strings.TrimPrefix(wsURL, "ws://"), "wss://")),
		})
	}
	if len(targets) == 0 {
		// Always show at least one target so DevTools can connect
		wsURL := fmt.Sprintf("%s/devtools/page/default", wsBase)
		targets = append(targets, map[string]interface{}{
			"id":                     "default",
			"title":                  "browser-env-sandbox",
			"url":                    "about:blank",
			"type":                   "page",
			"webSocketDebuggerUrl":   wsURL,
			"devtoolsFrontendUrl":    fmt.Sprintf("devtools://devtools/bundled/inspector.html?ws=%s", strings.TrimPrefix(strings.TrimPrefix(wsURL, "ws://"), "wss://")),
		})
	}
	json.NewEncoder(w).Encode(targets)
}

// cdpWebSocketURL builds the advertised WebSocket base URL from the request's
// Host header so the URL is reachable from the client's perspective (behind
// an SSH tunnel the host is 127.0.0.1, not the bind address).
func cdpWebSocketURL(r *http.Request, addr string) string {
	host := r.Host
	if host == "" {
		_, port, _ := net.SplitHostPort(addr)
		host = "127.0.0.1:" + port
	}
	return "ws://" + host
}

func (s *CDPServer) handleProtocol(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Minimal protocol JSON — real CDP protocol is huge, return subset
	json.NewEncoder(w).Encode(map[string]interface{}{
		"domains": []string{"Runtime", "Network", "Console", "Debugger", "Page"},
	})
}

// CDPClient represents a connected DevTools client.
type CDPClient struct {
	conn     net.Conn
	mu       sync.Mutex // protects conn writes (per-client write-frame lock) — see writeFrame
	br       *bufio.Reader
	bw       *bufio.Writer
	requests chan CDPRequest
	targetID string // sandbox session this client attaches to
}

// CDPRequest is a CDP protocol request.
type CDPRequest struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// CDPResponse is a CDP protocol response.
type CDPResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *CDPError       `json:"error,omitempty"`
}

// CDPError is a CDP protocol error.
type CDPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CDPEvent is a CDP protocol event (server → client).
type CDPEvent struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}


