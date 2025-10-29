# Multi-stage Dockerfile for web-scraper
# Stage 1: Build the Go binary (maximum optimization)
FROM golang:1.21-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Copy source code
COPY web-scraper.go ./

# Build with aggressive optimization
# CGO_ENABLED=0 for static binary (no dynamic linking)
# -ldflags for stripping debug info and reducing size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -trimpath \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'docker') -X main.buildTime=$(date -u '+%Y-%m-%d_%H:%M:%S')" \
    -o web-scraper \
    web-scraper.go

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1000 scraper && \
    adduser -D -u 1000 -G scraper scraper

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/web-scraper /app/web-scraper

# Create downloads directory and set permissions
RUN mkdir -p /app/downloads && \
    chown -R scraper:scraper /app

# Switch to non-root user
USER scraper

# Default output directory
VOLUME ["/app/downloads"]

# Set default environment variables
ENV OUTPUT_DIR=/app/downloads
ENV WORKERS=5
ENV DEPTH=2

# Entrypoint
ENTRYPOINT ["/app/web-scraper"]

# Default arguments (can be overridden)
CMD ["-output", "/app/downloads", "-workers", "5", "-depth", "2"]

# Labels for metadata
LABEL maintainer="you@example.com"
LABEL description="Modern parallel web scraper - the zen successor to wget"
LABEL version="1.0"

# Health check (optional - will fail if no URL provided, but that's expected)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=1 \
    CMD pgrep -f web-scraper || exit 1
