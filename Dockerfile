# browser-env-sandbox Dockerfile
# Multi-stage build for the signature service (bes-server)

# ── Build stage ──
FROM golang:1.23-bookworm AS builder

WORKDIR /build

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build bes-server only (signature service)
RUN CGO_ENABLED=1 go build -o /bes-server ./cmd/bes-server

# ── Runtime stage ──
FROM debian:bookworm-slim

# ca-certificates + curl (for healthcheck)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy binary
COPY --from=builder /bes-server /usr/local/bin/bes-server

# Copy SDK JS files (for 
COPY experiments/sso-waf-challenge/js/ /opt/bes/js/

# Runtime config
ENV BES_SERVER_PORT=8080
ENV BES_POOL_SIZE=4
ENV BES_AUTH_TOKEN=""

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/health || exit 1

# Start the server
ENTRYPOINT ["bes-server"]
CMD ["--port", "8080", "--pool", "4"]