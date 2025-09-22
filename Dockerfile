# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tezos-delegation-service cmd/server/main.go

# Test stage - runs tests during build
FROM builder AS test
RUN go test -v -short -race ./...

# Final stage - using golang image to support testing
FROM golang:1.23-alpine AS final

# Install runtime dependencies including postgresql-client for migrations
RUN apk --no-cache add ca-certificates tzdata postgresql-client bash gcc musl-dev

WORKDIR /app

# Copy source code for testing
COPY --from=builder /app /app

# Make scripts executable
RUN chmod +x /app/scripts/*.sh 2>/dev/null || true

# Create backup directory
RUN mkdir -p /backups

# Create non-root user
RUN addgroup -g 1000 -S tezos && \
    adduser -u 1000 -S tezos -G tezos

# Change ownership
RUN chown -R tezos:tezos /app /backups

USER tezos

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application with startup script
CMD ["/app/startup.sh"]