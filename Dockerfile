# PaperValet Dockerfile
# Multi-stage build: builder -> runtime

# ===== Builder stage =====
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies.
# 仅需 git 与 make；不依赖 CGO（modernc.org/sqlite 走 pure-Go）。
RUN apk add --no-cache git make

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with version info
RUN make build

# ===== Runtime stage =====
FROM alpine:3.20 AS runtime

# Install runtime dependencies.
# 无需 sqlite-libs：modernc.org/sqlite 是纯 Go 实现，不链接 libsqlite。
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 -S papervalet && \
    adduser -u 1000 -S papervalet -G papervalet

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/papervalet /usr/local/bin/papervalet

# Create directories for config and data
RUN mkdir -p /app/config /app/data /app/plugins && \
    chown -R papervalet:papervalet /app

# Switch to non-root user
USER papervalet

# Expose nothing (userbot doesn't need exposed ports)

# Health check - just verify binary exists and runs
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD papervalet --version || exit 1

# Default command
ENTRYPOINT ["papervalet"]
CMD ["-config", "/app/config/config.json"]