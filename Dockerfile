# Multi-stage build for optimized production image

# Stage 1: Build Go binaries
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git make gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all binaries with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /api-server ./cmd/api-server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /fetch-relay ./cmd/fetch-relay
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /threshold-analysis ./cmd/threshold-analysis

# Stage 2: Python dependencies
FROM python:3.11-slim AS python-builder

WORKDIR /app

# Install Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

# Stage 3: Final production image
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates python3 py3-pip tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy Go binaries from builder
COPY --from=builder /api-server /app/
COPY --from=builder /fetch-relay /app/
COPY --from=builder /threshold-analysis /app/

# Copy Python site-packages
COPY --from=python-builder /root/.local /home/appuser/.local
ENV PATH=/home/appuser/.local/bin:$PATH

# Copy application code
COPY --chown=appuser:appuser analysis/ /app/analysis/
COPY --chown=appuser:appuser scripts/ /app/scripts/

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/api-server", "--health-check"]

# Expose API port
EXPOSE 8080

# Default command (API server)
CMD ["/app/api-server"]
