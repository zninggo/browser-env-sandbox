// Package bes provides a Go SDK for browser-env-sandbox.
// This is a skeleton — actual implementation requires generated gRPC code.
package bes

// Sandbox wraps a gRPC connection to a bes-server.
type Sandbox struct {
	serverAddr string
	sessionID  string
}

// New creates a new Sandbox client.
func New(serverAddr string) *Sandbox {
	return &Sandbox{serverAddr: serverAddr}
}

// CreateSession creates a sandbox session.
// TODO: implement with gRPC client.
func (s *Sandbox) CreateSession(opts SessionOptions) error {
	return nil // TODO Phase 6
}

// Eval executes JavaScript.
// TODO: implement with gRPC client.
func (s *Sandbox) Eval(code string) (string, error) {
	return "", nil // TODO Phase 6
}

// SessionOptions configures a session.
type SessionOptions struct {
	Browser  string
	OS       string
	Seed     uint64
	Location string
	Cookies  map[string]string
	Proxy    string
}
