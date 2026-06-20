# Multi-stage build for the wayfinder server.
#
# Stage 1: Build the static binary.
FROM golang:1.25-bookworm AS builder

WORKDIR /build

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /build/wayfinder ./cmd/wayfinder

# Stage 2: Runtime image with only the binary.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/wayfinder /app/

# Health check: the probe server listens on port 8080 and exposes /health.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# 8080: health/readiness probes. 8081: WebSocket + ASD frontend.
EXPOSE 8080 8081

# Configuration is read from the environment (12-Factor, CLAUDE.md Abschnitt 7).
# Defaults match Firefly's demo multicast feed.
ENV FIREFLY_CAT062_GROUP=239.255.0.62
ENV FIREFLY_CAT062_PORT=8600

ENTRYPOINT ["/app/wayfinder"]
