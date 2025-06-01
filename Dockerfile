# Build stage
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-s -w" -o mukhostho cmd/mukhostho/main.go

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S mukhostho && \
    adduser -u 1001 -S mukhostho -G mukhostho

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/mukhostho .

# Copy configuration file
COPY config.json .

# Create data directory
RUN mkdir -p data && chown -R mukhostho:mukhostho /app

# Switch to non-root user
USER mukhostho

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Set environment variables
ENV MUKHOSTHO_HOST=0.0.0.0
ENV MUKHOSTHO_PORT=8080
ENV MUKHOSTHO_DATA_DIR=/app/data

# Run the application
CMD ["./mukhostho"]
