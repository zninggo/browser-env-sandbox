# browser-env-sandbox Dockerfile
# Multi-stage build: build the Go binary, then minimal runtime image

# ── Build stage ──
FROM golang:1.25-bookworm AS builder

WORKDIR /build

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build all binaries
RUN CGO_ENABLED=1 go build -o /bes ./cmd/bes
RUN CGO_ENABLED=1 go build -o /bes-server ./cmd/bes-server
RUN CGO_ENABLED=1 go build -o /bes-selftest ./cmd/bes-selftest

# ── Runtime stage ──
FROM debian:bookworm-slim

# Install runtime deps
# curl-impersonate: TLS fingerprint matching (optional, auto-detected)
# python3 + curl_cffi: fallback TLS client (optional, auto-detected)
# ca-certificates: TLS verification
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    python3 \
    python3-pip \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install curl_cffi if available (graceful failure if not)
RUN pip3 install --no-cache-dir curl_cffi 2>/dev/null || true

# Copy binaries
COPY --from=builder /bes /usr/local/bin/bes
COPY --from=builder /bes-server /usr/local/bin/bes-server
COPY --from=builder /bes-selftest /usr/local/bin/bes-selftest

# Copy SDK
COPY sdk/ /opt/bes/sdk/

# Default config
ENV BES_SERVER_PORT=19821
ENV BES_CDP_PORT=9223
ENV BES_LOG_LEVEL=info

# Expose ports
# 19821: JSON-RPC API (bes-server)
# 9223: CDP debug (Chrome DevTools Protocol)
EXPOSE 19821 9223

# Health check
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD bes version || exit 1

# Default: start the server
ENTRYPOINT ["bes-server"]
CMD ["--port", "19821"]
