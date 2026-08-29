// Package bes provides a Go SDK for browser-env-sandbox.
// Connects to bes-server via JSON-RPC over HTTP.
package bes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Sandbox wraps a connection to a bes-server.
type Sandbox struct {
	baseURL   string
	client    *http.Client
	sessionID string
}

// New creates a new Sandbox client.
func New(serverAddr string) *Sandbox {
	return &Sandbox{
		baseURL: "http://" + serverAddr,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SessionOptions configures a session.
type SessionOptions struct {
	Browser  string            `json:"browser"`
	OS       string            `json:"os"`
	Seed     uint64            `json:"seed"`
	Location string            `json:"location"`
	Cookies  map[string]string `json:"cookies"`
	Proxy    string            `json:"proxy"`
	NetMode  string            `json:"net_mode"`
}

// CreateSession creates a sandbox session.
func (s *Sandbox) CreateSession(opts SessionOptions) error {
	resp, err := s.post("/api/session", opts)
	if err != nil {
		return err
	}
	s.sessionID = resp["session_id"].(string)
	return nil
}

// Eval executes JavaScript and returns the result.
func (s *Sandbox) Eval(code string) (string, error) {
	resp, err := s.post(fmt.Sprintf("/api/session/%s/eval", s.sessionID), map[string]string{"code": code})
	if err != nil {
		return "", err
	}
	if e, ok := resp["error"].(string); ok && e != "" {
		return "", fmt.Errorf("%s", e)
	}
	result, _ := resp["result"].(string)
	return result, nil
}

// LoadScript loads and executes a script.
func (s *Sandbox) LoadScript(name, content string) error {
	_, err := s.post(fmt.Sprintf("/api/session/%s/script", s.sessionID), map[string]string{
		"name":    name,
		"content": content,
	})
	return err
}

// Call calls a global function by name.
func (s *Sandbox) Call(functionName string, args ...string) (string, error) {
	resp, err := s.post(fmt.Sprintf("/api/session/%s/call", s.sessionID), map[string]interface{}{
		"function_name": functionName,
		"args":          args,
	})
	if err != nil {
		return "", err
	}
	if e, ok := resp["error"].(string); ok && e != "" {
		return "", fmt.Errorf("%s", e)
	}
	result, _ := resp["result"].(string)
	return result, nil
}

// GetFingerprint returns the session fingerprint.
func (s *Sandbox) GetFingerprint() (map[string]interface{}, error) {
	return s.get(fmt.Sprintf("/api/session/%s/fingerprint", s.sessionID))
}

// GetCookies returns the session cookies.
func (s *Sandbox) GetCookies() (string, error) {
	resp, err := s.get(fmt.Sprintf("/api/session/%s/cookies", s.sessionID))
	if err != nil {
		return "", err
	}
	c, _ := resp["cookies"].(string)
	return c, nil
}

// SetCookie sets a cookie.
func (s *Sandbox) SetCookie(name, value string) error {
	_, err := s.post(fmt.Sprintf("/api/session/%s/cookies", s.sessionID), map[string]string{"name": name, "value": value})
	return err
}

// Close closes the session.
func (s *Sandbox) Close() error {
	if s.sessionID == "" {
		return nil
	}
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/session/%s", s.baseURL, s.sessionID), nil)
	if err != nil {
		return err
	}
	_, err = s.client.Do(req)
	s.sessionID = ""
	return err
}

// --- HTTP helpers ---

func (s *Sandbox) post(path string, body interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", s.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.do(req)
}

func (s *Sandbox) get(path string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", s.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return s.do(req)
}

func (s *Sandbox) do(req *http.Request) (map[string]interface{}, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w (body: %s)", err, string(data))
	}
	return result, nil
}
