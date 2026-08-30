package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/captcha"
	"github.com/zninggo/browser-env-sandbox/internal/session"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// preloadDir 是 preload 脚本唯一允许的根目录（相对工作目录）。客户端传入的
// preload 名必须解析到该目录子树内，禁止绝对路径/盘符/UNC/.. 穿越。
const preloadDir = "data/preload"

// Server is the JSON-over-HTTP API server. It replaces the originally planned
// gRPC server with a lightweight net/http + encoding/json implementation that
// needs no protoc/codegen.
//
// Endpoints:
//
//	GET    /api/session                       list sessions
	//	POST   /api/session                       create session (supports preload + init)
//	POST   /api/session/{id}/eval             evaluate JS
//	POST   /api/session/{id}/script           load & run a named script
//	POST   /api/session/{id}/call             call a global function
//	GET    /api/session/{id}/fingerprint      get full fingerprint
//	GET    /api/session/{id}/cookies          get cookie jar
//	POST   /api/session/{id}/cookies          set a cookie
//	DELETE /api/session/{id}                  close session
//	GET    /api/profile                       list saved profiles
//	GET    /api/profile/{id}                  read one profile
//	POST   /api/profile/{id}/resume           create a session from a profile
//	POST   /api/session/{id}/save-profile     snapshot a session into the store
//	DELETE /api/profile/{id}                  delete a profile
//	GET    /api/session/{id}/stream/console   SSE stream of console messages
//	GET    /api/session/{id}/stream/network   SSE stream of network events
//	GET    /health                            liveness probe (no auth)
type Server struct {
	svc      *Service
	server   *http.Server
	authToken string // empty = no auth
}

// NewServer builds a Server bound to addr and wires all routes onto a
// Go 1.22 ServeMux (method + path-pattern routing, no third-party deps).
// authToken: if non-empty, all /api/ endpoints require "Authorization: Bearer <token>".
func NewServer(addr string, svc *Service, authToken string) *Server {
	mux := http.NewServeMux()
	s := &Server{svc: svc, authToken: authToken}

	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/session", s.auth(s.listSessions))
	mux.HandleFunc("POST /api/session", s.auth(s.createSession))
	mux.HandleFunc("POST /api/session/{id}/eval", s.auth(s.eval))
	mux.HandleFunc("POST /api/session/{id}/script", s.auth(s.loadScript))
	mux.HandleFunc("POST /api/session/{id}/call", s.auth(s.callFunction))
	mux.HandleFunc("GET /api/session/{id}/fingerprint", s.auth(s.fingerprint))
	mux.HandleFunc("POST /api/session/{id}/swap-fingerprint", s.auth(s.swapFingerprint))
	mux.HandleFunc("POST /api/session/{id}/solve-captcha", s.auth(s.solveCaptcha))
	mux.HandleFunc("GET /api/session/{id}/cookies", s.auth(s.getCookies))
	mux.HandleFunc("POST /api/session/{id}/cookies", s.auth(s.setCookie))
	mux.HandleFunc("DELETE /api/session/{id}", s.auth(s.closeSession))
	mux.HandleFunc("POST /api/session/{id}/save-profile", s.auth(s.saveProfile))
	mux.HandleFunc("GET /api/profile", s.auth(s.listProfiles))
	mux.HandleFunc("GET /api/profile/{id}", s.auth(s.getProfile))
	mux.HandleFunc("DELETE /api/profile/{id}", s.auth(s.deleteProfile))
	mux.HandleFunc("POST /api/profile/{id}/resume", s.auth(s.resumeProfile))
	mux.HandleFunc("GET /api/session/{id}/stream/console", s.auth(s.streamConsole))
	mux.HandleFunc("GET /api/session/{id}/stream/network", s.auth(s.streamNetwork))

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
	Timezone  string            `json:"timezone,omitempty"` // pin tz+languages (e.g. match proxy geo)
	Location  string            `json:"location,omitempty"`
	Cookies   map[string]string `json:"cookies,omitempty"`
	Proxy     string            `json:"proxy,omitempty"`
	NetMode   string            `json:"net_mode,omitempty"`
	Recording string            `json:"recording,omitempty"`
	Referer   string            `json:"referer,omitempty"`   // session-level default Referer for netlayer
	Origin    string            `json:"origin,omitempty"`    // session-level default Origin for netlayer
	UserAgent string            `json:"user_agent,omitempty"` // session-level default UA (empty = fingerprint UA)
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"` // session-level extra headers
	Preload   []string          `json:"preload,omitempty"` // script file paths to load on session creation
	Init      string            `json:"init,omitempty"`    // JS code to execute after preload
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

// resolvePreloadPath 把客户端提供的 preload 名解析到 preloadDir 子树内，
// 拒绝绝对路径、Windows 盘符、UNC 前缀及 .. 穿越（防任意文件读取）。
// 反斜杠统一归一化为正斜杠后再判，确保 Linux 部署也能拦 Windows 风格穿越。
func resolvePreloadPath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty preload name")
	}
	slash := strings.ReplaceAll(name, "\\", "/")
	slash = filepath.ToSlash(slash)
	if filepath.IsAbs(name) || strings.HasPrefix(slash, "/") {
		return "", fmt.Errorf("absolute path not allowed")
	}
	if len(slash) >= 2 && slash[1] == ':' {
		return "", fmt.Errorf("drive letter not allowed")
	}
	if strings.HasPrefix(slash, "//") {
		return "", fmt.Errorf("unc path not allowed")
	}
	cleaned := filepath.Clean(slash)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path traversal not allowed")
	}
	abs := filepath.Join(preloadDir, cleaned)
	rel, err := filepath.Rel(preloadDir, abs)
	if err != nil {
		return "", err
	}
	relSlash := filepath.ToSlash(rel)
	if relSlash == ".." || strings.HasPrefix(relSlash, "../") {
		return "", fmt.Errorf("path traversal not allowed")
	}
	return abs, nil
}

// writeSSE writes one SSE event: `event: <name>\ndata: <data>\n\n` and flushes.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, name, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
	flusher.Flush()
}

// ---------- auth middleware ----------

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authToken == "" {
			next(w, r)
			return
		}
		token := r.Header.Get("Authorization")
		if token == "Bearer "+s.authToken {
			next(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
	}
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
		Seed:         req.Seed,
		Browser:      req.Browser,
		OS:           req.OS,
		Timezone:     req.Timezone,
		Location:     req.Location,
		Cookies:      req.Cookies,
		Proxy:        req.Proxy,
		NetMode:      req.NetMode,
		Recording:    req.Recording,
		Referer:      req.Referer,
		Origin:       req.Origin,
		UserAgent:    req.UserAgent,
		ExtraHeaders: req.ExtraHeaders,
	}
	id, fp, err := s.svc.CreateSession(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Preload scripts and run init code if specified
	if len(req.Preload) > 0 {
		for _, scriptPath := range req.Preload {
			resolved, err := resolvePreloadPath(scriptPath)
			if err != nil {
				s.svc.CloseSession(id)
				writeError(w, http.StatusBadRequest, "preload "+scriptPath+": "+err.Error())
				return
			}
			content, err := os.ReadFile(resolved)
			if err != nil {
				s.svc.CloseSession(id)
				writeError(w, http.StatusInternalServerError, "preload read "+scriptPath+": "+err.Error())
				return
			}
			if err := s.svc.LoadScript(id, scriptPath, string(content)); err != nil {
				s.svc.CloseSession(id)
				writeError(w, http.StatusInternalServerError, "preload "+scriptPath+": "+err.Error())
				return
			}
		}
	}
	if req.Init != "" {
		if _, err := s.svc.Eval(id, req.Init); err != nil {
			s.svc.CloseSession(id)
			writeError(w, http.StatusInternalServerError, "init: "+err.Error())
			return
		}
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

func (s *Server) swapFingerprint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req createSessionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	opts := api.SessionOptions{
		Seed:      req.Seed,
		Browser:   req.Browser,
		OS:        req.OS,
		Timezone:  req.Timezone,
		Location:  req.Location,
		Proxy:     req.Proxy,
		NetMode:   req.NetMode,
		Recording: req.Recording,
	}
	fp, err := s.svc.SwapFingerprint(id, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, createSessionResponse{SessionID: id, Fingerprint: fp})
}

type solveCaptchaRequest struct {
	Type     string  `json:"type"`
	SiteKey  string  `json:"site_key"`
	PageURL  string  `json:"page_url"`
	Action   string  `json:"action,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
	ImageB64 string  `json:"image_b64,omitempty"`
}

func (s *Server) solveCaptcha(w http.ResponseWriter, r *http.Request) {
	var req solveCaptchaRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	sol, err := captcha.Solve(captcha.Challenge{
		Type:     captcha.CaptchaType(req.Type),
		SiteKey:  req.SiteKey,
		PageURL:  req.PageURL,
		Action:   req.Action,
		MinScore: req.MinScore,
		ImageB64: req.ImageB64,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"solved": false, "reason": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sol)
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

// ---------- profile handlers ----------

type saveProfileRequest struct {
	Name string `json:"name,omitempty"`
}

type profileResponse struct {
	Profile *session.Profile `json:"profile"`
}

type profileListResponse struct {
	Profiles []session.Profile `json:"profiles"`
}

type resumeProfileResponse struct {
	SessionID   string           `json:"session_id"`
	Fingerprint *api.Fingerprint `json:"fingerprint"`
}

func (s *Server) saveProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req saveProfileRequest
	if r.ContentLength > 0 {
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}
	p, err := s.svc.SaveProfile(id, req.Name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, profileResponse{Profile: p})
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.svc.ListProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if profiles == nil {
		profiles = []session.Profile{}
	}
	writeJSON(w, http.StatusOK, profileListResponse{Profiles: profiles})
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	p, err := s.svc.GetProfile(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profileResponse{Profile: p})
}

func (s *Server) deleteProfile(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteProfile(r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) resumeProfile(w http.ResponseWriter, r *http.Request) {
	sessID, fp, err := s.svc.ResumeProfile(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resumeProfileResponse{SessionID: sessID, Fingerprint: fp})
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
