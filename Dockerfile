# browser-env-sandbox Dockerfile
# Multi-stage build: build the Go binary, then minimal runtime image
#
# NOTE: builder/runtime use Debian trixie (glibc 2.41+). The v8go v0.35.1
# prebuilt V8 static libraries reference __isoc23_* symbols added in
# glibc 2.38, which bookworm (2.36) does not provide.
# v8go v0.35.1 uses V8 15.2 (Clang 19+ CREL relocations), requires lld linker.

# ── Build stage ──
FROM golang:1.25-trixie AS builder

WORKDIR /build

# Install lld linker (required by v8go v0.35.1 CREL relocations)
RUN apt-get update && apt-get install -y --no-install-recommends lld && rm -rf /var/lib/apt/lists/*

# Cache deps
COPY go.mod go.sum ./
RUN GOPRIVATE=github.com/zninggo/v8go GOSUMDB=off GONOSUMCHECK=github.com/zninggo/v8go go mod download

# Copy source
COPY . .

# Build all binaries
ENV GOPRIVATE=github.com/zninggo/v8go
ENV GOSUMDB=off
ENV GONOSUMCHECK=github.com/zninggo/v8go
ENV CGO_LDFLAGS_ALLOW="-fuse-ld=.*"
RUN CGO_ENABLED=1 go build -o /bes ./cmd/bes
RUN CGO_ENABLED=1 go build -o /bes-server ./cmd/bes-server
RUN CGO_ENABLED=1 go build -o /bes-selftest ./cmd/bes-selftest
RUN CGO_ENABLED=1 go build -o /bes-bench ./cmd/bes-bench

# ── Runtime stage ──
FROM debian:trixie-slim

# Install runtime deps (curl_cffi/python3 fallback removed: netlayer now pure-Go utls)
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*
# Copy binaries
COPY --from=builder /bes /usr/local/bin/bes
COPY --from=builder /bes-server /usr/local/bin/bes-server
COPY --from=builder /bes-selftest /usr/local/bin/bes-selftest
COPY --from=builder /bes-bench /usr/local/bin/bes-bench

# Copy SDK
COPY sdk/ /opt/bes/sdk/


# Expose ports
# 19821: JSON-RPC API (bes-server)
# 9223: CDP debug (Chrome DevTools Protocol)
EXPOSE 19821 9223

# Health check: real liveness probe against the HTTP API /health endpoint
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=10s \
    CMD curl -fsS http://127.0.0.1:19821/health || exit 1

# Default: start the server
ENTRYPOINT ["bes-server"]
CMD ["--port", "19821"]
