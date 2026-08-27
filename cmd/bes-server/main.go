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

	"github.com/xiaoxun/bes/internal/bridge"
	"github.com/xiaoxun/bes/internal/fpengine"
	"github.com/xiaoxun/bes/internal/sandbox"
)

func main() {
	addr := flag.String("addr", "0.0.0.0", "bind address")
	port := flag.Int("port", 8080, "HTTP API server port")
	poolSize := flag.Int("pool", 8, "V8 isolate pool size")
	flag.Parse()

	// Wire the stack: fingerprint engine -> sandbox engine -> bridge service.
	fpEng := fpengine.New()
	engine := sandbox.New(fpEng, *poolSize)
	svc := bridge.NewService(engine)

	listenAddr := fmt.Sprintf("%s:%d", *addr, *port)
	srv := bridge.NewServer(listenAddr, svc)

	go func() {
		log.Printf("[bes-server] HTTP API listening on http://%s", listenAddr)
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
