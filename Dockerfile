# Multi-stage Dockerfile for youtube-manager MCP server
# Stage 1: Build the Go binary

FROM golang:1.26 AS builder

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# - CGO_ENABLED=0 for static binary
# - -ldflags="-s -w" strips debug info for smaller binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /youtube-manager-mcp \
    ./cmd/youtube-manager-mcp

# Stage 2: Final minimal image

FROM alpine:latest

# Install ca-certificates for HTTPS requests and ffmpeg for audio processing
RUN apk --no-cache add ca-certificates tzdata ffmpeg

# Create non-root user for security
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /youtube-manager-mcp /app/youtube-manager-mcp

# Change ownership to appuser
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose the default server port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Environment variables:
# - PORT: HTTP listen port (default 8080)
# - CREDENTIALS_DIR: Path to credentials directory with OAuth files
ENTRYPOINT ["/app/youtube-manager-mcp"]
