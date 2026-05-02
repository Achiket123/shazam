# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build optimized binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o app .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install runtime dependencies only
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/app .

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

USER appuser

EXPOSE 8080

CMD ["./app"]
