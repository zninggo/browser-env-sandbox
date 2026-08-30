// Package mcp provides a Model Context Protocol (MCP) server adapter
// for browser-env-sandbox, allowing AI agents (Claude, Cursor, etc.)
// to directly create sandbox sessions and execute JS.
//
// MCP is a JSON-RPC 2.0 protocol over stdio. This adapter exposes
// sandbox operations as MCP tools.
package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
	"github.com/zninggo/browser-env-sandbox/internal/sandbox"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// MCPServer reads MCP requests from stdin and writes responses to stdout.
type MCPServer struct {
	fpEngine *fpengine.Engine
	sbEngine *sandbox.Engine
	mu       sync.Mutex
	sessions map[string]*sandbox.Session
}

// New creates an MCP server.
func New() *MCPServer {
	fpEng := fpengine.New()
	return &MCPServer{
		fpEngine: fpEng,
		sbEngine: sandbox.New(fpEng, 4),
		sessions: make(map[string]*sandbox.Session),
	}
}

// Run starts the MCP server, reading from stdin and writing to stdout.
func (s *MCPServer) Run() error {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req MCPRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			continue
		}

		resp := s.handle(req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("[mcp] encode error: %v", err)
		}
	}
}

// MCPRequest is a JSON-RPC 2.0 request.
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse is a JSON-RPC 2.0 response.
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError is a JSON-RPC error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *MCPServer) handle(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolCall(req)
	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func (s *MCPServer) handleInitialize(req MCPRequest) MCPResponse {
	result, _ := json.Marshal(map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "browser-env-sandbox",
			"version": "0.2.0",
		},
	})
	return MCPResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *MCPServer) handleToolsList(req MCPRequest) MCPResponse {
	tools := []map[string]interface{}{
		{
			"name": "create_session",
			"description": "Create a new browser sandbox session with a random self-consistent fingerprint",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"browser":  map[string]interface{}{"type": "string", "default": "chrome"},
					"os":       map[string]interface{}{"type": "string", "default": "windows"},
					"location": map[string]interface{}{"type": "string", "default": "https://example.com/"},
				},
			},
		},
		{
			"name": "eval",
			"description": "Execute JavaScript in a sandbox session",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{"type": "string"},
					"code":       map[string]interface{}{"type": "string"},
				},
				"required": []string{"session_id", "code"},
			},
		},
		{
			"name": "load_script",
			"description": "Load and execute a JS script in a session",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{"type": "string"},
					"name":       map[string]interface{}{"type": "string"},
					"content":    map[string]interface{}{"type": "string"},
				},
				"required": []string{"session_id", "name", "content"},
			},
		},
		{
			"name": "get_fingerprint",
			"description": "Get the fingerprint of a session",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{"type": "string"},
				},
				"required": []string{"session_id"},
			},
		},
		{
			"name": "close_session",
			"description": "Close a sandbox session",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{"type": "string"},
				},
				"required": []string{"session_id"},
			},
		},
	}
	result, _ := json.Marshal(map[string]interface{}{"tools": tools})
	return MCPResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *MCPServer) handleToolCall(req MCPRequest) MCPResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	json.Unmarshal(req.Params, &params)

	switch params.Name {
	case "create_session":
		return s.toolCreateSession(req.ID, params.Arguments)
	case "eval":
		return s.toolEval(req.ID, params.Arguments)
	case "load_script":
		return s.toolLoadScript(req.ID, params.Arguments)
	case "get_fingerprint":
		return s.toolGetFingerprint(req.ID, params.Arguments)
	case "close_session":
		return s.toolCloseSession(req.ID, params.Arguments)
	default:
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32602, Message: "Unknown tool: " + params.Name}}
	}
}

func (s *MCPServer) toolCreateSession(id json.RawMessage, args json.RawMessage) MCPResponse {
	var opts struct {
		Browser  string `json:"browser"`
		OS       string `json:"os"`
		Location string `json:"location"`
	}
	json.Unmarshal(args, &opts)
	if opts.Browser == "" {
		opts.Browser = "chrome"
	}
	if opts.OS == "" {
		opts.OS = "windows"
	}
	if opts.Location == "" {
		opts.Location = "https://example.com/"
	}

	sess, err := s.sbEngine.CreateSession(api.SessionOptions{
		Browser:  opts.Browser,
		OS:       opts.OS,
		Location: opts.Location,
	})
	if err != nil {
		return MCPResponse{JSONRPC: "2.0", ID: id, Error: &MCPError{Code: -1, Message: err.Error()}}
	}
	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()

	result, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": fmt.Sprintf("Session created: %s\nUA: %s", sess.ID, sess.GetFingerprint().Navigator["userAgent"])},
		},
		"session_id": sess.ID,
	})
	return MCPResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func (s *MCPServer) toolEval(id json.RawMessage, args json.RawMessage) MCPResponse {
	var params struct {
		SessionID string `json:"session_id"`
		Code      string `json:"code"`
	}
	json.Unmarshal(args, &params)

	s.mu.Lock()
	sess, ok := s.sessions[params.SessionID]
	s.mu.Unlock()
	if !ok {
		return MCPResponse{JSONRPC: "2.0", ID: id, Error: &MCPError{Code: -1, Message: "Session not found"}}
	}

	result, err := sess.Eval(params.Code)
	if err != nil {
		return MCPResponse{JSONRPC: "2.0", ID: id, Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": result},
		},
	})
	return MCPResponse{JSONRPC: "2.0", ID: id, Result: resp}
}

func (s *MCPServer) toolLoadScript(id json.RawMessage, args json.RawMessage) MCPResponse {
	var params struct {
		SessionID string `json:"session_id"`
		Name      string `json:"name"`
		Content   string `json:"content"`
	}
	json.Unmarshal(args, &params)

	s.mu.Lock()
	sess, ok := s.sessions[params.SessionID]
	s.mu.Unlock()
	if !ok {
		return MCPResponse{JSONRPC: "2.0", ID: id, Error: &MCPError{Code: -1, Message: "Session not found"}}
	}

	err := sess.LoadScript(params.Name, params.Content)
	if err != nil {
		return MCPResponse{JSONRPC: "2.0", ID: id, Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": "Script loaded: " + params.Name},
		},
	})
	return MCPResponse{JSONRPC: "2.0", ID: id, Result: resp}
}

func (s *MCPServer) toolGetFingerprint(id json.RawMessage, args json.RawMessage) MCPResponse {
	var params struct {
		SessionID string `json:"session_id"`
	}
	json.Unmarshal(args, &params)

	s.mu.Lock()
	sess, ok := s.sessions[params.SessionID]
	s.mu.Unlock()
	if !ok {
		return MCPResponse{JSONRPC: "2.0", ID: id, Error: &MCPError{Code: -1, Message: "Session not found"}}
	}

	fp := sess.GetFingerprint()
	resp, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": fmt.Sprintf("UA: %s\nPlatform: %s\nTimezone: %s\nGPU: %s", fp.Navigator["userAgent"], fp.OS.Platform, fp.Timezone, fp.GPU.Renderer)},
		},
	})
	return MCPResponse{JSONRPC: "2.0", ID: id, Result: resp}
}

func (s *MCPServer) toolCloseSession(id json.RawMessage, args json.RawMessage) MCPResponse {
	var params struct {
		SessionID string `json:"session_id"`
	}
	json.Unmarshal(args, &params)

	s.mu.Lock()
	sess, ok := s.sessions[params.SessionID]
	if ok {
		sess.Dispose()
		delete(s.sessions, params.SessionID)
	}
	s.mu.Unlock()

	resp, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": "Session closed: " + params.SessionID},
		},
	})
	return MCPResponse{JSONRPC: "2.0", ID: id, Result: resp}
}

// Dispose cleans up all sessions.
func (s *MCPServer) Dispose() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.sessions {
		sess.Dispose()
	}
	s.sbEngine.Dispose()
}
