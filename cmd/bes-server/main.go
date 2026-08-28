// Command bes-server runs the browser-env-sandbox HTTP API server.
//
// This replaces the originally-planned gRPC entrypoint with a lightweight
// JSON-over-HTTP server (internal/bridge), which needs no protoc/codegen.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zninggo/bes/internal/bridge"
	"github.com/zninggo/bes/internal/debug"
	"github.com/zninggo/bes/internal/fpengine"
	"github.com/zninggo/bes/internal/netlayer"
	"github.com/zninggo/bes/internal/sandbox"
	"github.com/zninggo/bes/pkg/api"
)

func main() {
	addr := flag.String("addr", "0.0.0.0", "bind address")
	port := flag.Int("port", 8080, "HTTP API server port")
	cdpPort := flag.Int("cdp-port", 0, "CDP debug server port (0 = disabled)")
	poolSize := flag.Int("pool", 8, "V8 isolate pool size")
	authToken := flag.String("auth-token", "", "API auth token (empty = no auth); overridden by BES_AUTH_TOKEN env")
	flag.Parse()

	// BES_AUTH_TOKEN env overrides flag (same pattern as other BES_* vars).
	if envToken := os.Getenv("BES_AUTH_TOKEN"); envToken != "" {
		*authToken = envToken
	}

	// Wire the stack: fingerprint engine -> sandbox engine -> bridge service.
	fpEng := fpengine.New()
	engine := sandbox.New(fpEng, *poolSize)

	// Wire the network layer: each session gets a netlayer.Handler that makes
	// real HTTP requests with TLS fingerprint matching (curl_cffi). When
	// NetMode is "live" (default), XHR/fetch in the sandbox hit the real
	// internet. When "replay", they return pre-recorded responses.
	engine.SetNetHandlerFactory(func(opts api.SessionOptions, cookieStore *sandbox.CookieStore) sandbox.NetHandler {
		netMode := opts.NetMode
		if netMode == "" {
			netMode = "live"
		}
		tlsTarget := "chrome" + defaultChromeVersion
		handler, err := netlayer.New(netlayer.Mode(netMode), opts.Recording, opts.Proxy, tlsTarget)
		if err != nil {
			log.Printf("[bes-server] net handler init failed (falling back to stubs): %v", err)
			return nil
		}
		backend := handler.TLSBackend()
		log.Printf("[bes-server] session net: mode=%s tls=%s proxy=%s", netMode, backend, opts.Proxy)
		return &netHandlerAdapter{handler: handler, cookieStore: cookieStore}
	})

	svc := bridge.NewService(engine)

	// CDP debug server: exposes /json/list + WebSocket so Chrome DevTools can
	// connect to sandbox sessions. Off by default; enable with --cdp-port.
	// Bind to loopback only — CDP gives full JS control, never expose it
	// unauthenticated. Use an SSH tunnel for remote debugging.
	if *cdpPort != 0 {
		cdp := debug.NewCDPServer(fmt.Sprintf("127.0.0.1:%d", *cdpPort), &cdpBridge{svc})
		if err := cdp.Start(); err != nil {
			log.Printf("[bes-server] CDP server start failed: %v", err)
		} else {
			log.Printf("[bes-server] CDP debug on http://127.0.0.1:%d (use SSH tunnel for remote)", *cdpPort)
		}
	}

	listenAddr := fmt.Sprintf("%s:%d", *addr, *port)
	srv := bridge.NewServer(listenAddr, svc, *authToken)

	go func() {
		log.Printf("[bes-server] HTTP API listening on http://%s", listenAddr)
		if *authToken != "" {
			log.Printf("[bes-server] auth required: Authorization: Bearer <token>")
		} else {
			log.Printf("[bes-server] WARNING: no auth token set, API is open")
		}
		log.Printf("[bes-server] endpoints under /api/session (see internal/bridge/server.go)")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[bes-server] listen error: %v", err)
		}
	}()

	// Wait for interrupt / SIGTERM, then shut down gracefully.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("[bes-server] shutdown signal received, draining...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[bes-server] http shutdown error: %v", err)
	}
	svc.Dispose() // close all sessions + release the isolate pool
	log.Println("[bes-server] stopped")
}

// defaultChromeVersion is the Chrome version used for TLS fingerprint target.
// Should match the fingerprint engine's default browser version.
const defaultChromeVersion = "150"

// cdpBridge adapts bridge.Service to the debug.SessionProvider interface
// (Eval + GetSessions), keeping both packages unaware of each other.
type cdpBridge struct {
	svc *bridge.Service
}

func (c *cdpBridge) Eval(sessionID, code string) (string, error) {
	return c.svc.Eval(sessionID, code)
}

func (c *cdpBridge) GetSessions() []debug.SessionInfo {
	summaries := c.svc.ListSessions()
	out := make([]debug.SessionInfo, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, debug.SessionInfo{
			ID:    s.SessionID,
			Title: s.Browser,
			URL:   s.UA,
			Type:  "page",
		})
	}
	return out
}

// netHandlerAdapter adapts netlayer.Handler to sandbox.NetHandler.
// netlayer.Response and sandbox.NetResponse are structurally identical, but
// live in different packages, so we need a thin adapter.
type netHandlerAdapter struct {
	handler     *netlayer.Handler
	cookieStore *sandbox.CookieStore
}

func (a *netHandlerAdapter) Request(method, urlStr string, headers map[string]string, body []byte) (*sandbox.NetResponse, error) {
	resp, err := a.handler.Request(method, urlStr, headers, body)
	if err != nil {
		return nil, err
	}
	return &sandbox.NetResponse{
		Status:     resp.Status,
		Headers:    resp.Headers,
		Body:       resp.Body,
		Cookies:    resp.Cookies,
		SetCookies: resp.SetCookies,
	}, nil
}
