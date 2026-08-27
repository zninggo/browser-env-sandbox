package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/zninggo/bes/pkg/api"
)

// Server is the JSON-over-HTTP API server. It replaces the originally planned
// gRPC server with a lightweight net/http + encoding/json implementation that
// needs no protoc/codegen.
//
// Endpoints:
//
//	GET    /api/session                       list sessions
//	POST   /api/session                       create session
//	POST   /api/session/{id}/eval             evaluate JS
//	POST   /api/session/{id}/script           load & run a named script
//	POST   /api/session/{id}/call             call a global function
//	GET    /api/session/{id}/fingerprint      get full fingerprint
//	GET    /api/session/{id}/cookies          get cookie jar
//	POST   /api/session/{id}/cookies          set a cookie
//	DELETE /api/session/{id}                  close session
//	GET    /api/session/{id}/stream/console   SSE stream of console messages
//	GET    /api/session/{id}/stream/network   SSE stream of network events
//	GET    /health                            liveness probe
type Server struct {
	svc    *Service
	server *http.Server
}

// NewServer builds a Server bound to addr and wires all routes onto a
// Go 1.22 ServeMux (method + path-pattern routing, no third-party deps).
func NewServer(addr string, svc *Service) *Server {
	mux := http.NewServeMux()
	s := &Server{svc: svc}

	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/session", s.listSessions)
	mux.HandleFunc("POST /api/session", s.createSession)
	mux.HandleFunc("POST /api/session/{id}/eval", s.eval)
	mux.HandleFunc("POST /api/session/{id}/script", s.loadScript)
	mux.HandleFunc("POST /api/session/{id}/call", s.callFunction)
	mux.HandleFunc("GET /api/session/{id}/fingerprint", s.fingerprint)
	mux.HandleFunc("GET /api/session/{id}/cookies", s.getCookies)
	mux.HandleFunc("POST /api/session/{id}/cookies", s.setCookie)
	mux.HandleFunc("DELETE /api/session/{id}", s.closeSession)
	mux.HandleFunc("GET /api/session/{id}/stream/console", s.streamConsole)
	mux.HandleFunc("GET /api/session/{id}/stream/network", s.streamNetwork)

	s.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// ---------- DTOs ----------

type createSessionRequest struct {
	Seed      uint64            `json:"seed,omitempty"`
	Browser   string            `json:"browser,omitempty"`
	OS        string            `json:"os,omitempty"`
	Location  string            `json:"location,omitempty"`
	Cookies   map[string]string `json:"cookies,omitempty"`
	Proxy     string            `json:"proxy,omitempty"`
	NetMode   string            `json:"net_mode,omitempty"`
	Recording string            `json:"recording,omitempty"`
}

type createSessionResponse struct {
	SessionID   string           `json:"session_id"`
	Fingerprint *api.Fingerprint `json:"fingerprint"`
}

type evalRequest struct {
	Code string `json:"code"`
}

type evalResponse struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type scriptRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type scriptResponse struct {
	Error string `json:"error,omitempty"`
}

type callRequest struct {
	Function string   `json:"function"`
	Args     []string `json:"args,omitempty"`
}

type callResponse struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type cookiesResponse struct {
	Cookies string `json:"cookies"`
}

type setCookieRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type listSessionsResponse struct {
	Sessions []SessionSummary `json:"sessions"`
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("[bridge] json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// writeSSE writes one SSE event: `event: <name>\ndata: <data>\n\n` and flushes.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, name, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
	flusher.Flush()
}

// ---------- handlers ----------

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, listSessionsResponse{Sessions: s.svc.ListSessions()})
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	opts := api.SessionOptions{
		Seed:      req.Seed,
		Browser:   req.Browser,
		OS:        req.OS,
		Location:  req.Location,
		Cookies:   req.Cookies,
		Proxy:     req.Proxy,
		NetMode:   req.NetMode,
		Recording: req.Recording,
	}
	id, fp, err := s.svc.CreateSession(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, createSessionResponse{SessionID: id, Fingerprint: fp})
}

func (s *Server) eval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req evalRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := s.svc.Eval(id, req.Code)
	if err != nil {
		writeJSON(w, http.StatusOK, evalResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, evalResponse{Result: result})
}

func (s *Server) loadScript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req scriptRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := s.svc.LoadScript(id, req.Name, req.Content); err != nil {
		writeJSON(w, http.StatusOK, scriptResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, scriptResponse{})
}

func (s *Server) callFunction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req callRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := s.svc.CallFunction(id, req.Function, req.Args)
	if err != nil {
		writeJSON(w, http.StatusOK, callResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, callResponse{Result: result})
}

func (s *Server) fingerprint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fp, err := s.svc.GetFingerprint(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fp)
}

func (s *Server) getCookies(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cookies, err := s.svc.GetCookies(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cookiesResponse{Cookies: cookies})
}

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req setCookieRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := s.svc.SetCookie(id, req.Name, req.Value); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) closeSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.CloseSession(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "closed"})
}

// streamConsole opens an SSE stream of console messages for a session.
//
// The connection stays open until the client disconnects. A `ready` event is
// sent first, then `console` events as the sandbox emits them, with `:
// keepalive` comments every 15s to keep proxies from dropping the idle link.
func (s *Server) streamConsole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ch, unsub, err := s.svc.SubscribeConsole(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer unsub()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	setSSEHeaders(w)
	writeSSE(w, flusher, "ready", `{"status":"connected"}`)

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(msg)
			writeSSE(w, flusher, "console", string(data))
		}
	}
}

// streamNetwork opens an SSE stream of network events for a session.
// See NetworkEvent: no events are emitted until the Phase 4 netlayer is wired.
func (s *Server) streamNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ch, unsub, err := s.svc.SubscribeNetwork(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer unsub()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	setSSEHeaders(w)
	writeSSE(w, flusher, "ready", `{"status":"connected"}`)

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			writeSSE(w, flusher, "network", string(data))
		}
	}
}

func setSSEHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
}
