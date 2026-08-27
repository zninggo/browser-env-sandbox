// Package main implements the bes-server gRPC server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/xiaoxun/bes/internal/fpengine"
	"github.com/xiaoxun/bes/internal/sandbox"
	"github.com/xiaoxun/bes/pkg/api"
)

// Server is the gRPC server that manages sandbox sessions.
// Note: This is a skeleton — the actual protobuf-generated code will
// be created by `protoc` in Phase 6. For now this demonstrates the
// session management structure.
type Server struct {
	mu       sync.RWMutex
	sessions map[string]*sandbox.Session
	engine   *sandbox.Engine
}

func NewServer() *Server {
	fpEng := fpengine.New()
	return &Server{
		sessions: make(map[string]*sandbox.Session),
		engine:   sandbox.New(fpEng, 8),
	}
}

// CreateSession creates a new sandbox session.
func (s *Server) CreateSession(ctx context.Context, opts api.SessionOptions) (*sandbox.Session, error) {
	sess, err := s.engine.CreateSession(opts)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()
	log.Printf("[server] session created: %s", sess.ID)
	return sess, nil
}

// Eval executes JS in a session.
func (s *Server) Eval(ctx context.Context, sessionID, code string) (string, error) {
	sess := s.getSession(sessionID)
	if sess == nil {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}
	return sess.Eval(code)
}

// GetFingerprint returns a session's fingerprint.
func (s *Server) GetFingerprint(ctx context.Context, sessionID string) (*api.Fingerprint, error) {
	sess := s.getSession(sessionID)
	if sess == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return sess.GetFingerprint(), nil
}

// CloseSession closes and removes a session.
func (s *Server) CloseSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	sess.Dispose()
	delete(s.sessions, sessionID)
	log.Printf("[server] session closed: %s", sessionID)
	return nil
}

func (s *Server) getSession(id string) *sandbox.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

func main() {
	port := flag.Int("port", 50051, "gRPC server port")
	flag.Parse()

	// TODO: Phase 6 — register actual protobuf service and start gRPC server.
	// For now, just demonstrate the server can start.
	server := NewServer()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("[bes-server] listening on :%d", *port)
	log.Printf("[bes-server] gRPC service registration: TODO (Phase 6 — run protoc first)")
	log.Printf("[bes-server] placeholder listener ready")

	// Keep running
	select {}
	_ = server
	_ = lis
}
